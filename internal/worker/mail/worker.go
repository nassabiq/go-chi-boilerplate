package mail

import (
	"context"
	"log/slog"
	"time"

	mailsender "github.com/nassabiq/go-chi-boilerplate/internal/pkg/mail"
	natsclient "github.com/nassabiq/go-chi-boilerplate/internal/pkg/nats"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/telemetry"
)

// EmailSender abstracts SMTP delivery so the worker can be tested without a
// real server. *mailsender.Sender satisfies it.
type EmailSender interface {
	Send(ctx context.Context, message mailsender.Message) error
}

type Dependencies struct {
	NATS    *natsclient.Client
	Sender  EmailSender
	Logger  *slog.Logger
	Metrics *telemetry.Metrics

	// Optional overrides used by tests. Zero values fall back to production
	// defaults.
	ConsumerName  string
	FilterSubject string
	AckWait       time.Duration
	MaxDeliver    int
	RetryDelay    time.Duration
}

// Defaults for the mail worker consumer. The worker subscribes to the whole
// auth domain (auth.>) with a single durable consumer and routes each message
// by its subject. Overridable through Dependencies for tests.
const (
	DefaultConsumerName  = "mail-worker-v1"
	DefaultFilterSubject = "auth.>"
	DefaultAckWait       = 30 * time.Second
	DefaultMaxDeliver    = 5
	DefaultRetryDelay    = 10 * time.Second
)

type Worker struct {
	nats          *natsclient.Client
	sender        EmailSender
	logger        *slog.Logger
	metrics       *telemetry.Metrics
	consumerName  string
	filterSubject string
	handler       Handler
	ackWait       time.Duration
	maxDeliver    int
	retryDelay    time.Duration
}

// Config reports the effective consumer configuration (defaults applied).
type Config struct {
	ConsumerName  string
	FilterSubject string
	AckWait       time.Duration
	MaxDeliver    int
	RetryDelay    time.Duration
}

func (w *Worker) Config() Config {
	return Config{
		ConsumerName:  w.consumerName,
		FilterSubject: w.filterSubject,
		AckWait:       w.ackWait,
		MaxDeliver:    w.maxDeliver,
		RetryDelay:    w.retryDelay,
	}
}

func New(deps *Dependencies) *Worker {
	w := &Worker{
		nats:          deps.NATS,
		sender:        deps.Sender,
		logger:        deps.Logger,
		metrics:       deps.Metrics,
		consumerName:  deps.ConsumerName,
		filterSubject: deps.FilterSubject,
		ackWait:       deps.AckWait,
		maxDeliver:    deps.MaxDeliver,
		retryDelay:    deps.RetryDelay,
	}

	if w.consumerName == "" {
		w.consumerName = DefaultConsumerName
	}
	if w.filterSubject == "" {
		w.filterSubject = DefaultFilterSubject
	}
	if w.ackWait <= 0 {
		w.ackWait = DefaultAckWait
	}
	if w.maxDeliver <= 0 {
		w.maxDeliver = DefaultMaxDeliver
	}
	if w.retryDelay <= 0 {
		w.retryDelay = DefaultRetryDelay
	}

	w.handler = handlerForSubject(w.sender, w.logger, w.consumerName)

	return w
}
