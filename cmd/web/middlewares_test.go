package main

import (
	"bytes"
	"context"
	"github.com/blazskufca/goscrapyd/internal/assert"
	"github.com/blazskufca/goscrapyd/internal/database"
	"github.com/google/uuid"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPanicMiddleware(t *testing.T) {
	app := newTestApplication(t)
	rr := httptest.NewRecorder()
	handler := app.recoverPanic(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))
	testCases := []struct {
		name    string
		request *http.Request
	}{
		{
			name:    "Test GET panic handler",
			request: httptest.NewRequest(http.MethodGet, "/", nil),
		},
		{
			name:    "Test POST panic handler",
			request: httptest.NewRequest(http.MethodPost, "/", nil),
		},
		{
			name:    "Test PATCH panic handler",
			request: httptest.NewRequest(http.MethodPatch, "/", nil),
		},
		{
			name:    "Test DELETE panic handler",
			request: httptest.NewRequest(http.MethodDelete, "/", nil),
		},
		{
			name:    "Test OPTIONS panic handler",
			request: httptest.NewRequest(http.MethodOptions, "/", nil),
		},
		{
			name:    "Test HEAD panic handler",
			request: httptest.NewRequest(http.MethodHead, "/", nil),
		},
		{
			name:    "Test PUT panic handler",
			request: httptest.NewRequest(http.MethodPut, "/", nil),
		},
		{
			name:    "Test CONNECT panic handler",
			request: httptest.NewRequest(http.MethodConnect, "/", nil),
		},
		{
			name:    "Test TRACE panic handler",
			request: httptest.NewRequest(http.MethodTrace, "/", nil),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler.ServeHTTP(rr, tc.request)
			assert.Equal(t, rr.Code, http.StatusInternalServerError)
			assert.StringContains(t, strings.TrimSpace(rr.Body.String()), "The error has been recorded, and we will work to resolve it as soon as possible.")
			assert.StringContains(t, strings.TrimSpace(rr.Body.String()), "Sorry, we are experiencing an issue with our system.")
		})
	}
}

func TestSecureHeaders(t *testing.T) {
	app := newTestApplication(t)
	rr := httptest.NewRecorder()
	r, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	app.securityHeaders(next).ServeHTTP(rr, r)
	rs := rr.Result()

	expectedValue := "origin-when-cross-origin"
	assert.Equal(t, rs.Header.Get("Referrer-Policy"), expectedValue)
	expectedValue = "nosniff"
	assert.Equal(t, rs.Header.Get("X-Content-Type-Options"), expectedValue)
	expectedValue = "deny"
	assert.Equal(t, rs.Header.Get("X-Frame-Options"), expectedValue)
	assert.Equal(t, rs.StatusCode, http.StatusOK)
	defer rs.Body.Close()
	body, err := io.ReadAll(rs.Body)
	if err != nil {
		t.Fatal(err)
	}
	body = bytes.TrimSpace(body)
	assert.Equal(t, string(body), "OK")
}

