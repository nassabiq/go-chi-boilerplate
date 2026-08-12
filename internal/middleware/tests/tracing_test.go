package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nassabiq/go-chi-boilerplate/internal/middleware"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/telemetry"
	"github.com/stretchr/testify/assert"
)

func TestTracing_SetsTraceIDHeader(t *testing.T) {
	_, err := telemetry.InitTracer("test-service")
	assert.NoError(t, err)

	handler := middleware.Tracing("test-service")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test-path", nil)

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	traceID := rec.Header().Get("X-Trace-ID")
	assert.NotEmpty(t, traceID, "X-Trace-ID header should be present")
}
