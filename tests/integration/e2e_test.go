//go:build integration
// +build integration

package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/dto"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
)

type mailDevEmail struct {
	ID      string `json:"id"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
	HTML    string `json:"html"`
	To      []struct {
		Address string `json:"address"`
		Name    string `json:"name"`
	} `json:"to"`
}

func pollMailDevEmail(t *testing.T, mailDevURL, toEmail, subjectPrefix string, timeout time.Duration) *mailDevEmail {
	t.Helper()
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := http.Get(mailDevURL + "/email")
		if err == nil && resp.StatusCode == http.StatusOK {
			var emails []mailDevEmail
			if err := json.NewDecoder(resp.Body).Decode(&emails); err == nil {
				_ = resp.Body.Close()
				for i := len(emails) - 1; i >= 0; i-- {
					email := emails[i]
					matchesTo := false
					for _, to := range email.To {
						if strings.EqualFold(to.Address, toEmail) {
							matchesTo = true
							break
						}
					}
					if matchesTo && (subjectPrefix == "" || strings.Contains(email.Subject, subjectPrefix)) {
						return &email
					}
				}
			} else {
				_ = resp.Body.Close()
			}
		} else if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(150 * time.Millisecond)
	}

	t.Fatalf("timed out after %v waiting for email to %s (subject: %q) in MailDev (%s)", timeout, toEmail, subjectPrefix, mailDevURL)
	return nil
}

func extractTokenFromURL(t *testing.T, rawContent string) string {
	t.Helper()
	// Match path token: /verify/<token>, /reset/<token>, /confirm-email/<token>
	rePath := regexp.MustCompile(`/(?:verify|reset|confirm-email)/([a-zA-Z0-9_%-]+)`)
	if m := rePath.FindStringSubmatch(rawContent); len(m) >= 2 {
		unescaped, err := url.PathUnescape(m[1])
		if err == nil {
			return unescaped
		}
		return m[1]
	}
	// Match query param: ?token=<token> or &token=<token>
	reQuery := regexp.MustCompile(`[?&]token=([a-zA-Z0-9_%-]+)`)
	if m := reQuery.FindStringSubmatch(rawContent); len(m) >= 2 {
		unescaped, err := url.QueryUnescape(m[1])
		if err == nil {
			return unescaped
		}
		return m[1]
	}
	t.Fatalf("failed to extract token from content: %s", rawContent)
	return ""
}

// -----------------------------------------------------------------------------
// FLOW 1: Full Registration to Logout Lifecycle (Phase 17 Minimum Flow 1)
// -----------------------------------------------------------------------------
// REGISTER -> PENDING -> verification event -> mail worker -> MailDev ->
// extract verification link -> VERIFY EMAIL -> ACTIVE -> LOGIN -> AUTH/ME ->
// REFRESH -> SESSION LIST -> LOGOUT -> access protected endpoint fails
func TestE2E_FullRegistrationToLogoutLifecycle(t *testing.T) {
	env := getE2EEnv(t)
	env.reset(t)

	userEmail := fmt.Sprintf("e2e-reg-%d@example.com", time.Now().UnixNano())
	password := "SecurePass123!"

	// 1. REGISTER
	regResp := env.do(http.MethodPost, "/auth/register", payloads.RegisterRequest{
		Name:     "E2E Candidate",
		Email:    userEmail,
		Password: password,
	}, nil)
	requireStatus(t, regResp, http.StatusCreated)

	var regUser dto.User
	decode(t, regResp, &regUser)
	if regUser.Email != userEmail {
		t.Fatalf("expected registered email %s, got %s", userEmail, regUser.Email)
	}
	if regUser.StatusID != int16(enums.StatusPending) {
		t.Fatalf("expected PENDING status_id (%d) after registration, got %d", int16(enums.StatusPending), regUser.StatusID)
	}

	// 2. PENDING -> Login must be rejected
	pendingLoginResp := env.loginRaw(t, userEmail, password, "E2E-Agent")
	requireStatus(t, pendingLoginResp, http.StatusForbidden)
	requireCode(t, pendingLoginResp, "E_AUTH_10")

	// 3. Verification Event -> Mail Worker -> MailDev
	mail := pollMailDevEmail(t, env.mailDevURL, userEmail, "Verify your email", 10*time.Second)
	if mail == nil {
		t.Fatal("expected verification email in MailDev")
	}

	// 4. Extract verification link / token
	rawToken := extractTokenFromURL(t, mail.HTML)

	// 5. VERIFY EMAIL
	verifyResp := env.do(http.MethodPost, "/auth/verify-email", payloads.VerifyEmailRequest{
		Token: rawToken,
	}, nil)
	requireStatus(t, verifyResp, http.StatusOK)

	// 6. ACTIVE -> LOGIN succeeds
	login := env.login(t, userEmail, password, "E2E-Device-1")
	if login.Auth.AccessToken == "" || login.Auth.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens after successful login")
	}

	// 7. AUTH/ME
	meResp := env.authed(http.MethodGet, "/auth/me", login.Auth.AccessToken, nil)
	requireStatus(t, meResp, http.StatusOK)
	var me dto.User
	decode(t, meResp, &me)
	if me.Email != userEmail || me.StatusID != int16(enums.StatusActive) {
		t.Fatalf("expected active user %s, got %+v", userEmail, me)
	}

	// 8. REFRESH
	refreshed, refResp := env.refresh(t, login.Auth.RefreshToken)
	requireStatus(t, refResp, http.StatusOK)
	if refreshed.AccessToken == "" || refreshed.RefreshToken == "" {
		t.Fatal("expected new access and refresh tokens")
	}

	// 9. SESSION LIST
	sessResp := env.authed(http.MethodGet, "/auth/sessions", refreshed.AccessToken, nil)
	requireStatus(t, sessResp, http.StatusOK)
	var sessions []dto.Session
	decode(t, sessResp, &sessions)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 active session, got %d", len(sessions))
	}
	if !sessions[0].Current {
		t.Fatal("expected session to be marked Current=true")
	}

	// 10. LOGOUT
	logoutResp := env.authed(http.MethodPost, "/auth/logout", refreshed.AccessToken, nil)
	requireStatus(t, logoutResp, http.StatusNoContent)

	// 11. Access protected endpoint fails
	meAfterLogout := env.authed(http.MethodGet, "/auth/me", refreshed.AccessToken, nil)
	requireStatus(t, meAfterLogout, http.StatusUnauthorized)
	requireCode(t, meAfterLogout, "E_AUTH_5")
}

// -----------------------------------------------------------------------------
// FLOW 2: Complete Forgot Password to Reset Flow (Phase 17 Minimum Flow 2)
// -----------------------------------------------------------------------------
// FORGOT PASSWORD -> MailDev -> RESET PASSWORD -> old password fails ->
// new password succeeds -> sessions revoked
func TestE2E_ForgotPasswordFlow(t *testing.T) {
	env := getE2EEnv(t)
	env.reset(t)

	userEmail := fmt.Sprintf("e2e-reset-%d@example.com", time.Now().UnixNano())
	oldPassword := "OldSecurePass123!"
	newPassword := "BrandNewSecurePass456!"

	env.createUserFixture(t, userEmail, oldPassword, enums.StatusActive)

	// Establish an active session before password reset
	initialLogin := env.login(t, userEmail, oldPassword, "E2E-Device-Old")

	// 1. FORGOT PASSWORD
	forgotResp := env.do(http.MethodPost, "/auth/forgot-password", payloads.ForgotPasswordRequest{
		Email: userEmail,
	}, nil)
	requireStatus(t, forgotResp, http.StatusOK)

	// 2. MailDev -> Poll reset email
	mail := pollMailDevEmail(t, env.mailDevURL, userEmail, "Reset your password", 10*time.Second)
	if mail == nil {
		t.Fatal("expected password reset email in MailDev")
	}

	// 3. Extract reset token
	rawToken := extractTokenFromURL(t, mail.Text)

	// 4. Same password rejected
	samePassResp := env.do(http.MethodPost, "/auth/reset-password", payloads.ResetPasswordRequest{
		Token:       rawToken,
		NewPassword: oldPassword,
	}, nil)
	requireStatus(t, samePassResp, http.StatusBadRequest)
	requireCode(t, samePassResp, "E_AUTH_15")

	// 5. RESET PASSWORD with new password
	resetResp := env.do(http.MethodPost, "/auth/reset-password", payloads.ResetPasswordRequest{
		Token:       rawToken,
		NewPassword: newPassword,
	}, nil)
	requireStatus(t, resetResp, http.StatusOK)

	// 6. Old password fails
	badLogin := env.loginRaw(t, userEmail, oldPassword, "E2E-Device")
	requireStatus(t, badLogin, http.StatusUnauthorized)
	requireCode(t, badLogin, "E_AUTH_1")

	// 7. Previous sessions revoked
	meWithOldSession := env.authed(http.MethodGet, "/auth/me", initialLogin.Auth.AccessToken, nil)
	requireStatus(t, meWithOldSession, http.StatusUnauthorized)
	requireCode(t, meWithOldSession, "E_AUTH_5")

	// 8. New password succeeds
	newLogin := env.login(t, userEmail, newPassword, "E2E-Device-New")
	if newLogin.Auth.AccessToken == "" {
		t.Fatal("expected new access token after reset login")
	}

	meWithNewSession := env.authed(http.MethodGet, "/auth/me", newLogin.Auth.AccessToken, nil)
	requireStatus(t, meWithNewSession, http.StatusOK)
}

// -----------------------------------------------------------------------------
// FLOW 3: Multiple Sessions and Logout All (Phase 17 Minimum Flow 3)
// -----------------------------------------------------------------------------
// LOGIN -> multiple sessions -> LOGOUT-ALL -> all sessions invalid
func TestE2E_LogoutAllFlow(t *testing.T) {
	env := getE2EEnv(t)
	env.reset(t)

	userEmail := fmt.Sprintf("e2e-multi-%d@example.com", time.Now().UnixNano())
	password := "MultiSessionPass123!"

	env.createUserFixture(t, userEmail, password, enums.StatusActive)

	// 1. LOGIN multiple sessions with different User-Agents
	sess1 := env.login(t, userEmail, password, "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")
	sess2 := env.login(t, userEmail, password, "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X)")
	sess3 := env.login(t, userEmail, password, "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	// 2. SESSION LIST via session 1
	listResp := env.authed(http.MethodGet, "/auth/sessions", sess1.Auth.AccessToken, nil)
	requireStatus(t, listResp, http.StatusOK)
	var sessions []dto.Session
	decode(t, listResp, &sessions)
	if len(sessions) != 3 {
		t.Fatalf("expected 3 active sessions, got %d", len(sessions))
	}

	// 3. Revoke single session (sess2)
	revokeResp := env.authed(http.MethodDelete, fmt.Sprintf("/auth/sessions/%s", sess2.SID), sess1.Auth.AccessToken, nil)
	requireStatus(t, revokeResp, http.StatusNoContent)

	// sess2 is revoked
	meSess2 := env.authed(http.MethodGet, "/auth/me", sess2.Auth.AccessToken, nil)
	requireStatus(t, meSess2, http.StatusUnauthorized)
	requireCode(t, meSess2, "E_AUTH_5")

	// sess1 and sess3 are still valid
	requireStatus(t, env.authed(http.MethodGet, "/auth/me", sess1.Auth.AccessToken, nil), http.StatusOK)
	requireStatus(t, env.authed(http.MethodGet, "/auth/me", sess3.Auth.AccessToken, nil), http.StatusOK)

	// 4. LOGOUT-ALL via session 1
	logoutAllResp := env.authed(http.MethodPost, "/auth/logout-all", sess1.Auth.AccessToken, nil)
	requireStatus(t, logoutAllResp, http.StatusNoContent)

	// 5. All sessions invalid
	requireStatus(t, env.authed(http.MethodGet, "/auth/me", sess1.Auth.AccessToken, nil), http.StatusUnauthorized)
	requireStatus(t, env.authed(http.MethodGet, "/auth/me", sess3.Auth.AccessToken, nil), http.StatusUnauthorized)

	_, ref1 := env.refresh(t, sess1.Auth.RefreshToken)
	requireStatus(t, ref1, http.StatusUnauthorized)

	_, ref3 := env.refresh(t, sess3.Auth.RefreshToken)
	requireStatus(t, ref3, http.StatusUnauthorized)

	// 6. User can log in again fresh
	freshLogin := env.login(t, userEmail, password, "Mozilla/5.0 (Macintosh)")
	requireStatus(t, env.authed(http.MethodGet, "/auth/me", freshLogin.Auth.AccessToken, nil), http.StatusOK)
}
