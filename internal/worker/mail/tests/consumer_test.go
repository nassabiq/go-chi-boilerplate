package tests

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/messaging"
	mailsender "github.com/nassabiq/go-chi-boilerplate/internal/pkg/mail"
	mailworker "github.com/nassabiq/go-chi-boilerplate/internal/worker/mail"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeMsg struct {
	data []byte

	subject string

	mu         sync.Mutex
	acked      bool
	nakDelay   time.Duration
	terminated bool
}

func (m *fakeMsg) Data() []byte { return m.data }
func (m *fakeMsg) Subject() string {
	if m.subject != "" {
		return m.subject
	}
	return messaging.TopicPasswordResetRequested
}
func (m *fakeMsg) Reply() string        { return "" }
func (m *fakeMsg) Headers() nats.Header { return nil }
func (m *fakeMsg) Metadata() (*jetstream.MsgMetadata, error) {
	return &jetstream.MsgMetadata{Sequence: jetstream.SequencePair{Consumer: 1}}, nil
}
func (m *fakeMsg) Ack() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.acked = true
	return nil
}
func (m *fakeMsg) DoubleAck(context.Context) error { return nil }
func (m *fakeMsg) Nak() error                      { return nil }
func (m *fakeMsg) NakWithDelay(d time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nakDelay = d
	return nil
}
func (m *fakeMsg) InProgress() error           { return nil }
func (m *fakeMsg) Term() error                 { m.terminated = true; return nil }
func (m *fakeMsg) TermWithReason(string) error { m.terminated = true; return nil }

var _ jetstream.Msg = (*fakeMsg)(nil)

type fakeSender struct {
	mu    sync.Mutex
	err   error
	calls []mailsender.Message
}

func (s *fakeSender) Send(_ context.Context, msg mailsender.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, msg)
	return s.err
}

var _ mailworker.EmailSender = (*fakeSender)(nil)

func newTestWorker(sender mailworker.EmailSender) *mailworker.Worker {
	return mailworker.New(&mailworker.Dependencies{
		Sender: sender,
		Logger: testLogger(),
	})
}

func TestHandleValidEventSendsAndAcks(t *testing.T) {
	event := map[string]any{
		"event_id":   "evt-42",
		"user_id":    7,
		"email":      "user@example.com",
		"name":       "Ada",
		"reset_url":  "https://app.example.com/reset/tok",
		"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
	}
	msg := &fakeMsg{data: marshalEvent(t, event)}
	sender := &fakeSender{}
	w := newTestWorker(sender)

	w.Process(context.Background(), msg)

	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.calls) != 1 {
		t.Fatalf("sender calls = %d, want 1", len(sender.calls))
	}
	call := sender.calls[0]
	if call.To != "user@example.com" || call.Subject != "Reset your password" ||
		!strings.Contains(call.HTML, "https://app.example.com/reset/tok") ||
		!strings.Contains(call.Text, "https://app.example.com/reset/tok") {
		t.Errorf("unexpected message: %+v", call)
	}
	if !msg.acked {
		t.Error("message not ACKed after successful send")
	}
	if msg.nakDelay != 0 || msg.terminated {
		t.Error("message should neither be NAKed nor terminated on success")
	}
}

func TestHandleInvalidJSONTerminates(t *testing.T) {
	msg := &fakeMsg{data: []byte("not-json{")}
	sender := &fakeSender{}
	w := newTestWorker(sender)

	w.Process(context.Background(), msg)

	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.calls) != 0 {
		t.Error("sender must not be called for malformed JSON")
	}
	if !msg.terminated {
		t.Error("malformed message should be terminated, not retried")
	}
	if msg.acked || msg.nakDelay != 0 {
		t.Error("malformed message should not be ACKed or NAKed")
	}
}

func TestHandleMissingEventIDTerminates(t *testing.T) {
	event := map[string]any{"user_id": 7, "email": "x@y.z"}
	msg := &fakeMsg{data: marshalEvent(t, event)}
	sender := &fakeSender{}
	w := newTestWorker(sender)

	w.Process(context.Background(), msg)

	if !msg.terminated {
		t.Error("missing event_id is a poison message and must be terminated")
	}
}

func TestHandleSMTPFailureNaksWithDelay(t *testing.T) {
	event := map[string]any{
		"event_id":   "evt-9",
		"user_id":    1,
		"email":      "user@example.com",
		"name":       "Ada",
		"reset_url":  "https://app.example.com/reset/tok",
		"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
	}
	msg := &fakeMsg{data: marshalEvent(t, event)}
	sender := &fakeSender{err: errors.New("smtp connection refused")}
	w := newTestWorker(sender)

	w.Process(context.Background(), msg)

	if msg.acked {
		t.Error("message must NOT be ACKed when SMTP fails")
	}
	if msg.nakDelay != mailworker.DefaultRetryDelay {
		t.Errorf("nak delay = %v, want %v", msg.nakDelay, mailworker.DefaultRetryDelay)
	}
	if msg.terminated {
		t.Error("transient SMTP failure must not terminate the message")
	}
}

