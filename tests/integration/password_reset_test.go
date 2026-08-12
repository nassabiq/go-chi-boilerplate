//go:build integration
// +build integration

package integration_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/messaging"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
)

func tokenFromResetURL(resetURL string) string {
	idx := strings.LastIndex(resetURL, "/")
	if idx < 0 {
		return resetURL
	}
	return resetURL[idx+1:]
}

// rawToken stored as SHA-256 hex in the DB; verify it matches the event URL.
func sha256Hex(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func TestForgotPassword_RegisteredAndUnregisteredSameEnvelope(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	env.createUserFixture(t, "reset-registered@example.com", "Password123!", enums.StatusActive)

	// Registered email → generic 200.
	respReg := env.do(http.MethodPost, "/auth/forgot-password", payloads.ForgotPasswordRequest{Email: "reset-registered@example.com"}, nil)
	requireStatus(t, respReg, http.StatusOK)

	// Unregistered email → identical public status.
	respUnreg := env.do(http.MethodPost, "/auth/forgot-password", payloads.ForgotPasswordRequest{Email: "reset-unknown@example.com"}, nil)
	requireStatus(t, respUnreg, http.StatusOK)

	// Normalization: mixed case / spaces should still resolve the canonical row.
	respNorm := env.do(http.MethodPost, "/auth/forgot-password", payloads.ForgotPasswordRequest{Email: "  RESET-REGISTERED@EXAMPLE.COM "}, nil)
	requireStatus(t, respNorm, http.StatusOK)
}

func TestForgotPassword_CreatesAuditRowAndPublishesEvent(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	env.createUserFixture(t, "audit@example.com", "Password123!", enums.StatusActive)

	resp := env.do(http.MethodPost, "/auth/forgot-password", payloads.ForgotPasswordRequest{Email: "audit@example.com"}, map[string]string{"User-Agent": "test-agent"})
	requireStatus(t, resp, http.StatusOK)

	// A password_reset_tokens row must exist.
	var count int64
	if err := env.db.Raw("SELECT count(*) FROM password_reset_tokens WHERE user_id = (SELECT id FROM users WHERE email = ?)", "audit@example.com").Scan(&count).Error; err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 reset token row, got %d", count)
	}

	// Raw token must NOT be persisted; only its hash.
	var tokenHash, requestedIP string
	var usedAt interface{}
	if err := env.db.Raw("SELECT token_hash, requested_ip, used_at FROM password_reset_tokens LIMIT 1").Row().Scan(&tokenHash, &requestedIP, &usedAt); err != nil {
		t.Fatalf("scan token: %v", err)
	}
	if tokenHash == "" {
		t.Fatalf("expected token_hash to be stored")
	}
	if len(tokenHash) != 64 {
		t.Fatalf("token_hash should be SHA-256 hex (64 chars), got %q", tokenHash)
	}

	// The captured event lets us recover the raw token and verify the hash.
	event, ok := env.events.lastResetEvent()
	if !ok {
		t.Fatalf("expected a published password reset event")
	}
	raw := tokenFromResetURL(event.ResetURL)
	if raw == "" {
		t.Fatalf("reset url missing token: %q", event.ResetURL)
	}
	if sha256Hex(raw) != tokenHash {
		t.Fatalf("stored token_hash does not match SHA-256 of raw token")
	}
	if requestedIP == "" {
		t.Fatalf("expected requested_ip to be persisted")
	}
	if event.UserID == 0 || event.Email != "audit@example.com" {
		t.Fatalf("unexpected event identity: %+v", event)
	}
}

