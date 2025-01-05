package main

import (
	"bytes"
	"context"
	"database/sql"
	"github.com/blazskufca/goscrapyd/assets"
	"github.com/blazskufca/goscrapyd/internal/assert"
	"github.com/blazskufca/goscrapyd/internal/database"
	"github.com/blazskufca/goscrapyd/internal/password"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/pressly/goose/v3"
	"html"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"sync"
	"testing"
	"time"
)

func newTestApplication(t *testing.T) *application {
	cfg := config{
		cookie: struct{ secretKey string }{
			secretKey: "secret",
		},
		session: struct {
			secretKey    string
			oldSecretKey string
		}{
			secretKey: "secret",
		},
		db: struct {
			dsn               string
			maxOpenConns      int
			maxIdleConns      int
			maxIdleTime       time.Duration
			autoMigrate       bool
			createDefaultUser bool
		}{
			dsn:               ":memory:",
			maxOpenConns:      25,
			maxIdleConns:      25,
			maxIdleTime:       30 * time.Minute,
			autoMigrate:       true,
			createDefaultUser: true,
		},
		workerCount:          4,
		DefaultTimeout:       30 * time.Second,
		ScrapydEncryptSecret: "test",
		autoUpdateNodes:      "test",
	}
	templateCache, err := newTemplateCache()
	if err != nil {
		t.Fatal(err)
	}
	keyPairs := [][]byte{[]byte(cfg.session.secretKey), nil}
	if cfg.session.oldSecretKey != "" {
		keyPairs = append(keyPairs, []byte(cfg.session.oldSecretKey), nil)
	}
	sessionStore := sessions.NewCookieStore(keyPairs...)
	sessionStore.Options = &sessions.Options{
		HttpOnly: true,
		MaxAge:   86400 * 7,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	}
	openDB := func(cfg config) (*database.Queries, *sql.DB, error) {
		db, err := sql.Open("sqlite3", cfg.db.dsn)
		if err != nil {
			return nil, nil, err
		}
		if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
			return nil, nil, err
		}
		db.SetMaxOpenConns(cfg.db.maxOpenConns)
		db.SetMaxIdleConns(cfg.db.maxIdleConns)
		db.SetConnMaxIdleTime(cfg.db.maxIdleTime)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = db.PingContext(ctx)
		if err != nil {
			_ = db.Close()
			return nil, nil, err
		}
		if cfg.db.autoMigrate {
			goose.SetLogger(log.New(io.Discard, "", 0))
			goose.SetBaseFS(assets.EmbeddedFiles)

			if err := goose.SetDialect("sqlite3"); err != nil {
				_ = db.Close()
				return nil, nil, err
			}

			if err := goose.Up(db, "migrations"); err != nil {
				_ = db.Close()
				return nil, nil, err
			}
		}
		preparedDb, err := database.Prepare(context.Background(), db)
		if err != nil {
			_ = db.Close()
			return nil, nil, err
		}
		if cfg.db.createDefaultUser {
			username := "admin"
			userUUID, err := uuid.NewRandom()
			if err != nil {
				_ = db.Close()
				return nil, nil, err
			}
			passwordHash, err := password.Hash("admin")
			if err != nil {
				_ = db.Close()
				return nil, nil, err
			}
			_, err = preparedDb.CreateNewUser(context.Background(), database.CreateNewUserParams{
				ID:                 userUUID,
				Username:           username,
				HashedPassword:     passwordHash,
				HasAdminPrivileges: true,
			})
			if err != nil {
				_ = db.Close()
				return nil, nil, err
			}
		}
		return preparedDb, db, nil
	}
	db, dbcon, err := openDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		goose.SetBaseFS(assets.EmbeddedFiles)
		goose.SetLogger(log.New(io.Discard, "", 0))
		if err := goose.SetDialect("sqlite3"); err != nil {
			t.Fatal(err)
		}
		err := goose.DownTo(dbcon, "migrations", 0)
		if err != nil {
			t.Fatal(err)
		}
		err = dbcon.Close()
		if err != nil {
			t.Fatal(err)
		}
	})
	return &application{
		config:       cfg,
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		mailer:       nil,
		sessionStore: sessionStore,
		wg:           sync.WaitGroup{},
		DB: struct {
			queries *database.Queries
			dbConn  *sql.DB
		}{
			queries: db,
			dbConn:  dbcon,
		},
		scheduler:     nil,
		reverseProxy:  nil,
		globalMu:      sync.Mutex{},
		templateCache: templateCache,
	}
}

