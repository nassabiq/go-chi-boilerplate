//go:build integration
// +build integration

package integration_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/handler"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/repositories"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/usecase"
	"github.com/nassabiq/go-chi-boilerplate/internal/messaging"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/auth"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/cache"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/database"
	mailsender "github.com/nassabiq/go-chi-boilerplate/internal/pkg/mail"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/nats"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/ratelimit"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/telemetry"
	"github.com/nassabiq/go-chi-boilerplate/internal/router"
	customvalidator "github.com/nassabiq/go-chi-boilerplate/internal/utils/validator"
	mailworker "github.com/nassabiq/go-chi-boilerplate/internal/worker/mail"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// testEnv wires the REAL application stack (config, Postgres via GORM, Redis via
// go-redis, JWT, SessionManager, repositories, usecases, handlers, and the Chi
// router with its middleware) exactly the way cmd/http/wire_gen.go does. Nothing
// in the auth core is mocked: these are true HTTP-level integration/E2E tests.
type testEnv struct {
	cfg     *config.Config
	db      *gorm.DB
	redis   *redis.Client
	jwt     *auth.JWT
	session *auth.SessionManager
	router  http.Handler
	events  *capturePublisher
	handler *handler.BaseHandler
	limiter *ratelimit.Limiter
	logger  *slog.Logger
}

var (
	envOnce sync.Once
	envInst *testEnv
	envErr  error
)

// getTestEnv builds the stack once and returns it. When the backing PostgreSQL
// or Redis is unreachable, every test skips with a clear message instead of
// pretending it passed. This keeps the suite honest in environments (e.g. CI
// sandboxes) where real infra is not provisioned.
func getTestEnv(t *testing.T) *testEnv {
	t.Helper()

	envOnce.Do(func() {
		envInst, envErr = bootstrap()
	})

	if envErr != nil {
		t.Skipf("integration infra unavailable, skipping E2E test: %v", envErr)
	}

	return envInst
}

// reset isolates each test: truncate users and related auth tables, and flush the
// Redis test DB so no state leaks between scenarios.
func (e *testEnv) reset(t *testing.T) {
	t.Helper()

	ctx := context.Background()

	if err := e.db.Exec(`TRUNCATE TABLE users, password_reset_tokens, email_verification_tokens, email_change_requests, activity_logs, user_roles RESTART IDENTITY CASCADE`).Error; err != nil {
		t.Fatalf("truncate auth tables: %v", err)
	}

	if err := e.redis.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush redis: %v", err)
	}
}