func TestResetPassword_FullFlowAndRevocation(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	userID := env.createUserFixture(t, "flow@example.com", "Password123!", enums.StatusActive)

	// Login to get a session, then forgot-password to get a token.
	login := env.login(t, "flow@example.com", "Password123!", "ua")
	atBefore := login.Auth.AccessToken
	rtBefore := login.Auth.RefreshToken

	resp := env.do(http.MethodPost, "/auth/forgot-password", payloads.ForgotPasswordRequest{Email: "flow@example.com"}, nil)
	requireStatus(t, resp, http.StatusOK)

	event, ok := env.events.lastResetEvent()
	if !ok {
		t.Fatalf("expected reset event")
	}
	raw := tokenFromResetURL(event.ResetURL)

	// Reset with the raw token.
	resetResp := env.do(http.MethodPost, "/auth/reset-password", payloads.ResetPasswordRequest{Token: raw, NewPassword: "NewPassword456!"}, nil)
	requireStatus(t, resetResp, http.StatusOK)

	// Token is single-use: reuse must fail.
	reuseResp := env.do(http.MethodPost, "/auth/reset-password", payloads.ResetPasswordRequest{Token: raw, NewPassword: "AnotherPass789!"}, nil)
	requireCode(t, reuseResp, "E_AUTH_13")

	// Old credentials no longer work.
	oldLogin := env.loginRaw(t, "flow@example.com", "Password123!", "ua")
	requireStatus(t, oldLogin, http.StatusUnauthorized)

	// New password works.
	newLogin := env.login(t, "flow@example.com", "NewPassword456!", "ua")
	if newLogin.Auth.AccessToken == "" {
		t.Fatalf("new password should authenticate")
	}

	// Old access/refresh tokens are revoked.
	meResp := env.authed(http.MethodGet, "/auth/me", atBefore, nil)
	requireCode(t, meResp, "E_AUTH_5")
	refreshResp := env.do(http.MethodPost, "/auth/refresh", payloads.RefreshTokenRequest{RefreshToken: rtBefore}, nil)
	requireCode(t, refreshResp, "E_AUTH_5")

	_ = userID
}

func TestResetPassword_InvalidTokensSameError(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	cases := []string{"not-a-real-token", "expired-or-unknown"}
	for _, tok := range cases {
		resp := env.do(http.MethodPost, "/auth/reset-password", payloads.ResetPasswordRequest{Token: tok, NewPassword: "NewPassword456!"}, nil)
		requireCode(t, resp, "E_AUTH_13")
	}
}

func TestChangePassword_Flow(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	env.createUserFixture(t, "change@example.com", "Password123!", enums.StatusActive)
	login := env.login(t, "change@example.com", "Password123!", "ua")
	if login.Auth.AccessToken == "" {
		t.Fatalf("login failed")
	}

	// Unauthenticated → rejected.
	noAuth := env.do(http.MethodPost, "/auth/change-password", payloads.ChangePasswordRequest{CurrentPassword: "Password123!", NewPassword: "NewPassword456!"}, nil)
	requireCode(t, noAuth, "E_AUTH_5")

	// Wrong current → rejected.
	wrong := env.authed(http.MethodPost, "/auth/change-password", login.Auth.AccessToken, payloads.ChangePasswordRequest{CurrentPassword: "WrongPass!", NewPassword: "NewPassword456!"})
	requireCode(t, wrong, "E_AUTH_14")

	// Same as current → rejected.
	same := env.authed(http.MethodPost, "/auth/change-password", login.Auth.AccessToken, payloads.ChangePasswordRequest{CurrentPassword: "Password123!", NewPassword: "Password123!"})
	requireCode(t, same, "E_AUTH_15")

	// Success.
	ok := env.authed(http.MethodPost, "/auth/change-password", login.Auth.AccessToken, payloads.ChangePasswordRequest{CurrentPassword: "Password123!", NewPassword: "NewPassword456!"})
	requireStatus(t, ok, http.StatusOK)

	// New password authenticates; old does not.
	if env.login(t, "change@example.com", "NewPassword456!", "ua").Auth.AccessToken == "" {
		t.Fatalf("new password should authenticate")
	}
	requireStatus(t, env.loginRaw(t, "change@example.com", "Password123!", "ua"), http.StatusUnauthorized)
}

// TestResetPassword_ConcurrentSingleUse proves the DB row lock makes the reset
// token single-use even under concurrent attempts. Exactly one may consume it.
func TestResetPassword_ConcurrentSingleUse(t *testing.T) {
	env := getTestEnv(t)
	env.reset(t)

	env.createUserFixture(t, "concurrent-reset@example.com", "Password123!", enums.StatusActive)
	env.do(http.MethodPost, "/auth/forgot-password", payloads.ForgotPasswordRequest{Email: "concurrent-reset@example.com"}, nil)
	event, ok := env.events.lastResetEvent()
	if !ok {
		t.Fatalf("expected reset event")
	}
	raw := tokenFromResetURL(event.ResetURL)

	const attempts = 5
	results := make(chan bool, attempts)
	for i := 0; i < attempts; i++ {
		go func() {
			resp := env.do(http.MethodPost, "/auth/reset-password", payloads.ResetPasswordRequest{Token: raw, NewPassword: "NewPassword456!"}, nil)
			results <- resp.StatusCode == http.StatusOK
		}()
	}

	successes := 0
	for i := 0; i < attempts; i++ {
		if <-results {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one successful reset, got %d", successes)
	}
}

var _ = auth.HashToken
var _ = messaging.TopicPasswordResetRequested
var _ = time.Now
