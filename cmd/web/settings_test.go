package main

import (
	"github.com/blazskufca/goscrapyd/internal/assert"
	"net/http"
	"net/url"
	"testing"
)

func TestProjectPath(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app.routes())
	ts.login(t)
	pathTests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "Double dot with different slashes",
			path:     "..\\..\\windows\\system32\\scrapy.cfg",
			expected: "<span>path contains directory traversal (&#39;..&#39;)</span>",
		},
		{
			name:     "URL encoded traversal",
			path:     "%2e%2e%2f%2e%2e%2fetc/scrapy.cfg",
			expected: "<span>path contains directory traversal (&#39;..&#39;)</span>",
		},
		{
			name:     "Unicode encoded dots",
			path:     "..%u2215..%u2215etc/scrapy.cfg",
			expected: "<span>invalid URL encoding in path</span>",
		},
		{
			name:     "Double slash normalization",
			path:     "////etc////scrapy.cfg",
			expected: "<span>stat /etc/scrapy.cfg: no such file or directory</span>",
		},
		{
			name:     "Mixed encoding",
			path:     "..%252f..%252fetc/scrapy.cfg",
			expected: "<span>path contains directory traversal (&#39;..&#39;)</span>",
		},
		{
			name:     "Backslash variation",
			path:     "..\\..\\etc\\scrapy.cfg",
			expected: "<span>path contains directory traversal (&#39;..&#39;)</span>",
		},
		{
			name:     "Forward slash with dots",
			path:     "./../../../etc/scrapy.cfg",
			expected: "<span>path contains directory traversal (&#39;..&#39;)</span>",
		},
		{
			name:     "Multiple slashes with dots",
			path:     ".../...//...//../etc/scrapy.cfg",
			expected: "<span>path contains directory traversal (&#39;..&#39;)</span>",
		},
		{
			name:     "Unicode right to left override",
			path:     "/etc/\u202Egfc.yparcsx",
			expected: "path contains non-ASCII characters",
		},
		{
			name:     "Nested traversal",
			path:     "legal/../../etc/scrapy.cfg",
			expected: "path contains directory traversal",
		},
		{
			name:     "UNC path attempt",
			path:     "\\\\server\\share\\scrapy.cfg",
			expected: "path contains dangerous sequences",
		},
		{
			name:     "Shell command injection attempt",
			path:     "/etc/`cat /etc/passwd`/scrapy.cfg",
			expected: "path contains forbidden character: `",
		},
		{
			name:     "Environment variable",
			path:     "$HOME/scrapy.cfg",
			expected: "path contains forbidden character: $",
		},
		{
			name:     "Null byte injection",
			path:     "../../../etc/passwd\x00/scrapy.cfg",
			expected: "<span>path contains directory traversal (&#39;..&#39;)</span>",
		},
		{
			name:     "Complex nested traversal",
			path:     "something/../../../etc/.//../etc/scrapy.cfg",
			expected: "path contains directory traversal",
		},
		{
			name:     "Path with directory traversal",
			path:     "../malicious/path",
			expected: "<span>path contains directory traversal (&#39;..&#39;)</span>",
		},
		{
			name:     "Path with forbidden character",
			path:     "/path/with/forbidden/;character",
			expected: "path contains forbidden character: ;",
		},
		{
			name:     "Update saved settings with non-ASCII character in path",
			path:     "/path/with/非ASCII字符",
			expected: "path contains non-ASCII characters",
		},
		{
			name: "Test project with dangerous sequence",
			path: "<span>path contains directory traversal (&#39;..&#39;)</span>",
		},
	}
	for _, tt := range pathTests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, body := ts.get(t, "/edit-settings")
			csrfToken := extractCSRFToken(t, body)
			urlValues := url.Values{}
			urlValues.Set("csrf_token", csrfToken)
			urlValues.Set("project_name", "Test project: "+tt.name)
			urlValues.Set("project_path", tt.path)

			statusCode, _, body := ts.postFormFollowRedirects(t, "/edit-settings", urlValues)
			if statusCode != http.StatusUnprocessableEntity {
				t.Fatalf("Expected error status code but got: %v", statusCode)
			}
			assert.StringContains(t, body, tt.expected)
		})
	}
}

