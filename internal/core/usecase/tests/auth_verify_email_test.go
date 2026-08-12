package tests

import (
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
)

func verifyTokenColumns() []string {
	return []string{"id", "user_id", "token_hash", "expires_at", "used_at", "requested_ip", "user_agent", "created_at"}
}

func verifyTokenRow(expiresAt time.Time, usedAt interface{}) []driver.Value {
	return []driver.Value{int64(99), int64(1), "hashabc", expiresAt, usedAt, nil, nil, time.Now()}
}

func verifyUserRow(status enums.Status) []driver.Value {
	return []driver.Value{
		int64(1), "Natsu", "natsu@example.com", "hashed", int16(status), int64(1),
		time.Now(), time.Now(), nil,
	}
}

func TestVerifyEmail_ValidToken_ActivatesUser(t *testing.T) {
	uc, mock := newUsecase(t)
	ctx := t.Context()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "email_verification_tokens"`).
		WillReturnRows(sqlmock.NewRows(verifyTokenColumns()).AddRow(verifyTokenRow(time.Now().Add(time.Hour), nil)...))
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows(loginColumns).AddRow(verifyUserRow(enums.StatusPending)...))
	mock.ExpectExec(`UPDATE "email_verification_tokens" SET`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "users" SET`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`INSERT INTO "activity_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectCommit()

	if err := uc.VerifyEmail(ctx, payloads.VerifyEmailRequest{Token: "rawtok"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestVerifyEmail_ExpiredToken(t *testing.T) {
	uc, mock := newUsecase(t)
	ctx := t.Context()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "email_verification_tokens"`).
		WillReturnRows(sqlmock.NewRows(verifyTokenColumns()).AddRow(verifyTokenRow(time.Now().Add(-time.Hour), nil)...))
	mock.ExpectRollback()

	err := uc.VerifyEmail(ctx, payloads.VerifyEmailRequest{Token: "rawtok"})
	assertCode(t, err, errconstant.ErrCodeAuthInvalidVerificationToken)
}

func TestVerifyEmail_AlreadyUsedToken(t *testing.T) {
	uc, mock := newUsecase(t)
	ctx := t.Context()

	used := sql.NullTime{Valid: true, Time: time.Now()}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "email_verification_tokens"`).
		WillReturnRows(sqlmock.NewRows(verifyTokenColumns()).AddRow(verifyTokenRow(time.Now().Add(time.Hour), used)...))
	mock.ExpectRollback()

	err := uc.VerifyEmail(ctx, payloads.VerifyEmailRequest{Token: "rawtok"})
	assertCode(t, err, errconstant.ErrCodeAuthInvalidVerificationToken)
}

func TestVerifyEmail_BlockedUser_Rejected(t *testing.T) {
	uc, mock := newUsecase(t)
	ctx := t.Context()

	// Valid, unexpired token but the account is BLOCKED: must NOT be activated.
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "email_verification_tokens"`).
		WillReturnRows(sqlmock.NewRows(verifyTokenColumns()).AddRow(verifyTokenRow(time.Now().Add(time.Hour), nil)...))
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows(loginColumns).AddRow(verifyUserRow(enums.StatusBlocked)...))
	mock.ExpectRollback()

	err := uc.VerifyEmail(ctx, payloads.VerifyEmailRequest{Token: "rawtok"})
	assertCode(t, err, errconstant.ErrCodeAuthInvalidVerificationToken)
}

func TestVerifyEmail_SuspendedUser_Rejected(t *testing.T) {
	uc, mock := newUsecase(t)
	ctx := t.Context()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "email_verification_tokens"`).
		WillReturnRows(sqlmock.NewRows(verifyTokenColumns()).AddRow(verifyTokenRow(time.Now().Add(time.Hour), nil)...))
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows(loginColumns).AddRow(verifyUserRow(enums.StatusSuspended)...))
	mock.ExpectRollback()

	err := uc.VerifyEmail(ctx, payloads.VerifyEmailRequest{Token: "rawtok"})
	assertCode(t, err, errconstant.ErrCodeAuthInvalidVerificationToken)
}

func TestVerifyEmail_InactiveUser_Rejected(t *testing.T) {
	uc, mock := newUsecase(t)
	ctx := t.Context()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "email_verification_tokens"`).
		WillReturnRows(sqlmock.NewRows(verifyTokenColumns()).AddRow(verifyTokenRow(time.Now().Add(time.Hour), nil)...))
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows(loginColumns).AddRow(verifyUserRow(enums.StatusInactive)...))
	mock.ExpectRollback()

	err := uc.VerifyEmail(ctx, payloads.VerifyEmailRequest{Token: "rawtok"})
	assertCode(t, err, errconstant.ErrCodeAuthInvalidVerificationToken)
}

func TestVerifyEmail_UnknownToken_Rejected(t *testing.T) {
	uc, mock := newUsecase(t)
	ctx := t.Context()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "email_verification_tokens"`).
		WillReturnRows(sqlmock.NewRows(verifyTokenColumns()))
	mock.ExpectRollback()

	err := uc.VerifyEmail(ctx, payloads.VerifyEmailRequest{Token: "nonexistent-token"})
	assertCode(t, err, errconstant.ErrCodeAuthInvalidVerificationToken)
}

func TestVerifyEmail_AlreadyActiveUser_IdempotentSuccess(t *testing.T) {
	uc, mock := newUsecase(t)
	ctx := t.Context()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "email_verification_tokens"`).
		WillReturnRows(sqlmock.NewRows(verifyTokenColumns()).AddRow(verifyTokenRow(time.Now().Add(time.Hour), nil)...))
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows(loginColumns).AddRow(verifyUserRow(enums.StatusActive)...))
	mock.ExpectExec(`UPDATE "email_verification_tokens" SET`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// Notice: no UPDATE "users" because status is already ACTIVE.
	mock.ExpectCommit()

	if err := uc.VerifyEmail(ctx, payloads.VerifyEmailRequest{Token: "rawtok"}); err != nil {
		t.Fatalf("expected success for already-active user, got: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestVerifyEmail_DatabaseError_Propagates(t *testing.T) {
	uc, mock := newUsecase(t)
	ctx := t.Context()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT \* FROM "email_verification_tokens"`).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	err := uc.VerifyEmail(ctx, payloads.VerifyEmailRequest{Token: "rawtok"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestVerifyEmail_ValidationError(t *testing.T) {
	uc, _ := newUsecase(t)

	err := uc.VerifyEmail(t.Context(), payloads.VerifyEmailRequest{Token: ""})
	assertCode(t, err, errconstant.ErrCodeValidation)
}
