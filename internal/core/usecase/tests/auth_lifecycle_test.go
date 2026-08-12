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

func newLifecycleHarness(t *testing.T) (*usecase.BaseUsecase, sqlmock.Sqlmock, *miniredis.Miniredis) {
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
	})

	return uc, mock, mr
}

func lifecycleActiveUserRow() []driver.Value {
	return []driver.Value{
		int64(1), "Natsu", "natsu@example.com", "hashed", int16(enums.StatusActive), int64(5),
		time.Now(), time.Now(), nil,
	}
}

func TestDeactivate_Active_Success(t *testing.T) {
	uc, mock, _ := newLifecycleHarness(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows(loginColumns).AddRow(lifecycleActiveUserRow()...))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := uc.Deactivate(ctx, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestDeactivate_Inactive_Rejected(t *testing.T) {
	uc, mock, _ := newLifecycleHarness(t)
	ctx := context.Background()

	row := []driver.Value{
		int64(2), "Bob", "bob@example.com", "hashed", int16(enums.StatusInactive), int64(1),
		time.Now(), time.Now(), nil,
	}
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows(loginColumns).AddRow(row...))

	err := uc.Deactivate(ctx, 2)
	assertCode(t, err, errconstant.ErrCodeAuthUserInactive)
}

func TestDeactivate_Blocked_Rejected(t *testing.T) {
	uc, mock, _ := newLifecycleHarness(t)
	ctx := context.Background()

	row := []driver.Value{
		int64(3), "Eve", "eve@example.com", "hashed", int16(enums.StatusBlocked), int64(1),
		time.Now(), time.Now(), nil,
	}
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows(loginColumns).AddRow(row...))

	err := uc.Deactivate(ctx, 3)
	assertCode(t, err, errconstant.ErrCodeAuthUserBlocked)
}

func TestDeleteAccount_Active_Success(t *testing.T) {
	uc, mock, _ := newLifecycleHarness(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows(loginColumns).AddRow(lifecycleActiveUserRow()...))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := uc.DeleteAccount(ctx, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestDeleteAccount_Inactive_Rejected(t *testing.T) {
	uc, mock, _ := newLifecycleHarness(t)
	ctx := context.Background()

	row := []driver.Value{
		int64(4), "Mallory", "m@example.com", "hashed", int16(enums.StatusInactive), int64(1),
		time.Now(), time.Now(), nil,
	}
	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows(loginColumns).AddRow(row...))

	err := uc.DeleteAccount(ctx, 4)
	assertCode(t, err, errconstant.ErrCodeAuthUserInactive)
}