func bootstrap() (*testEnv, error) {
	cfg := buildTestConfig()

	db, err := database.NewDB(cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}

	if err := applyMigrations(db); err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	}

	client, err := cache.NewRedis(cfg)
	if err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}

	validate := validator.New()
	_ = customvalidator.RegisterPasswordValidation(validate)
	jwtSvc := auth.NewJWT(cfg)
	session := auth.NewSessionManager(client)
	limiter := ratelimit.New(client)
	loginThrottler := auth.NewLoginThrottler(client, cfg.Auth.LoginThrottle)

	txManager := database.NewTxManager(db)
	repo := repositories.New(&repositories.Dependencies{TxManager: txManager})
	pub := &capturePublisher{}

	tel, _ := telemetry.InitTelemetry()
	var metrics *telemetry.Metrics
	if tel != nil {
		metrics = tel.Metrics()
	}

	uc := usecase.New(&usecase.Dependencies{
		Repo:          repo,
		TxManager:     txManager,
		Validator:     validate,
		JWT:           jwtSvc,
		Session:       session,
		LoginThrottle: loginThrottler,
		Limiter:       limiter,
		Config:        cfg,
		Publisher:     pub,
		Logger:        newTestLogger(),
		Metrics:       metrics,
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	baseHandler := handler.New(&handler.Dependencies{Usecase: uc, Logger: newTestLogger()})

	routerHandler := router.New(&router.Dependencies{
		Logger:  logger,
		Handler: baseHandler,
		Limiter: limiter,
		Config:  cfg,
		JWT:     jwtSvc,
		Session: session,
	})

	return &testEnv{
		cfg:     cfg,
		db:      db,
		redis:   client,
		jwt:     jwtSvc,
		session: session,
		router:  routerHandler,
		events:  pub,
		handler: baseHandler,
		limiter: limiter,
		logger:  logger,
	}, nil
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// buildTestConfig uses dedicated test secrets and short-but-practical TTLs. Real
// production values are never reused. Host/port/credentials can be overridden
// through environment variables so the suite runs against any provisioned
// PostgreSQL/Redis (including the bundled docker-compose.yml).
func buildTestConfig() *config.Config {
	get := func(key, fallback string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fallback
	}

	redisHost := get("TEST_REDIS_HOST", "127.0.0.1")
	redisPort := atoiEnv("TEST_REDIS_PORT", 6379)
	if addr := os.Getenv("TEST_REDIS_ADDR"); addr != "" {
		if h, p, err := net.SplitHostPort(addr); err == nil {
			redisHost = h
			if pInt, err := strconv.Atoi(p); err == nil {
				redisPort = pInt
			}
		} else {
			redisHost = addr
		}
	}

	return &config.Config{
		App:  config.AppConfig{Name: "e2e", Env: "test"},
		Http: config.HTTPConfig{Host: "localhost", Port: 0},
		DB: config.DatabaseConfig{
			Host:     get("TEST_DB_HOST", "localhost"),
			Port:     atoiEnv("TEST_DB_PORT", 5432),
			User:     get("TEST_DB_USER", "root"),
			Password: get("TEST_DB_PASSWORD", "root"),
			Name:     get("TEST_DB_NAME", "go-chi"),
			MaxConns: 25,
			MinConns: 1,
		},
		Redis: config.RedisConfig{
			Host:     redisHost,
			Port:     redisPort,
			DB:       atoiEnv("TEST_REDIS_DB", 1),
			Password: get("TEST_REDIS_PASSWORD", ""),
		},
		RateLimit: config.RateLimitConfig{
			Enabled: false,
			Forgot: config.RateLimitRule{
				Requests: 100,
				Window:   time.Hour,
			},
			Resend: config.RateLimitRule{
				Requests: 100,
				Window:   time.Hour,
			},
		},
		Auth: config.AuthConfig{
			JWT: config.JWTConfig{
				AccessSecret:  get("TEST_JWT_ACCESS_SECRET", "test-access-secret-do-not-use-in-prod"),
				RefreshSecret: get("TEST_JWT_REFRESH_SECRET", "test-refresh-secret-do-not-use-in-prod"),
				Issuer:        "go-chi",
				AccessExpiry:  15 * time.Minute,
				RefreshExpiry: 1 * time.Hour,
			},
			LoginThrottle: config.LoginThrottleConfig{
				// Generous limits so normal E2E attempts never trip throttling.
				FailureWindow: time.Hour,
				FirstLimit:    1000000,
				FirstBlock:    time.Second,
				SecondLimit:   1000000,
				SecondBlock:   time.Second,
				MaxLimit:      1000000,
				MaxBlock:      time.Second,
			},
			PasswordReset: config.PasswordResetConfig{
				TokenTTL: 15 * time.Minute,
				URL:      get("TEST_PASSWORD_RESET_URL", "http://localhost/reset"),
			},
			EmailVerification: config.EmailVerificationConfig{
				TokenTTL: 24 * time.Hour,
				URL:      get("TEST_VERIFICATION_URL", "http://localhost/verify"),
			},
			EmailChange: config.EmailChangeConfig{
				TokenTTL: 24 * time.Hour,
				URL:      get("TEST_EMAIL_CHANGE_URL", "http://localhost/confirm-email"),
			},
		},
	}
}

func atoiEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// applyMigrations executes the project's real Goose Up migrations (read directly
// from migrations/*.sql) against the integration database. This avoids adding a
// Goose dependency while still using the production schema. "already exists"
// errors are tolerated so the suite is idempotent across runs.
func applyMigrations(db *gorm.DB) error {
	root, err := findRepoRoot()
	if err != nil {
		return err
	}

	files, err := filepath.Glob(filepath.Join(root, "migrations", "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)

	ctx := context.Background()
	for _, file := range files {
		up, err := extractUp(file)
		if err != nil {
			return err
		}
		if err := execStatements(ctx, db, up); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
	}
	return nil
}

func findRepoRoot() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("unable to resolve caller")
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("repo root (go.mod) not found")
		}
		dir = parent
	}
}

func extractUp(file string) (string, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	content := string(raw)

	upIdx := strings.Index(content, "-- +goose Up")
	downIdx := strings.Index(content, "-- +goose Down")
	if upIdx < 0 {
		return "", fmt.Errorf("no -- +goose Up marker in %s", file)
	}

	up := content[upIdx:]
	if downIdx >= 0 {
		up = content[upIdx:downIdx]
	}
	return up, nil
}

func execStatements(ctx context.Context, db *gorm.DB, sql string) error {
	for _, stmt := range strings.Split(sql, ";") {
		// Strip leading comment lines so statements following the
		// "-- +goose Up" marker are not discarded wholesale (the marker
		// shares a chunk with the first statement after splitting on ";").
		lines := strings.Split(strings.TrimSpace(stmt), "\n")
		for len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "--") {
			lines = lines[1:]
		}
		stmt = strings.TrimSpace(strings.Join(lines, "\n"))
		if stmt == "" {
			continue
		}
		if err := db.WithContext(ctx).Exec(stmt).Error; err != nil {
			msg := err.Error()
			// Idempotent across repeated bootstraps: objects/seeds from a
			// previous run are tolerated, mirroring goose's tracked state.
			if strings.Contains(msg, "already exists") || strings.Contains(msg, "duplicate key") {
				continue
			}
			return err
		}
	}
	return nil
}

