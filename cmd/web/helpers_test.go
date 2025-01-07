package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/blazskufca/goscrapyd/internal/assert"
	"github.com/blazskufca/goscrapyd/internal/database"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"testing"
)

func TestMakeRequestToScrapyd(t *testing.T) {
	ctx := context.Background()
	ta := newTestApplication(t)
	ta.config.ScrapydEncryptSecret = "thisis16bytes123"

	tests := []struct {
		name        string
		nodeName    string
		setupNode   func() (database.ScrapydNode, error)
		method      string
		urlParams   func(url *url.URL) *url.URL
		body        io.Reader
		headers     *http.Header
		expectErr   bool
		validateReq func(*testing.T, *http.Request)
	}{
		{
			name:     "basic GET request",
			nodeName: "test-node",
			setupNode: func() (database.ScrapydNode, error) {
				return ta.DB.queries.NewScrapydNode(ctx, database.NewScrapydNodeParams{
					Nodename: "test-node",
					Url:      "http://localhost:6800",
					Username: sql.NullString{},
					Password: nil,
				})
			},
			method: http.MethodGet,
			urlParams: func(u *url.URL) *url.URL {
				q := u.Query()
				q.Set("project", "test")
				u.RawQuery = q.Encode()
				return u
			},
			validateReq: func(t *testing.T, req *http.Request) {
				assert.Equal(t, req.Method, http.MethodGet)
				assert.Equal(t, req.URL.String(), "http://localhost:6800?project=test")
				basicAuthUsername, basicAuthPassword, ok := req.BasicAuth()
				assert.Equal(t, ok, false)
				assert.Equal(t, basicAuthUsername, "")
				assert.Equal(t, basicAuthPassword, "")
			},
		},
		{
			name:     "with basic auth",
			nodeName: "auth-node",
			setupNode: func() (database.ScrapydNode, error) {
				username := "user"
				encryptedPassword, err := encrypt("test", ta.config.ScrapydEncryptSecret)
				if err != nil {
					return database.ScrapydNode{}, err
				}
				return ta.DB.queries.NewScrapydNode(ctx, database.NewScrapydNodeParams{
					Nodename: "auth-node",
					Url:      "http://localhost:6800",
					Username: database.CreateSqlNullString(&username),
					Password: encryptedPassword,
				})
			},
			method: http.MethodPost,
			urlParams: func(u *url.URL) *url.URL {
				u.Path = path.Join(u.Path, scrapydDaemonStatusReq)
				return u
			},
			validateReq: func(t *testing.T, req *http.Request) {
				assert.Equal(t, req.Method, http.MethodPost)
				assert.Equal(t, req.URL.String(), "http://localhost:6800/"+scrapydDaemonStatusReq)
				basicAuthUsername, basicAuthPassword, ok := req.BasicAuth()
				assert.Equal(t, ok, true)
				assert.Equal(t, basicAuthUsername, "user")
				assert.Equal(t, basicAuthPassword, "test")
			},
		},
		{
			name:     "with custom headers",
			nodeName: "header-node",
			setupNode: func() (database.ScrapydNode, error) {
				return ta.DB.queries.NewScrapydNode(ctx, database.NewScrapydNodeParams{
					Nodename: "header-node",
					Url:      "http://localhost:6800",
				})
			},
			method: http.MethodGet,
			headers: &http.Header{
				"X-Custom": []string{"test-value"},
			},
			validateReq: func(t *testing.T, req *http.Request) {
				assert.Equal(t, req.Header.Get("X-Custom"), "test-value")
			},
		},
		{
			name:     "nil urlParams function",
			nodeName: "no-params-node",
			setupNode: func() (database.ScrapydNode, error) {
				return ta.DB.queries.NewScrapydNode(ctx, database.NewScrapydNodeParams{
					Nodename: "no-params-node",
					Url:      "http://localhost:6800",
				})
			},
			method:    http.MethodGet,
			urlParams: nil,
			validateReq: func(t *testing.T, req *http.Request) {
				assert.Equal(t, req.URL.String(), "http://localhost:6800")
			},
		},
		{
			name:     "empty username with password",
			nodeName: "empty-user-node",
			setupNode: func() (database.ScrapydNode, error) {
				encryptedPassword, err := encrypt("test", ta.config.ScrapydEncryptSecret)
				if err != nil {
					return database.ScrapydNode{}, err
				}
				return ta.DB.queries.NewScrapydNode(ctx, database.NewScrapydNodeParams{
					Nodename: "empty-user-node",
					Url:      "http://localhost:6800",
					Username: database.CreateSqlNullString(new(string)),
					Password: encryptedPassword,
				})
			},
			method: http.MethodGet,
			validateReq: func(t *testing.T, req *http.Request) {
				_, _, ok := req.BasicAuth()
				assert.Equal(t, ok, false)
			},
		},
		{
			name:     "relative path in URL",
			nodeName: "path-node",
			setupNode: func() (database.ScrapydNode, error) {
				return ta.DB.queries.NewScrapydNode(ctx, database.NewScrapydNodeParams{
					Nodename: "path-node",
					Url:      "http://localhost:6800/api/",
				})
			},
			method: http.MethodGet,
			urlParams: func(u *url.URL) *url.URL {
				u.Path = path.Join(u.Path, "status")
				return u
			},
			validateReq: func(t *testing.T, req *http.Request) {
				assert.Equal(t, req.URL.Path, "/api/status")
			},
		},
		{
			name:     "with form body",
			nodeName: "form-body-node",
			setupNode: func() (database.ScrapydNode, error) {
				return ta.DB.queries.NewScrapydNode(ctx, database.NewScrapydNodeParams{
					Nodename: "form-body-node",
					Url:      "http://localhost:6800",
				})
			},
			method: http.MethodPost,
			body:   strings.NewReader("key1=value1&key2=value2"),
			headers: &http.Header{
				"Content-Type": []string{"application/x-www-form-urlencoded"},
			},
			validateReq: func(t *testing.T, req *http.Request) {
				body, err := io.ReadAll(req.Body)
				assert.NilError(t, err)
				assert.Equal(t, string(body), "key1=value1&key2=value2")
				assert.Equal(t, req.Header.Get("Content-Type"), "application/x-www-form-urlencoded")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupNode != nil {
				_, err := tt.setupNode()
				assert.NilError(t, err)
			}
			req, err := makeRequestToScrapyd(ctx, ta.DB.queries, tt.method, tt.nodeName,
				tt.urlParams, tt.body, tt.headers, ta.config.ScrapydEncryptSecret)
			if tt.expectErr {
				assert.NotEqual(t, err, nil)
				return
			} else {
				assert.NilError(t, err)
			}
			if req == nil {
				t.Fatal("got nil request")
			}
			if tt.validateReq != nil {
				tt.validateReq(t, req)
			}
		})
	}
}

