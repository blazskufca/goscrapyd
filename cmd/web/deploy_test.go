package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"github.com/blazskufca/goscrapyd/internal/assert"
	"github.com/blazskufca/goscrapyd/internal/cookies"
	"github.com/blazskufca/goscrapyd/internal/database"
	"github.com/google/uuid"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeploy(t *testing.T) {
	ta := newTestApplication(t)
	ta.eggBuildFunc = func(ctx context.Context, pythonPath, scrapyCfg string) ([]byte, error) {
		return []byte("build-egg"), nil
	}
	ta.config.cookie.secretKey = "thisis16bytes123"
	ta.config.workerCount = 2

	deployVersion := uuid.New()
	projectName := "test-project"

	type testCase struct {
		name           string
		node           string
		expectedChunks []string
		setup          func(deployVersion, ProjectName string) (*httptest.Server, error)
	}

	tests := []testCase{
		{
			name: "successful deploy",
			node: "node1",
			expectedChunks: []string{
				"event: status_node1\ndata: ok\n\n",
				"event: spider_node1\ndata: 3\n\n",
				"event: deployment-complete\ndata: <div class=\"p-4 mb-4 mt-4 text-sm text-green-800 rounded-lg bg-green-50 dark:bg-gray-800 dark:text-green-400\" role=\"alert\" id=\"notification\"> <span class=\"font-medium\">Deployment finished!</span> </div>\n\n",
			},
			setup: func(deployVersion, projectName string) (*httptest.Server, error) {
				mockScrapyd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, http.MethodPost)
					assert.Equal(t, r.URL.Path, "/addversion.json")
					err := r.ParseMultipartForm(32 << 20)
					assert.NilError(t, err)

					assert.Equal(t, r.FormValue("version"), deployVersion)
					assert.Equal(t, r.FormValue("project"), projectName)

					file, header, err := r.FormFile("egg")
					assert.NilError(t, err)
					defer file.Close()

					assert.Equal(t, header.Filename, fmt.Sprintf("%s.egg", projectName))

					eggData, err := io.ReadAll(file)
					assert.NilError(t, err)
					assert.Equal(t, string(eggData), "build-egg")

					w.Header().Set("Content-Type", "application/json")
					_, err = w.Write([]byte("{\"node_name\": \"node1\", \"status\": \"ok\", \"spiders\": 3}"))
					assert.NilError(t, err)
				}))
				_, err := ta.DB.queries.NewScrapydNode(context.TODO(), database.NewScrapydNodeParams{
					Nodename: "node1",
					Url:      mockScrapyd.URL,
				})
				if err != nil {
					return nil, err
				}
				return mockScrapyd, nil
			},
		},
		{
			name: "scrapyd error response",
			node: "failed_deploy",
			expectedChunks: []string{
				"event: status_failed_deploy\ndata: <span class=\"text-red-500\">Deployment failed: error getting response from Scrapyd: error when decoding response body: this is an error, not accessible or whatever</span>\n\n",
				"event: deployment-complete\ndata: <div class=\"p-4 mb-4 mt-4 text-sm text-green-800 rounded-lg bg-green-50 dark:bg-gray-800 dark:text-green-400\" role=\"alert\" id=\"notification\"> <span class=\"font-medium\">Deployment finished!</span> </div>\n\n",
			},
			setup: func(deployVersion, projectName string) (*httptest.Server, error) {
				mockScrapyd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, r.Method, http.MethodPost)
					assert.Equal(t, r.URL.Path, "/addversion.json")
					err := r.ParseMultipartForm(32 << 20)
					assert.NilError(t, err)

					assert.Equal(t, r.FormValue("version"), deployVersion)
					assert.Equal(t, r.FormValue("project"), projectName)

					file, header, err := r.FormFile("egg")
					assert.NilError(t, err)
					defer file.Close()

					assert.Equal(t, header.Filename, fmt.Sprintf("%s.egg", projectName))

					eggData, err := io.ReadAll(file)
					assert.NilError(t, err)
					assert.Equal(t, string(eggData), "build-egg")

					w.Header().Set("Content-Type", "application/json")
					_, err = w.Write([]byte("this is an error, not accessible or whatever"))
					assert.NilError(t, err)
				}))
				_, err := ta.DB.queries.NewScrapydNode(context.TODO(), database.NewScrapydNodeParams{
					Nodename: "failed_deploy",
					Url:      mockScrapyd.URL,
				})
				if err != nil {
					return nil, err
				}
				return mockScrapyd, nil
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts, err := tc.setup(deployVersion.String(), projectName)
			assert.NilError(t, err)
			defer ts.Close()
			cookieData := deployCookieType{
				Version:     deployVersion,
				Nodes:       []string{tc.node},
				ProjectName: projectName,
			}

			var buf bytes.Buffer
			if err := gob.NewEncoder(&buf).Encode(cookieData); err != nil {
				t.Fatal(err)
			}

			w := httptest.NewRecorder()
			cookie := http.Cookie{
				Name:     "deploy-session",
				Value:    buf.String(),
				Path:     "/deploy-sse",
				MaxAge:   0,
				HttpOnly: true,
				Secure:   true,
				SameSite: http.SameSiteLaxMode,
			}
			if err := cookies.WriteEncrypted(w, cookie, ta.config.cookie.secretKey); err != nil {
				t.Fatal(err)
			}

			req := httptest.NewRequest(http.MethodGet, "/deploy", nil)
			req.Header.Set("Cookie", w.Header().Get("Set-Cookie"))

			testW := &testResponseRecorder{
				ResponseRecorder: httptest.NewRecorder(),
				chunks:           make([]string, 0),
			}

			ta.buildAndDeployEggSSE(testW, req)

			if testW.Header().Get("Content-Type") != "text/event-stream" {
				t.Errorf("expected Content-Type: text/event-stream, got %s", testW.Header().Get("Content-Type"))
			}
			if testW.Header().Get("Cache-Control") != "no-cache" {
				t.Errorf("expected Cache-Control: no-cache, got %s", testW.Header().Get("Cache-Control"))
			}

			if len(testW.chunks) != len(tc.expectedChunks) {
				t.Fatalf("expected %d chunks, got %d", len(tc.expectedChunks), len(testW.chunks))
			}

			for i, expectedChunk := range tc.expectedChunks {
				assert.StringContains(t, testW.chunks[i], expectedChunk)
			}
		})
	}
}

// testResponseRecorder implements http.Flusher and records SSE chunks
type testResponseRecorder struct {
	*httptest.ResponseRecorder
	chunks []string
}

func (r *testResponseRecorder) Flush() {
	if len(r.Body.Bytes()) > 0 {
		r.chunks = append(r.chunks, r.Body.String())
		r.Body.Reset()
	}
}
