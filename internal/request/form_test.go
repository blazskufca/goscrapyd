package request

import (
	"github.com/blazskufca/goscrapyd/internal/assert"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type testStruct struct {
	Name    string   `form:"name"`
	Age     int      `form:"age"`
	Email   string   `form:"email"`
	Hobbies []string `form:"hobbies"`
}

func TestDecodeForm(t *testing.T) {
	tests := []struct {
		name     string
		formData string
		want     testStruct
		wantErr  bool
	}{
		{
			name:     "basic form data",
			formData: "name=John&age=25&email=john@example.com&hobbies=reading&hobbies=gaming",
			want: testStruct{
				Name:    "John",
				Age:     25,
				Email:   "john@example.com",
				Hobbies: []string{"reading", "gaming"},
			},
			wantErr: false,
		},
		{
			name:     "missing optional fields",
			formData: "name=Alice&age=30",
			want: testStruct{
				Name: "Alice",
				Age:  30,
			},
			wantErr: false,
		},
		{
			name:     "invalid age format",
			formData: "name=Bob&age=invalid",
			want:     testStruct{Name: "Bob"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "/test", strings.NewReader(tt.formData))
			assert.NilError(t, err)
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

			var got testStruct
			err = DecodeForm(req, &got)

			if tt.wantErr {
				assert.NotEqual(t, err, nil)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, got.Name, tt.want.Name)
				assert.Equal(t, got.Age, tt.want.Age)
				assert.Equal(t, got.Email, tt.want.Email)
				for i, h := range tt.want.Hobbies {
					assert.Equal(t, h, tt.want.Hobbies[i])
				}
			}
		})
	}
}

func TestDecodePostForm(t *testing.T) {
	tests := []struct {
		name     string
		formData string
		want     testStruct
		wantErr  bool
	}{
		{
			name:     "valid post form",
			formData: "name=Jane&age=28&email=jane@example.com",
			want: testStruct{
				Name:  "Jane",
				Age:   28,
				Email: "jane@example.com",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "/test", strings.NewReader(tt.formData))
			assert.NilError(t, err)
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

			var got testStruct
			err = DecodePostForm(req, &got)

			if tt.wantErr {
				assert.NotEqual(t, err, nil)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, got.Name, tt.want.Name)
				assert.Equal(t, got.Age, tt.want.Age)
				assert.Equal(t, got.Email, tt.want.Email)
			}
		})
	}
}

func TestDecodeQueryString(t *testing.T) {
	tests := []struct {
		name        string
		queryString string
		want        testStruct
		wantErr     bool
	}{
		{
			name:        "valid query string",
			queryString: "?name=Mark&age=35&email=mark@example.com",
			want: testStruct{
				Name:  "Mark",
				Age:   35,
				Email: "mark@example.com",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "/test"+tt.queryString, nil)
			assert.NilError(t, err)

			var got testStruct
			err = DecodeQueryString(req, &got)

			if tt.wantErr {
				assert.NotEqual(t, err, nil)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, got.Name, tt.want.Name)
				assert.Equal(t, got.Age, tt.want.Age)
				assert.Equal(t, got.Email, tt.want.Email)
			}
		})
	}
}

func TestDecodeURLValues(t *testing.T) {
	tests := []struct {
		name    string
		values  url.Values
		want    testStruct
		wantErr bool
	}{
		{
			name: "valid url values",
			values: url.Values{
				"name":    []string{"Sarah"},
				"age":     []string{"40"},
				"email":   []string{"sarah@example.com"},
				"hobbies": []string{"swimming", "running"},
			},
			want: testStruct{
				Name:    "Sarah",
				Age:     40,
				Email:   "sarah@example.com",
				Hobbies: []string{"swimming", "running"},
			},
			wantErr: false,
		},
		{
			name: "invalid type conversion",
			values: url.Values{
				"name": []string{"Tom"},
				"age":  []string{"not-a-number"},
			},
			want:    testStruct{Name: "Tom"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got testStruct
			err := decodeURLValues(tt.values, &got)

			if tt.wantErr {
				assert.NotEqual(t, err, nil)
			} else {
				assert.NilError(t, err)
				assert.Equal(t, got.Name, tt.want.Name)
				assert.Equal(t, got.Age, tt.want.Age)
				assert.Equal(t, got.Email, tt.want.Email)
				for i, h := range tt.want.Hobbies {
					assert.Equal(t, h, tt.want.Hobbies[i])
				}
			}
		})
	}
}
