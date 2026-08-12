package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-playground/validator/v10"
	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/usecase"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	customvalidator "github.com/nassabiq/go-chi-boilerplate/internal/utils/validator"
	"github.com/redis/go-redis/v9"
)

func newRefreshHarness(t *testing.T) (*usecase.BaseUsecase, *auth.JWT, *auth.SessionManager, *miniredis.Miniredis) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{
		Addr:         mr.Addr(),
		MaxRetries:   0,
		DialTimeout:  100 * time.Millisecond,
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: 200 * time.Millisecond,
	})
	t.Cleanup(func() { _ = client.Close() })

	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWT: config.JWTConfig{
				AccessSecret:  "access-secret",
				RefreshSecret: "refresh-secret",
				Issuer:        "go-chi",
				AccessExpiry:  15 * time.Minute,
				RefreshExpiry: 168 * time.Hour,
			},
		},
	}
	jwtSvc := auth.NewJWT(cfg)
	sessMgr := auth.NewSessionManager(client)

	v := validator.New()
	_ = customvalidator.RegisterPasswordValidation(v)

	uc := usecase.New(&usecase.Dependencies{
		Validator: v,
		JWT:       jwtSvc,
		Session:   sessMgr,
	})

	return uc, jwtSvc, sessMgr, mr
}

// seedRefreshSession creates a valid session in Redis and returns a refresh
// token whose hash matches the stored refresh_hash (exactly like Login does).
func seedRefreshSession(t *testing.T, jwtSvc *auth.JWT, sessMgr *auth.SessionManager, userID int64, sid string, expiresAt time.Time) string {
	t.Helper()
	ctx := context.Background()

	refreshToken, err := jwtSvc.GenerateRefreshToken(userID, sid, expiresAt, nil, nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	if err := sessMgr.Create(ctx, auth.Session{
		ID:          sid,
		UserID:      userID,
		RefreshHash: auth.HashToken(refreshToken),
		CreatedAt:   time.Now(),
		ExpiresAt:   expiresAt,
	}); err != nil {
		t.Fatalf("session.Create: %v", err)
	}

	return refreshToken
}

func assertCode(t *testing.T, err error, want errconstant.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s, got nil", want)
	}
	var ae *apperror.Error
	if !errors.As(err, &ae) {
		t.Fatalf("expected *apperror.Error, got %T (%v)", err, err)
	}
	if ae.Code != want {
		t.Fatalf("expected code %s, got %s", want, ae.Code)
	}
}

func TestRefresh_ValidFlow(t *testing.T) {
	uc, jwtSvc, sessMgr, _ := newRefreshHarness(t)
	ctx := context.Background()

	expiry := time.Now().Add(48 * time.Hour).Truncate(time.Second)
	oldToken := seedRefreshSession(t, jwtSvc, sessMgr, 1, "sid-R", expiry)

	result, err := uc.Refresh(ctx, payloads.RefreshTokenRequest{RefreshToken: oldToken})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	// 2/3/4: tokens returned and the new refresh token differs from the old.
	if result.AccessToken == "" {
		t.Fatalf("expected access token")
	}
	if result.RefreshToken == "" {
		t.Fatalf("expected refresh token")
	}
	if result.RefreshToken == oldToken {
		t.Fatalf("new refresh token must differ from old")
	}
	if result.TokenType != "Bearer" {
		t.Fatalf("expected Bearer token type, got %q", result.TokenType)
	}
	// expires_in must be the ACCESS token lifetime, not the refresh TTL.
	if result.ExpiresIn != 900 {
		t.Fatalf("expected expires_in=900 (access TTL), got %d", result.ExpiresIn)
	}

	// 5: session id unchanged.
	accessClaims, err := jwtSvc.ParseAccessToken(result.AccessToken)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if accessClaims.SessionID != "sid-R" {
		t.Fatalf("session id changed: %q", accessClaims.SessionID)
	}
	if accessClaims.UserID != 1 {
		t.Fatalf("user id mismatch: %d", accessClaims.UserID)
	}

	// 6/7: token types are correct.
	if accessClaims.Type != "access" {
		t.Fatalf("access token type = %q", accessClaims.Type)
	}
	refreshClaims, err := jwtSvc.ParseRefreshToken(result.RefreshToken)
	if err != nil {
		t.Fatalf("parse refresh token: %v", err)
	}
	if refreshClaims.Type != "refresh" {
		t.Fatalf("refresh token type = %q", refreshClaims.Type)
	}

	// 8: absolute refresh expiry preserved (no sliding).
	if !refreshClaims.ExpiresAt.Time.Equal(expiry) {
		t.Fatalf("refresh expiry not preserved: got %v want %v", refreshClaims.ExpiresAt.Time, expiry)
	}
	// It must NOT have been extended to the full refresh TTL.
	if refreshClaims.ExpiresAt.Time.Equal(time.Now().Add(168 * time.Hour).Truncate(time.Second)) {
		t.Fatalf("refresh expiry was extended (sliding) instead of absolute")
	}
}