func TestReverseProxyMiddleware(t *testing.T) {
	tests := []struct {
		name         string
		setupNode    func(app *application) error
		path         string
		expectedCode int
		validateReq  func(*testing.T, *http.Request)
	}{
		{
			name: "valid request with auth",
			setupNode: func(app *application) error {
				username := "testUser"
				encrypted, err := encrypt("testPassword", app.config.ScrapydEncryptSecret)
				if err != nil {
					return err
				}
				_, err = app.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
					Nodename: "testNode",
					Url:      "http://localhost:6800",
					Username: database.CreateSqlNullString(&username),
					Password: encrypted,
				})
				return err
			},
			path:         "/testNode/scrapyd-backend/some/path",
			expectedCode: http.StatusOK,
			validateReq: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "/some/path", r.URL.Path)
				parsedURL := r.Context().Value(backendUrl).(*url.URL)
				assert.Equal(t, parsedURL.String(), "http://localhost:6800")

				username, password, ok := r.BasicAuth()
				if !ok {
					t.Fatal("missing basic auth")
				}
				assert.Equal(t, username, "testUser")
				assert.Equal(t, password, "testPassword")
				assert.Equal(t, r.Context().Value(xForwardedForPrefix), "/testNode/scrapyd-backend")
			},
		},
		{
			name: "invalid node",
			setupNode: func(app *application) error {
				return nil
			},
			path:         "/nonexistent/scrapyd-backend/path",
			expectedCode: http.StatusInternalServerError,
			validateReq:  nil,
		},
		{
			name: "node without auth",
			setupNode: func(app *application) error {
				_, err := app.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
					Nodename: "noauth",
					Url:      "http://localhost:6800",
					Username: database.CreateSqlNullString(nil),
					Password: nil,
				})
				return err
			},
			path:         "/noauth/scrapyd-backend/path",
			expectedCode: http.StatusOK,
			validateReq: func(t *testing.T, r *http.Request) {
				username, password, ok := r.BasicAuth()
				assert.Equal(t, ok, false)
				assert.Equal(t, username, "")
				assert.Equal(t, password, "")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newTestApplication(t)
			app.config.ScrapydEncryptSecret = "thisis16bytes123"

			if err := tt.setupNode(app); err != nil {
				t.Fatal(err)
			}

			var handlerCalled bool
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				if tt.validateReq != nil {
					tt.validateReq(t, r)
				}
				w.WriteHeader(tt.expectedCode)
			})

			mux := http.NewServeMux()
			mux.Handle("GET /{node}/scrapyd-backend/", app.reverseProxyMiddleware(next))

			ts := newTestServer(t, mux)
			defer ts.Close()

			code, _, _ := ts.get(t, tt.path)
			assert.Equal(t, code, tt.expectedCode)

			if tt.expectedCode == http.StatusOK {
				assert.Equal(t, handlerCalled, true)
			}
		})
	}
}

func TestCSRFMiddleware(t *testing.T) {
	// Correct token validation is already tested in any integration test with POST form I guess...
	tests := []struct {
		name       string
		method     string
		setupReq   func(*http.Request)
		wantStatus int
		checkResp  func(*testing.T, *http.Response)
	}{
		{
			name:       "GET request sets CSRF cookie",
			method:     http.MethodGet,
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp *http.Response) {
				cookie := resp.Header.Get("Set-Cookie")
				assert.NotEqual(t, cookie, "")
				assert.StringContains(t, cookie, "csrf_token")
				assert.StringContains(t, cookie, "SameSite=Lax")
				assert.StringContains(t, cookie, "HttpOnly")
				assert.StringContains(t, cookie, "Secure")
			},
		},
		{
			name:       "POST without token fails",
			method:     http.MethodPost,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "POST with invalid token fails",
			method: http.MethodPost,
			setupReq: func(r *http.Request) {
				r.Header.Set("Cookie", "csrf_token=invalid")
				r.Header.Set("X-CSRF-Token", "invalid")
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "PUT without token fails",
			method:     http.MethodPut,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "DELETE without token fails",
			method:     http.MethodDelete,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newTestApplication(t)

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("OK"))
			})

			handler := app.preventCSRF(next)

			r, err := http.NewRequest(tt.method, "/", nil)
			if err != nil {
				t.Fatal(err)
			}

			if tt.setupReq != nil {
				tt.setupReq(r)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, r)

			resp := rr.Result()
			assert.Equal(t, resp.StatusCode, tt.wantStatus)

			if tt.checkResp != nil {
				tt.checkResp(t, resp)
			}
		})
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	t.Run("Rate Limit Middleware", func(t *testing.T) {
		app := newTestApplication(t)
		app.config.limiter.enabled = true
		app.config.limiter.rps = 2
		app.config.limiter.burst = 5
		handler := app.rateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("OK"))
		}))
		for i := 0; i < 7; i++ {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "127.0.0.1"
			handler.ServeHTTP(rr, req)
			if i < 5 {
				assert.Equal(t, rr.Code, http.StatusOK)
				assert.Equal(t, rr.Body.String(), "OK")
			} else {
				assert.Equal(t, rr.Code, http.StatusTooManyRequests)
				assert.StringContains(t, rr.Body.String(), "You're issuing too many requests too fast.")
				assert.StringContains(t, rr.Body.String(), "Please slow down and try again in a few moments.")
			}
		}
	})

	t.Run("Rate Limit Disabled", func(t *testing.T) {
		app := newTestApplication(t)
		app.config.limiter.enabled = false

		handler := app.rateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("OK"))
		}))
		for i := 0; i < 7; i++ {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "127.0.0.1"
			handler.ServeHTTP(rr, req)
			assert.Equal(t, rr.Code, http.StatusOK)
			assert.Equal(t, rr.Body.String(), "OK")
		}
	})
}