type testServer struct {
	*httptest.Server
}

func newTestServer(t *testing.T, h http.Handler) *testServer {

	ts := httptest.NewTLSServer(h)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	ts.Client().Jar = jar
	ts.Client().CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &testServer{ts}
}

func (ts *testServer) login(t *testing.T) {
	_, _, body := ts.get(t, "/login")
	token := extractCSRFToken(t, body)
	form := url.Values{}
	form.Add("username", "admin")
	form.Add("password", "admin")
	form.Add("csrf_token", token)
	responseStatus, headers, _ := ts.postForm(t, "/login", form)
	if responseStatus != http.StatusSeeOther {
		t.Fatalf("got status %d; want %d", responseStatus, http.StatusSeeOther)
	}
	assert.StringContains(t, headers.Get("Set-Cookie"), "session=")
}

func (ts *testServer) postFormFollowRedirects(t *testing.T, urlPath string, form url.Values) (int, http.Header, string) {
	rs, err := ts.Client().PostForm(ts.URL+urlPath, form)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Body.Close()

	for rs.StatusCode == http.StatusSeeOther || rs.StatusCode == http.StatusFound {
		redirectURL, err := rs.Location()
		if err != nil {
			t.Fatalf("Failed to get redirect location: %v", err)
		}
		if !redirectURL.IsAbs() {
			baseURL, _ := url.Parse(ts.URL)
			redirectURL = baseURL.JoinPath(redirectURL.Path)
		}
		rs, err = ts.Client().Get(redirectURL.String())
		if err != nil {
			t.Fatalf("Failed to follow redirect: %v", err)
		}
		defer rs.Body.Close()
	}

	body, err := io.ReadAll(rs.Body)
	if err != nil {
		t.Fatal(err)
	}
	body = bytes.TrimSpace(body)

	return rs.StatusCode, rs.Header, string(body)
}

func (ts *testServer) get(t *testing.T, urlPath string) (int, http.Header, string) {
	rs, err := ts.Client().Get(ts.URL + urlPath)
	if err != nil {
		t.Fatal(err)
	}

	defer rs.Body.Close()
	body, err := io.ReadAll(rs.Body)
	if err != nil {
		t.Fatal(err)
	}
	body = bytes.TrimSpace(body)

	return rs.StatusCode, rs.Header, string(body)
}

func (ts *testServer) delete(t *testing.T, urlPath string) (int, http.Header, []byte) {
	req, err := http.NewRequest(http.MethodDelete, ts.URL+urlPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	rs, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}

	defer rs.Body.Close()
	body, err := io.ReadAll(rs.Body)
	if err != nil {
		t.Fatal(err)
	}

	return rs.StatusCode, rs.Header, body
}

var csrfTokenRX = regexp.MustCompile(`<input type="hidden" name="csrf_token" value="(.+)">`)

func extractCSRFToken(t *testing.T, body string) string {
	matches := csrfTokenRX.FindStringSubmatch(body)
	if len(matches) < 2 {
		t.Fatal("no csrf token found in body")
	}
	return html.UnescapeString(string(matches[1]))
}

func (ts *testServer) postForm(t *testing.T, urlPath string, form url.Values) (int, http.Header, string) {
	rs, err := ts.Client().PostForm(ts.URL+urlPath, form)
	if err != nil {
		t.Fatal(err)
	}

	defer rs.Body.Close()
	body, err := io.ReadAll(rs.Body)
	if err != nil {
		t.Fatal(err)
	}
	body = bytes.TrimSpace(body)
	return rs.StatusCode, rs.Header, string(body)
}
