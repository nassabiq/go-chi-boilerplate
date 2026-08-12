//go:build wireinject
// +build wireinject

package main

import (
	"log/slog"

	"github.com/google/wire"
	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/dependency"
	mailsender "github.com/nassabiq/go-chi-boilerplate/internal/pkg/mail"
	natsclient "github.com/nassabiq/go-chi-boilerplate/internal/pkg/nats"
	mailworker "github.com/nassabiq/go-chi-boilerplate/internal/worker/mail"
)

// App is the mail worker composition root. It intentionally contains no HTTP
// router, database, Redis or JWT: the worker only needs config, NATS, the SMTP
// sender and a logger.
type App struct {
	Config *config.Config
	NATS   *natsclient.Client
	Sender *mailsender.Sender
	Logger *slog.Logger
	Worker *mailworker.Worker
}

func InitializeApp() (*App, error) {
	wire.Build(
		config.Load,
		dependency.NewLogger,
		dependency.NewTelemetry,
		natsclient.New,
		mailsender.NewSender,
		wire.Bind(new(mailworker.EmailSender), new(*mailsender.Sender)),
		wire.Struct(new(mailworker.Dependencies), "NATS", "Sender", "Logger", "Metrics"),
		mailworker.New,
		wire.Struct(new(App), "*"),
	)
	return nil, nil
}
