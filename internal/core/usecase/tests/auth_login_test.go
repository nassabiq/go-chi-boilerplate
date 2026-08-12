package tests

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-playground/validator/v10"
	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/repositories"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/usecase"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/database"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
	apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/helper"
	customvalidator "github.com/nassabiq/go-chi-boilerplate/internal/utils/validator"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const loginCorrectPassword = "correct-password"

func newLoginHarness(t *testing.T) (*usecase.BaseUsecase, *miniredis.Miniredis, sqlmock.Sqlmock) {
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
			LoginThrottle: config.LoginThrottleConfig{
				FailureWindow: time.Hour,
				FirstLimit:    5,
				FirstBlock:    30 * time.Second,
				SecondLimit:   7,
				SecondBlock:   5 * time.Minute,
				MaxLimit:      10,
				MaxBlock:      15 * time.Minute,
			},
			Password: config.PasswordConfig{
				BcryptCost: 4,
			},
		},
	}

	jwtSvc := auth.NewJWT(cfg)
	sessMgr := auth.NewSessionManager(client)
	throttle := auth.NewLoginThrottler(client, cfg.Auth.LoginThrottle)

	v := validator.New()
	_ = customvalidator.RegisterPasswordValidation(v)

	uc := usecase.New(&usecase.Dependencies{
		Repo:          repo,
		TxManager:     txMgr,
		Validator:     v,
		JWT:           jwtSvc,
		Session:       sessMgr,
		LoginThrottle: throttle,
		Config:        cfg,
	})

	return uc, mr, mock
}

var loginColumns = []string{
	"id", "name", "email", "password", "status_id", "version", "created_at", "updated_at", "deleted_at",
}

func loginUserRow(email, passwordHash string) []driver.Value {
	return []driver.Value{
		int64(1), "Alice", email, passwordHash, int16(enums.StatusActive), int64(1),
		time.Now(), time.Now(), nil,
	}
}

func rowsFor(row []driver.Value) *sqlmock.Rows {
	return sqlmock.NewRows(loginColumns).AddRow(row...)
}

// throttleFailKey mirrors auth.hashEmail + failKeyPrefix so tests can assert
// the per-account failure counter was recorded under the NORMALIZED email.
func throttleFailKey(email string) string {
	sum := sha256.Sum256([]byte(helper.NormalizeEmail(email)))
	return "auth:login:fail:" + hex.EncodeToString(sum[:])
}

// rawFailKey hashes the email WITHOUT normalization, to prove the throttle
// never keys on the raw, un-normalized request input.
func rawFailKey(email string) string {
	sum := sha256.Sum256([]byte(email))
	return "auth:login:fail:" + hex.EncodeToString(sum[:])
}

