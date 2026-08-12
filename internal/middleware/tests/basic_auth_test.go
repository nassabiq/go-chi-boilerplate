package tests

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/middleware"
)

func TestBasicAuth_BypassWhenUnconfigured(t *testing.T) {
	cfg := config.BasicAuthConfig{
		Username: "",
		Password: "",
	}

	handler := middleware.BasicAuth(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK when basic auth is unconfigured, got %d", rr.Code)
	}
}

func TestBasicAuth_RejectsMissingHeader(t *testing.T) {
	cfg := config.BasicAuthConfig{
		Username: "admin",
		Password: "secret-password-123",
	}

	handler := middleware.BasicAuth(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for missing auth header, got %d", rr.Code)
	}
	if rr.Header().Get("WWW-Authenticate") != `Basic realm="Restricted"` {
		t.Errorf("expected WWW-Authenticate header, got %q", rr.Header().Get("WWW-Authenticate"))
	}
}

func TestBasicAuth_RejectsInvalidCredentials(t *testing.T) {
	cfg := config.BasicAuthConfig{
		Username: "admin",
		Password: "secret-password-123",
	}

	handler := middleware.BasicAuth(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	testCases := []struct {
		name     string
		user     string
		password string
	}{
		{"wrong_username", "wrong", "secret-password-123"},
		{"wrong_password", "admin", "wrong-password"},
		{"both_wrong", "wrong", "wrong"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
			auth := base64.StdEncoding.EncodeToString([]byte(tc.user + ":" + tc.password))
			req.Header.Set("Authorization", "Basic "+auth)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 Unauthorized, got %d", rr.Code)
			}
		})
	}
}

func TestBasicAuth_AllowsValidCredentials(t *testing.T) {
	cfg := config.BasicAuthConfig{
		Username: "client-app",
		Password: "client-secret-999",
	}

	handler := middleware.BasicAuth(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("access granted"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	auth := base64.StdEncoding.EncodeToString([]byte("client-app:client-secret-999"))
	req.Header.Set("Authorization", "Basic "+auth)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK for valid basic auth, got %d: %s", rr.Code, rr.Body.String())
	}
}