func TestRequirePrivileged(t *testing.T) {
	app := newTestApplication(t)
	t.Run("Privileged GET", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = contextSetAuthenticatedUser(req, &database.User{
			ID:                 uuid.New(),
			CreatedAt:          time.Now(),
			Username:           "TestUser",
			HashedPassword:     "",
			HasAdminPrivileges: true,
		})
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := contextGetAuthenticatedUser(r)
			assert.Equal(t, user.Username, "TestUser")
		})
		app.requirePermission(next).ServeHTTP(rr, req)
		rs := rr.Result()
		assert.Equal(t, rs.StatusCode, http.StatusOK)
	})
	t.Run("Unprivileged GET", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = contextSetAuthenticatedUser(req, &database.User{
			ID:                 uuid.New(),
			CreatedAt:          time.Now(),
			Username:           "TestUser",
			HashedPassword:     "",
			HasAdminPrivileges: false,
		})
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Unprivileged user managed to access privileged endpoint with GET!")
		})
		app.requirePermission(next).ServeHTTP(rr, req)
		rs := rr.Result()
		assert.Equal(t, rs.StatusCode, http.StatusForbidden)
	})
	t.Run("Privileged POST", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req = contextSetAuthenticatedUser(req, &database.User{
			ID:                 uuid.New(),
			CreatedAt:          time.Now(),
			Username:           "TestUser",
			HashedPassword:     "",
			HasAdminPrivileges: true,
		})
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := contextGetAuthenticatedUser(r)
			assert.Equal(t, user.Username, "TestUser")
		})
		app.requirePermission(next).ServeHTTP(rr, req)
		rs := rr.Result()
		assert.Equal(t, rs.StatusCode, http.StatusOK)
	})
	t.Run("Unprivileged GET", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		req = contextSetAuthenticatedUser(req, &database.User{
			ID:                 uuid.New(),
			CreatedAt:          time.Now(),
			Username:           "TestUser",
			HashedPassword:     "",
			HasAdminPrivileges: false,
		})
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Unprivileged user managed to access privileged endpoint with POST!")
		})
		app.requirePermission(next).ServeHTTP(rr, req)
		rs := rr.Result()
		assert.Equal(t, rs.StatusCode, http.StatusForbidden)
	})
	t.Run("Privileged DELETE", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		req = contextSetAuthenticatedUser(req, &database.User{
			ID:                 uuid.New(),
			CreatedAt:          time.Now(),
			Username:           "TestUser",
			HashedPassword:     "",
			HasAdminPrivileges: true,
		})
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := contextGetAuthenticatedUser(r)
			assert.Equal(t, user.Username, "TestUser")
		})
		app.requirePermission(next).ServeHTTP(rr, req)
		rs := rr.Result()
		assert.Equal(t, rs.StatusCode, http.StatusOK)
	})
	t.Run("Unprivileged DELETE", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		req = contextSetAuthenticatedUser(req, &database.User{
			ID:                 uuid.New(),
			CreatedAt:          time.Now(),
			Username:           "TestUser",
			HashedPassword:     "",
			HasAdminPrivileges: false,
		})
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Unprivileged user managed to access privileged endpoint with DELETE!")
		})
		app.requirePermission(next).ServeHTTP(rr, req)
		rs := rr.Result()
		assert.Equal(t, rs.StatusCode, http.StatusForbidden)
	})

}