func expectFindUserByEmail(mock sqlmock.Sqlmock, email string, rows *sqlmock.Rows) {
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE email = \$1`).
		WithArgs(email, sqlmock.AnyArg()).
		WillReturnRows(rows)
}

// hashFor returns a bcrypt hash for the given plaintext using the same cost as
// production. It is only used to seed rows in tests.
func hashFor(t *testing.T, pw string) string {
	t.Helper()
	h, err := auth.HashPassword(pw, 12)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	return h
}

func TestLogin_ExistingUserCorrectPassword_Succeeds(t *testing.T) {
	uc, mr, mock := newLoginHarness(t)
	ctx := context.Background()

	expectFindUserByEmail(mock, "alice@example.com", rowsFor(loginUserRow("alice@example.com", hashFor(t, loginCorrectPassword))))
	// Expect roles/permissions queries (empty results)
	mock.ExpectQuery(`SELECT r\.\* FROM roles r JOIN user_roles ur ON ur\.role_id = r\.id WHERE ur\.user_id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "description", "created_at", "updated_at"}))
	mock.ExpectQuery(`SELECT DISTINCT p\.\* FROM permissions p JOIN role_permissions rp ON rp\.permission_id = p\.id JOIN user_roles ur ON ur\.role_id = rp\.role_id WHERE ur\.user_id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "description", "created_at"}))

	result, err := uc.Login(ctx, payloads.LoginRequest{Email: "alice@example.com", Password: loginCorrectPassword}, auth.SessionMeta{})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if result == nil || result.AccessToken == "" || result.RefreshToken == "" {
		t.Fatalf("expected tokens, got %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
	if mr.Exists(throttleFailKey("alice@example.com")) {
		t.Fatalf("throttle failure key must not exist after successful login")
	}
}

func TestLogin_ExistingUserWrongPassword_InvalidCredential(t *testing.T) {
	uc, mr, mock := newLoginHarness(t)
	ctx := context.Background()

	expectFindUserByEmail(mock, "alice@example.com", rowsFor(loginUserRow("alice@example.com", hashFor(t, loginCorrectPassword))))

	_, err := uc.Login(ctx, payloads.LoginRequest{Email: "alice@example.com", Password: "wrong-password"}, auth.SessionMeta{})
	assertCode(t, err, errconstant.ErrCodeAuthInvalidCredential)
	if err == nil {
		t.Fatalf("expected invalid credential, got %v", err)
	}
	// The generic error must map to the same HTTP status as the missing-user
	// path (no separate "wrong password" message/status).
	var ae *apperror.Error
	_ = errors.As(err, &ae)
	if ae.HTTPStatus != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password, got %d", ae.HTTPStatus)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
	// Failure recorded under the normalized email.
	if !mr.Exists(throttleFailKey("alice@example.com")) {
		t.Fatalf("throttle failure must be recorded for wrong password")
	}
}

func TestLogin_NonexistentUser_SameInvalidCredential(t *testing.T) {
	uc, mr, mock := newLoginHarness(t)
	ctx := context.Background()

	// Empty result set -> repository returns ErrCodeUserNotFound.
	expectFindUserByEmail(mock, "ghost@example.com", sqlmock.NewRows(loginColumns))

	_, err := uc.Login(ctx, payloads.LoginRequest{Email: "ghost@example.com", Password: "whatever"}, auth.SessionMeta{})
	assertCode(t, err, errconstant.ErrCodeAuthInvalidCredential)
	if err == nil {
		t.Fatalf("expected invalid credential, got %v", err)
	}
	var ae *apperror.Error
	_ = errors.As(err, &ae)
	if ae.HTTPStatus != http.StatusUnauthorized {
		t.Fatalf("expected 401 for nonexistent user, got %d", ae.HTTPStatus)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
	// Critical: the missing-user path performs dummy bcrypt work AND records a
	// failure. Because there is no user record, the only bcrypt executed is
	// the pre-generated dummy comparison; it did not panic and returned the
	// identical generic error.
	if !mr.Exists(throttleFailKey("ghost@example.com")) {
		t.Fatalf("throttle failure must be recorded for nonexistent email")
	}
}

func TestLogin_WrongPasswordAndMissingUser_SameErrorCode(t *testing.T) {
	uc, _, mockMissing := newLoginHarness(t)
	ctx := context.Background()

	// Missing user branch.
	expectFindUserByEmail(mockMissing, "ghost@example.com", sqlmock.NewRows(loginColumns))
	_, missingErr := uc.Login(ctx, payloads.LoginRequest{Email: "ghost@example.com", Password: "x"}, auth.SessionMeta{})
	assertCode(t, missingErr, errconstant.ErrCodeAuthInvalidCredential)

	// Wrong password branch reuses the same harness instance's repo mock is
	// consumed; build a fresh harness for the existing-user case.
	uc2, _, mockExisting := newLoginHarness(t)
	expectFindUserByEmail(mockExisting, "alice@example.com", rowsFor(loginUserRow("alice@example.com", hashFor(t, loginCorrectPassword))))
	_, wrongErr := uc2.Login(ctx, payloads.LoginRequest{Email: "alice@example.com", Password: "wrong"}, auth.SessionMeta{})
	assertCode(t, wrongErr, errconstant.ErrCodeAuthInvalidCredential)

	var missAE, wrongAE *apperror.Error
	_ = errors.As(missingErr, &missAE)
	_ = errors.As(wrongErr, &wrongAE)
	// The PUBLIC contract is identical: same error code and same HTTP status.
	// (The internal wrapped detail differs but is never sent to the client;
	// response.ErrorResponse returns only code + message.)
	if missAE.Code != wrongAE.Code {
		t.Fatalf("nonexistent vs wrong-password must share error code: missing=%s wrong=%s", missAE.Code, wrongAE.Code)
	}
	if missAE.HTTPStatus != wrongAE.HTTPStatus {
		t.Fatalf("nonexistent vs wrong-password must share HTTP status: missing=%d wrong=%d", missAE.HTTPStatus, wrongAE.HTTPStatus)
	}
}

func TestLogin_DatabaseError_Propagates(t *testing.T) {
	uc, _, mock := newLoginHarness(t)
	ctx := context.Background()

	// A non-"record not found" repository error must NOT collapse into an
	// invalid credential; it must keep its real (internal) classification.
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE email = \$1`).
		WithArgs("alice@example.com", sqlmock.AnyArg()).
		WillReturnError(sql.ErrConnDone)

	_, err := uc.Login(ctx, payloads.LoginRequest{Email: "alice@example.com", Password: "x"}, auth.SessionMeta{})
	if err == nil {
		t.Fatal("expected database error to propagate")
	}
	assertCode(t, err, errconstant.ErrCodeInternal)
	if ae := (&apperror.Error{}); errors.As(err, &ae) && ae.Code == errconstant.ErrCodeAuthInvalidCredential {
		t.Fatalf("database error must not become invalid credential: %v", err)
	}
}

