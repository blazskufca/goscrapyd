package funcs

import (
	"html/template"
	"net/url"
	"testing"
	"time"
)

func TestFormatTime(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		time     time.Time
		expected string
	}{
		{
			name:     "RFC3339 format",
			format:   time.RFC3339,
			time:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			expected: "2024-01-01T12:00:00Z",
		},
		{
			name:     "custom format",
			format:   "2006-01-02",
			time:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			expected: "2024-01-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTime(tt.format, tt.time)
			if result != tt.expected {
				t.Errorf("formatTime() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSafeBase64Decode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid base64",
			input:    "SGVsbG8gV29ybGQ=",
			expected: "Hello World",
		},
		{
			name:     "invalid base64",
			input:    "invalid-base64",
			expected: "Error decoding: illegal base64 data at input byte 7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeBase64Decode(tt.input)
			if result != tt.expected {
				t.Errorf("SafeBase64Decode() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestApproxDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "less than second",
			duration: 500 * time.Millisecond,
			expected: "less than 1 second",
		},
		{
			name:     "one second",
			duration: time.Second,
			expected: "1 second",
		},
		{
			name:     "multiple seconds",
			duration: 45 * time.Second,
			expected: "45 seconds",
		},
		{
			name:     "one minute",
			duration: time.Minute,
			expected: "1 minute",
		},
		{
			name:     "multiple minutes",
			duration: 45 * time.Minute,
			expected: "45 minutes",
		},
		{
			name:     "one hour",
			duration: time.Hour,
			expected: "1 hour",
		},
		{
			name:     "multiple hours",
			duration: 23 * time.Hour,
			expected: "23 hours",
		},
		{
			name:     "one day",
			duration: 24 * time.Hour,
			expected: "1 day",
		},
		{
			name:     "multiple days",
			duration: 300 * 24 * time.Hour,
			expected: "300 days",
		},
		{
			name:     "one year",
			duration: 365 * 24 * time.Hour,
			expected: "1 year",
		},
		{
			name:     "multiple years",
			duration: 3 * 365 * 24 * time.Hour,
			expected: "3 years",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := approxDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("approxDuration() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		name        string
		count       any
		singular    string
		plural      string
		expected    string
		expectError bool
	}{
		{
			name:        "int singular",
			count:       1,
			singular:    "item",
			plural:      "items",
			expected:    "item",
			expectError: false,
		},
		{
			name:        "int plural",
			count:       2,
			singular:    "item",
			plural:      "items",
			expected:    "items",
			expectError: false,
		},
		{
			name:        "string number",
			count:       "3",
			singular:    "item",
			plural:      "items",
			expected:    "items",
			expectError: false,
		},
		{
			name:        "invalid type",
			count:       "invalid",
			singular:    "item",
			plural:      "items",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := pluralize(tt.count, tt.singular, tt.plural)
			if (err != nil) != tt.expectError {
				t.Errorf("pluralize() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if result != tt.expected {
				t.Errorf("pluralize() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic text",
			input:    "Hello World",
			expected: "hello-world",
		},
		{
			name:     "special characters",
			input:    "Hello! @World#",
			expected: "hello-world",
		},
		{
			name:     "numbers",
			input:    "Hello 123 World",
			expected: "hello-123-world",
		},
		{
			name:     "unicode characters",
			input:    "Hello Värld",
			expected: "hello-vrld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := slugify(tt.input)
			if result != tt.expected {
				t.Errorf("slugify() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSafeHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected template.HTML
	}{
		{
			name:     "basic HTML",
			input:    "<p>Hello World</p>",
			expected: template.HTML("<p>Hello World</p>"),
		},
		{
			name:     "script tag",
			input:    "<script>alert('xss')</script>",
			expected: template.HTML(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := safeHTML(tt.input)
			if result != tt.expected {
				t.Errorf("safeHTML() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIncr(t *testing.T) {
	tests := []struct {
		name        string
		input       any
		expected    int64
		expectError bool
	}{
		{
			name:        "int",
			input:       42,
			expected:    43,
			expectError: false,
		},
		{
			name:        "string number",
			input:       "42",
			expected:    43,
			expectError: false,
		},
		{
			name:        "invalid input",
			input:       "not a number",
			expected:    0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := incr(tt.input)
			if (err != nil) != tt.expectError {
				t.Errorf("incr() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if result != tt.expected {
				t.Errorf("incr() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDecr(t *testing.T) {
	tests := []struct {
		name        string
		input       any
		expected    int64
		expectError bool
	}{
		{
			name:        "int",
			input:       42,
			expected:    41,
			expectError: false,
		},
		{
			name:        "string number",
			input:       "42",
			expected:    41,
			expectError: false,
		},
		{
			name:        "invalid input",
			input:       "not a number",
			expected:    0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := decr(tt.input)
			if (err != nil) != tt.expectError {
				t.Errorf("decr() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if result != tt.expected {
				t.Errorf("decr() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatInt(t *testing.T) {
	tests := []struct {
		name        string
		input       any
		expected    string
		expectError bool
	}{
		{
			name:        "basic int",
			input:       1234,
			expected:    "1,234",
			expectError: false,
		},
		{
			name:        "string number",
			input:       "1234",
			expected:    "1,234",
			expectError: false,
		},
		{
			name:        "invalid input",
			input:       "not a number",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatInt(tt.input)
			if (err != nil) != tt.expectError {
				t.Errorf("formatInt() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if result != tt.expected {
				t.Errorf("formatInt() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		dp       int
		expected string
	}{
		{
			name:     "two decimal places",
			input:    123.456,
			dp:       2,
			expected: "123.46",
		},
		{
			name:     "zero decimal places",
			input:    123.456,
			dp:       0,
			expected: "123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatFloat(tt.input, tt.dp)
			if result != tt.expected {
				t.Errorf("formatFloat() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestYesNo(t *testing.T) {
	tests := []struct {
		name     string
		input    bool
		expected string
	}{
		{
			name:     "true value",
			input:    true,
			expected: "Yes",
		},
		{
			name:     "false value",
			input:    false,
			expected: "No",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := yesno(tt.input)
			if result != tt.expected {
				t.Errorf("yesno() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestURLSetParam(t *testing.T) {
	baseURL, _ := url.Parse("https://example.com?existing=value")

	tests := []struct {
		name     string
		url      *url.URL
		key      string
		value    any
		expected string
	}{
		{
			name:     "add new parameter",
			url:      baseURL,
			key:      "new",
			value:    "test",
			expected: "https://example.com?existing=value&new=test",
		},
		{
			name:     "update existing parameter",
			url:      baseURL,
			key:      "existing",
			value:    "newvalue",
			expected: "https://example.com?existing=newvalue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := urlSetParam(tt.url, tt.key, tt.value)
			if result.String() != tt.expected {
				t.Errorf("urlSetParam() = %v, want %v", result.String(), tt.expected)
			}
		})
	}
}

func TestURLDelParam(t *testing.T) {
	baseURL, _ := url.Parse("https://example.com?existing=value&remove=test")

	tests := []struct {
		name     string
		url      *url.URL
		key      string
		expected string
	}{
		{
			name:     "remove existing parameter",
			url:      baseURL,
			key:      "remove",
			expected: "https://example.com?existing=value",
		},
		{
			name:     "remove non-existent parameter",
			url:      baseURL,
			key:      "notexist",
			expected: "https://example.com?existing=value&remove=test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := urlDelParam(tt.url, tt.key)
			if result.String() != tt.expected {
				t.Errorf("urlDelParam() = %v, want %v", result.String(), tt.expected)
			}
		})
	}
}