func TestNewDefaults(t *testing.T) {
	w := newTestWorker(&fakeSender{})

	cfg := w.Config()
	if cfg.ConsumerName != mailworker.DefaultConsumerName ||
		cfg.AckWait != mailworker.DefaultAckWait ||
		cfg.MaxDeliver != mailworker.DefaultMaxDeliver ||
		cfg.RetryDelay != mailworker.DefaultRetryDelay {
		t.Errorf("defaults wrong: %+v", cfg)
	}
}

func TestHandlePasswordChangedEventSendsAndAcks(t *testing.T) {
	event := map[string]any{
		"event_id":    "evt-pc-1",
		"user_id":     7,
		"email":       "user@example.com",
		"name":        "Ada",
		"reason":      "reset_password",
		"occurred_at": time.Now().Format(time.RFC3339),
	}
	msg := &fakeMsg{data: marshalEvent(t, event), subject: messaging.TopicPasswordChanged}
	sender := &fakeSender{}
	w := newTestWorker(sender)

	w.Process(context.Background(), msg)

	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.calls) != 1 {
		t.Fatalf("sender calls = %d, want 1", len(sender.calls))
	}
	call := sender.calls[0]
	if call.To != "user@example.com" || call.Subject != "Your password was changed" {
		t.Errorf("unexpected message: %+v", call)
	}
	if !msg.acked {
		t.Error("message not ACKed after successful send")
	}
}

func TestHandleEmailVerificationRequestedEventSendsAndAcks(t *testing.T) {
	event := map[string]any{
		"event_id":         "evt-ev-1",
		"user_id":          8,
		"email":            "verify@example.com",
		"name":             "Natsu",
		"verification_url": "https://app.example.com/verify?token=tok123",
		"expires_at":       time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	}
	msg := &fakeMsg{data: marshalEvent(t, event), subject: messaging.TopicEmailVerificationRequested}
	sender := &fakeSender{}
	w := newTestWorker(sender)

	w.Process(context.Background(), msg)

	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.calls) != 1 {
		t.Fatalf("sender calls = %d, want 1", len(sender.calls))
	}
	call := sender.calls[0]
	if call.To != "verify@example.com" || call.Subject != "Verify your email" {
		t.Errorf("unexpected message: %+v", call)
	}
	if !strings.Contains(call.HTML, "https://app.example.com/verify?token=tok123") {
		t.Errorf("expected verification URL in email HTML, got: %s", call.HTML)
	}
	if !msg.acked {
		t.Error("message not ACKed after successful send")
	}
}

func TestHandleEmailChangeRequestedEventSendsAndAcks(t *testing.T) {
	event := map[string]any{
		"event_id":         "evt-ec-1",
		"user_id":          8,
		"old_email":        "old@example.com",
		"new_email":        "new@example.com",
		"name":             "Natsu",
		"verification_url": "https://app.example.com/confirm-email?token=tok456",
		"expires_at":       time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	}
	msg := &fakeMsg{data: marshalEvent(t, event), subject: messaging.TopicEmailChanged}
	sender := &fakeSender{}
	w := newTestWorker(sender)

	w.Process(context.Background(), msg)

	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.calls) != 1 {
		t.Fatalf("sender calls = %d, want 1", len(sender.calls))
	}
	call := sender.calls[0]
	if call.To != "new@example.com" || call.Subject != "Confirm your new email address" {
		t.Errorf("unexpected message: %+v", call)
	}
	if !strings.Contains(call.HTML, "https://app.example.com/confirm-email?token=tok456") {
		t.Errorf("expected confirmation URL in email HTML, got: %s", call.HTML)
	}
	if !msg.acked {
		t.Error("message not ACKed after successful send")
	}
}

func TestHandleUnknownSubjectTerminates(t *testing.T) {
	event := map[string]any{
		"event_id": "evt-unknown",
	}
	msg := &fakeMsg{data: marshalEvent(t, event), subject: "unknown.topic.event"}
	sender := &fakeSender{}
	w := newTestWorker(sender)

	w.Process(context.Background(), msg)

	if !msg.terminated {
		t.Error("unknown subject is unrouteable and must be terminated")
	}
	if msg.acked {
		t.Error("unknown subject should not be acked")
	}
}

func marshalEvent(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