func TestLogin_NonExistentEmail_NormalizedForThrottle(t *testing.T) {
	uc, mr, mock := newLoginHarness(t)
	ctx := context.Background()

	// Raw input with mixed case and surrounding whitespace.
	expectFindUserByEmail(mock, "user@example.com", sqlmock.NewRows(loginColumns))

	_, err := uc.Login(ctx, payloads.LoginRequest{Email: "  User@Example.COM ", Password: "x"}, auth.SessionMeta{})
	assertCode(t, err, errconstant.ErrCodeAuthInvalidCredential)

	// Throttle failure must be keyed on the NORMALIZED email, not the raw
	// input. The raw form (no normalization) must not be present.
	if !mr.Exists(throttleFailKey("user@example.com")) {
		t.Fatalf("throttle must be recorded under normalized email user@example.com")
	}
	if mr.Exists(rawFailKey("  User@Example.COM ")) {
		t.Fatalf("throttle must not be keyed on the raw, un-normalized email")
	}
}

func TestLogin_SuccessResetsPriorThrottleState(t *testing.T) {
	uc, mr, mock := newLoginHarness(t)
	ctx := context.Background()

	// Seed a prior failure so we can prove the Success path clears it.
	mr.Set(throttleFailKey("alice@example.com"), "3")

	expectFindUserByEmail(mock, "alice@example.com", rowsFor(loginUserRow("alice@example.com", hashFor(t, loginCorrectPassword))))
	// Expect roles/permissions queries (empty results)
	mock.ExpectQuery(`SELECT r\.\* FROM roles r JOIN user_roles ur ON ur\.role_id = r\.id WHERE ur\.user_id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "description", "created_at", "updated_at"}))
	mock.ExpectQuery(`SELECT DISTINCT p\.\* FROM permissions p JOIN role_permissions rp ON rp\.permission_id = p\.id JOIN user_roles ur ON ur\.role_id = rp\.role_id WHERE ur\.user_id = \$1`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "description", "created_at"}))

	if _, err := uc.Login(ctx, payloads.LoginRequest{Email: "alice@example.com", Password: loginCorrectPassword}, auth.SessionMeta{}); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if mr.Exists(throttleFailKey("alice@example.com")) {
		t.Fatalf("successful login must reset/clear throttle failure state")
	}
}

func TestLogin_StatusGating(t *testing.T) {
	tests := []struct {
		name         string
		status       enums.Status
		expectedCode errconstant.ErrorCode
	}{
		{"inactive", enums.StatusInactive, errconstant.ErrCodeAuthUserInactive},
		{"pending", enums.StatusPending, errconstant.ErrCodeAuthUserPending},
		{"suspended", enums.StatusSuspended, errconstant.ErrCodeAuthUserSuspended},
		{"blocked", enums.StatusBlocked, errconstant.ErrCodeAuthUserBlocked},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uc, _, mock := newLoginHarness(t)
			ctx := context.Background()

			row := []driver.Value{
				int64(1), "Alice", "alice@example.com", hashFor(t, loginCorrectPassword), int16(tc.status), int64(1),
				time.Now(), time.Now(), nil,
			}
			expectFindUserByEmail(mock, "alice@example.com", rowsFor(row))

			_, err := uc.Login(ctx, payloads.LoginRequest{
				Email:    "alice@example.com",
				Password: loginCorrectPassword,
			}, auth.SessionMeta{})

			if err == nil {
				t.Fatalf("expected error for status %s, got nil", tc.name)
			}
			assertCode(t, err, tc.expectedCode)
		})
	}
}

func TestLogin_Throttled_ReturnsError(t *testing.T) {
	uc, mr, _ := newLoginHarness(t)
	ctx := context.Background()

	email := "victim@example.com"
	sum := sha256.Sum256([]byte(email))
	blockKey := "auth:login:block:" + hex.EncodeToString(sum[:])
	// Simulate lockout state in Redis with active TTL
	mr.Set(blockKey, "1")
	mr.SetTTL(blockKey, 10*time.Minute)

	_, err := uc.Login(ctx, payloads.LoginRequest{
		Email:    email,
		Password: "any-password",
	}, auth.SessionMeta{})

	if err == nil {
		t.Fatal("expected login to be throttled, got nil error")
	}
	assertCode(t, err, errconstant.ErrCodeAuthLoginThrottled)
}