func TestRefresh_MissingSession(t *testing.T) {
	uc, jwtSvc, _, _ := newRefreshHarness(t)
	ctx := context.Background()

	// Valid JWT, but the session was never created in Redis.
	token, err := jwtSvc.GenerateRefreshToken(1, "sid-missing", time.Now().Add(48*time.Hour), nil, nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	_, err = uc.Refresh(ctx, payloads.RefreshTokenRequest{RefreshToken: token})
	if err == nil {
		t.Fatalf("expected failure for missing session")
	}
	// Must not issue usable credentials.
}

func TestRefresh_RevokedSession(t *testing.T) {
	uc, jwtSvc, sessMgr, _ := newRefreshHarness(t)
	ctx := context.Background()

	expiry := time.Now().Add(48 * time.Hour)
	token := seedRefreshSession(t, jwtSvc, sessMgr, 1, "sid-rev", expiry)

	if err := sessMgr.Revoke(ctx, "sid-rev", 1); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	_, err := uc.Refresh(ctx, payloads.RefreshTokenRequest{RefreshToken: token})
	if err == nil {
		t.Fatalf("expected failure for revoked session")
	}
}

func TestRefresh_WrongTokenType(t *testing.T) {
	uc, jwtSvc, _, _ := newRefreshHarness(t)
	ctx := context.Background()

	// An access token presented as a refresh token.
	accessToken, err := jwtSvc.GenerateAccessToken(1, "sid-x", nil, nil)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	_, err = uc.Refresh(ctx, payloads.RefreshTokenRequest{RefreshToken: accessToken})
	if err == nil {
		t.Fatalf("expected failure for wrong token type")
	}
}

func TestRefresh_ExpiredToken(t *testing.T) {
	uc, jwtSvc, _, _ := newRefreshHarness(t)
	ctx := context.Background()

	// Refresh token whose exp is in the past.
	token, err := jwtSvc.GenerateRefreshToken(1, "sid-x", time.Now().Add(-1*time.Hour), nil, nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	_, err = uc.Refresh(ctx, payloads.RefreshTokenRequest{RefreshToken: token})
	if err == nil {
		t.Fatalf("expected failure for expired token")
	}
}

func TestRefresh_InvalidSignature(t *testing.T) {
	uc, _, _, _ := newRefreshHarness(t)
	ctx := context.Background()

	// Token signed with a different refresh secret than the app's JWT.
	badCfg := &config.Config{
		Auth: config.AuthConfig{
			JWT: config.JWTConfig{
				AccessSecret:  "access-secret",
				RefreshSecret: "other-refresh-secret",
				Issuer:        "go-chi",
				AccessExpiry:  15 * time.Minute,
				RefreshExpiry: 168 * time.Hour,
			},
		},
	}
	badJWT := auth.NewJWT(badCfg)
	token, err := badJWT.GenerateRefreshToken(1, "sid-x", time.Now().Add(48*time.Hour), nil, nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	_, err = uc.Refresh(ctx, payloads.RefreshTokenRequest{RefreshToken: token})
	if err == nil {
		t.Fatalf("expected failure for invalid signature")
	}
}

func TestRefresh_WrongIssuer(t *testing.T) {
	uc, _, _, _ := newRefreshHarness(t)
	ctx := context.Background()

	// Token signed with a different issuer than the app's JWT expects.
	badCfg := &config.Config{
		Auth: config.AuthConfig{
			JWT: config.JWTConfig{
				AccessSecret:  "access-secret",
				RefreshSecret: "refresh-secret",
				Issuer:        "evil-issuer",
				AccessExpiry:  15 * time.Minute,
				RefreshExpiry: 168 * time.Hour,
			},
		},
	}
	badJWT := auth.NewJWT(badCfg)
	token, err := badJWT.GenerateRefreshToken(1, "sid-x", time.Now().Add(48*time.Hour), nil, nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	_, err = uc.Refresh(ctx, payloads.RefreshTokenRequest{RefreshToken: token})
	if err == nil {
		t.Fatalf("expected failure for wrong issuer")
	}
}

func TestRefresh_ReuseRevokesSession(t *testing.T) {
	uc, jwtSvc, sessMgr, mr := newRefreshHarness(t)
	ctx := context.Background()

	expiry := time.Now().Add(48 * time.Hour)
	oldToken := seedRefreshSession(t, jwtSvc, sessMgr, 1, "sid-reuse", expiry)

	// First rotation succeeds.
	if _, err := uc.Refresh(ctx, payloads.RefreshTokenRequest{RefreshToken: oldToken}); err != nil {
		t.Fatalf("first refresh should succeed: %v", err)
	}

	// Replaying the OLD token must be rejected (reuse) and revoke the session.
	_, err := uc.Refresh(ctx, payloads.RefreshTokenRequest{RefreshToken: oldToken})
	if err == nil {
		t.Fatalf("expected reuse to be rejected")
	}
	assertCode(t, err, errconstant.ErrCodeAuthInvalidRefreshToken)

	// Session key deleted and removed from the user-session index.
	if exists := mr.Exists("auth:session:sid-reuse"); exists {
		t.Fatalf("reused session key must be deleted")
	}
	if member, _ := mr.SIsMember("auth:user:sessions:1", "sid-reuse"); member {
		t.Fatalf("reused session must be removed from index")
	}

	// 17: the old token cannot be used after a successful rotation.
	_, err = uc.Refresh(ctx, payloads.RefreshTokenRequest{RefreshToken: oldToken})
	if err == nil {
		t.Fatalf("old token must remain unusable")
	}
}

func TestRefresh_RedisUnavailable(t *testing.T) {
	uc, jwtSvc, sessMgr, mr := newRefreshHarness(t)
	ctx := context.Background()

	expiry := time.Now().Add(48 * time.Hour)
	token := seedRefreshSession(t, jwtSvc, sessMgr, 1, "sid-down", expiry)

	// Kill Redis so session validation fails with an infrastructure error.
	mr.Close()

	_, err := uc.Refresh(ctx, payloads.RefreshTokenRequest{RefreshToken: token})
	if err == nil {
		t.Fatalf("expected failure when Redis is down")
	}
	assertCode(t, err, errconstant.ErrCodeServiceUnavailable)
}

func TestRefresh_ConcurrentOnlyOneSuccess(t *testing.T) {
	uc, jwtSvc, sessMgr, _ := newRefreshHarness(t)
	ctx := context.Background()

	expiry := time.Now().Add(48 * time.Hour)
	oldToken := seedRefreshSession(t, jwtSvc, sessMgr, 1, "sid-conc", expiry)

	const n = 10
	results := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			_, err := uc.Refresh(ctx, payloads.RefreshTokenRequest{RefreshToken: oldToken})
			results <- err
		}()
	}

	success := 0
	for i := 0; i < n; i++ {
		if err := <-results; err == nil {
			success++
		}
	}
	// Exactly one rotation wins; the rest are reuse detections.
	if success != 1 {
		t.Fatalf("expected exactly one successful rotation, got %d", success)
	}
}
