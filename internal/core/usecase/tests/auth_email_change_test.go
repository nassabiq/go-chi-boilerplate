package tests

import (
	"context"
	"database/sql/driver"
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
	customvalidator "github.com/nassabiq/go-chi-boilerplate/internal/utils/validator"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newEmailChangeHarness(t *testing.T) (*usecase.BaseUsecase, sqlmock.Sqlmock, *stubPublisher, *miniredis.Miniredis) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
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
	pub := &stubPublisher{}

	jwtSvc := auth.NewJWT(&config.Config{
		Auth: config.AuthConfig{
			JWT: config.JWTConfig{
				AccessSecret:  "access-secret",
				RefreshSecret: "refresh-secret",
				Issuer:        "go-chi",
				RefreshExpiry: 168 * time.Hour,
			},
		},
	})
	sessMgr := auth.NewSessionManager(client)

	v := validator.New()
	_ = customvalidator.RegisterPasswordValidation(v)

	uc := usecase.New(&usecase.Dependencies{
		Repo:      repo,
		TxManager: txMgr,
		Validator: v,
		JWT:       jwtSvc,
		Session:   sessMgr,
		Config: &config.Config{
			Auth: config.AuthConfig{
				EmailChange: config.EmailChangeConfig{
					TokenTTL: 24 * time.Hour,
					URL:      "https://app.example.com/change-email",
				},
			},
		},
		Publisher: pub,
	})

	return uc, mock, pub, mr
}

func emailChangeUserRow(t *testing.T, email string) []driver.Value {
	return []driver.Value{
		int64(1), "Natsu", email, hashFor(t, "CurrentPass123!"), int16(enums.StatusActive), int64(1),
		time.Now(), time.Now(), nil,
	}
}

func emailChangeColumns() []string {
	return []string{"id", "user_id", "new_email", "token_hash", "expires_at", "used_at", "requested_ip", "user_agent", "created_at"}
}

func TestChangeEmail_Valid_RequestsAndPublishes(t *testing.T) {
	uc, mock, pub, _ := newEmailChangeHarness(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows(loginColumns).AddRow(emailChangeUserRow(t, "old@example.com")...))
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE email = \$1`).
		WillReturnRows(sqlmock.NewRows(loginColumns))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "email_change_requests" SET`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`INSERT INTO "email_change_requests"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(99)))
	mock.ExpectCommit()

	err := uc.RequestEmailChange(ctx, 1, payloads.ChangeEmailRequest{Password: "CurrentPass123!", NewEmail: "new@example.com"}, auth.SessionMeta{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pub.EmailChangeEvents) != 1 {
		t.Fatalf("expected one email-change event, got %d", len(pub.EmailChangeEvents))
	}
	if pub.EmailChangeEvents[0].NewEmail != "new@example.com" || pub.EmailChangeEvents[0].OldEmail != "old@example.com" {
		t.Fatalf("event emails wrong: %+v", pub.EmailChangeEvents[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestChangeEmail_WrongPassword(t *testing.T) {
	uc, mock, _, _ := newEmailChangeHarness(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows(loginColumns).AddRow(emailChangeUserRow(t, "old@example.com")...))

	err := uc.RequestEmailChange(ctx, 1, payloads.ChangeEmailRequest{Password: "wrong", NewEmail: "new@example.com"}, auth.SessionMeta{})
	assertCode(t, err, errconstant.ErrCodeAuthInvalidCurrentPassword)
}

func TestChangeEmail_SameEmail(t *testing.T) {
	uc, mock, _, _ := newEmailChangeHarness(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows(loginColumns).AddRow(emailChangeUserRow(t, "old@example.com")...))

	err := uc.RequestEmailChange(ctx, 1, payloads.ChangeEmailRequest{Password: "CurrentPass123!", NewEmail: "OLD@example.com"}, auth.SessionMeta{})
	assertCode(t, err, errconstant.ErrCodeAuthEmailUnchanged)
}

func TestChangeEmail_TakenEmail(t *testing.T) {
	uc, mock, _, _ := newEmailChangeHarness(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows(loginColumns).AddRow(emailChangeUserRow(t, "old@example.com")...))
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE email = \$1`).
		WillReturnRows(sqlmock.NewRows(loginColumns).AddRow(emailChangeUserRow(t, "new@example.com")...))

	err := uc.RequestEmailChange(ctx, 1, payloads.ChangeEmailRequest{Password: "CurrentPass123!", NewEmail: "new@example.com"}, auth.SessionMeta{})
	assertCode(t, err, errconstant.ErrCodeUserAlreadyExists)
}

func TestVerifyEmailChange_Valid_UpdatesEmail(t *testing.T) {
	uc, mock, _, _ := newEmailChangeHarness(t)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "email_change_requests"`).
		WillReturnRows(sqlmock.NewRows(emailChangeColumns()).AddRow([]driver.Value{
			int64(99), int64(1), "new@example.com", "hashabc", time.Now().Add(time.Hour), nil, nil, nil, time.Now(),
		}...))
	mock.ExpectExec(`UPDATE "email_change_requests" SET`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "users" SET`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := uc.VerifyEmailChange(ctx, payloads.VerifyEmailChangeRequest{Token: "rawtok"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestVerifyEmailChange_InvalidToken(t *testing.T) {
	uc, mock, _, _ := newEmailChangeHarness(t)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "email_change_requests"`).
		WillReturnRows(sqlmock.NewRows(emailChangeColumns()).AddRow([]driver.Value{
			int64(99), int64(1), "new@example.com", "hashabc", time.Now().Add(time.Hour), time.Now(), nil, nil, time.Now(),
		}...))
	mock.ExpectRollback()

	err := uc.VerifyEmailChange(ctx, payloads.VerifyEmailChangeRequest{Token: "rawtok"})
	assertCode(t, err, errconstant.ErrCodeAuthInvalidEmailChangeToken)
}

func TestVerifyEmailChange_ExpiredToken(t *testing.T) {
	uc, mock, _, _ := newEmailChangeHarness(t)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "email_change_requests"`).
		WillReturnRows(sqlmock.NewRows(emailChangeColumns()).AddRow([]driver.Value{
			int64(99), int64(1), "new@example.com", "hashabc", time.Now().Add(-time.Hour), nil, nil, nil, time.Now(),
		}...))
	mock.ExpectRollback()

	err := uc.VerifyEmailChange(ctx, payloads.VerifyEmailChangeRequest{Token: "rawtok"})
	assertCode(t, err, errconstant.ErrCodeAuthInvalidEmailChangeToken)
}