func TestRequestJSONResourceFromScrapyd(t *testing.T) {
	type TestResponse struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}

	tests := []struct {
		name         string
		setupServer  func(t *testing.T) *httptest.Server
		setupRequest func(*httptest.Server) *http.Request
		expectErr    bool
		validate     func(*testing.T, TestResponse, error)
	}{
		{
			name: "successful JSON response",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					err := json.NewEncoder(w).Encode(TestResponse{
						Status:  "ok",
						Message: "success",
					})
					if err != nil {
						t.Fatal(err)
					}
				}))
			},
			setupRequest: func(s *httptest.Server) *http.Request {
				req, _ := http.NewRequest(http.MethodGet, s.URL, nil)
				return req
			},
			validate: func(t *testing.T, resp TestResponse, err error) {
				assert.NilError(t, err)
				assert.Equal(t, resp.Status, "ok")
				assert.Equal(t, resp.Message, "success")
			},
		},
		{
			name: "non-200 status code",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
			},
			setupRequest: func(s *httptest.Server) *http.Request {
				req, _ := http.NewRequest(http.MethodGet, s.URL, nil)
				return req
			},
			expectErr: true,
			validate: func(t *testing.T, resp TestResponse, err error) {
				assert.NotEqual(t, err, nil)
				assert.StringContains(t, err.Error(), "request returned status code 500")
			},
		},
		{
			name: "invalid JSON response",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"status": "ok", "message"`)) // Invalid JSON
				}))
			},
			setupRequest: func(s *httptest.Server) *http.Request {
				req, _ := http.NewRequest(http.MethodGet, s.URL, nil)
				return req
			},
			expectErr: true,
			validate: func(t *testing.T, resp TestResponse, err error) {
				assert.NotEqual(t, err, nil)
				assert.StringContains(t, err.Error(), "error when decoding response body:")
			},
		},
		{
			name: "connection error",
			setupServer: func(t *testing.T) *httptest.Server {
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
				s.Close()
				return s
			},
			setupRequest: func(s *httptest.Server) *http.Request {
				req, _ := http.NewRequest(http.MethodGet, s.URL, nil)
				return req
			},
			expectErr: true,
			validate: func(t *testing.T, resp TestResponse, err error) {
				assert.NotEqual(t, err, nil)
				assert.StringContains(t, err.Error(), "No connection could be made")
			},
		},
		{
			name: "response body read error",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Length", "1")
					// Don't write anything to simulate read error
				}))
			},
			setupRequest: func(s *httptest.Server) *http.Request {
				req, _ := http.NewRequest(http.MethodGet, s.URL, nil)
				return req
			},
			expectErr: true,
			validate: func(t *testing.T, resp TestResponse, err error) {
				assert.NotEqual(t, err, nil)
				assert.StringContains(t, err.Error(), "unexpected EOF")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer(t)
			defer server.Close()

			req := tt.setupRequest(server)

			resp, err := requestJSONResourceFromScrapyd[TestResponse](req, nil)
			if !tt.expectErr {
				assert.NilError(t, err)
			}

			if tt.validate != nil {
				tt.validate(t, resp, err)
			}
		})
	}
}

func TestListScrapydNodesWorkerFunc(t *testing.T) {
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/daemonstatus.json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			resp := `{"status": "ok", "running": 1, "finished": 2, "pending": 3}`
			_, err := w.Write([]byte(resp))
			if err != nil {
				t.Fatal(err)
			}
		} else {
			t.Errorf("unexpected URL path: %s", r.URL.Path)
		}
	}))
	defer fakeServer.Close()
	ta := newTestApplication(t)
	ta.config.ScrapydEncryptSecret = "thisis16bytes123"
	// Request for error handler, does not matter for tests but otherwise it'll throw panics if its nil so this satisfies it
	fakeErrorReq, err := http.NewRequest(http.MethodGet, "", nil)
	testNode := database.ScrapydNode{
		ID:       1,
		Nodename: "test-node",
		Url:      fakeServer.URL,
	}
	_, err = ta.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
		Nodename: testNode.Nodename,
		Url:      testNode.Url,
	})
	if err != nil {
		t.Fatal(err)
	}
	resultChan := make(chan listScrapydNodesType)
	jobs := make(chan database.ScrapydNode, 1)
	jobs <- testNode
	close(jobs)
	go ta.listScrapydNodesWorkerFunc(context.Background(), fakeErrorReq, jobs, resultChan)
	result := <-resultChan
	assert.Equal(t, result.Name, "test-node")
	assert.Equal(t, result.URL, fakeServer.URL)
	assert.Equal(t, result.Status, "ok")
	assert.Equal(t, result.Running, 1)
	assert.Equal(t, result.Finished, 2)
	assert.Equal(t, result.Pending, 3)
}

func TestEncrypt(t *testing.T) {
	testCases := []struct {
		name      string
		toEncrypt string
		secretKey string
		expectErr bool
	}{
		{
			name:      "success",
			toEncrypt: `TestValue`,
			secretKey: "thisis16bytes123",
			expectErr: false,
		},
		{
			name:      "too short key",
			toEncrypt: `TestValue`,
			secretKey: "tooShort",
			expectErr: true,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			encrypted, err := encrypt(testCase.toEncrypt, testCase.secretKey)
			if testCase.expectErr {
				assert.NotEqual(t, err, nil)
				return
			} else {
				assert.NilError(t, err)
			}
			decrypted, err := decrypt(encrypted, testCase.secretKey)
			assert.NilError(t, err)
			assert.Equal(t, decrypted, testCase.toEncrypt)
		})
	}
}

func TestDecrypt(t *testing.T) {
	testCases := []struct {
		name           string
		setupEncrypted func() ([]byte, error)
		encryptedValue string
		secretKey      string
		expectErr      bool
	}{
		{
			name:           "success",
			encryptedValue: "testValue",
			setupEncrypted: func() ([]byte, error) {
				return encrypt("testValue", "thisis16bytes123")
			},
			secretKey: "thisis16bytes123",
			expectErr: false,
		},
		{
			name:           "nil slice",
			encryptedValue: "",
			secretKey:      "thisis16bytes123",
			setupEncrypted: func() ([]byte, error) {
				return nil, nil
			},
			expectErr: true,
		},
		{
			name:      "byte slice short than nonce",
			secretKey: "thisis16bytes123",
			setupEncrypted: func() ([]byte, error) {
				return []byte{}, nil
			},
			expectErr: true,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			encryptedValue, err := testCase.setupEncrypted()
			assert.NilError(t, err)
			decrypted, err := decrypt(encryptedValue, testCase.secretKey)
			if testCase.expectErr {
				assert.NotEqual(t, err, nil)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, decrypted, testCase.encryptedValue)
		})
	}
}
