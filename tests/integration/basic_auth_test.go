//go:build integration
// +build integration

package integration_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/router"
)

func TestBasicAuth_EnforcedWhenConfigured(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	// Build a router instance where BasicAuth is explicitly configured
	customCfg := *env.cfg
	customCfg.Auth.BasicAuth = config.BasicAuthConfig{
		Username: "client-id",
		Password: "client-secret-777",
	}

	customRouter := router.New(&router.Dependencies{
		Logger:  env.logger,
		Handler: env.handler,
		Limiter: env.limiter,
		Config:  &customCfg,
		JWT:     env.jwt,
		Session: env.session,
	})

	loginPayload := payloads.LoginRequest{
		Email:    "test@example.com",
		Password: "SomePassword123!",
	}
	payloadBytes, _ := json.Marshal(loginPayload)

	doRequest := func(headers map[string]string) *http.Response {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		customRouter.ServeHTTP(rec, req)
		return rec.Result()
	}

	// 1. Without Basic Auth header -> Expect 401 Unauthorized with WWW-Authenticate
	respNoAuth := doRequest(nil)
	if respNoAuth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized when missing basic auth, got %d", respNoAuth.StatusCode)
	}
	if respNoAuth.Header.Get("WWW-Authenticate") != `Basic realm="Restricted"` {
		t.Fatalf("expected WWW-Authenticate header, got %q", respNoAuth.Header.Get("WWW-Authenticate"))
	}

	// 2. With wrong Basic Auth credentials -> Expect 401 Unauthorized
	wrongCreds := base64.StdEncoding.EncodeToString([]byte("client-id:wrong-secret"))
	respWrongAuth := doRequest(map[string]string{
		"Authorization": "Basic " + wrongCreds,
	})
	if respWrongAuth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for wrong basic auth, got %d", respWrongAuth.StatusCode)
	}

	// 3. With correct Basic Auth credentials -> Passes BasicAuth middleware
	validCreds := base64.StdEncoding.EncodeToString([]byte("client-id:client-secret-777"))
	respValidAuth := doRequest(map[string]string{
		"Authorization": "Basic " + validCreds,
	})
	// Should reach login handler (which returns 401 for unregistered user, NOT basic auth WWW-Authenticate)
	if respValidAuth.Header.Get("WWW-Authenticate") != "" {
		t.Fatalf("did not expect WWW-Authenticate header when basic auth is valid")
	}
}
