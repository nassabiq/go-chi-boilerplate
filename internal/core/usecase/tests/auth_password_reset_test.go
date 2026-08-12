package tests

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-playground/validator/v10"
	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/entities"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/repositories"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/usecase"
	"github.com/nassabiq/go-chi-boilerplate/internal/messaging"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/database"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/ratelimit"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	customvalidator "github.com/nassabiq/go-chi-boilerplate/internal/utils/validator"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// fakePublisher is an in-memory EventPublisher used to assert that the usecase
// attempts to publish the right domain events without requiring a live NATS.
type fakePublisher struct {
	mu             sync.Mutex
	resetRequested []messaging.PasswordResetRequested
	changed        []messaging.PasswordChanged
	verification   []messaging.EmailVerificationRequested
	emailChange    []messaging.EmailChangeRequested
	resetErr       error
	changedErr     error
}

func (f *fakePublisher) PublishPasswordResetRequested(_ context.Context, e messaging.PasswordResetRequested) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.resetErr != nil {
		return f.resetErr
	}
	f.resetRequested = append(f.resetRequested, e)
	return nil
}

func (f *fakePublisher) PublishPasswordChanged(_ context.Context, e messaging.PasswordChanged) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.changedErr != nil {
		return f.changedErr
	}
	f.changed = append(f.changed, e)
	return nil
}

func (f *fakePublisher) PublishEmailVerificationRequested(_ context.Context, e messaging.EmailVerificationRequested) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.verification = append(f.verification, e)
	return nil
}

func (f *fakePublisher) PublishEmailChangeRequested(_ context.Context, e messaging.EmailChangeRequested) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.emailChange = append(f.emailChange, e)
	return nil
}

func (f *fakePublisher) resetEvents() []messaging.PasswordResetRequested {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]messaging.PasswordResetRequested, len(f.resetRequested))
	copy(out, f.resetRequested)
	return out
}

// newPasswordResetHarness builds a usecase backed by sqlmock and a Redis
// client that never connects. Tests that do not exercise session storage use
// this so they run without a bindable TCP port (miniredis).
func newPasswordResetHarness(t *testing.T) (*usecase.BaseUsecase, *fakePublisher, sqlmock.Sqlmock) {
	t.Helper()

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

	// Working in-memory Redis for the rate limiter (per-account throttle on
	// forgot/resend password). The session manager intentionally stays on a
	// dead client: only the reset-password flow's best-effort revocation
	// touches it, and that is covered by the redis-backed variant below.
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	limitClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = limitClient.Close() })

	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Forgot: config.RateLimitRule{Requests: 1000, Window: time.Hour},
		},
		Auth: config.AuthConfig{
			JWT: config.JWTConfig{
				AccessSecret:  "access-secret",
				RefreshSecret: "refresh-secret",
				Issuer:        "go-chi",
				AccessExpiry:  15 * time.Minute,
				RefreshExpiry: 168 * time.Hour,
			},
			PasswordReset: config.PasswordResetConfig{
				TokenTTL: 15 * time.Minute,
				URL:      "http://localhost/reset",
			},
		},
	}

	jwtSvc := auth.NewJWT(cfg)
	// Dead Redis client: the session manager is only touched by the
	// reset-password flow's best-effort revocation, which is covered by the
	// redis-backed variant below.
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = client.Close() })
	sessMgr := auth.NewSessionManager(client)
	pub := &fakePublisher{}

	v := validator.New()
	_ = customvalidator.RegisterPasswordValidation(v)

	uc := usecase.New(&usecase.Dependencies{
		Repo:      repo,
		TxManager: txMgr,
		Validator: v,
		JWT:       jwtSvc,
		Session:   sessMgr,
		Limiter:   ratelimit.New(limitClient),
		Config:    cfg,
		Publisher: pub,
	})

	return uc, pub, mock
}