func TestPage(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app.routes())
	ts.login(t)
	testCases := []struct {
		name           string
		projectName    string
		extraArguments map[string]string
	}{
		{
			name:        "Set project name",
			projectName: "test",
		},
		{
			name: "Set Some Extra Arguments",
			extraArguments: map[string]string{
				"custom_setting_1": "TestSetting1",
				"custom_setting_2": "TestSetting2",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, body := ts.get(t, "/edit-settings")
			if code != http.StatusOK {
				t.Fatal("Expected status code 200 but got: ", code)
			}
			csrfToken := extractCSRFToken(t, body)
			urlValues := url.Values{}
			urlValues.Set("project_name", tc.projectName)
			urlValues.Set("csrf_token", csrfToken)
			if tc.extraArguments != nil {
				for k, v := range tc.extraArguments {
					urlValues.Set(k, v)
				}
			}
			code, _, body = ts.postFormFollowRedirects(t, "/edit-settings", urlValues)
			if code != http.StatusOK {
				t.Fatal("Expected status code 200 but got: ", code)
			}
			assert.StringContains(t, body, tc.projectName)
			if tc.extraArguments != nil {
				for k, v := range tc.extraArguments {
					assert.StringContains(t, body, k)
					assert.StringContains(t, body, v)
					code, _, body := ts.get(t, "/fire-spider")
					if code != http.StatusOK {
						t.Fatal("Expected status code 200 but got: ", code)
					}
					assert.StringContains(t, body, v)
					assert.StringContains(t, body, k)
					code, _, body = ts.get(t, "/add-task")
					if code != http.StatusOK {
						t.Fatal("Expected status code 200 but got: ", code)
					}
					assert.StringContains(t, body, v)
					assert.StringContains(t, body, k)
				}
			}
			code, _, body = ts.get(t, "/deploy-project")
			if code != http.StatusOK {
				t.Fatal("Expected status code 200 but got: ", code)
			}
			assert.StringContains(t, body, tc.projectName)
		})
	}
}

func FuzzProjectPath(f *testing.F) {
	pathTests := []string{
		"..\\..\\windows\\system32\\scrapy.cfg",
		"%2e%2e%2f%2e%2e%2fetc/scrapy.cfg",
		"..%u2215..%u2215etc/scrapy.cfg",
		"////etc////scrapy.cfg",
		"..%252f..%252fetc/scrapy.cfg",
		"..\\..\\etc\\scrapy.cfg",
		"./../../../etc/scrapy.cfg",
		".../...//...//../etc/scrapy.cfg",
		"/etc/\u202Egfc.yparcsx",
		"legal/../../etc/scrapy.cfg",
		"\\\\server\\share\\scrapy.cfg",
		"/etc/`cat /etc/passwd`/scrapy.cfg",
		"$HOME/scrapy.cfg",
		"../../../etc/passwd\x00/scrapy.cfg",
		"something/../../../etc/.//../etc/scrapy.cfg",
		"../malicious/path",
		"/path/with/forbidden/;character",
		"/path/with/非ASCII字符",
	}

	for _, path := range pathTests {
		f.Add(path)
	}

	f.Fuzz(func(t *testing.T, fuzzedPath string) {
		// Skip empty or invalid fuzzed paths during the fuzz execution
		if fuzzedPath == "" {
			t.Skip("Skipping empty fuzzed path")
			return
		}
		app := newTestApplication(t)
		ts := newTestServer(t, app.routes())
		ts.login(t)

		code, _, body := ts.get(t, "/edit-settings")
		if code != http.StatusOK {
			t.Skip("Failed to get settings page")
		}
		csrfToken := extractCSRFToken(t, body)

		urlValues := url.Values{}
		urlValues.Set("csrf_token", csrfToken)
		urlValues.Set("project_name", "Test project")
		urlValues.Set("project_path", fuzzedPath)
		statusCode, _, _ := ts.postFormFollowRedirects(t, "/edit-settings", urlValues)
		if statusCode != http.StatusUnprocessableEntity {
			t.Errorf("Path validation may have failed. Got status %d for path: %q",
				statusCode, fuzzedPath)
		}
	})
}
