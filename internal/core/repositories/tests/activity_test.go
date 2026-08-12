package tests

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/repositories"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newActivityRepo(t *testing.T) (*repositories.BaseRepository, sqlmock.Sqlmock) {
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

func TestLogActivity_Success(t *testing.T) {
	repo, mock := newActivityRepo(t)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "activity_logs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectCommit()

	meta := map[string]any{"action": "test"}
	err := repo.LogActivity(ctx, 100, "user.login", "127.0.0.1", "Go-Test-Agent", meta)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