// newPasswordResetHarnessRedis is like newPasswordResetHarness but uses a real
// in-memory Redis so session revocation can be asserted. It skips when a
// bindable port is unavailable (e.g. restricted sandboxes).
func newPasswordResetHarnessRedis(t *testing.T) (*usecase.BaseUsecase, *fakePublisher, sqlmock.Sqlmock, *miniredis.Miniredis) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Skipf("miniredis unavailable: %v", err)
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
				AccessSecret:  "access-secret",
				RefreshSecret: "refresh-secret",
				Issuer:        "go-chi",
				AccessExpiry:  15 * time.Minute,
				RefreshExpiry: 168 * time.Hour,
			},
			PasswordReset: config.PasswordResetConfig{
				TokenTTL: 15 * time.Minute,
				URL:      "http://localhost/reset",
			},
		},
	}

	jwtSvc := auth.NewJWT(cfg)
	sessMgr := auth.NewSessionManager(client)
	pub := &fakePublisher{}

	v := validator.New()
	_ = customvalidator.RegisterPasswordValidation(v)

	uc := usecase.New(&usecase.Dependencies{
		Repo:      repo,
		TxManager: txMgr,
		Validator: v,
		JWT:       jwtSvc,
		Session:   sessMgr,
		Config:    cfg,
		Publisher: pub,
	})

	return uc, pub, mock, mr
}

func userRowForReset(id int64, email, name, passwordHash string) []driver.Value {
	return []driver.Value{id, name, email, passwordHash, int16(1), int64(1), time.Now(), time.Now(), nil}
}

func resetTokenRow(id int64, userID int64, tokenHash string, expiresAt time.Time, usedAt sql.NullTime) []driver.Value {
	return []driver.Value{id, userID, tokenHash, expiresAt, usedAt, nil, nil, time.Now()}
}

