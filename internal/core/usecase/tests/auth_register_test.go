package tests

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/repositories"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/usecase"
	"github.com/nassabiq/go-chi-boilerplate/internal/messaging"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/database"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
	customvalidator "github.com/nassabiq/go-chi-boilerplate/internal/utils/validator"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// stubPublisher records published events for assertions; it never touches NATS.
type stubPublisher struct {
	VerificationEvents []messaging.EmailVerificationRequested
	EmailChangeEvents  []messaging.EmailChangeRequested
}

func (s *stubPublisher) PublishPasswordResetRequested(context.Context, messaging.PasswordResetRequested) error {
	return nil
}

func (s *stubPublisher) PublishPasswordChanged(context.Context, messaging.PasswordChanged) error {
	return nil
}

func (s *stubPublisher) PublishEmailVerificationRequested(_ context.Context, event messaging.EmailVerificationRequested) error {
	s.VerificationEvents = append(s.VerificationEvents, event)
	return nil
}

func (s *stubPublisher) PublishEmailChangeRequested(_ context.Context, event messaging.EmailChangeRequested) error {
	s.EmailChangeEvents = append(s.EmailChangeEvents, event)
	return nil
}

func TestRegister_Success_PendingStatus(t *testing.T) {
	uc, mock, pub := newRegisterHarness(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "users"`).WithArgs(args(9)...).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectCommit()

	// requestEmailVerification: supersede pending tokens, then persist new one.
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "email_verification_tokens" SET`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`INSERT INTO "email_verification_tokens"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(99)))
	mock.ExpectCommit()

	user, err := uc.Register(t.Context(), payloads.RegisterRequest{
		Name:     "Natsu",
		Email:    "  User@Example.COM ",
		Password: "Sup3r!secret",
	}, auth.SessionMeta{IPAddress: "127.0.0.1", UserAgent: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != 7 {
		t.Fatalf("expected id 7, got %d", user.ID)
	}
	if user.Email != "user@example.com" {
		t.Fatalf("expected normalized email, got %q", user.Email)
	}
	if user.StatusID != int16(enums.StatusPending) {
		t.Fatalf("expected pending status, got %d", user.StatusID)
	}
	if user.Version != 1 {
		t.Fatalf("expected initial version 1, got %d", user.Version)
	}
	if len(pub.VerificationEvents) != 1 {
		t.Fatalf("expected one verification event, got %d", len(pub.VerificationEvents))
	}
	if pub.VerificationEvents[0].Email != "user@example.com" {
		t.Fatalf("event email not normalized: %q", pub.VerificationEvents[0].Email)
	}
	if pub.VerificationEvents[0].VerificationURL == "" {
		t.Fatal("expected verification url in event")
	}
}

func TestRegister_DuplicateEmail_UserAlreadyExists(t *testing.T) {
	uc, mock, _ := newRegisterHarness(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "users"`).
		WillReturnError(&pgconn.PgError{Code: "23505", Message: "duplicate"})
	mock.ExpectRollback()

	_, err := uc.Register(t.Context(), payloads.RegisterRequest{
		Name:     "Natsu",
		Email:    "natsu@example.com",
		Password: "Sup3r!secret",
	}, auth.SessionMeta{})
	if err == nil {
		t.Fatal("expected duplicate error, got nil")
	}
	assertCode(t, err, errconstant.ErrCodeUserAlreadyExists)
}

func TestRegister_ValidationError(t *testing.T) {
	uc, _, _ := newRegisterHarness(t)

	// Weak password (below min=8) and empty name.
	_, err := uc.Register(t.Context(), payloads.RegisterRequest{
		Name:     "",
		Email:    "bad",
		Password: "short",
	}, auth.SessionMeta{})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	assertCode(t, err, errconstant.ErrCodeValidation)
}

func TestRegister_InvalidEmail(t *testing.T) {
	uc, _, _ := newRegisterHarness(t)

	invalidEmails := []string{
		"notanemail",
		"@example.com",
		"user@",
		"user@.com",
		"",
	}

	for _, email := range invalidEmails {
		t.Run("email="+email, func(t *testing.T) {
			_, err := uc.Register(t.Context(), payloads.RegisterRequest{
				Name:     "Test",
				Email:    email,
				Password: "ValidPassword123!",
			}, auth.SessionMeta{})
			if err == nil {
				t.Fatalf("expected validation error for email %q, got nil", email)
			}
			assertCode(t, err, errconstant.ErrCodeValidation)
		})
	}
}

func TestRegister_WeakPassword_Failures(t *testing.T) {
	uc, _, _ := newRegisterHarness(t)

	tests := []struct {
		name     string
		password string
	}{
		{"too_short", "Aa1!abc"},
		{"too_long", strings.Repeat("A", 70) + "a1!"},
		{"no_uppercase", "password123!"},
		{"no_lowercase", "PASSWORD123!"},
		{"no_digit", "PasswordHere!"},
		{"no_special", "Password1234"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uc.Register(t.Context(), payloads.RegisterRequest{
				Name:     "Test",
				Email:    "test@example.com",
				Password: tc.password,
			}, auth.SessionMeta{})
			if err == nil {
				t.Fatalf("expected validation error for password %q (%s), got nil", tc.password, tc.name)
			}
			assertCode(t, err, errconstant.ErrCodeValidation)
		})
	}
}

func TestRegister_PasswordHashing(t *testing.T) {
	uc, mock, _ := newRegisterHarness(t)

	rawPassword := "SecureP@ssw0rd!"

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "users"`).
		WithArgs(
			"Lucy",
			"lucy@example.com",
			sqlmock.AnyArg(), // password hash
			nil,              // avatar_url
			int16(enums.StatusPending),
			int64(0),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			nil,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(10)))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "email_verification_tokens" SET`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`INSERT INTO "email_verification_tokens"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(101)))
	mock.ExpectCommit()

	user, err := uc.Register(t.Context(), payloads.RegisterRequest{
		Name:     "Lucy",
		Email:    "lucy@example.com",
		Password: rawPassword,
	}, auth.SessionMeta{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != 10 {
		t.Fatalf("expected user id 10, got %d", user.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock expectations unmet: %v", err)
	}
}

// newRegisterHarness builds a usecase with the config + publisher that Register
// needs to issue and publish an email verification token.
func newRegisterHarness(t *testing.T) (*usecase.BaseUsecase, sqlmock.Sqlmock, *stubPublisher) {
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
	pub := &stubPublisher{}
	v := validator.New()
	_ = customvalidator.RegisterPasswordValidation(v)
	uc := usecase.New(&usecase.Dependencies{
		Repo:      repo,
		TxManager: txMgr,
		Validator: v,
		Config: &config.Config{
			Auth: config.AuthConfig{
				Password: config.PasswordConfig{
					BcryptCost: 4,
				},
				EmailVerification: config.EmailVerificationConfig{
					TokenTTL: 24 * time.Hour,
					URL:      "https://app.example.com/verify",
				},
			},
		},
		Publisher: pub,
	})

	return uc, mock, pub
}
