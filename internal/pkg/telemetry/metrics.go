package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics encapsulates all OpenTelemetry metric instruments for auth and mail services.
// Every recording method is nil-safe.
type Metrics struct {
	loginSuccess             metric.Int64Counter
	loginFailure             metric.Int64Counter
	loginThrottled           metric.Int64Counter
	registrationSuccess      metric.Int64Counter
	emailVerificationSuccess metric.Int64Counter
	passwordResetRequested   metric.Int64Counter
	passwordResetSuccess     metric.Int64Counter
	passwordResetFailure     metric.Int64Counter
	sessionCreated           metric.Int64Counter
	sessionRevoked           metric.Int64Counter

	mailSent   metric.Int64Counter
	mailFailed metric.Int64Counter
	mailRetry  metric.Int64Counter
}

// NewMetrics initializes and registers domain metric instruments with the provided Meter.
func NewMetrics(meter metric.Meter) (*Metrics, error) {
	var (
		m   = &Metrics{}
		err error
	)

	// Auth metrics
	if m.loginSuccess, err = meter.Int64Counter("auth.login.success",
		metric.WithDescription("Total number of successful user logins")); err != nil {
		return nil, fmt.Errorf("create auth.login.success: %w", err)
	}

	if m.loginFailure, err = meter.Int64Counter("auth.login.failure",
		metric.WithDescription("Total number of failed user logins")); err != nil {
		return nil, fmt.Errorf("create auth.login.failure: %w", err)
	}

	if m.loginThrottled, err = meter.Int64Counter("auth.login.throttled",
		metric.WithDescription("Total number of login attempts blocked by throttler")); err != nil {
		return nil, fmt.Errorf("create auth.login.throttled: %w", err)
	}

	if m.registrationSuccess, err = meter.Int64Counter("auth.registration.success",
		metric.WithDescription("Total number of successful user registrations")); err != nil {
		return nil, fmt.Errorf("create auth.registration.success: %w", err)
	}

	if m.emailVerificationSuccess, err = meter.Int64Counter("auth.email-verification.success",
		metric.WithDescription("Total number of successful email verifications")); err != nil {
		return nil, fmt.Errorf("create auth.email-verification.success: %w", err)
	}

	if m.passwordResetRequested, err = meter.Int64Counter("auth.password-reset.requested",
		metric.WithDescription("Total number of password reset requests initiated")); err != nil {
		return nil, fmt.Errorf("create auth.password-reset.requested: %w", err)
	}

	if m.passwordResetSuccess, err = meter.Int64Counter("auth.password-reset.success",
		metric.WithDescription("Total number of successful password resets")); err != nil {
		return nil, fmt.Errorf("create auth.password-reset.success: %w", err)
	}

	if m.passwordResetFailure, err = meter.Int64Counter("auth.password-reset.failure",
		metric.WithDescription("Total number of failed password reset attempts")); err != nil {
		return nil, fmt.Errorf("create auth.password-reset.failure: %w", err)
	}

	if m.sessionCreated, err = meter.Int64Counter("auth.session.created",
		metric.WithDescription("Total number of authenticated sessions created")); err != nil {
		return nil, fmt.Errorf("create auth.session.created: %w", err)
	}

	if m.sessionRevoked, err = meter.Int64Counter("auth.session.revoked",
		metric.WithDescription("Total number of authenticated sessions revoked")); err != nil {
		return nil, fmt.Errorf("create auth.session.revoked: %w", err)
	}

	// Mail worker metrics
	if m.mailSent, err = meter.Int64Counter("mail.sent",
		metric.WithDescription("Total number of emails successfully sent by mail worker")); err != nil {
		return nil, fmt.Errorf("create mail.sent: %w", err)
	}

	if m.mailFailed, err = meter.Int64Counter("mail.failed",
		metric.WithDescription("Total number of failed email deliveries")); err != nil {
		return nil, fmt.Errorf("create mail.failed: %w", err)
	}

	if m.mailRetry, err = meter.Int64Counter("mail.retry",
		metric.WithDescription("Total number of transient email retries")); err != nil {
		return nil, fmt.Errorf("create mail.retry: %w", err)
	}

	return m, nil
}

// -----------------------------------------------------------------------------
// Auth Metric Recorders (Zero PII - safe categoricals only)
// -----------------------------------------------------------------------------

func (m *Metrics) RecordLoginSuccess(ctx context.Context) {
	if m == nil || m.loginSuccess == nil {
		return
	}
	m.loginSuccess.Add(ctx, 1)
}

func (m *Metrics) RecordLoginFailure(ctx context.Context, reason string) {
	if m == nil || m.loginFailure == nil {
		return
	}
	m.loginFailure.Add(ctx, 1, metric.WithAttributes(
		attribute.String("reason", reason),
	))
}

func (m *Metrics) RecordLoginThrottled(ctx context.Context) {
	if m == nil || m.loginThrottled == nil {
		return
	}
	m.loginThrottled.Add(ctx, 1)
}

func (m *Metrics) RecordRegistrationSuccess(ctx context.Context) {
	if m == nil || m.registrationSuccess == nil {
		return
	}
	m.registrationSuccess.Add(ctx, 1)
}

func (m *Metrics) RecordEmailVerificationSuccess(ctx context.Context) {
	if m == nil || m.emailVerificationSuccess == nil {
		return
	}
	m.emailVerificationSuccess.Add(ctx, 1)
}

func (m *Metrics) RecordPasswordResetRequested(ctx context.Context) {
	if m == nil || m.passwordResetRequested == nil {
		return
	}
	m.passwordResetRequested.Add(ctx, 1)
}

func (m *Metrics) RecordPasswordResetSuccess(ctx context.Context) {
	if m == nil || m.passwordResetSuccess == nil {
		return
	}
	m.passwordResetSuccess.Add(ctx, 1)
}

func (m *Metrics) RecordPasswordResetFailure(ctx context.Context, reason string) {
	if m == nil || m.passwordResetFailure == nil {
		return
	}
	m.passwordResetFailure.Add(ctx, 1, metric.WithAttributes(
		attribute.String("reason", reason),
	))
}

func (m *Metrics) RecordSessionCreated(ctx context.Context, sessionType string) {
	if m == nil || m.sessionCreated == nil {
		return
	}
	m.sessionCreated.Add(ctx, 1, metric.WithAttributes(
		attribute.String("type", sessionType),
	))
}

func (m *Metrics) RecordSessionRevoked(ctx context.Context, reason string) {
	if m == nil || m.sessionRevoked == nil {
		return
	}
	m.sessionRevoked.Add(ctx, 1, metric.WithAttributes(
		attribute.String("reason", reason),
	))
}

// -----------------------------------------------------------------------------
// Mail Worker Metric Recorders
// -----------------------------------------------------------------------------

func (m *Metrics) RecordMailSent(ctx context.Context, topic string) {
	if m == nil || m.mailSent == nil {
		return
	}
	m.mailSent.Add(ctx, 1, metric.WithAttributes(
		attribute.String("topic", topic),
	))
}

func (m *Metrics) RecordMailFailed(ctx context.Context, topic, failType string) {
	if m == nil || m.mailFailed == nil {
		return
	}
	m.mailFailed.Add(ctx, 1, metric.WithAttributes(
		attribute.String("topic", topic),
		attribute.String("type", failType),
	))
}

func (m *Metrics) RecordMailRetry(ctx context.Context, topic string) {
	if m == nil || m.mailRetry == nil {
		return
	}
	m.mailRetry.Add(ctx, 1, metric.WithAttributes(
		attribute.String("topic", topic),
	))
}
