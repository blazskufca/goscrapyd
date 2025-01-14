package database

import (
	"database/sql"
	"github.com/blazskufca/goscrapyd/internal/assert"
	"testing"
	"time"
)

func TestCreateSqlNullInt64FromInt(t *testing.T) {
	tests := []struct {
		name     string
		input    *int
		expected sql.NullInt64
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: sql.NullInt64{Valid: false, Int64: 0},
		},
		{
			name:     "zero value",
			input:    intPtr(0),
			expected: sql.NullInt64{Valid: true, Int64: 0},
		},
		{
			name:     "positive number",
			input:    intPtr(42),
			expected: sql.NullInt64{Valid: true, Int64: 42},
		},
		{
			name:     "negative number",
			input:    intPtr(-42),
			expected: sql.NullInt64{Valid: true, Int64: -42},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, CreateSqlNullInt64FromInt(tt.input), tt.expected)
		})
	}
}

func TestCreateSqlNullString(t *testing.T) {
	tests := []struct {
		name     string
		input    *string
		expected sql.NullString
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: sql.NullString{Valid: false, String: ""},
		},
		{
			name:     "empty string",
			input:    strPtr(""),
			expected: sql.NullString{Valid: false, String: ""},
		},
		{
			name:     "whitespace only",
			input:    strPtr("   "),
			expected: sql.NullString{Valid: false, String: "   "},
		},
		{
			name:     "valid string",
			input:    strPtr("hello"),
			expected: sql.NullString{Valid: true, String: "hello"},
		},
		{
			name:     "string with spaces",
			input:    strPtr("  hello  "),
			expected: sql.NullString{Valid: true, String: "  hello  "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, CreateSqlNullString(tt.input), tt.expected)
		})
	}
}

func TestCreateSqlNullTimePtr(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		input    *time.Time
		expected sql.NullTime
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: sql.NullTime{Valid: false},
		},
		{
			name:     "zero time",
			input:    timePtr(time.Time{}),
			expected: sql.NullTime{Valid: true, Time: time.Time{}},
		},
		{
			name:     "valid time",
			input:    &now,
			expected: sql.NullTime{Valid: true, Time: now},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateSqlNullTimePtr(tt.input)
			assert.Equal(t, result.Valid, tt.expected.Valid)
			if result.Valid {
				assert.Equal(t, result.Time, tt.expected.Time)
			}
		})
	}
}

func TestCreateSqlNullTimeNonPtr(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		input    time.Time
		expected sql.NullTime
	}{
		{
			name:     "zero time",
			input:    time.Time{},
			expected: sql.NullTime{Valid: true, Time: time.Time{}},
		},
		{
			name:     "valid time",
			input:    now,
			expected: sql.NullTime{Valid: true, Time: now},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateCreateSqlNullTimeNonPtr(tt.input)
			assert.Equal(t, result.Valid, tt.expected.Valid)
			assert.Equal(t, result.Time, tt.expected.Time)
		})
	}
}

func TestReadSqlNullString(t *testing.T) {
	tests := []struct {
		name     string
		input    sql.NullString
		expected *string
	}{
		{
			name:     "invalid null string",
			input:    sql.NullString{Valid: false, String: ""},
			expected: nil,
		},
		{
			name:     "valid empty string",
			input:    sql.NullString{Valid: true, String: ""},
			expected: strPtr(""),
		},
		{
			name:     "valid string",
			input:    sql.NullString{Valid: true, String: "hello"},
			expected: strPtr("hello"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ReadSqlNullString(tt.input)
			if tt.expected == nil {
				assert.Equal(t, result, tt.expected)
			} else if result != nil && tt.expected != nil {
				assert.Equal(t, *tt.expected, *result)
			}
		})
	}
}

// Helper functions for creating pointers
func intPtr(i int) *int {
	return &i
}

func strPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}
