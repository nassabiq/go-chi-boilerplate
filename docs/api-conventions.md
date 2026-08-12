# API Conventions & Transport Guide

This document specifies HTTP request parsing, response formatting, query filtering/sorting/pagination helpers, Swagger OpenAPI annotations, and routing conventions.

---

## 1. Request Decoding & Parameter Parsing

Use the typed request helpers in `internal/utils/request`:

### JSON Request Body
```go
// Disallows unknown fields and decodes strictly into generic struct
payload, err := request.DecodeJSON[payloads.CreateProductRequest](r)
if err != nil {
    response.ErrorResponse(w, apperror.New(errconstant.ErrCodeInvalidRequest, err))
    return
}
```

### URL Path Parameters
```go
// Parse path integer ID (e.g. /products/{id})
id, err := request.PathInt64(r, "id")
if err != nil {
    response.ErrorResponse(w, apperror.New(errconstant.ErrCodeInvalidParameter, err))
    return
}

// Parse string slug parameter (e.g. /categories/{slug})
slug, err := request.PathParam(r, "slug")
```

### Client IP & Metadata
```go
ip := request.ClientIP(r)
ua := r.UserAgent()
```

---

## 2. Standardized JSON Responses

All HTTP responses must use helpers from `internal/utils/response`:

### Single Object or Custom Status
```go
// 200 OK
response.Success(w, http.StatusOK, productDTO)

// 201 Created
response.Success(w, http.StatusCreated, createdProductDTO)

// 204 No Content
w.WriteHeader(http.StatusNoContent)
```

### Paginated List Response
```go
response.SuccessResponse(w, items, total, params.Skip, params.Limit)
```

Produces:
```json
{
  "items": [
    { "id": 1, "name": "Keyboard", "price": 49.99 }
  ],
  "meta": {
    "total": 1,
    "skip": 0,
    "limit": 10
  }
}
```

---

## 3. Query Filtering, Sorting & Pagination

For list endpoints (e.g. `GET /products?skip=0&limit=10&search=key&sort_by=price&sort=asc&filters[status]=active`):

### In Handler:
```go
payload, err := request.List(r)
if err != nil {
    response.ErrorResponse(w, apperror.New(errconstant.ErrCodeValidation, err))
    return
}

items, total, params, err := handler.uc.FindProducts(r.Context(), payload)
if err != nil {
    response.ErrorResponse(w, err)
    return
}

response.SuccessResponse(w, items, total, params.Skip, params.Limit)
```

### In Repository:
Define a `helper.ListConfig`:
```go
var productListConfig = helper.ListConfig{
    SearchColumns: []string{"name", "description"},
    SortColumns: map[string]string{
        "name":       "name",
        "price":      "price",
        "created_at": "created_at",
    },
    FilterColumns: map[string]string{
        "status": "status_id",
    },
    DefaultSortBy:        "created_at",
    DefaultSortDirection: "DESC",
}

func (repo *BaseRepository) FindProducts(ctx context.Context, params helper.ListParams) ([]*entities.Product, int64, error) {
    var (
        items []*entities.Product
        total int64
    )

    query := repo.DB(ctx).Model(&entities.Product{}).Where("deleted_at IS NULL")
    query = helper.ApplySearch(query, params.Search, productListConfig.SearchColumns)
    query = helper.ApplyFilter(query, params.Filters, productListConfig.FilterColumns)

    if err := query.Count(&total).Error; err != nil {
        return nil, 0, apperror.New(errconstant.ErrCodeInternal, err)
    }

    query = helper.ApplySort(query, params, productListConfig)
    query = helper.ApplyPagination(query, params)

    if err := query.Find(&items).Error; err != nil {
        return nil, 0, apperror.New(errconstant.ErrCodeInternal, err)
    }

    return items, total, nil
}
```

---

## 4. Swagger / OpenAPI Annotations

Every handler method must have annotations for `swag`.

### Annotation Template:
```go
// CreateProduct godoc
// @Summary Create product
// @Description Creates a new product catalog item
// @Tags Products
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body payloads.CreateProductRequest true "Product Payload"
// @Success 201 {object} dto.Product
// @Failure 400 {object} response.Error
// @Failure 401 {object} response.Error
// @Failure 409 {object} response.Error
// @Failure 500 {object} response.Error
// @Router /products [post]
func (handler *BaseHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
    // ...
}
```

### Regenerating Docs:
```bash
make swagger-gen
```

---

## 5. Router & Middleware Conventions

Route groups are defined in `internal/router/<module>.go`:

```go
package router

import (
    "github.com/go-chi/chi/v5"
    "github.com/nassabiq/go-chi-boilerplate/internal/core/handler"
    appmiddleware "github.com/nassabiq/go-chi-boilerplate/internal/middleware"
)

func registerProductRoutes(route chi.Router, deps *Dependencies) {
    route.Group(func(r chi.Router) {
        // Enforce JWT Bearer Auth & active session check
        r.Use(appmiddleware.Auth(deps.JWT, deps.Session))

        // Role-based guard
        r.With(appmiddleware.RequireRole("superadmin")).Post("/products", deps.Handler.CreateProduct)

        // Permission-based guard
        r.With(appmiddleware.RequirePermission("products:update")).Put("/products/{id}", deps.Handler.UpdateProduct)

        // Authenticated read
        r.Get("/products", deps.Handler.FindProducts)
        r.Get("/products/{id}", deps.Handler.FindProductByID)
    })
}
```
Mount the route in `internal/router/router.go`:
```go
registerProductRoutes(route, deps)
```
