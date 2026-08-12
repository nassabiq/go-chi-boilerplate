package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	healthhandler "github.com/nassabiq/go-chi-boilerplate/internal/core/handler/health"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/health"
	"github.com/stretchr/testify/assert"
)

func TestHealthHandler_Live(t *testing.T) {
	checker := health.NewChecker(nil, nil, nil)
	h := healthhandler.NewHealthHandler(checker)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)

	h.Live(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]string
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "live", resp["status"])
}

func TestHealthHandler_Ready_Unavailable(t *testing.T) {
	checker := health.NewChecker(nil, nil, nil)
	h := healthhandler.NewHealthHandler(checker)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)

	h.Ready(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	var resp health.ReadinessResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "down", resp.Status)
}
