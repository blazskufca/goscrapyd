package response

import (
	"github.com/blazskufca/goscrapyd/internal/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetricsResponseWriter(t *testing.T) {
	t.Run("defaults to 200 status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		mw := NewMetricsResponseWriter(w)

		assert.Equal(t, mw.StatusCode, http.StatusOK)
	})

	t.Run("tracks custom status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		mw := NewMetricsResponseWriter(w)

		mw.WriteHeader(http.StatusNotFound)

		assert.Equal(t, mw.StatusCode, http.StatusNotFound)
		assert.Equal(t, w.Code, http.StatusNotFound)
	})

	t.Run("counts bytes written", func(t *testing.T) {
		w := httptest.NewRecorder()
		mw := NewMetricsResponseWriter(w)

		data := []byte("hello world")
		n, err := mw.Write(data)
		assert.NilError(t, err)
		assert.Equal(t, n, len(data))
		assert.Equal(t, mw.BytesCount, len(data))
	})

	t.Run("handles multiple writes", func(t *testing.T) {
		w := httptest.NewRecorder()
		mw := NewMetricsResponseWriter(w)

		data1 := []byte("hello")
		data2 := []byte(" world")

		_, err := mw.Write(data1)
		assert.NilError(t, err)
		_, err = mw.Write(data2)
		assert.NilError(t, err)

		assert.Equal(t, mw.BytesCount, len(data1)+len(data2))
	})

	t.Run("forwards headers", func(t *testing.T) {
		w := httptest.NewRecorder()
		mw := NewMetricsResponseWriter(w)
		setValue := "value"
		mw.Header().Set("X-Test", setValue)
		assert.Equal(t, w.Header().Get("X-Test"), setValue)
	})

	t.Run("supports flusher interface", func(t *testing.T) {
		w := httptest.NewRecorder()
		mw := NewMetricsResponseWriter(w)

		mw.Flush()

		assert.Equal(t, w.Flushed, true)
	})

	t.Run("only records first status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		mw := NewMetricsResponseWriter(w)

		mw.WriteHeader(http.StatusBadRequest)
		mw.WriteHeader(http.StatusOK)

		assert.Equal(t, mw.StatusCode, http.StatusBadRequest)
	})
}
