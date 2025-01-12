package password

import (
	"github.com/blazskufca/goscrapyd/internal/assert"
	"testing"
)

func TestHash(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		expectErr bool
	}{
		{
			name:      "valid password",
			password:  "mySecurePassword123",
			expectErr: false,
		},
		{
			name:      "empty password",
			password:  "",
			expectErr: false,
		},
		{
			name:      "long password",
			password:  string(make([]byte, 72)),
			expectErr: false,
		},
		{
			name:      "too long password",
			password:  string(make([]byte, 73)),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hashedPassword, err := Hash(tt.password)

			if tt.expectErr {
				assert.NotEqual(t, err, nil)
				assert.Equal(t, hashedPassword, "")
				return
			}

			assert.NilError(t, err)
			assert.NotEqual(t, tt.password, hashedPassword)

			matches, err := Matches(tt.password, hashedPassword)
			assert.NilError(t, err)
			assert.Equal(t, matches, true)
		})
	}
}

func TestMatches(t *testing.T) {
	tests := []struct {
		name           string
		password       string
		hashedPassword string
		expectMatch    bool
		expectErr      bool
	}{
		{
			name:           "matching password",
			password:       "mySecurePassword123",
			hashedPassword: "",
			expectMatch:    true,
			expectErr:      false,
		},
		{
			name:           "non-matching password",
			password:       "wrongPassword123",
			hashedPassword: "",
			expectMatch:    false,
			expectErr:      false,
		},
		{
			name:           "invalid hash format",
			password:       "mySecurePassword123",
			hashedPassword: "invalid_hash_format",
			expectMatch:    false,
			expectErr:      true,
		},
	}
	correctPassword := "mySecurePassword123"
	validHash, err := Hash(correctPassword)
	assert.NilError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hashedPassword := tt.hashedPassword
			if hashedPassword == "" {
				hashedPassword = validHash
			}

			matches, err := Matches(tt.password, hashedPassword)

			if tt.expectErr {
				assert.NotEqual(t, err, nil)
				assert.Equal(t, matches, false)
				return
			}

			assert.NilError(t, err)
			assert.Equal(t, tt.expectMatch, matches)
		})
	}
}
