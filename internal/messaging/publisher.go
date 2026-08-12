package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	natsclient "github.com/nassabiq/go-chi-boilerplate/internal/pkg/nats"
	"github.com/nats-io/nats.go/jetstream"
)

// EventPublisher delivers domain events to the messaging backbone. The usecase
// layer depends on this abstraction rather than the low-level JetStream API.
type EventPublisher interface {
	PublishPasswordResetRequested(ctx context.Context, event PasswordResetRequested) error
	PublishPasswordChanged(ctx context.Context, event PasswordChanged) error
	PublishEmailVerificationRequested(ctx context.Context, event EmailVerificationRequested) error
	PublishEmailChangeRequested(ctx context.Context, event EmailChangeRequested) error
}

// Publisher publishes domain events to NATS JetStream and waits for the
// publish acknowledgement before returning.
type Publisher struct {
	nats *natsclient.Client
}

// NewPublisher builds a JetStream-backed event publisher.
func NewPublisher(nats *natsclient.Client) *Publisher {
	return &Publisher{nats: nats}
}

// PublishPasswordResetRequested marshals and publishes the password-reset
// requested event, waiting for the JetStream acknowledgement. EventID is used as
// the NATS message ID for idempotent redelivery.
func (p *Publisher) PublishPasswordResetRequested(ctx context.Context, event PasswordResetRequested) error {
	return p.publish(ctx, TopicPasswordResetRequested, event.EventID, event)
}

// PublishPasswordChanged marshals and publishes the password-changed event,
// waiting for the JetStream acknowledgement.
func (p *Publisher) PublishPasswordChanged(ctx context.Context, event PasswordChanged) error {
	return p.publish(ctx, TopicPasswordChanged, event.EventID, event)
}

// PublishEmailVerificationRequested marshals and publishes the
// email-verification requested event, waiting for the JetStream
// acknowledgement. EventID is used as the NATS message ID for idempotent
// redelivery.
func (p *Publisher) PublishEmailVerificationRequested(ctx context.Context, event EmailVerificationRequested) error {
	return p.publish(ctx, TopicEmailVerificationRequested, event.EventID, event)
}

// PublishEmailChangeRequested marshals and publishes the email-change
// verification event (sent to the NEW email), waiting for the JetStream
// acknowledgement. EventID is used as the NATS message ID for idempotent
// redelivery.
func (p *Publisher) PublishEmailChangeRequested(ctx context.Context, event EmailChangeRequested) error {
	return p.publish(ctx, TopicEmailChanged, event.EventID, event)
}

func (p *Publisher) publish(ctx context.Context, subject, msgID string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s event: %w", subject, err)
	}

	if _, err := p.nats.JetStream.Publish(ctx, subject, data, jetstream.WithMsgID(msgID)); err != nil {
		return fmt.Errorf("publish %s: %w", subject, err)
	}

	return nil
}
