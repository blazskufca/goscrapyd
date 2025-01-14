package validator

import (
	"github.com/blazskufca/goscrapyd/internal/assert"
	"testing"
)

func TestNotBlank(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty string", "", false},
		{"spaces only", "   ", false},
		{"tabs and newlines", "\t\n", false},
		{"single character", "a", true},
		{"string with spaces", "  hello  ", true},
		{"mixed whitespace with content", "\t hello \n", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, NotBlank(tt.input), tt.expected)
		})
	}
}

func TestMinRunes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		min      int
		expected bool
	}{
		{"empty string", "", 1, false},
		{"exact length", "abc", 3, true},
		{"longer than min", "abcd", 3, true},
		{"shorter than min", "ab", 3, false},
		{"unicode characters", "🌟🌟🌟", 3, true},
		{"mixed ascii and unicode", "a🌟b", 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, MinRunes(tt.input, tt.min), tt.expected)
		})
	}
}

func TestMaxRunes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		max      int
		expected bool
	}{
		{"empty string", "", 1, true},
		{"exact length", "abc", 3, true},
		{"shorter than max", "ab", 3, true},
		{"longer than max", "abcd", 3, false},
		{"unicode characters", "🌟🌟", 2, true},
		{"mixed ascii and unicode", "a🌟b", 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, MaxRunes(tt.input, tt.max), tt.expected)
		})
	}
}

func TestBetween(t *testing.T) {
	t.Run("integers", func(t *testing.T) {
		tests := []struct {
			name     string
			value    int
			min      int
			max      int
			expected bool
		}{
			{"exact min", 1, 1, 3, true},
			{"exact max", 3, 1, 3, true},
			{"in between", 2, 1, 3, true},
			{"below min", 0, 1, 3, false},
			{"above max", 4, 1, 3, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, Between(tt.value, tt.min, tt.max), tt.expected)
			})
		}
	})

	t.Run("floats", func(t *testing.T) {
		tests := []struct {
			name     string
			value    float64
			min      float64
			max      float64
			expected bool
		}{
			{"exact min", 1.0, 1.0, 3.0, true},
			{"exact max", 3.0, 1.0, 3.0, true},
			{"in between", 2.5, 1.0, 3.0, true},
			{"below min", 0.9, 1.0, 3.0, false},
			{"above max", 3.1, 1.0, 3.0, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, Between(tt.value, tt.min, tt.max), tt.expected)
			})
		}
	})
}

func TestIn(t *testing.T) {
	t.Run("strings", func(t *testing.T) {
		safelist := []string{"apple", "banana", "cherry"}
		tests := []struct {
			name     string
			value    string
			expected bool
		}{
			{"exists in list", "apple", true},
			{"not in list", "grape", false},
			{"empty string", "", false},
			{"case sensitive", "Apple", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, In(tt.value, safelist...), tt.expected)
			})
		}
	})
}

func TestAllIn(t *testing.T) {
	safelist := []string{"apple", "banana", "cherry"}
	tests := []struct {
		name     string
		values   []string
		expected bool
	}{
		{"all exist", []string{"apple", "banana"}, true},
		{"some don't exist", []string{"apple", "grape"}, false},
		{"empty input", []string{}, true},
		{"single invalid", []string{"grape"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, AllIn(tt.values, safelist...), tt.expected)
		})
	}
}

func TestNotIn(t *testing.T) {
	blocklist := []string{"spam", "scam", "phishing"}
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"not in blocklist", "hello", true},
		{"in blocklist", "spam", false},
		{"empty string", "", true},
		{"case sensitive", "Spam", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, NotIn(tt.value, blocklist...), tt.expected)
		})
	}
}

func TestNoDuplicates(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		expected bool
	}{
		{"no duplicates", []string{"a", "b", "c"}, true},
		{"has duplicates", []string{"a", "b", "a"}, false},
		{"empty slice", []string{}, true},
		{"single value", []string{"a"}, true},
		{"case sensitive duplicates", []string{"a", "A"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, NoDuplicates(tt.values), tt.expected)
		})
	}
}

func TestIsEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		{"valid email", "test@example.com", true},
		{"valid email with subdomain", "test@sub.example.com", true},
		{"valid email with plus", "test+label@example.com", true},
		{"no @", "testexample.com", false},
		{"empty string", "", false},
		{"too long", string(make([]byte, 255)) + "@example.com", false},
		{"special chars", "test!@example.com", true},
		{"multiple @", "test@@example.com", false},
		{"no domain", "test@", false},
		{"no local part", "@example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, IsEmail(tt.email), tt.expected)
		})
	}
}

func TestIsURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"valid http", "http://example.com", true},
		{"valid https", "https://example.com", true},
		{"valid with path", "http://example.com/path", true},
		{"valid with query", "http://example.com?q=test", true},
		{"no scheme", "example.com", false},
		{"empty string", "", false},
		{"invalid chars", "http://exa mple.com", false},
		{"missing host", "http://", false},
		{"local file", "file:///path", false},
		{"ftp protocol", "ftp://example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, IsURL(tt.url), tt.expected)
		})
	}
}
