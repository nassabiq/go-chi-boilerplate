package mail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/nassabiq/go-chi-boilerplate/internal/messaging"
	email "github.com/nassabiq/go-chi-boilerplate/internal/worker/mail/email"
	mailsender "github.com/nassabiq/go-chi-boilerplate/internal/pkg/mail"
	"github.com/nats-io/nats.go/jetstream"
)

// Handler processes a single message for one topic. It returns nil on success
// (the caller ACKs), a transient error to request redelivery (NAK-with-delay),
// or a *poisonError to terminate the message (no retry).
type Handler func(ctx context.Context, msg jetstream.Msg) error

// handlePasswordResetRequested decodes a PasswordResetRequested event, renders
// the email and delivers it. ACK happens in the caller only after this returns
// nil: an SMTP failure must keep the message pending for redelivery
// (at-least-once semantics).
func handlePasswordResetRequested(ctx context.Context, sender EmailSender, logger *slog.Logger, consumerName string, msg jetstream.Msg) error {
	var event messaging.PasswordResetRequested

	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return &poisonError{cause: fmt.Errorf("decode event: %w", err)}
	}

	if event.EventID == "" {
		return &poisonError{cause: errors.New("event missing event_id")}
	}

	subject, text, html := email.RenderPasswordReset(event)

	if err := sender.Send(ctx, mailsender.Message{
		To:      event.Email,
		Subject: subject,
		Text:    text,
		HTML:    html,
	}); err != nil {
		// Transient SMTP failure: NAK with delay so JetStream redelivers.
		return fmt.Errorf("send password reset email: %w", err)
	}

	logger.InfoContext(ctx, "password reset email sent",
		"event_id", event.EventID,
		"user_id", event.UserID,
		"consumer", consumerName,
		"topic", msg.Subject(),
	)
	return nil
}

// handlePasswordChanged decodes a PasswordChanged event and sends a
// security-notification email. Errors follow the same ACK/NAK/Term semantics
// as handlePasswordResetRequested.
func handlePasswordChanged(ctx context.Context, sender EmailSender, logger *slog.Logger, consumerName string, msg jetstream.Msg) error {
	var event messaging.PasswordChanged

	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return &poisonError{cause: fmt.Errorf("decode event: %w", err)}
	}

	if event.EventID == "" {
		return &poisonError{cause: errors.New("event missing event_id")}
	}

	subject, text, html := email.RenderPasswordChanged(event)

	if err := sender.Send(ctx, mailsender.Message{
		To:      event.Email,
		Subject: subject,
		Text:    text,
		HTML:    html,
	}); err != nil {
		return fmt.Errorf("send password changed email: %w", err)
	}

	logger.InfoContext(ctx, "password changed email sent",
		"event_id", event.EventID,
		"user_id", event.UserID,
		"consumer", consumerName,
		"topic", msg.Subject(),
	)
	return nil
}

// handleEmailVerificationRequested decodes an EmailVerificationRequested event
// and sends the verification email. ACK/NAK/Term semantics mirror the other
// handlers.
func handleEmailVerificationRequested(ctx context.Context, sender EmailSender, logger *slog.Logger, consumerName string, msg jetstream.Msg) error {
	var event messaging.EmailVerificationRequested

	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return &poisonError{cause: fmt.Errorf("decode event: %w", err)}
	}

	if event.EventID == "" {
		return &poisonError{cause: errors.New("event missing event_id")}
	}

	subject, text, html := email.RenderEmailVerification(event)

	if err := sender.Send(ctx, mailsender.Message{
		To:      event.Email,
		Subject: subject,
		Text:    text,
		HTML:    html,
	}); err != nil {
		return fmt.Errorf("send email verification email: %w", err)
	}

	logger.InfoContext(ctx, "email verification email sent",
		"event_id", event.EventID,
		"user_id", event.UserID,
		"consumer", consumerName,
		"topic", msg.Subject(),
	)
	return nil
}

// handleEmailChangeRequested decodes an EmailChangeRequested event (sent to the
// NEW email) and sends the confirmation email. ACK/NAK/Term semantics mirror
// the other handlers.
func handleEmailChangeRequested(ctx context.Context, sender EmailSender, logger *slog.Logger, consumerName string, msg jetstream.Msg) error {
	var event messaging.EmailChangeRequested

	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return &poisonError{cause: fmt.Errorf("decode event: %w", err)}
	}

	if event.EventID == "" {
		return &poisonError{cause: errors.New("event missing event_id")}
	}

	subject, text, html := email.RenderEmailChange(event)

	if err := sender.Send(ctx, mailsender.Message{
		To:      event.NewEmail,
		Subject: subject,
		Text:    text,
		HTML:    html,
	}); err != nil {
		return fmt.Errorf("send email change email: %w", err)
	}

	logger.InfoContext(ctx, "email change email sent",
		"event_id", event.EventID,
		"user_id", event.UserID,
		"consumer", consumerName,
		"topic", msg.Subject(),
	)
	return nil
}

// handlerForSubject returns the single worker handler that routes each message
// by its real subject (msg.Subject()) to the right email logic. The worker
// subscribes to the whole auth domain (auth.>), so routing must use the
// message subject, not the consumer's filter subject. Add a case here when a
// new mail event is introduced. Unknown subjects are treated as poison so they
// are terminated rather than mishandled.
func handlerForSubject(sender EmailSender, logger *slog.Logger, consumerName string) Handler {
	return func(ctx context.Context, msg jetstream.Msg) error {
		switch msg.Subject() {
		case messaging.TopicPasswordResetRequested:
			return handlePasswordResetRequested(ctx, sender, logger, consumerName, msg)
		case messaging.TopicPasswordChanged:
			return handlePasswordChanged(ctx, sender, logger, consumerName, msg)
		case messaging.TopicEmailVerificationRequested:
			return handleEmailVerificationRequested(ctx, sender, logger, consumerName, msg)
		case messaging.TopicEmailChanged:
			return handleEmailChangeRequested(ctx, sender, logger, consumerName, msg)
		default:
			return &poisonError{cause: fmt.Errorf("unhandled subject %q", msg.Subject())}
		}
	}
}

// poisonError marks a message that can never succeed (malformed payload).
// The consumer terminates it instead of retrying forever.
type poisonError struct {
	cause error
}

func (e *poisonError) Error() string { return e.cause.Error() }
func (e *poisonError) Unwrap() error { return e.cause }
