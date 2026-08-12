package tests

import (
	"context"
	"testing"

	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/telemetry"
	"go.opentelemetry.io/otel/sdk/metric"
)

func TestMetrics_RecordAllWithoutPanic(t *testing.T) {
	provider := metric.NewMeterProvider()
	meter := provider.Meter("test")

	m, err := telemetry.NewMetrics(meter)
	if err != nil {
		t.Fatalf("failed to create metrics: %v", err)
	}

	ctx := context.Background()

	// Verify all record calls execute cleanly without panic
	m.RecordLoginSuccess(ctx)
	m.RecordLoginFailure(ctx, "invalid_credentials")
	m.RecordLoginFailure(ctx, "user_pending")
	m.RecordLoginThrottled(ctx)
	m.RecordRegistrationSuccess(ctx)
	m.RecordEmailVerificationSuccess(ctx)
	m.RecordPasswordResetRequested(ctx)
	m.RecordPasswordResetSuccess(ctx)
	m.RecordPasswordResetFailure(ctx, "same_password")
	m.RecordSessionCreated(ctx, "login")
	m.RecordSessionCreated(ctx, "refresh")
	m.RecordSessionRevoked(ctx, "logout")
	m.RecordSessionRevoked(ctx, "logout_all")
	m.RecordMailSent(ctx, "password_reset")
	m.RecordMailFailed(ctx, "password_reset", "transient")
	m.RecordMailRetry(ctx, "password_reset")
}

func TestMetrics_NilSafety(t *testing.T) {
	var m *telemetry.Metrics
	ctx := context.Background()

	// Should not panic when nil
	m.RecordLoginSuccess(ctx)
	m.RecordLoginFailure(ctx, "invalid_credentials")
	m.RecordLoginThrottled(ctx)
	m.RecordRegistrationSuccess(ctx)
	m.RecordEmailVerificationSuccess(ctx)
	m.RecordPasswordResetRequested(ctx)
	m.RecordPasswordResetSuccess(ctx)
	m.RecordPasswordResetFailure(ctx, "same_password")
	m.RecordSessionCreated(ctx, "login")
	m.RecordSessionRevoked(ctx, "logout")
	m.RecordMailSent(ctx, "password_reset")
	m.RecordMailFailed(ctx, "password_reset", "poison")
	m.RecordMailRetry(ctx, "password_reset")
}
