package validator

import (
	"github.com/blazskufca/goscrapyd/internal/assert"
	"maps"
	"slices"
	"testing"
)

func TestValidator_HasErrors(t *testing.T) {
	tests := []struct {
		name     string
		errors   []string
		fields   map[string]string
		expected bool
	}{
		{
			name:     "no errors",
			errors:   nil,
			fields:   nil,
			expected: false,
		},
		{
			name:     "has general error",
			errors:   []string{"general error"},
			fields:   nil,
			expected: true,
		},
		{
			name:     "has field error",
			errors:   nil,
			fields:   map[string]string{"field": "error"},
			expected: true,
		},
		{
			name:     "has both errors",
			errors:   []string{"general error"},
			fields:   map[string]string{"field": "error"},
			expected: true,
		},
		{
			name:     "empty slices and maps",
			errors:   []string{},
			fields:   map[string]string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Validator{
				Errors:      tt.errors,
				FieldErrors: tt.fields,
			}
			assert.Equal(t, v.HasErrors(), tt.expected)
		})
	}
}

func TestValidator_AddError(t *testing.T) {
	tests := []struct {
		name           string
		initialErrors  []string
		errorToAdd     string
		expectedErrors []string
	}{
		{
			name:           "add to nil errors",
			initialErrors:  nil,
			errorToAdd:     "first error",
			expectedErrors: []string{"first error"},
		},
		{
			name:           "add to empty errors",
			initialErrors:  []string{},
			errorToAdd:     "first error",
			expectedErrors: []string{"first error"},
		},
		{
			name:           "add to existing errors",
			initialErrors:  []string{"first error"},
			errorToAdd:     "second error",
			expectedErrors: []string{"first error", "second error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Validator{Errors: tt.initialErrors}
			v.AddError(tt.errorToAdd)
			assert.Equal(t, slices.Equal(v.Errors, tt.expectedErrors), true)
		})
	}
}

func TestValidator_AddFieldError(t *testing.T) {
	tests := []struct {
		name           string
		initialFields  map[string]string
		key            string
		message        string
		expectedFields map[string]string
	}{
		{
			name:          "add to nil fields",
			initialFields: nil,
			key:           "email",
			message:       "invalid email",
			expectedFields: map[string]string{
				"email": "invalid email",
			},
		},
		{
			name:          "add to empty fields",
			initialFields: map[string]string{},
			key:           "email",
			message:       "invalid email",
			expectedFields: map[string]string{
				"email": "invalid email",
			},
		},
		{
			name: "add to existing fields",
			initialFields: map[string]string{
				"name": "required field",
			},
			key:     "email",
			message: "invalid email",
			expectedFields: map[string]string{
				"name":  "required field",
				"email": "invalid email",
			},
		},
		{
			name: "don't override existing field error",
			initialFields: map[string]string{
				"email": "first error",
			},
			key:     "email",
			message: "second error",
			expectedFields: map[string]string{
				"email": "first error",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Validator{FieldErrors: tt.initialFields}
			v.AddFieldError(tt.key, tt.message)
			assert.Equal(t, maps.Equal(v.FieldErrors, tt.expectedFields), true)
		})
	}
}

func TestValidator_Check(t *testing.T) {
	tests := []struct {
		name          string
		condition     bool
		message       string
		shouldHaveErr bool
	}{
		{
			name:          "condition true",
			condition:     true,
			message:       "this error shouldn't appear",
			shouldHaveErr: false,
		},
		{
			name:          "condition false",
			condition:     false,
			message:       "validation failed",
			shouldHaveErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Validator{}
			v.Check(tt.condition, tt.message)

			hasError := len(v.Errors) > 0
			assert.Equal(t, hasError, tt.shouldHaveErr)
			if tt.shouldHaveErr {
				assert.Equal(t, v.Errors[0], tt.message)
			}

		})
	}
}

func TestValidator_CheckField(t *testing.T) {
	tests := []struct {
		name          string
		condition     bool
		key           string
		message       string
		shouldHaveErr bool
	}{
		{
			name:          "condition true",
			condition:     true,
			key:           "field",
			message:       "this error shouldn't appear",
			shouldHaveErr: false,
		},
		{
			name:          "condition false",
			condition:     false,
			key:           "field",
			message:       "validation failed",
			shouldHaveErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Validator{}
			v.CheckField(tt.condition, tt.key, tt.message)

			hasError := len(v.FieldErrors) > 0
			assert.Equal(t, hasError, tt.shouldHaveErr)

			if tt.shouldHaveErr {
				err, exists := v.FieldErrors[tt.key]
				assert.Equal(t, exists, true)
				assert.Equal(t, err, tt.message)
			}
		})
	}
}

func TestValidator_Integration(t *testing.T) {
	t.Run("multiple validations", func(t *testing.T) {
		v := Validator{}

		v.AddError("general error 1")
		v.AddFieldError("field1", "field error 1")
		v.Check(false, "general error 2")
		v.CheckField(false, "field2", "field error 2")

		expectedErrors := []string{"general error 1", "general error 2"}
		expectedFields := map[string]string{
			"field1": "field error 1",
			"field2": "field error 2",
		}

		assert.Equal(t, slices.Equal(v.Errors, expectedErrors), true)

		assert.Equal(t, maps.Equal(v.FieldErrors, expectedFields), true)

		assert.Equal(t, v.HasErrors(), true)
	})
}
