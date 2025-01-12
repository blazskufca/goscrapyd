package cookies

import (
	"github.com/blazskufca/goscrapyd/internal/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBasicWriteRead(t *testing.T) {
	tests := []struct {
		name      string
		cookie    http.Cookie
		wantError bool
	}{
		{
			name: "basic cookie",
			cookie: http.Cookie{
				Name:  "test",
				Value: "hello world",
			},
			wantError: false,
		},
		{
			name: "empty value",
			cookie: http.Cookie{
				Name:  "test",
				Value: "",
			},
			wantError: false,
		},
		{
			name: "very long value",
			cookie: http.Cookie{
				Name:  "test",
				Value: string(make([]byte, 4096)),
			},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			err := Write(w, tt.cookie)
			if tt.wantError {
				assert.NotEqual(t, err, nil)
				return
			}
			assert.NotEqual(t, w.Header().Get("Set-Cookie"), "")
			req := &http.Request{Header: http.Header{"Cookie": {w.Header().Get("Set-Cookie")}}}
			readCookie, err := Read(req, tt.cookie.Name)
			assert.NilError(t, err)
			assert.Equal(t, readCookie, tt.cookie.Value)
		})
	}
}

func TestWriteReadSigned(t *testing.T) {
	encryptKey := "thisis16bytes123"
	tests := []struct {
		name       string
		cookie     http.Cookie
		wantError  bool
		decryptKey string
	}{
		{
			name: "basic cookie",
			cookie: http.Cookie{
				Name:  "test",
				Value: "hello world",
			},
			wantError:  false,
			decryptKey: encryptKey,
		},
		{
			name: "empty value",
			cookie: http.Cookie{
				Name:  "test",
				Value: "",
			},
			wantError:  false,
			decryptKey: encryptKey,
		},
		{
			name: "very long value",
			cookie: http.Cookie{
				Name:  "test",
				Value: string(make([]byte, 4096)),
			},
			wantError:  true,
			decryptKey: "someDifferentKey",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			err := WriteSigned(w, tt.cookie, encryptKey)
			if tt.wantError {
				assert.NotEqual(t, err, nil)
				return
			}
			assert.NotEqual(t, w.Header().Get("Set-Cookie"), "")
			req := &http.Request{Header: http.Header{"Cookie": {w.Header().Get("Set-Cookie")}}}
			readCookie, err := ReadSigned(req, tt.cookie.Name, tt.decryptKey)
			assert.NilError(t, err)
			assert.Equal(t, readCookie, tt.cookie.Value)
		})
	}
}

func TestWriteReadEncrypted(t *testing.T) {
	tests := []struct {
		name                     string
		cookie                   http.Cookie
		wantError                bool
		encryptKey               string
		decryptKey               string
		dontExpectCorrectCookies bool
	}{
		{
			name: "basic cookie",
			cookie: http.Cookie{
				Name:  "test",
				Value: "hello world",
			},
			wantError:  false,
			encryptKey: "thisis16bytes123",
			decryptKey: "thisis16bytes123",
		},
		{
			name: "empty value",
			cookie: http.Cookie{
				Name:  "test",
				Value: "",
			},
			wantError:  false,
			encryptKey: "thisis16bytes123",
			decryptKey: "thisis16bytes123",
		},
		{
			name: "very long value",
			cookie: http.Cookie{
				Name:  "test",
				Value: string(make([]byte, 4096)),
			},
			wantError:  true,
			encryptKey: "thisis16bytes123",
			decryptKey: "thisis16bytes123",
		},
		{
			name: "tampered cookie",
			cookie: http.Cookie{
				Name:  "test",
				Value: "testValue",
			},
			wantError:                false,
			encryptKey:               "thisis16bytes123",
			decryptKey:               string(make([]byte, 16)),
			dontExpectCorrectCookies: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			err := WriteEncrypted(w, tt.cookie, tt.encryptKey)
			if tt.wantError {
				assert.NotEqual(t, err, nil)
				return
			}
			assert.NotEqual(t, w.Header().Get("Set-Cookie"), "")
			req := &http.Request{Header: http.Header{"Cookie": {w.Header().Get("Set-Cookie")}}}
			readCookie, err := ReadEncrypted(req, tt.cookie.Name, tt.decryptKey)
			if tt.dontExpectCorrectCookies {
				assert.NotEqual(t, err, nil)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, readCookie, tt.cookie.Value)
		})
	}
}
