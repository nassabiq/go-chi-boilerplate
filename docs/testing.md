# Testing & Quality Assurance Guide

This document defines testing conventions, directory layout, test isolation strategies, and test execution commands.

---

## 1. Test Directory Structure

Tests are co-located in dedicated `tests` subpackages:

```
├── internal/
│   ├── middleware/
│   │   └── tests/              # Middleware unit tests (Auth, RBAC, Tracing, CORS)
│   ├── core/
│   │   ├── handler/
│   │   │   └── tests/          # Handler HTTP transport tests (httptest)
│   │   ├── usecase/
│   │   │   └── tests/          # Business logic unit tests
│   │   └── repositories/
│   │       └── tests/          # GORM query & database integration tests
└── tests/
    └── integration/            # End-to-end integration tests (requires docker)
```

---

## 2. Handler Unit Testing

Use `net/http/httptest` to test handlers in isolation:

```go
func TestCreateProduct_Success(t *testing.T) {
    mockUC := &MockUsecase{}
    h := handler.New(&handler.Dependencies{
        Usecase: mockUC,
        Logger:  slog.New(slog.NewJSONHandler(io.Discard, nil)),
    })

    payload := `{"name":"Mechanical Keyboard","price":99.50}`
    req := httptest.NewRequest(http.MethodPost, "/products", strings.NewReader(payload))
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()

    h.CreateProduct(rec, req)

    if rec.Code != http.StatusCreated {
        t.Fatalf("expected status 201, got %d", rec.Code)
    }

    var res dto.Product
    if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
        t.Fatalf("failed to decode response: %v", err)
    }
    if res.Name != "Mechanical Keyboard" {
        t.Errorf("expected name 'Mechanical Keyboard', got '%s'", res.Name)
    }
}
```

---

## 3. Usecase Unit Testing

Test business rules and error branches:

```go
func TestCreateProduct_ValidationError(t *testing.T) {
    uc := &BaseUsecase{
        validator: validator.New(),
    }

    // Negative price should fail validation
    payload := payloads.CreateProductRequest{
        Name:  "Test",
        Price: -10.0,
    }

    res, err := uc.CreateProduct(context.Background(), payload)
    if err == nil {
        t.Fatal("expected validation error, got nil")
    }
    if res != nil {
        t.Errorf("expected nil response, got %v", res)
    }

    var appErr *apperror.Error
    if !errors.As(err, &appErr) || appErr.Code != errconstant.ErrCodeValidation {
        t.Errorf("expected ErrCodeValidation, got %v", err)
    }
}
```

---

## 4. Repository & Database Testing

Run repository queries inside rolled-back transactions to keep the database clean:

```go
func TestBaseRepository_CreateProduct(t *testing.T) {
    db := setupTestDB(t)
    tx := db.Begin()
    defer tx.Rollback()

    repo := repositories.New(&repositories.Dependencies{
        TxManager: database.NewTxManager(tx),
    })

    product := &entities.Product{
        Name:     "Headphones",
        Price:    150.00,
        StatusID: 1,
    }

    err := repo.CreateProduct(context.Background(), product)
    if err != nil {
        t.Fatalf("failed to create product: %v", err)
    }
    if product.ID == 0 {
        t.Errorf("expected generated product ID > 0, got %d", product.ID)
    }
}
```

---

## 5. Integration & E2E Testing

Integration tests in `tests/integration/` require running infrastructure (PostgreSQL, Redis, NATS, MailDev).

Tagged with `//go:build integration`:
```go
//go:build integration

package integration_test

import (
    "testing"
)

func TestProduct_E2E(t *testing.T) {
    // E2E test making real HTTP calls against booted test server
}
```

---

## 6. Testing Commands

| Purpose | Command |
|---|---|
| **Run All Unit Tests** | `go test -count=1 ./...` |
| **Run Unit Tests with Race Detector** | `go test -race -count=1 ./...` |
| **Run Integration Tests** | `make test-integration` |
| **Run Specific Package Tests** | `go test -count=1 ./internal/core/handler/tests/...` |
| **Run Specific Test by Name** | `go test -v -run TestCreateProduct ./internal/core/...` |
| **Run Linter** | `golangci-lint run` or `make lint` |
