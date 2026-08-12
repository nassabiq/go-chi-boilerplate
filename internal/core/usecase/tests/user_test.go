package tests

import (
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/repositories"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/usecase"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/database"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
	customvalidator "github.com/nassabiq/go-chi-boilerplate/internal/utils/validator"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newUsecase(t *testing.T) (*usecase.BaseUsecase, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		TranslateError:       true,
		DisableAutomaticPing: true,
	})
	if err != nil {
		t.Fatalf("failed to open mock gorm: %v", err)
	}

	// Disable gorm's connection pool
	if sqlDB2, err := db.DB(); err == nil {
		sqlDB2.SetMaxOpenConns(1)
		sqlDB2.SetMaxIdleConns(1)
		sqlDB2.SetConnMaxLifetime(0)
		sqlDB2.SetConnMaxIdleTime(0)
	}

	v := validator.New()
	_ = customvalidator.RegisterPasswordValidation(v)

	cfg := &config.Config{
		Auth: config.AuthConfig{
			Password: config.PasswordConfig{
				BcryptCost: 4,
			},
		},
	}

	txMgr := database.NewTxManager(db)
	repo := repositories.New(&repositories.Dependencies{TxManager: txMgr})
	uc := usecase.New(&usecase.Dependencies{
		Repo:      repo,
		TxManager: txMgr,
		Validator: v,
		Config:    cfg,
	})

	return uc, mock
}

var userColumns = []string{
	"id", "name", "email", "avatar_url", "status_id", "version", "created_at", "updated_at", "deleted_at",
}

func userRow() []driver.Value {
	return []driver.Value{
		int64(1), "Alice", "alice@example.com", nil, int16(enums.StatusPending), int64(1),
		time.Now(), time.Now(), nil,
	}
}

func rowsWith(row []driver.Value) *sqlmock.Rows {
	return sqlmock.NewRows(userColumns).AddRow(row...)
}

func args(n int) []driver.Value {
	out := make([]driver.Value, n)
	for i := range out {
		out[i] = sqlmock.AnyArg()
	}
	return out
}

func TestFindUsers_Success(t *testing.T) {
	uc, mock := newUsecase(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "users"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(2)))
	mock.ExpectQuery(`SELECT \* FROM "users"`).WithArgs(args(1)...).
		WillReturnRows(rowsWith(userRow()))

	items, total, params, err := uc.FindUsers(t.Context(), payloads.ListPayload{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 || len(items) != 1 || items[0].Email != "alice@example.com" {
		t.Fatalf("unexpected result: total=%d items=%+v", total, items)
	}
	if params.Limit != 10 {
		t.Fatalf("expected default limit 10, got %d", params.Limit)
	}
}

func TestFindUsers_InvalidStatusFilter(t *testing.T) {
	uc, _ := newUsecase(t)

	_, _, _, err := uc.FindUsers(t.Context(), payloads.ListPayload{
		Filters: map[string]string{"status": "not-a-status"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFindUserByID_Success(t *testing.T) {
	uc, mock := newUsecase(t)

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WithArgs(args(2)...).
		WillReturnRows(rowsWith(userRow()))

	user, err := uc.FindUserByID(t.Context(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != 1 || user.StatusID != int16(enums.StatusPending) {
		t.Fatalf("unexpected user: %+v", user)
	}
}

func TestCreateUser_Success(t *testing.T) {
	uc, mock := newUsecase(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "users" \(.+\)`).WithArgs(args(9)...).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectCommit()

	user, err := uc.CreateUser(t.Context(), payloads.CreateUserRequest{
		Name:     "Alice",
		Email:    "alice@example.com",
		Password: "Secret123!",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.StatusID != int16(enums.StatusPending) {
		t.Fatalf("expected pending status, got %d", user.StatusID)
	}
	if user.Version != 1 {
		t.Fatalf("expected initial version 1, got %d", user.Version)
	}
}

func TestCreateUser_ValidationError(t *testing.T) {
	uc, _ := newUsecase(t)

	_, err := uc.CreateUser(t.Context(), payloads.CreateUserRequest{Name: "", Email: "bad"})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestCreateUser_Duplicate(t *testing.T) {
	uc, mock := newUsecase(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "users" \(.+\)`).
		WillReturnError(&pgconn.PgError{Code: "23505", Message: "duplicate"})
	mock.ExpectRollback()

	_, err := uc.CreateUser(t.Context(), payloads.CreateUserRequest{
		Name:     "Alice",
		Email:    "alice@example.com",
		Password: "secret",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpdateUser_Success(t *testing.T) {
	uc, mock := newUsecase(t)

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET`).WithArgs(args(5)...).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	user, err := uc.UpdateUser(t.Context(), 1, payloads.UpdateUserRequest{
		Name:     "Alice",
		Email:    "a@x.com",
		StatusID: enums.StatusActive,
		Version:  2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != 1 {
		t.Fatalf("unexpected user: %+v", user)
	}
	if user.Version != 3 {
		t.Fatalf("expected incremented version 3, got %d", user.Version)
	}
}

func TestUpdateUser_InvalidStatus(t *testing.T) {
	uc, _ := newUsecase(t)

	_, err := uc.UpdateUser(t.Context(), 1, payloads.UpdateUserRequest{
		Name:     "Alice",
		Email:    "a@x.com",
		StatusID: enums.StatusUnknown,
		Version:  2,
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestDeleteUser_Success(t *testing.T) {
	uc, mock := newUsecase(t)

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET`).WithArgs(args(2)...).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := uc.DeleteUser(t.Context(), payloads.MutationRequest{ID: 1, Version: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteUser_ValidationError(t *testing.T) {
	uc, _ := newUsecase(t)

	err := uc.DeleteUser(t.Context(), payloads.MutationRequest{Version: 2})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestRestoreUser_Success(t *testing.T) {
	uc, mock := newUsecase(t)

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET`).WithArgs(args(3)...).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := uc.RestoreUser(t.Context(), payloads.MutationRequest{ID: 1, Version: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
