package database

import (
	"context"
	"fmt"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type txKey struct{}

func NewDB(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}

	sqlDB.SetMaxOpenConns(int(cfg.DB.MaxConns))
	sqlDB.SetMaxIdleConns(int(cfg.DB.MaxIdleConn))
	sqlDB.SetConnMaxIdleTime(time.Hour * 1)
	sqlDB.SetConnMaxIdleTime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()

		return nil, fmt.Errorf("ping db: %w", err)
	}

	return db, nil
}

type TxManager struct {
	db *gorm.DB
}

func NewTxManager(db *gorm.DB) *TxManager {
	return &TxManager{
		db: db,
	}
}

func (m *TxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}

	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txKey{}, tx)
		return fn(txCtx)
	})
}

func (m *TxManager) DB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}

	return m.db.WithContext(ctx)
}