func TestForgotPassword_Unregistered_IsGenericAndQuiet(t *testing.T) {
	uc, pub, mock := newPasswordResetHarness(t)
	ctx := context.Background()

	expectFindUserByEmail(mock, "ghost@example.com", sqlmock.NewRows(loginColumns))

	if err := uc.ForgotPassword(ctx, payloads.ForgotPasswordRequest{Email: "ghost@example.com"}, auth.SessionMeta{}); err != nil {
		t.Fatalf("expected nil (generic success), got %v", err)
	}
	if len(pub.resetEvents()) != 0 {
		t.Fatalf("must not publish for unregistered email")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestForgotPassword_Registered_CreatesRowAndPublishes(t *testing.T) {
	uc, pub, mock := newPasswordResetHarness(t)
	ctx := context.Background()

	expectFindUserByEmail(mock, "alice@example.com", rowsFor(userRowForReset(1, "alice@example.com", "Alice", "hash")))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "password_reset_tokens"`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`INSERT INTO "password_reset_tokens"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectCommit()

	if err := uc.ForgotPassword(ctx, payloads.ForgotPasswordRequest{Email: "alice@example.com"}, auth.SessionMeta{IPAddress: "1.2.3.4", UserAgent: "ua"}); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	events := pub.resetEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 reset event, got %d", len(events))
	}
	e := events[0]
	if e.UserID != 1 || e.Email != "alice@example.com" || e.Name != "Alice" {
		t.Fatalf("unexpected event identity: %+v", e)
	}
	if e.EventID == "" {
		t.Fatalf("event id must be set")
	}
	if e.ResetURL == "" || e.ResetURL[:len("http://localhost/reset/")] != "http://localhost/reset/" {
		t.Fatalf("unexpected reset url: %q", e.ResetURL)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestForgotPassword_PublishFailureStaysGeneric(t *testing.T) {
	uc, pub, mock := newPasswordResetHarness(t)
	ctx := context.Background()

	expectFindUserByEmail(mock, "alice@example.com", rowsFor(userRowForReset(1, "alice@example.com", "Alice", "hash")))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "password_reset_tokens"`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`INSERT INTO "password_reset_tokens"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectCommit()

	pub.resetErr = errors.New("nats down")

	if err := uc.ForgotPassword(ctx, payloads.ForgotPasswordRequest{Email: "alice@example.com"}, auth.SessionMeta{}); err != nil {
		t.Fatalf("publish failure must not break the generic response: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestResetPassword_UnknownToken_IsGenericError(t *testing.T) {
	uc, _, mock := newPasswordResetHarness(t)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "password_reset_tokens" WHERE token_hash = \$1`).
		WillReturnRows(sqlmock.NewRows(resetTokenColumns))
	mock.ExpectRollback()

	err := uc.ResetPassword(ctx, payloads.ResetPasswordRequest{Token: "does-not-exist", NewPassword: "NewPassword123!"})
	assertCode(t, err, errconstant.ErrCodeAuthInvalidResetToken)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestResetPassword_UsedToken_IsSameError(t *testing.T) {
	uc, _, mock := newPasswordResetHarness(t)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "password_reset_tokens" WHERE token_hash = \$1`).
		WillReturnRows(sqlmock.NewRows(resetTokenColumns).AddRow(resetTokenRow(1, 1, "h", time.Now().Add(time.Hour), sql.NullTime{Valid: true, Time: time.Now()})...))
	mock.ExpectRollback()

	err := uc.ResetPassword(ctx, payloads.ResetPasswordRequest{Token: "used-token", NewPassword: "NewPassword123!"})
	assertCode(t, err, errconstant.ErrCodeAuthInvalidResetToken)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestResetPassword_ExpiredToken_IsSameError(t *testing.T) {
	uc, _, mock := newPasswordResetHarness(t)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "password_reset_tokens" WHERE token_hash = \$1`).
		WillReturnRows(sqlmock.NewRows(resetTokenColumns).AddRow(resetTokenRow(1, 1, "h", time.Now().Add(-time.Hour), sql.NullTime{})...))
	mock.ExpectRollback()

	err := uc.ResetPassword(ctx, payloads.ResetPasswordRequest{Token: "expired-token", NewPassword: "NewPassword123!"})
	assertCode(t, err, errconstant.ErrCodeAuthInvalidResetToken)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestResetPassword_ValidToken_UpdatesPasswordAndRevokesSessions(t *testing.T) {
	uc, _, mock, mr := newPasswordResetHarnessRedis(t)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "password_reset_tokens" WHERE token_hash = \$1`).
		WillReturnRows(sqlmock.NewRows(resetTokenColumns).AddRow(resetTokenRow(1, 9, "h", time.Now().Add(time.Hour), sql.NullTime{})...))
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WillReturnRows(rowsFor(userRowForReset(9, "bob@example.com", "Bob", "oldhash")))
	mock.ExpectExec(`UPDATE "users"`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE "password_reset_tokens"`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := uc.ResetPassword(ctx, payloads.ResetPasswordRequest{Token: "valid-token", NewPassword: "NewPassword123!"}); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !mr.Exists("auth:user:revoked_before:9") {
		t.Fatalf("expected sessions to be revoked (revoked_before marker set)")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestResetPassword_SamePassword_Rejected(t *testing.T) {
	uc, _, mock := newPasswordResetHarness(t)
	ctx := context.Background()

	currentPasswordHash, err := auth.HashPassword("SamePassword123!", 4)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "password_reset_tokens" WHERE token_hash = \$1`).
		WillReturnRows(sqlmock.NewRows(resetTokenColumns).AddRow(resetTokenRow(1, 9, "h", time.Now().Add(time.Hour), sql.NullTime{})...))
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WillReturnRows(rowsFor(userRowForReset(9, "bob@example.com", "Bob", currentPasswordHash)))
	mock.ExpectRollback()

	err = uc.ResetPassword(ctx, payloads.ResetPasswordRequest{Token: "valid-token", NewPassword: "SamePassword123!"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != errconstant.ErrCodeAuthSamePassword {
		t.Fatalf("expected ErrCodeAuthSamePassword, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

// TestResetPassword_ValidToken_PublishesPasswordChangedEvent locks in the
// requirement that a successful reset emits a security notification (reason
// reset_password) so the user is alerted even if they triggered it.
func TestResetPassword_ValidToken_PublishesPasswordChangedEvent(t *testing.T) {
	uc, pub, mock, _ := newPasswordResetHarnessRedis(t)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "password_reset_tokens" WHERE token_hash = \$1`).
		WillReturnRows(sqlmock.NewRows(resetTokenColumns).AddRow(resetTokenRow(1, 9, "h", time.Now().Add(time.Hour), sql.NullTime{})...))
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WillReturnRows(rowsFor(userRowForReset(9, "bob@example.com", "Bob", "oldhash")))
	mock.ExpectExec(`UPDATE "users"`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE "password_reset_tokens"`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := uc.ResetPassword(ctx, payloads.ResetPasswordRequest{Token: "valid-token", NewPassword: "NewPassword123!"}); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(pub.changed) != 1 {
		t.Fatalf("expected one password-changed event, got %d", len(pub.changed))
	}
	if pub.changed[0].Reason != "reset_password" {
		t.Fatalf("expected reason reset_password, got %q", pub.changed[0].Reason)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestChangePassword_WrongCurrent_Fails(t *testing.T) {
	uc, _, mock := newPasswordResetHarness(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1 AND deleted_at IS NULL AND "users"\."deleted_at" IS NULL ORDER BY "users"\."id" LIMIT \$2`).
		WillReturnRows(rowsFor(userRowForReset(1, "a@b.com", "A", hashFor(t, "CurrentPass123!"))))

	err := uc.ChangePassword(ctx, 1, payloads.ChangePasswordRequest{CurrentPassword: "wrong", NewPassword: "NewPassword123!"})
	assertCode(t, err, errconstant.ErrCodeAuthInvalidCurrentPassword)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestChangePassword_SameAsCurrent_Fails(t *testing.T) {
	uc, _, mock := newPasswordResetHarness(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1 AND deleted_at IS NULL AND "users"\."deleted_at" IS NULL ORDER BY "users"\."id" LIMIT \$2`).
		WillReturnRows(rowsFor(userRowForReset(1, "a@b.com", "A", hashFor(t, "CurrentPass123!"))))

	err := uc.ChangePassword(ctx, 1, payloads.ChangePasswordRequest{CurrentPassword: "CurrentPass123!", NewPassword: "CurrentPass123!"})
	assertCode(t, err, errconstant.ErrCodeAuthSamePassword)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestChangePassword_Success_UpdatesPassword(t *testing.T) {
	uc, pub, mock := newPasswordResetHarness(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1 AND deleted_at IS NULL AND "users"\."deleted_at" IS NULL ORDER BY "users"\."id" LIMIT \$2`).
		WillReturnRows(rowsFor(userRowForReset(1, "a@b.com", "A", hashFor(t, "CurrentPass123!"))))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users"`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := uc.ChangePassword(ctx, 1, payloads.ChangePasswordRequest{CurrentPassword: "CurrentPass123!", NewPassword: "NewPassword123!"}); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
	if len(pub.changed) != 1 {
		t.Fatalf("expected 1 password-changed event, got %d", len(pub.changed))
	}
}

func TestChangePassword_RevokesSessionsAndPublishesEvent(t *testing.T) {
	uc, pub, mock, mr := newPasswordResetHarnessRedis(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1 AND deleted_at IS NULL AND "users"\."deleted_at" IS NULL ORDER BY "users"\."id" LIMIT \$2`).
		WillReturnRows(rowsFor(userRowForReset(1, "a@b.com", "A", hashFor(t, "CurrentPass123!"))))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users"`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := uc.ChangePassword(ctx, 1, payloads.ChangePasswordRequest{CurrentPassword: "CurrentPass123!", NewPassword: "NewPassword123!"}); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
	if len(pub.changed) != 1 {
		t.Fatalf("expected 1 password-changed event, got %d", len(pub.changed))
	}
	if pub.changed[0].Reason != "change_password" {
		t.Errorf("expected reason change_password, got %s", pub.changed[0].Reason)
	}
	if !mr.Exists("auth:user:revoked_before:1") {
		t.Fatal("expected user sessions to be revoked via revoked_before marker")
	}
}

var resetTokenColumns = []string{
	"id", "user_id", "token_hash", "expires_at", "used_at", "requested_ip", "user_agent", "created_at",
}

var _ = entities.PasswordResetToken{}
