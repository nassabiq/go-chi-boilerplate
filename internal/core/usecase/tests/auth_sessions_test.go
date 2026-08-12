package tests

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/repositories"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/usecase"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/database"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newSessionsHarness(t *testing.T) (*usecase.BaseUsecase, *auth.SessionManager, *miniredis.Miniredis, sqlmock.Sqlmock) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() { _ = client.Close() })

	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}

	txMgr := database.NewTxManager(db)
	repo := repositories.New(&repositories.Dependencies{TxManager: txMgr})

	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWT: config.JWTConfig{
				AccessSecret:  "access-secret-1234567890-test-key",
				RefreshSecret: "refresh-secret-1234567890-test-key",
				Issuer:        "test-issuer",
				AccessExpiry:  15 * time.Minute,
				RefreshExpiry: 7 * 24 * time.Hour,
			},
		},
	}

	jwtSvc := auth.NewJWT(cfg)
	sessMgr := auth.NewSessionManager(client)

	uc := usecase.New(&usecase.Dependencies{
		Repo:      repo,
		TxManager: txMgr,
		JWT:       jwtSvc,
		Session:   sessMgr,
		Config:    cfg,
	})

	return uc, sessMgr, mr, mock
}

func TestSessions_FindSessions_Success(t *testing.T) {
	uc, sessMgr, _, _ := newSessionsHarness(t)
	ctx := context.Background()

	now := time.Now()
	sess1 := auth.Session{
		ID:          "sess-1",
		UserID:      42,
		RefreshHash: "ref-token-hash-1",
		IPAddress:   "192.168.1.1",
		UserAgent:   "Mozilla/5.0 Chrome/120.0",
		CreatedAt:   now,
		ExpiresAt:   now.Add(24 * time.Hour),
	}
	if err := sessMgr.Create(ctx, sess1); err != nil {
		t.Fatalf("failed to create session 1: %v", err)
	}

	sess2 := auth.Session{
		ID:          "sess-2",
		UserID:      42,
		RefreshHash: "ref-token-hash-2",
		IPAddress:   "10.0.0.1",
		UserAgent:   "Mobile Safari",
		CreatedAt:   now,
		ExpiresAt:   now.Add(24 * time.Hour),
	}
	if err := sessMgr.Create(ctx, sess2); err != nil {
		t.Fatalf("failed to create session 2: %v", err)
	}

	// Request session list, marking sess1 as current
	list, err := uc.FindSessions(ctx, 42, sess1.ID)
	if err != nil {
		t.Fatalf("unexpected error finding sessions: %v", err)
	}

	if len(list) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(list))
	}

	foundCurrent := false
	foundOther := false
	for _, s := range list {
		if s.ID == sess1.ID {
			foundCurrent = true
			if !s.Current {
				t.Errorf("expected sess1 to have Current=true")
			}
			if s.IPAddress != "192.168.1.1" {
				t.Errorf("expected sess1 IP 192.168.1.1, got %s", s.IPAddress)
			}
		} else if s.ID == sess2.ID {
			foundOther = true
			if s.Current {
				t.Errorf("expected sess2 to have Current=false")
			}
			if s.IPAddress != "10.0.0.1" {
				t.Errorf("expected sess2 IP 10.0.0.1, got %s", s.IPAddress)
			}
		}
	}

	if !foundCurrent || !foundOther {
		t.Errorf("missing expected sessions in list: current=%v, other=%v", foundCurrent, foundOther)
	}
}

func TestSessions_RevokeSession_Success(t *testing.T) {
	uc, sessMgr, _, _ := newSessionsHarness(t)
	ctx := context.Background()

	now := time.Now()
	sess := auth.Session{
		ID:          "sess-to-revoke",
		UserID:      42,
		RefreshHash: "token-hash",
		CreatedAt:   now,
		ExpiresAt:   now.Add(24 * time.Hour),
	}
	if err := sessMgr.Create(ctx, sess); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := uc.RevokeSession(ctx, 42, sess.ID); err != nil {
		t.Fatalf("unexpected error revoking session: %v", err)
	}

	// Verify session is revoked
	err := sessMgr.Validate(ctx, sess.ID, 42)
	if err == nil {
		t.Fatal("expected session to be invalid after revocation, got nil")
	}
}

func TestSessions_RevokeSession_EmptyID_Fails(t *testing.T) {
	uc, _, _, _ := newSessionsHarness(t)
	ctx := context.Background()

	err := uc.RevokeSession(ctx, 42, "")
	assertCode(t, err, errconstant.ErrCodeAuthInvalidToken)
}

func TestSessions_Logout_Success(t *testing.T) {
	uc, sessMgr, _, mock := newSessionsHarness(t)
	ctx := context.Background()

	now := time.Now()
	sess := auth.Session{
		ID:          "sess-logout",
		UserID:      42,
		RefreshHash: "token-hash",
		CreatedAt:   now,
		ExpiresAt:   now.Add(24 * time.Hour),
	}
	if err := sessMgr.Create(ctx, sess); err != nil {
		t.Fatalf("create session: %v", err)
	}

	mock.ExpectQuery(`INSERT INTO "activity_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	if err := uc.Logout(ctx, 42, sess.ID); err != nil {
		t.Fatalf("unexpected error during logout: %v", err)
	}

	err := sessMgr.Validate(ctx, sess.ID, 42)
	if err == nil {
		t.Fatal("session should be revoked after logout")
	}
}

func TestSessions_LogoutAll_Success(t *testing.T) {
	uc, sessMgr, _, mock := newSessionsHarness(t)
	ctx := context.Background()

	now := time.Now()
	sess1 := auth.Session{
		ID:          "sess-1",
		UserID:      42,
		RefreshHash: "hash1",
		CreatedAt:   now,
		ExpiresAt:   now.Add(24 * time.Hour),
	}
	sess2 := auth.Session{
		ID:          "sess-2",
		UserID:      42,
		RefreshHash: "hash2",
		CreatedAt:   now,
		ExpiresAt:   now.Add(24 * time.Hour),
	}
	_ = sessMgr.Create(ctx, sess1)
	_ = sessMgr.Create(ctx, sess2)

	mock.ExpectQuery(`INSERT INTO "activity_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	if err := uc.LogoutAll(ctx, 42); err != nil {
		t.Fatalf("unexpected error during logout all: %v", err)
	}

	// Both sessions must fail validation due to revoked_before
	if err := sessMgr.Validate(ctx, sess1.ID, 42); err == nil {
		t.Fatal("sess1 should be revoked after logout-all")
	}
	if err := sessMgr.Validate(ctx, sess2.ID, 42); err == nil {
		t.Fatal("sess2 should be revoked after logout-all")
	}
}
