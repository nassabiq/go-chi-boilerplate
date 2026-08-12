package tests

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/entities"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/repositories"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newVerificationRepo(t *testing.T) (*repositories.BaseRepository, sqlmock.Sqlmock) {
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

	return repo, mock
}

func TestEmailVerificationToken_CreateAndMarkUsed(t *testing.T) {
	repo, mock := newVerificationRepo(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "email_verification_tokens"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(11)))
	mock.ExpectCommit()

	token := &entities.EmailVerificationToken{
		UserID:    1,
		TokenHash: "abc123",
		ExpiresAt: now.Add(time.Hour),
	}
	if err := repo.CreateEmailVerificationToken(ctx, token); err != nil {
		t.Fatalf("create: %v", err)
	}
	if token.ID != 11 {
		t.Fatalf("expected id 11, got %d", token.ID)
	}

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "email_verification_tokens" SET`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.MarkEmailVerificationTokenUsed(ctx, token.ID, now); err != nil {
		t.Fatalf("mark used: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestEmailVerificationToken_InvalidatePending(t *testing.T) {
	repo, mock := newVerificationRepo(t)
	ctx := context.Background()
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "email_verification_tokens" SET`).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	if err := repo.InvalidatePendingEmailVerificationTokens(ctx, 1, now); err != nil {
		t.Fatalf("invalidate: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func emailVerificationColumns() []string {
	return []string{"id", "user_id", "token_hash", "expires_at", "used_at", "requested_ip", "user_agent", "created_at"}
}

func TestEmailVerificationToken_FindForUpdate(t *testing.T) {
	repo, mock := newVerificationRepo(t)
	ctx := context.Background()

	row := []driver.Value{int64(11), int64(1), "abc123", time.Now().Add(time.Hour), nil, nil, nil, time.Now()}
	mock.ExpectQuery(`SELECT \* FROM "email_verification_tokens"`).
		WillReturnRows(sqlmock.NewRows(emailVerificationColumns()).AddRow(row...))

	found, err := repo.FindEmailVerificationTokenForUpdate(ctx, "abc123")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found.ID != 11 || found.TokenHash != "abc123" {
		t.Fatalf("unexpected token: %+v", found)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}
