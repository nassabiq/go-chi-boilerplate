package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/middleware"
)

func TestSecurityHeaders_Applied(t *testing.T) {
	handler := middleware.SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("expected nosniff, got %q", rr.Header().Get("X-Content-Type-Options"))
	}
	if rr.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("expected DENY, got %q", rr.Header().Get("X-Frame-Options"))
	}
	if rr.Header().Get("Content-Security-Policy") != "default-src 'none'; frame-ancestors 'none'" {
		t.Errorf("expected CSP, got %q", rr.Header().Get("Content-Security-Policy"))
	}
	if rr.Header().Get("Referrer-Policy") != "same-origin" {
		t.Errorf("expected same-origin, got %q", rr.Header().Get("Referrer-Policy"))
	}
	if rr.Header().Get("Strict-Transport-Security") == "" {
		t.Error("expected HSTS header to be set")
	}
}

func TestSecurityHeaders_CacheControlForAuth(t *testing.T) {
	handler := middleware.SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Non-auth route without token should not set no-store
	req1 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	if rr1.Header().Get("Cache-Control") != "" {
		t.Errorf("expected empty Cache-Control on healthz, got %q", rr1.Header().Get("Cache-Control"))
	}

	// Auth route without token MUST set no-store
	req2 := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Header().Get("Cache-Control") != "no-store, no-cache, must-revalidate, private" {
		t.Errorf("expected no-store Cache-Control on /auth/login, got %q", rr2.Header().Get("Cache-Control"))
	}

	// Request with Authorization header MUST set no-store
	req3 := httptest.NewRequest(http.MethodGet, "/users", nil)
	req3.Header.Set("Authorization", "Bearer some-token")
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)
	if rr3.Header().Get("Cache-Control") != "no-store, no-cache, must-revalidate, private" {
		t.Errorf("expected no-store Cache-Control with Authorization, got %q", rr3.Header().Get("Cache-Control"))
	}
}

func TestCORS_Preflight_AllowedOrigin(t *testing.T) {
	cfg := config.CORSConfig{
		AllowedOrigins:   []string{"https://example.com"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           600,
	}

	handler := middleware.CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/auth/login", nil)
	req.Header.Set("Origin", "https://example.com")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content for preflight, got %d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("expected https://example.com, got %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
	if rr.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("expected credentials true, got %q", rr.Header().Get("Access-Control-Allow-Credentials"))
	}
	if rr.Header().Get("Access-Control-Max-Age") != "600" {
		t.Errorf("expected Max-Age 600, got %q", rr.Header().Get("Access-Control-Max-Age"))
	}
}

func TestCORS_Preflight_DisallowedOrigin(t *testing.T) {
	cfg := config.CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
	}

	handler := middleware.CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/auth/login", nil)
	req.Header.Set("Origin", "https://attacker.com")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for disallowed origin preflight, got %d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("expected empty allow-origin, got %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_Wildcard_DisallowsCredentials(t *testing.T) {
	cfg := config.CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: false,
	}

	handler := middleware.CORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://example.com")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected *, got %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
	if rr.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Errorf("expected no credentials with wildcard, got %q", rr.Header().Get("Access-Control-Allow-Credentials"))
	}
}
