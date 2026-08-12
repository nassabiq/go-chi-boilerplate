# Step-by-Step Module Development Guide

This guide walks through creating a new module (for example, `Product`) across all layers from database migration to router, wire injection, documentation, and tests.

---

## Complete Implementation Checklist

- [ ] **Step 1**: Migration file in `migrations/`
- [ ] **Step 2**: Entity model in `internal/core/domain/entities/`
- [ ] **Step 3**: Payloads in `internal/core/domain/payloads/`
- [ ] **Step 4**: DTO in `internal/core/domain/dto/`
- [ ] **Step 5**: Register domain error codes in `internal/utils/constants/err/messages.go`
- [ ] **Step 6**: Repository in `internal/core/repositories/`
- [ ] **Step 7**: Usecase in `internal/core/usecase/`
- [ ] **Step 8**: Handler in `internal/core/handler/`
- [ ] **Step 9**: Router registration in `internal/router/`
- [ ] **Step 10**: Generate Wire DI (`make wire`)
- [ ] **Step 11**: Generate Swagger docs (`make swagger-gen`)
- [ ] **Step 12**: Unit & Integration tests

---

### Step 1: Database Migration

Create `migrations/20261025000000_create_products.sql`:

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS products (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price NUMERIC(12, 2) NOT NULL DEFAULT 0.00,
    status_id SMALLINT NOT NULL DEFAULT 1 REFERENCES statuses(id),
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_products_deleted_at ON products(deleted_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS products;
-- +goose StatementEnd
```

Run migration:
```bash
make migrate-up
```

---

### Step 2: Domain Entity

Create `internal/core/domain/entities/product.go`:

```go
package entities

import (
    "time"
    "github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
    "gorm.io/gorm"
)

type Product struct {
    ID          int64          `gorm:"primaryKey;autoIncrement"`
    Name        string         `gorm:"column:name;size:255;not null"`
    Description string         `gorm:"column:description;type:text"`
    Price       float64        `gorm:"column:price;type:numeric(12,2);not null"`
    StatusID    enums.Status   `gorm:"column:status_id;not null;default:1"`
    Version     int64          `gorm:"column:version;not null;default:1"`
    CreatedAt   time.Time      `gorm:"column:created_at;not null"`
    UpdatedAt   time.Time      `gorm:"column:updated_at;not null"`
    DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index"`
}
```

---

### Step 3: Domain Payloads

Create `internal/core/domain/payloads/product.go`:

```go
package payloads

type CreateProductRequest struct {
    Name        string  `json:"name" validate:"required,max=255"`
    Description string  `json:"description" validate:"max=1000"`
    Price       float64 `json:"price" validate:"required,gt=0"`
}

type UpdateProductRequest struct {
    Name        string  `json:"name" validate:"required,max=255"`
    Description string  `json:"description" validate:"max=1000"`
    Price       float64 `json:"price" validate:"required,gt=0"`
    Version     int64   `json:"version" validate:"required,gt=0"`
}
```

---

### Step 4: Domain DTO

Create `internal/core/domain/dto/product.go`:

```go
package dto

import "time"

type Product struct {
    ID          int64     `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Price       float64   `json:"price"`
    StatusID    int16     `json:"status_id"`
    Version     int64     `json:"version"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

---

### Step 5: Error Registration

In `internal/utils/constants/err/messages.go`, add:

```go
ErrCodeProductNotFound      = response("E_PRD_1", "product not found", http.StatusNotFound)
ErrCodeProductAlreadyExists = response("E_PRD_2", "product already exists", http.StatusConflict)
```

---

### Step 6: Repository Layer

Add methods in `internal/core/repositories/product.go`:

```go
package repositories

import (
    "context"
    "errors"

    "github.com/nassabiq/go-chi-boilerplate/internal/core/domain/entities"
    errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
    apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
    "gorm.io/gorm"
)

func (repo *BaseRepository) FindProductByID(ctx context.Context, id int64) (*entities.Product, error) {
    var product entities.Product
    err := repo.DB(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&product).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, apperror.New(errconstant.ErrCodeProductNotFound, err)
        }
        return nil, apperror.New(errconstant.ErrCodeInternal, err)
    }
    return &product, nil
}

func (repo *BaseRepository) CreateProduct(ctx context.Context, p *entities.Product) error {
    if err := repo.DB(ctx).Create(p).Error; err != nil {
        return apperror.New(errconstant.ErrCodeInternal, err)
    }
    return nil
}

func (repo *BaseRepository) UpdateProduct(ctx context.Context, p *entities.Product) error {
    result := repo.DB(ctx).Model(&entities.Product{}).
        Where("id = ? AND deleted_at IS NULL AND version = ?", p.ID, p.Version).
        Updates(map[string]any{
            "name":        p.Name,
            "description": p.Description,
            "price":       p.Price,
            "version":     gorm.Expr("version + 1"),
            "updated_at":  gorm.Expr("NOW()"),
        })

    if result.Error != nil {
        return apperror.New(errconstant.ErrCodeInternal, result.Error)
    }
    if result.RowsAffected == 0 {
        return apperror.New(errconstant.ErrCodeStaleResource, nil)
    }

    p.Version++
    return nil
}
```

---

### Step 7: Usecase Layer

Add methods in `internal/core/usecase/product.go`:

```go
package usecase

import (
    "context"

    "github.com/nassabiq/go-chi-boilerplate/internal/core/domain/dto"
    "github.com/nassabiq/go-chi-boilerplate/internal/core/domain/entities"
    "github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
    errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
    "github.com/nassabiq/go-chi-boilerplate/internal/utils/enums"
    apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
)

func composeProduct(p *entities.Product) *dto.Product {
    return &dto.Product{
        ID:          p.ID,
        Name:        p.Name,
        Description: p.Description,
        Price:       p.Price,
        StatusID:    int16(p.StatusID),
        Version:     p.Version,
        CreatedAt:   p.CreatedAt,
        UpdatedAt:   p.UpdatedAt,
    }
}

func (uc *BaseUsecase) CreateProduct(ctx context.Context, payload payloads.CreateProductRequest) (*dto.Product, error) {
    if err := uc.validator.Struct(payload); err != nil {
        return nil, apperror.New(errconstant.ErrCodeValidation, err)
    }

    product := &entities.Product{
        Name:        payload.Name,
        Description: payload.Description,
        Price:       payload.Price,
        StatusID:    enums.StatusActive,
        Version:     1,
    }

    if err := uc.repo.CreateProduct(ctx, product); err != nil {
        return nil, err
    }

    return composeProduct(product), nil
}

func (uc *BaseUsecase) UpdateProduct(ctx context.Context, id int64, payload payloads.UpdateProductRequest) (*dto.Product, error) {
    if err := uc.validator.Struct(payload); err != nil {
        return nil, apperror.New(errconstant.ErrCodeValidation, err)
    }

    product := &entities.Product{
        ID:          id,
        Name:        payload.Name,
        Description: payload.Description,
        Price:       payload.Price,
        Version:     payload.Version,
    }

    if err := uc.repo.UpdateProduct(ctx, product); err != nil {
        return nil, err
    }

    return composeProduct(product), nil
}

func (uc *BaseUsecase) FindProductByID(ctx context.Context, id int64) (*dto.Product, error) {
    product, err := uc.repo.FindProductByID(ctx, id)
    if err != nil {
        return nil, err
    }
    return composeProduct(product), nil
}
```

---

### Step 8: Handler Layer

Add methods in `internal/core/handler/product.go`:

```go
package handler

import (
    "net/http"

    "github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
    errconstant "github.com/nassabiq/go-chi-boilerplate/internal/utils/constants/err"
    apperror "github.com/nassabiq/go-chi-boilerplate/internal/utils/errors"
    "github.com/nassabiq/go-chi-boilerplate/internal/utils/request"
    "github.com/nassabiq/go-chi-boilerplate/internal/utils/response"
)

// CreateProduct godoc
// @Summary Create product
// @Description Creates a new catalog product
// @Tags Products
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body payloads.CreateProductRequest true "Product payload"
// @Success 201 {object} dto.Product
// @Failure 400 {object} response.Error
// @Failure 401 {object} response.Error
// @Router /products [post]
func (handler *BaseHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
    payload, err := request.DecodeJSON[payloads.CreateProductRequest](r)
    if err != nil {
        response.ErrorResponse(w, apperror.New(errconstant.ErrCodeInvalidRequest, err))
        return
    }

    product, err := handler.uc.CreateProduct(r.Context(), payload)
    if err != nil {
        response.ErrorResponse(w, err)
        return
    }

    response.Success(w, http.StatusCreated, product)
}

// FindProductByID godoc
// @Summary Get product by ID
// @Description Returns product by ID
// @Tags Products
// @Produce json
// @Security BearerAuth
// @Param id path int true "Product ID"
// @Success 200 {object} dto.Product
// @Failure 400 {object} response.Error
// @Failure 404 {object} response.Error
// @Router /products/{id} [get]
func (handler *BaseHandler) FindProductByID(w http.ResponseWriter, r *http.Request) {
    id, err := request.PathInt64(r, "id")
    if err != nil {
        response.ErrorResponse(w, apperror.New(errconstant.ErrCodeInvalidParameter, err))
        return
    }

    product, err := handler.uc.FindProductByID(r.Context(), id)
    if err != nil {
        response.ErrorResponse(w, err)
        return
    }

    response.Success(w, http.StatusOK, product)
}
```

---

### Step 9: Router Registration

Create `internal/router/product.go`:

```go
package router

import (
    "github.com/go-chi/chi/v5"
    appmiddleware "github.com/nassabiq/go-chi-boilerplate/internal/middleware"
)

func registerProductRoutes(route chi.Router, deps *Dependencies) {
    route.Group(func(r chi.Router) {
        r.Use(appmiddleware.Auth(deps.JWT, deps.Session))
        r.Post("/products", deps.Handler.CreateProduct)
        r.Get("/products/{id}", deps.Handler.FindProductByID)
    })
}
```

Mount inside `internal/router/router.go`:
```go
registerProductRoutes(route, deps)
```

---

### Step 10 & 11: Wire & Swagger Generation

```bash
make wire
make swagger-gen
```

---

### Step 12: Verify & Test

```bash
go test -count=1 ./...
```
