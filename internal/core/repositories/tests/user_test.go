package tests

import (
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/entities"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/repositories"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/database"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/helper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newRepo(t *testing.T) (*repositories.BaseRepository, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("failed to open mock gorm: %v", err)
	}

	txMgr := database.NewTxManager(db)
	repo := repositories.New(&repositories.Dependencies{TxManager: txMgr})

	return repo, mock
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
	repo, mock := newRepo(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "users"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(2)))

	mock.ExpectQuery(`SELECT \* FROM "users"`).WithArgs(args(1)...).
		WillReturnRows(rowsWith(userRow()))

	users, total, err := repo.FindUsers(t.Context(), helper.ListParams{
		Skip: 0, Limit: 10, Filters: map[string]string{},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 || len(users) != 1 || users[0].ID != 1 || users[0].Email != "alice@example.com" {
		t.Fatalf("unexpected result: total=%d users=%+v", total, users)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestFindUsers_CountError(t *testing.T) {
	repo, mock := newRepo(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "users"`).
		WillReturnError(gorm.ErrNotImplemented)

	_, _, err := repo.FindUsers(t.Context(), helper.ListParams{Limit: 10})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFindUserById_Success(t *testing.T) {
	repo, mock := newRepo(t)

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WithArgs(args(2)...).
		WillReturnRows(rowsWith(userRow()))

	user, err := repo.FindUserById(t.Context(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Name != "Alice" {
		t.Fatalf("unexpected user: %+v", user)
	}
}

func TestFindUserById_NotFound(t *testing.T) {
	repo, mock := newRepo(t)

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WithArgs(args(2)...).
		WillReturnRows(sqlmock.NewRows(userColumns))

	_, err := repo.FindUserById(t.Context(), 99)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFindUserByEmail_Success(t *testing.T) {
	repo, mock := newRepo(t)

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE email = \$1`).
		WithArgs(args(2)...).
		WillReturnRows(rowsWith(userRow()))

	user, err := repo.FindUserByEmail(t.Context(), "alice@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "alice@example.com" {
		t.Fatalf("unexpected user: %+v", user)
	}
}

func TestFindUserByEmail_NotFound(t *testing.T) {
	repo, mock := newRepo(t)

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE email = \$1`).
		WithArgs(args(2)...).
		WillReturnRows(sqlmock.NewRows(userColumns))

	_, err := repo.FindUserByEmail(t.Context(), "missing@example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreateUser_Success(t *testing.T) {
	repo, mock := newRepo(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "users"`).WithArgs(args(9)...).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectCommit()

	err := repo.CreateUser(t.Context(), &entities.User{
		Name:     "Alice",
		Email:    "alice@example.com",
		StatusID: enums.StatusPending,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateUser_Duplicate(t *testing.T) {
	repo, mock := newRepo(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "users"`).
		WillReturnError(&pgconn.PgError{Code: "23505", Message: "duplicate"})
	mock.ExpectRollback()

	err := repo.CreateUser(t.Context(), &entities.User{
		Name:  "Alice",
		Email: "alice@example.com",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "user already exists") {
		t.Fatalf("expected duplicate error, got: %v", err)
	}
}

func TestUpdateUser_Success(t *testing.T) {
	repo, mock := newRepo(t)

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET`).WithArgs(args(5)...).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	u := &entities.User{
		ID: 1, Name: "Alice", Email: "a@x.com", StatusID: enums.StatusActive, Version: 2,
	}
	err := repo.UpdateUser(t.Context(), u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Version != 3 {
		t.Fatalf("expected version to be incremented to 3, got %d", u.Version)
	}
}

func TestUpdateUser_Stale(t *testing.T) {
	repo, mock := newRepo(t)

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET`).WithArgs(args(5)...).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := repo.UpdateUser(t.Context(), &entities.User{ID: 1, Version: 2})
	if err == nil {
		t.Fatal("expected stale resource error, got nil")
	}
}

func TestDeleteUser_Success(t *testing.T) {
	repo, mock := newRepo(t)

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET`).WithArgs(args(2)...).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.DeleteUser(t.Context(), 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteUser_Stale(t *testing.T) {
	repo, mock := newRepo(t)

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET`).WithArgs(args(2)...).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := repo.DeleteUser(t.Context(), 1, 2)
	if err == nil {
		t.Fatal("expected stale resource error, got nil")
	}
}

func TestRestoreUser_Success(t *testing.T) {
	repo, mock := newRepo(t)

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET`).WithArgs(args(3)...).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.RestoreUser(t.Context(), 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
