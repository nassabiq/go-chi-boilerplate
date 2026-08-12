//go:build integration
// +build integration

package integration_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
)

func TestMetrics_EndpointExposition(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	// 1. Scrape /metrics initially
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /metrics, got %d: %s", rec.Code, rec.Body.String())
	}

	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("failed to read /metrics response: %v", err)
	}

	// 2. Perform a failed login to trigger auth.login.failure metric
	loginBody := payloads.LoginRequest{
		Email:    "nonexistent@example.com",
		Password: "WrongPassword123!",
	}
	loginResp := env.do(http.MethodPost, "/auth/login", loginBody, nil)
	if loginResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for failed login, got %d", loginResp.StatusCode)
	}

	// 3. Scrape /metrics again
	req2 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec2 := httptest.NewRecorder()
	env.router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /metrics, got %d", rec2.Code)
	}

	metricsOutput := rec2.Body.String()

	// In Prometheus text format, metric names with dots become underscores with _total suffix
	expectedSubstrings := []string{
		"auth_login_failure",
		"invalid_credentials",
	}

	for _, expected := range expectedSubstrings {
		if !strings.Contains(metricsOutput, expected) {
			t.Errorf("expected /metrics output to contain %q, but got:\n%s", expected, metricsOutput)
		}
	}

	_ = body
}
