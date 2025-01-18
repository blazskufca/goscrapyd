package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"github.com/blazskufca/goscrapyd/internal/assert"
	"github.com/blazskufca/goscrapyd/internal/database"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"reflect"
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
	fakeErrorReq, _ := http.NewRequest(http.MethodGet, "", nil)
	testNode := database.ScrapydNode{
		ID:       1,
		Nodename: "test-node",
		Url:      fakeServer.URL,
	}
	_, err := ta.DB.queries.NewScrapydNode(context.Background(), database.NewScrapydNodeParams{
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

func TestParseCSV(t *testing.T) {
	tests := []struct {
		name        string
		csvContent  string
		fileName    string
		contentType string
		maxMemory   int64
		want        []map[string]string
		wantErr     bool
		errMsg      string
	}{
		{
			name: "Valid CSV",
			csvContent: `name,age,city
John,30,New York
Alice,25,Los Angeles`,
			fileName:    "file",
			contentType: "text/csv",
			maxMemory:   1024 * 1024,
			want: []map[string]string{
				{"name": "John", "age": "30", "city": "New York"},
				{"name": "Alice", "age": "25", "city": "Los Angeles"},
			},
			wantErr: false,
		},
		{
			name: "Empty CSV",
			csvContent: `name,age,city
`,
			fileName:    "file",
			contentType: "text/csv",
			maxMemory:   1024 * 1024,
			want:        []map[string]string{},
			wantErr:     false,
		},
		{
			name: "Invalid Content Type",
			csvContent: `name,age,city
John,30,New York`,
			fileName:    "file",
			contentType: "text/plain",
			maxMemory:   1024 * 1024,
			want:        nil,
			wantErr:     true,
			errMsg:      "invalid file type: expected csv, got text/plain",
		},
		{
			name: "Single Column CSV",
			csvContent: `name
John
Alice`,
			fileName:    "file",
			contentType: "text/csv",
			maxMemory:   1024 * 1024,
			want: []map[string]string{
				{"name": "John"},
				{"name": "Alice"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			part, err := writer.CreateFormFile(tt.fileName, "test.csv")
			if err != nil {
				t.Fatalf("Failed to create form file: %v", err)
			}
			_, err = part.Write([]byte(tt.csvContent))
			assert.NilError(t, err)
			err = writer.Close()
			assert.NilError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/upload", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			if tt.contentType != "" {
				err = req.ParseMultipartForm(tt.maxMemory)
				assert.NilError(t, err)

				_, fh, err := req.FormFile(tt.fileName)
				if err == nil {
					fh.Header.Set("Content-Type", tt.contentType)
				}
			}

			got, err := parseCSV(req, tt.fileName, tt.maxMemory)

			if tt.wantErr {
				assert.NotEqual(t, err, nil)
				assert.Equal(t, err.Error(), tt.errMsg)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, len(got), len(tt.want))
			for i := 0; i < len(got); i++ {
				assert.Equal(t, maps.Equal(got[i], tt.want[i]), true)
			}
		})
	}
}

func TestHasKeys(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]string
		keys []string
		want bool
	}{
		{
			name: "Empty map and no keys",
			m:    map[string]string{},
			keys: []string{},
			want: true,
		},
		{
			name: "Empty map with keys",
			m:    map[string]string{},
			keys: []string{"name"},
			want: false,
		},
		{
			name: "Map with single matching key",
			m:    map[string]string{"name": "John"},
			keys: []string{"name"},
			want: true,
		},
		{
			name: "Map with multiple matching keys",
			m: map[string]string{
				"name": "John",
				"age":  "30",
				"city": "New York",
			},
			keys: []string{"name", "age"},
			want: true,
		},
		{
			name: "Map with one missing key",
			m: map[string]string{
				"name": "John",
				"age":  "30",
			},
			keys: []string{"name", "country"},
			want: false,
		},
		{
			name: "Map with all missing keys",
			m: map[string]string{
				"name": "John",
				"age":  "30",
			},
			keys: []string{"country", "city"},
			want: false,
		},
		{
			name: "Nil map",
			m:    nil,
			keys: []string{"name"},
			want: false,
		},
		{
			name: "Map with empty string key",
			m: map[string]string{
				"":    "empty",
				"age": "30",
			},
			keys: []string{"", "age"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasKeys(tt.m, tt.keys...)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestAddToURLValues(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    any
		initial  url.Values
		expected url.Values
	}{
		{
			name:     "String value",
			key:      "name",
			value:    "John",
			initial:  make(url.Values),
			expected: url.Values{"name": []string{"John"}},
		},
		{
			name:     "Float value",
			key:      "price",
			value:    123.45,
			initial:  make(url.Values),
			expected: url.Values{"price": []string{"123.45"}},
		},
		{
			name:     "Boolean true value",
			key:      "active",
			value:    true,
			initial:  make(url.Values),
			expected: url.Values{"active": []string{"true"}},
		},
		{
			name:     "Boolean false value",
			key:      "active",
			value:    false,
			initial:  make(url.Values),
			expected: url.Values{"active": []string{"false"}},
		},
		{
			name:     "Array of strings",
			key:      "tags",
			value:    []any{"tag1", "tag2", "tag3"},
			initial:  make(url.Values),
			expected: url.Values{"tags": []string{"tag1", "tag2", "tag3"}},
		},
		{
			name:     "Array of mixed types",
			key:      "values",
			value:    []any{"string", 123.45, true},
			initial:  make(url.Values),
			expected: url.Values{"values": []string{"string", "123.45", "true"}},
		},
		{
			name:     "Empty array",
			key:      "empty",
			value:    []any{},
			initial:  make(url.Values),
			expected: url.Values{},
		},
		{
			name:     "Append to existing value",
			key:      "tags",
			value:    "tag2",
			initial:  url.Values{"tags": []string{"tag1"}},
			expected: url.Values{"tags": []string{"tag1", "tag2"}},
		},
		{
			name:     "Append array to existing value",
			key:      "tags",
			value:    []any{"tag2", "tag3"},
			initial:  url.Values{"tags": []string{"tag1"}},
			expected: url.Values{"tags": []string{"tag1", "tag2", "tag3"}},
		},
		{
			name:     "Integer value",
			key:      "count",
			value:    42,
			initial:  make(url.Values),
			expected: url.Values{"count": []string{"42"}},
		},
		{
			name:     "Empty string value",
			key:      "empty",
			value:    "",
			initial:  make(url.Values),
			expected: url.Values{"empty": []string{""}},
		},
		{
			name:     "Nil value",
			key:      "nil",
			value:    nil,
			initial:  make(url.Values),
			expected: url.Values{"nil": []string{"<nil>"}},
		},
		{
			name:     "Multiple operations on same key",
			key:      "multi",
			value:    []any{"value1", 123, "value2"},
			initial:  url.Values{"multi": []string{"existing"}},
			expected: url.Values{"multi": []string{"existing", "value1", "123", "value2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := make(url.Values)
			for k, v := range tt.initial {
				values[k] = append([]string{}, v...)
			}

			addToURLValues(values, tt.key, tt.value)

			if !reflect.DeepEqual(values, tt.expected) {
				t.Errorf("addToURLValues() got = %v, want %v", values, tt.expected)
			}

			actualLen, expectedLen := len(values[tt.key]), len(tt.expected[tt.key])
			assert.Equal(t, actualLen, expectedLen)
		})
	}
}
