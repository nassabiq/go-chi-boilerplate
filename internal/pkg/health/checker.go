package health

import (
	"context"
	"errors"
	"sync"
	"time"

	natsclient "github.com/nassabiq/go-chi-boilerplate/internal/pkg/nats"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	ErrDatabaseNil = errors.New("database connection is not initialized")
	ErrRedisNil    = errors.New("redis client is not initialized")
	ErrNATSNotConnected = errors.New("nats connection is not connected")
)

type ComponentStatus struct {
	Status  string `json:"status"` // "up" or "down"
	Latency string `json:"latency"`
	Error   string `json:"error,omitempty"`
}

type ReadinessResponse struct {
	Status     string                     `json:"status"` // "up" or "down"
	Timestamp  time.Time                  `json:"timestamp"`
	Components map[string]ComponentStatus `json:"components"`
}

type Checker struct {
	db    *gorm.DB
	redis *redis.Client
	nats  *natsclient.Client
}

func NewChecker(db *gorm.DB, redis *redis.Client, nats *natsclient.Client) *Checker {
	return &Checker{
		db:    db,
		redis: redis,
		nats:  nats,
	}
}

func (c *Checker) CheckLive() bool {
	return true
}

func (c *Checker) CheckReady(ctx context.Context) ReadinessResponse {
	resp := ReadinessResponse{
		Status:     "up",
		Timestamp:  time.Now().UTC(),
		Components: make(map[string]ComponentStatus, 3),
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	// Database check
	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		var err error

		if c.db == nil {
			err = ErrDatabaseNil
		} else {
			sqlDB, dbErr := c.db.DB()
			if dbErr != nil {
				err = dbErr
			} else {
				err = sqlDB.PingContext(ctx)
			}
		}

		status := ComponentStatus{
			Status:  "up",
			Latency: time.Since(start).String(),
		}
		if err != nil {
			status.Status = "down"
			status.Error = err.Error()
		}

		mu.Lock()
		resp.Components["database"] = status
		if err != nil {
			resp.Status = "down"
		}
		mu.Unlock()
	}()

	// Redis check
	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		var err error

		if c.redis == nil {
			err = ErrRedisNil
		} else {
			err = c.redis.Ping(ctx).Err()
		}

		status := ComponentStatus{
			Status:  "up",
			Latency: time.Since(start).String(),
		}
		if err != nil {
			status.Status = "down"
			status.Error = err.Error()
		}

		mu.Lock()
		resp.Components["redis"] = status
		if err != nil {
			resp.Status = "down"
		}
		mu.Unlock()
	}()

	// NATS check
	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		var err error

		if c.nats == nil || c.nats.Conn == nil || !c.nats.Conn.IsConnected() {
			err = ErrNATSNotConnected
		}

		status := ComponentStatus{
			Status:  "up",
			Latency: time.Since(start).String(),
		}
		if err != nil {
			status.Status = "down"
			status.Error = err.Error()
		}

		mu.Lock()
		resp.Components["nats"] = status
		if err != nil {
			resp.Status = "down"
		}
		mu.Unlock()
	}()

	wg.Wait()
	return resp
}
