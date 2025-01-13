package request

import (
	"bytes"
	"github.com/blazskufca/goscrapyd/internal/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testJson struct {
	Name    string   `json:"name"`
	Age     int      `json:"age"`
	Email   string   `json:"email"`
	Tags    []string `json:"tags"`
	Country string   `json:"country,omitempty"`
}

func TestDecodeJSON(t *testing.T) {
	tests := []struct {
		name          string
		json          string
		want          testJson
		wantErr       bool
		expectedError string
	}{
		{
			name: "valid JSON",
			json: `{
               "name": "John",
               "age": 30,
               "email": "john@example.com",
               "tags": ["developer", "golang"]
           }`,
			want: testJson{
				Name:  "John",
				Age:   30,
				Email: "john@example.com",
				Tags:  []string{"developer", "golang"},
			},
			wantErr: false,
		},
		{
			name: "JSON with unknown field",
			json: `{
               "name": "John",
               "age": 30,
               "email": "john@example.com",
               "unknown_field": "value"
           }`,
			want: testJson{
				Name:  "John",
				Age:   30,
				Email: "john@example.com",
			},
			wantErr: false,
		},
		{
			name:          "empty body",
			json:          "",
			wantErr:       true,
			expectedError: "body must not be empty",
		},
		{
			name:          "malformed JSON",
			json:          `{"name": "John", "age": }`,
			wantErr:       true,
			expectedError: "body contains badly-formed JSON (at character 25)",
		},
		{
			name:          "incorrect type",
			json:          `{"name": "John", "age": "thirty"}`,
			wantErr:       true,
			expectedError: `body contains incorrect JSON type for field "age"`,
		},
		{
			name:          "multiple JSON values",
			json:          `{"name": "John"} {"name": "Jane"}`,
			wantErr:       true,
			expectedError: "body must only contain a single JSON value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := bytes.NewBufferString(tt.json)
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/", body)

			var got testJson
			err := DecodeJSON(w, r, &got)

			if tt.wantErr {
				assert.NotEqual(t, err, nil)
				assert.Equal(t, err.Error(), tt.expectedError)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, got.Name, tt.want.Name)
				assert.Equal(t, got.Age, tt.want.Age)
				assert.Equal(t, got.Email, tt.want.Email)
				assert.Equal(t, got.Country, tt.want.Country)
				for i, tag := range tt.want.Tags {
					assert.Equal(t, tag, got.Tags[i])
				}
			}
		})
	}
}

func TestDecodeJSONStrict(t *testing.T) {
	tests := []struct {
		name          string
		json          string
		want          testJson
		wantErr       bool
		expectedError string
	}{
		{
			name: "valid JSON",
			json: `{
              "name": "Jane",
              "age": 25,
              "email": "jane@example.com",
              "tags": ["designer"]
          }`,
			want: testJson{
				Name:  "Jane",
				Age:   25,
				Email: "jane@example.com",
				Tags:  []string{"designer"},
			},
			wantErr: false,
		},
		{
			name: "JSON with unknown field",
			json: `{
              "name": "Jane",
              "age": 25,
              "email": "jane@example.com",
              "unknown_field": "value"
          }`,
			wantErr:       true,
			expectedError: `body contains unknown key "unknown_field"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := bytes.NewBufferString(tt.json)
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/", body)

			var got testJson
			err := DecodeJSONStrict(w, r, &got)

			if tt.wantErr {
				assert.NotEqual(t, err, nil)
				assert.Equal(t, err.Error(), tt.expectedError)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, got.Name, tt.want.Name)
				assert.Equal(t, got.Age, tt.want.Age)
				assert.Equal(t, got.Email, tt.want.Email)
				assert.Equal(t, got.Country, tt.want.Country)
				for i, tag := range tt.want.Tags {
					assert.Equal(t, tag, got.Tags[i])
				}
			}
		})
	}
}

func TestRequestBodyTooLarge(t *testing.T) {
	largeValue := strings.Repeat("a", 1_048_577)
	largeJSON := `{"name": "` + largeValue + `"}`

	body := bytes.NewBufferString(largeJSON)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", body)

	var got testJson
	err := DecodeJSON(w, r, &got)

	assert.NotEqual(t, err, nil)
	assert.Equal(t, err.Error(), "body must not be larger than 1048576 bytes")
}

func TestInvalidUnmarshalError(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("The code did not panic")
		} else {
			err, ok := r.(error)
			if !ok {
				t.Fatalf("could not get error from panic")
			}
			assert.Equal(t, err.Error(), "json: Unmarshal(non-pointer request.testJson)")
		}
	}()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))

	var got testJson

	_ = DecodeJSON(w, r, got) // Note: passing got, not &got
}
