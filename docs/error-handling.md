# Error Handling & Response Mapping Guide

This document defines error management, registration of domain-specific error codes, wrapping in `AppError`, and client error formatting.

---

## 1. Core Error Invariants

1. **NEVER return raw Go errors to handlers or clients**: Never send `err.Error()`, `fmt.Errorf(...)`, or untyped `errors.New(...)` in HTTP responses.
2. **ALL domain errors must be wrapped with `apperror.New`**:
   ```go
   apperror.New(errconstant.ErrCode<Domain><Reason>, err)
   ```
3. **Every business error code must be registered in `internal/utils/constants/err/messages.go`**.
4. **All handler errors must be dispatched via `response.ErrorResponse(w, err)`**.

---

## 2. Error Code Structure & Registration

Error codes are registered in `internal/utils/constants/err/messages.go` using the `response()` helper:

```go
package err

import "net/http"

var (
    // Format: response("E_<MODULE>_<INDEX>", "Default user-facing message", HTTP_STATUS)
    ErrCodeProductNotFound      = response("E_PRD_1", "product not found", http.StatusNotFound)
    ErrCodeProductAlreadyExists = response("E_PRD_2", "product already exists", http.StatusConflict)
    ErrCodeProductOutOfStock    = response("E_PRD_3", "product is out of stock", http.StatusBadRequest)
)
```

### Module Prefix Convention

| Prefix | Domain / Layer | Example |
|---|---|---|
| `E_SRV_` | System & Infrastructure | `E_SRV_1` (Internal Server Error) |
| `E_VAL_` | Validation & Payload Format | `E_VAL_1` (Validation Failed) |
| `E_AUTH_`| Authentication & Authorization | `E_AUTH_1` (Invalid Credentials), `E_AUTH_3` (Forbidden) |
| `E_USR_` | User Management | `E_USR_1` (User Not Found) |
| `E_STR_` | Storage & S3 | `E_STR_1` (Upload Failed) |
| `E_CNF_` | Concurrency & Conflict | `E_CNF_1` (Resource Modified / Stale Resource) |
| `E_<MOD>_`| New Feature Domain | e.g. `E_ORD_1` for Orders, `E_PRD_1` for Products |

> [!NOTE]
> The `response()` helper panics on startup if duplicate error codes or empty messages are registered, preventing runtime collisions.

---

## 3. The `AppError` Type

Defined in `internal/utils/errors/error.go`:

```go
type Error struct {
    Code       errconstant.ErrorCode
    Message    string
    HTTPStatus int
    Err        error
}

func New(code errconstant.ErrorCode, err error) *Error {
    res := errconstant.GetErrorResponse(code)
    return &Error{
        Code:       code,
        Message:    res.Message,
        HTTPStatus: res.HTTPStatus,
        Err:        err,
    }
}
```

### Usage in Layers:

#### In Repositories:
```go
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
```

#### In Usecases:
```go
if product.Stock < quantity {
    return nil, apperror.New(errconstant.ErrCodeProductOutOfStock, nil)
}
```

#### In Handlers:
```go
product, err := handler.uc.FindProductByID(r.Context(), id)
if err != nil {
    response.ErrorResponse(w, err)
    return
}
```

---

## 4. Standard Client Error Response Schema

When `response.ErrorResponse(w, err)` renders an error, it produces:

```json
{
  "code": "E_VAL_1",
  "message": "validation failed",
  "errors": [
    {
      "field": "price",
      "rule": "gt",
      "message": "price must be greater than 0"
    }
  ]
}
```

If it is not a validation error, `"errors"` is omitted:

```json
{
  "code": "E_PRD_1",
  "message": "product not found"
}
```
