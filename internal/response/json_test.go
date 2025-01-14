package response

import (
	"encoding/base64"
	"github.com/blazskufca/goscrapyd/internal/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSON(t *testing.T) {
	testCases := []struct {
		name          string
		statusCode    int
		data          any
		marshaledData string
	}{
		{
			name:       "map marshal",
			statusCode: http.StatusOK,
			data: map[string]string{
				"foo": "bar",
			},
			marshaledData: "{\n\t\"foo\": \"bar\"\n}\n",
		},
		{
			name:          "string marshal",
			statusCode:    http.StatusOK,
			data:          "test",
			marshaledData: "\"test\"\n",
		},
		{
			name:       "struct marshal",
			statusCode: http.StatusOK,
			data: struct {
				Foo string `json:"foo"`
				Bar string `json:"bar"`
			}{},
			marshaledData: "{\n\t\"foo\": \"\",\n\t\"bar\": \"\"\n}\n",
		},
	}
	for _, tt := range testCases {
		w := httptest.NewRecorder()
		err := JSON(w, tt.statusCode, tt.data)
		assert.NilError(t, err)
		assert.Equal(t, tt.statusCode, w.Code)
		assert.Equal(t, w.Body.String(), tt.marshaledData)
		assert.Equal(t, w.Header().Get("Content-Type"), "application/json")
	}
}

func TestJSONWithHeaders(t *testing.T) {
	testCases := []struct {
		name              string
		statusCode        int
		data              any
		marshaledData     string
		additionalHeaders http.Header
	}{
		{
			name:       "map marshal",
			statusCode: http.StatusOK,
			data: map[string]string{
				"foo": "bar",
			},
			marshaledData: "{\n\t\"foo\": \"bar\"\n}\n",
			additionalHeaders: http.Header{
				"Authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))},
			},
		},
		{
			name:          "string marshal",
			statusCode:    http.StatusOK,
			data:          "test",
			marshaledData: "\"test\"\n",
			additionalHeaders: http.Header{
				"Accept-Encoding":           []string{"gzip", "deflate", "br", "zstd"},
				"Upgrade-Insecure-Requests": []string{"1"},
				"Connection":                []string{"keep-alive"},
				"User-Agent":                []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0"},
			},
		},
		{
			name:       "struct marshal",
			statusCode: http.StatusOK,
			data: struct {
				Foo string `json:"foo"`
				Bar string `json:"bar"`
			}{},
			marshaledData: "{\n\t\"foo\": \"\",\n\t\"bar\": \"\"\n}\n",
		},
	}
	for _, tt := range testCases {
		w := httptest.NewRecorder()
		err := JSONWithHeaders(w, tt.statusCode, tt.data, tt.additionalHeaders)
		assert.NilError(t, err)
		assert.Equal(t, tt.statusCode, w.Code)
		assert.Equal(t, w.Body.String(), tt.marshaledData)
		assert.Equal(t, w.Header().Get("Content-Type"), "application/json")
		for k, v := range tt.additionalHeaders {
			got := w.Header().Get(k)
			assert.Equal(t, v[0], got)
		}
	}
}
