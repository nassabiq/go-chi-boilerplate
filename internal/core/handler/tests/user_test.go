package tests

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/handler"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/repositories"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/usecase"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/database"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
	"github.com/nassabiq/go-chi-boilerplate/internal/utils/response"
	customvalidator "github.com/nassabiq/go-chi-boilerplate/internal/utils/validator"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newHandler(t *testing.T) (*handler.BaseHandler, sqlmock.Sqlmock) {
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
	h := handler.New(&handler.Dependencies{Usecase: uc})

	return h, mock
}

func withRouter(h *handler.BaseHandler, fn func(chi.Router)) chi.Router {
	r := chi.NewRouter()
	fn(r)
	return r
}

func perform(r chi.Router, method, target string, body []byte) *httptest.ResponseRecorder {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, target, bytes.NewReader(body))
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

var userColumns = []string{
	"id", "name", "email", "avatar_url", "status_id", "version", "created_at", "updated_at", "deleted_at",
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

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("failed to decode response: %v; body=%s", err, rr.Body.String())
	}
	return out
}

func userRow() []driver.Value {
	return []driver.Value{
		int64(1), "Alice", "alice@example.com", nil, int16(enums.StatusPending), int64(1),
		time.Now(), time.Now(), nil,
	}
}

func TestFindUsers_Success(t *testing.T) {
	h, mock := newHandler(t)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "users"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT \* FROM "users"`).WithArgs(args(1)...).
		WillReturnRows(rowsWith(userRow()))

	router := withRouter(h, func(r chi.Router) { r.Get("/users/{id}", h.FindUsers) })
	rr := perform(router, http.MethodGet, "/users/1", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["meta"].(map[string]any)["total"].(float64) != 1 {
		t.Fatalf("unexpected body: %v", body)
	}
}

func TestFindUserByID_Success(t *testing.T) {
	h, mock := newHandler(t)

	mock.ExpectQuery(`SELECT \* FROM "users" WHERE id = \$1`).
		WithArgs(args(2)...).
		WillReturnRows(rowsWith(userRow()))

	router := withRouter(h, func(r chi.Router) { r.Get("/users/{id}", h.FindUserByID) })
	rr := perform(router, http.MethodGet, "/users/1", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeBody(t, rr)
	if body["name"] != "Alice" {
		t.Fatalf("unexpected body: %v", body)
	}
}

func TestCreateUser_Success(t *testing.T) {
	h, mock := newHandler(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "users"`).WithArgs(args(9)...).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectCommit()

	router := withRouter(h, func(r chi.Router) { r.Post("/users", h.CreateUser) })
	rr := perform(router, http.MethodPost, "/users",
		[]byte(`{"name":"Alice","email":"alice@example.com","password":"Secret123!"}`))

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateUser_ValidationError(t *testing.T) {
	h, _ := newHandler(t)

	router := withRouter(h, func(r chi.Router) { r.Post("/users", h.CreateUser) })
	rr := perform(router, http.MethodPost, "/users", []byte(`{"name":"","email":"bad"}`))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	var res response.Error
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(res.Errors) == 0 {
		t.Fatalf("expected validation errors to be populated, got empty: %s", rr.Body.String())
	}
}

func TestUpdateUser_Success(t *testing.T) {
	h, mock := newHandler(t)

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET`).WithArgs(args(5)...).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	router := withRouter(h, func(r chi.Router) { r.Put("/users/{id}", h.UpdateUser) })
	rr := perform(router, http.MethodPut, "/users/1",
		[]byte(`{"name":"Alice","email":"a@x.com","status_id":1,"version":2}`))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDeleteUser_Success(t *testing.T) {
	h, mock := newHandler(t)

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET`).WithArgs(args(2)...).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	router := withRouter(h, func(r chi.Router) { r.Delete("/users/{id}", h.DeleteUser) })
	rr := perform(router, http.MethodDelete, "/users/1", []byte(`{"version":2}`))

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRestoreUser_Success(t *testing.T) {
	h, mock := newHandler(t)

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET`).WithArgs(args(3)...).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	router := withRouter(h, func(r chi.Router) { r.Patch("/users/{id}/restore", h.RestoreUser) })
	rr := perform(router, http.MethodPatch, "/users/1/restore", []byte(`{"version":2}`))

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDeleteUser_ValidationError(t *testing.T) {
	h, _ := newHandler(t)

	router := withRouter(h, func(r chi.Router) { r.Delete("/users/{id}", h.DeleteUser) })
	rr := perform(router, http.MethodDelete, "/users/1", []byte(`{"version":0}`))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
