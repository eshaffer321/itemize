package middleware_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/eshaffer321/monarchmoney-sync-backend/internal/api/middleware"
)

func TestLogging(t *testing.T) {
	t.Run("logs request and passes through", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		})

		wrapped := middleware.Logging(logger)(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "OK", rec.Body.String())
	})

	t.Run("captures non-200 status codes", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		wrapped := middleware.Logging(logger)(handler)

		req := httptest.NewRequest(http.MethodGet, "/notfound", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestCORS(t *testing.T) {
	cfg := middleware.CORSConfig{
		AllowedOrigins: []string{"http://localhost:3000"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := middleware.CORS(cfg)(handler)

	t.Run("sets CORS headers for allowed origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET, POST", rec.Header().Get("Access-Control-Allow-Methods"))
		assert.Equal(t, "Content-Type", rec.Header().Get("Access-Control-Allow-Headers"))
	})

	t.Run("does not set CORS headers for disallowed origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "http://evil.com")
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("handles OPTIONS preflight request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestDefaultCORSConfig(t *testing.T) {
	cfg := middleware.DefaultCORSConfig()

	assert.Contains(t, cfg.AllowedOrigins, "http://localhost:3000")
	assert.Contains(t, cfg.AllowedMethods, "GET")
	assert.Contains(t, cfg.AllowedHeaders, "Content-Type")
}