// capturePublisher is an in-memory EventPublisher used by the integration
// suite so flows can be exercised without a live NATS server when testing
// HTTP layers. It records events so tests can assert and recover tokens.
type capturePublisher struct {
	mu                    sync.Mutex
	events                []messaging.PasswordResetRequested
	verificationEvents    []messaging.EmailVerificationRequested
	emailChangeEvents     []messaging.EmailChangeRequested
	passwordChangedEvents []messaging.PasswordChanged
}

func (c *capturePublisher) PublishPasswordResetRequested(_ context.Context, e messaging.PasswordResetRequested) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
	return nil
}

func (c *capturePublisher) PublishPasswordChanged(_ context.Context, e messaging.PasswordChanged) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.passwordChangedEvents = append(c.passwordChangedEvents, e)
	return nil
}

func (c *capturePublisher) PublishEmailVerificationRequested(_ context.Context, e messaging.EmailVerificationRequested) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.verificationEvents = append(c.verificationEvents, e)
	return nil
}

func (c *capturePublisher) PublishEmailChangeRequested(_ context.Context, e messaging.EmailChangeRequested) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.emailChangeEvents = append(c.emailChangeEvents, e)
	return nil
}

func (c *capturePublisher) lastResetEvent() (messaging.PasswordResetRequested, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.events) == 0 {
		return messaging.PasswordResetRequested{}, false
	}
	return c.events[len(c.events)-1], true
}

func (c *capturePublisher) lastVerificationEvent() (messaging.EmailVerificationRequested, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.verificationEvents) == 0 {
		return messaging.EmailVerificationRequested{}, false
	}
	return c.verificationEvents[len(c.verificationEvents)-1], true
}

// e2eEnv wires the TRUE end-to-end environment connecting real PostgreSQL,
// real Redis, real NATS JetStream, and real MailDev (via SMTP + REST API).
// Nothing in the delivery or persistence path is mocked.
type e2eEnv struct {
	*testEnv
	natsClient   *nats.Client
	mailDevURL   string
	workerCancel context.CancelFunc
}

func (e *e2eEnv) reset(t *testing.T) {
	t.Helper()
	e.testEnv.reset(t)
	clearMailDev(t, e.mailDevURL)
}

