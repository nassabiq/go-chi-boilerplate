# Database, Migrations & Transactions Guide

This document explains database management, Goose schema migrations, GORM persistence patterns, database transactions using `TxManager`, and optimistic concurrency control (OCC).

---

## 1. Schema Migrations with Goose

Database schema changes are managed through [Goose](https://github.com/pressly/goose).

### Migration File Rules
1. Migration files reside in `./migrations/`.
2. Name migrations using standard timestamp prefixes: `YYYYMMDDHHMMSS_<descriptive_name>.sql`.
3. **NEVER modify or rewrite an existing migration file that has been committed or run in production.** Always write a forward migration.
4. Each migration must have both an `Up` and a `Down` block enclosed in `StatementBegin` and `StatementEnd`.

### Standard Migration Template
```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    total_amount NUMERIC(12, 2) NOT NULL DEFAULT 0.00,
    status_id SMALLINT NOT NULL DEFAULT 1 REFERENCES statuses(id),
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_deleted_at ON orders(deleted_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS orders;
-- +goose StatementEnd
```

### Essential Migration Commands
```bash
# Create a new migration file
make migrate-create

# Run all pending migrations
make migrate-up

# Roll back the single latest migration
make migrate-down

# Check migration status
make migrate-status
```

---

## 2. GORM Entity Conventions

Every persistent model must live in `internal/core/domain/entities/`:

```go
package entities

import (
    "time"
    "gorm.io/gorm"
)

type Order struct {
    ID          int64          `gorm:"primaryKey;autoIncrement"`
    UserID      int64          `gorm:"column:user_id;not null"`
    TotalAmount float64        `gorm:"column:total_amount;type:numeric(12,2);not null"`
    StatusID    int16          `gorm:"column:status_id;not null"`
    Version     int64          `gorm:"column:version;not null;default:1"`
    CreatedAt   time.Time      `gorm:"column:created_at;not null"`
    UpdatedAt   time.Time      `gorm:"column:updated_at;not null"`
    DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index"`
}
```

### Standards
- Primary keys must be `int64` (`BIGSERIAL` in Postgres).
- Soft deletes are handled by `gorm.DeletedAt`. Queries must exclude soft-deleted records: `Where("deleted_at IS NULL")`.
- Concurrently modifiable entities must declare `Version int64`.

---

## 3. Database Transactions with `TxManager`

Transactions are managed via `internal/pkg/database.TxManager`. This allows usecases to orchestrate transactions across multiple repositories without passing raw GORM references.

### How `TxManager` Context Propagation Works
1. `TxManager.WithTx(ctx, fn)` begins a database transaction.
2. It injects the `*gorm.DB` transaction instance into the `context.Context`.
3. In repositories, `repo.DB(ctx)` checks the context for an existing transaction.
   - If present, it uses the active transaction.
   - If absent, it uses the base connection pool.

### Transaction Example in Usecase
```go
func (uc *OrderUsecase) PlaceOrder(ctx context.Context, payload payloads.CreateOrderRequest) (*dto.Order, error) {
    var createdOrder *entities.Order

    err := uc.txManager.WithTx(ctx, func(txCtx context.Context) error {
        // Step 1: Create Order record
        order := &entities.Order{
            UserID:      payload.UserID,
            TotalAmount: payload.TotalAmount,
            StatusID:    1,
        }
        if err := uc.repo.CreateOrder(txCtx, order); err != nil {
            return err
        }

        // Step 2: Deduct stock or create order items
        for _, item := range payload.Items {
            if err := uc.repo.DeductProductStock(txCtx, item.ProductID, item.Quantity); err != nil {
                return err // Triggers automatic ROLLBACK
            }
        }

        createdOrder = order
        return nil // Triggers COMMIT
    })

    if err != nil {
        return nil, err
    }

    return composeOrder(createdOrder), nil
}
```

---

## 4. Optimistic Concurrency Control (OCC)

To prevent race conditions, lost updates, and double-mutations without blocking database locks, use OCC with the `version` column.

### OCC Update Pattern in Repositories
```go
func (repo *BaseRepository) UpdateOrder(ctx context.Context, o *entities.Order) error {
    result := repo.DB(ctx).Model(&entities.Order{}).
        Where("id = ? AND deleted_at IS NULL AND version = ?", o.ID, o.Version).
        Updates(map[string]any{
            "total_amount": o.TotalAmount,
            "status_id":    o.StatusID,
            "version":      gorm.Expr("version + 1"),
            "updated_at":   gorm.Expr("NOW()"),
        })

    if result.Error != nil {
        return apperror.New(errconstant.ErrCodeInternal, result.Error)
    }

    // If 0 rows were affected, the version changed or the record does not exist
    if result.RowsAffected == 0 {
        return apperror.New(errconstant.ErrCodeStaleResource, nil)
    }

    // Bump Go struct in-memory so Usecase response reflects the new DB version
    o.Version++

    return nil
}
```

### Critical Rules for OCC:
1. **Creation**: When initializing an entity for insert, always set `Version: 1` explicitly so memory and DB defaults align.
2. **Post-Update Increment**: Since GORM `Updates(map[string]any{...})` executes a raw SQL expression without re-reading values back into the Go pointer, you **must increment `entity.Version++`** in the repository after `RowsAffected > 0`.
3. **DTO Exposure**: Outgoing DTOs must expose `Version int64` (`json:"version"`). Clients require this version to populate subsequent `PUT`, `DELETE`, or `RESTORE` requests (`validate:"required,gt=0"`).

### OCC Soft Delete & Restore
```go
// Soft Delete
func (repo *BaseRepository) DeleteOrder(ctx context.Context, id int64, version int64) error {
    result := repo.DB(ctx).Model(&entities.Order{}).
        Where("id = ? AND deleted_at IS NULL AND version = ?", id, version).
        Updates(map[string]any{
            "deleted_at": gorm.Expr("NOW()"),
            "version":    gorm.Expr("version + 1"),
        })

    if result.Error != nil {
        return apperror.New(errconstant.ErrCodeInternal, result.Error)
    }
    if result.RowsAffected == 0 {
        return apperror.New(errconstant.ErrCodeStaleResource, nil)
    }
    return nil
}

// Restore
func (repo *BaseRepository) RestoreOrder(ctx context.Context, id int64, version int64) error {
    result := repo.DB(ctx).Model(&entities.Order{}).
        Where("id = ? AND deleted_at IS NOT NULL AND version = ?", id, version).
        Updates(map[string]any{
            "deleted_at": nil,
            "version":    gorm.Expr("version + 1"),
        })

    if result.Error != nil {
        return apperror.New(errconstant.ErrCodeInternal, result.Error)
    }
    if result.RowsAffected == 0 {
        return apperror.New(errconstant.ErrCodeStaleResource, nil)
    }
    return nil
}
```