func getE2EEnv(t *testing.T) *e2eEnv {
	t.Helper()
	base := getTestEnv(t)

	get := func(key, fallback string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fallback
	}

	natsURL := get("TEST_NATS_URL", "nats://localhost:4222")
	cfgNATS := &config.Config{NATS: config.NATSConfig{URL: natsURL}}
	natsClient, err := nats.New(cfgNATS)
	if err != nil {
		t.Skipf("NATS unavailable (%s), skipping E2E test: %v", natsURL, err)
	}

	mailDevURL := get("TEST_MAILDEV_API_URL", "http://localhost:1080")
	resp, err := http.Get(mailDevURL + "/email")
	if err != nil || resp.StatusCode != http.StatusOK {
		natsClient.Close()
		t.Skipf("MailDev unavailable (%s), skipping E2E test: %v", mailDevURL, err)
	}
	_ = resp.Body.Close()

	// Clear MailDev on initialization
	clearMailDev(t, mailDevURL)

	// Create real NATS event publisher
	pub := messaging.NewPublisher(natsClient)

	// Build app stack wired with the REAL publisher
	txManager := database.NewTxManager(base.db)
	repo := repositories.New(&repositories.Dependencies{TxManager: txManager})
	validate := validator.New()
	_ = customvalidator.RegisterPasswordValidation(validate)
	loginThrottler := auth.NewLoginThrottler(base.redis, base.cfg.Auth.LoginThrottle)
	limiter := ratelimit.New(base.redis)

	tel, _ := telemetry.InitTelemetry()
	var metrics *telemetry.Metrics
	if tel != nil {
		metrics = tel.Metrics()
	}

	uc := usecase.New(&usecase.Dependencies{
		Repo:          repo,
		TxManager:     txManager,
		Validator:     validate,
		JWT:           base.jwt,
		Session:       base.session,
		LoginThrottle: loginThrottler,
		Limiter:       limiter,
		Config:        base.cfg,
		Publisher:     pub,
		Logger:        newTestLogger(),
		Metrics:       metrics,
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	baseHandler := handler.New(&handler.Dependencies{Usecase: uc, Logger: newTestLogger()})

	routerHandler := router.New(&router.Dependencies{
		Logger:  logger,
		Handler: baseHandler,
		Limiter: limiter,
		Config:  base.cfg,
		JWT:     base.jwt,
		Session: base.session,
	})

	// Start real Mail Worker consuming from NATS and sending via real SMTP to MailDev
	smtpSender := mailsender.NewSender(&config.Config{
		SMTP: config.SMTPConfig{
			Host: get("TEST_MAILDEV_SMTP_HOST", "localhost"),
			Port: atoiEnv("TEST_MAILDEV_SMTP_PORT", 1025),
			From: "no-reply@gochi.local",
			Name: "Boilerplate E2E",
		},
	})

	workerName := fmt.Sprintf("e2e-worker-%d", time.Now().UnixNano())
	worker := mailworker.New(&mailworker.Dependencies{
		NATS:         natsClient,
		Sender:       smtpSender,
		Logger:       newTestLogger(),
		ConsumerName: workerName,
		Metrics:      metrics,
	})

	workerCtx, workerCancel := context.WithCancel(context.Background())
	go func() {
		_ = worker.Run(workerCtx)
	}()

	t.Cleanup(func() {
		workerCancel()
		natsClient.Close()
	})

	return &e2eEnv{
		testEnv: &testEnv{
			cfg:     base.cfg,
			db:      base.db,
			redis:   base.redis,
			jwt:     base.jwt,
			session: base.session,
			router:  routerHandler,
			events:  base.events,
		},
		natsClient:   natsClient,
		mailDevURL:   mailDevURL,
		workerCancel: workerCancel,
	}
}

func clearMailDev(t *testing.T, mailDevURL string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, mailDevURL+"/email/all", nil)
	if err != nil {
		t.Fatalf("clear maildev request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("clear maildev: %v", err)
	}
	_ = resp.Body.Close()
}
