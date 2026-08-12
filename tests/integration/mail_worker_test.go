//go:build integration
// +build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/messaging"
	mailsender "github.com/nassabiq/go-chi-boilerplate/internal/pkg/mail"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/nats"
	mailworker "github.com/nassabiq/go-chi-boilerplate/internal/worker/mail"
)

type countingSender struct {
	mu     sync.Mutex
	calls  int
	err    error
	bodies []string
}

func (s *countingSender) Send(_ context.Context, msg mailsender.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.bodies = append(s.bodies, msg.Text)
	return s.err
}

// countFor counts deliveries whose body contains marker.
func (s *countingSender) countFor(marker string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, b := range s.bodies {
		if strings.Contains(b, marker) {
			n++
		}
	}
	return n
}

func (s *countingSender) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func getNATSClient(t *testing.T) *nats.Client {
	t.Helper()
	cfg := &config.Config{NATS: config.NATSConfig{URL: "nats://localhost:4222"}}
	client, err := nats.New(cfg)
	if err != nil {
		t.Skipf("NATS unavailable, skipping: %v", err)
	}
	return client
}

func publishResetEvent(t *testing.T, client *nats.Client, eventID string) {
	t.Helper()
	pub := messaging.NewPublisher(client)
	err := pub.PublishPasswordResetRequested(context.Background(), messaging.PasswordResetRequested{
		EventID:    eventID,
		UserID:     1,
		Email:      "worker@example.com",
		Name:       "Worker Test",
		ResetURL:   "https://app.example.com/reset/" + eventID,
		ExpiresAt:  time.Now().Add(time.Hour),
		OccurredAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
}

// TestMailWorkerProcessesResetEvent verifies the durable mail-password-reset-v1
// consumer receives a published PasswordResetRequested event and ACKs it after
// the sender succeeds.
func TestMailWorkerProcessesResetEvent(t *testing.T) {
	client := getNATSClient(t)
	defer client.Close()

	durable := fmt.Sprintf("test-mail-ok-%d", time.Now().UnixNano())

	sender := &countingSender{}
	worker := mailworker.New(&mailworker.Dependencies{
		NATS:         client,
		Sender:       sender,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		ConsumerName: durable,
	})

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- worker.Run(runCtx) }()

	time.Sleep(500 * time.Millisecond)

	eventMarker := fmt.Sprintf("evt-worker-%d", time.Now().UnixNano())
	publishResetEvent(t, client, eventMarker)

	deadline := time.After(15 * time.Second)
	for sender.countFor(eventMarker) == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for worker to process event")
		case <-time.After(50 * time.Millisecond):
		}
	}
	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("worker run: %v", err)
	}
	if got := sender.countFor(eventMarker); got != 1 {
		t.Fatalf("deliveries for this event = %d, want 1", got)
	}
}

// TestMailWorkerRedeliversOnSMTPFailure verifies a failing sender NAKs the
// message and JetStream redelivers until MaxDeliver is exhausted.
func TestMailWorkerRedeliversOnSMTPFailure(t *testing.T) {
	client := getNATSClient(t)
	defer client.Close()

	durable := fmt.Sprintf("test-mail-fail-%d", time.Now().UnixNano())

	const maxDeliver = 3
	sender := &countingSender{err: errors.New("smtp unavailable")}
	worker := mailworker.New(&mailworker.Dependencies{
		NATS:         client,
		Sender:       sender,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		ConsumerName: durable,
		AckWait:      5 * time.Second,
		MaxDeliver:   maxDeliver,
		RetryDelay:   200 * time.Millisecond,
	})

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = worker.Run(runCtx) }()

	time.Sleep(500 * time.Millisecond)

	eventMarker := fmt.Sprintf("evt-fail-%d", time.Now().UnixNano())
	publishResetEvent(t, client, eventMarker)

	deadline := time.After(20 * time.Second)
	for sender.countFor(eventMarker) < maxDeliver {
		select {
		case <-deadline:
			t.Fatalf("sender calls = %d, want %d", sender.count(), maxDeliver)
		case <-time.After(50 * time.Millisecond):
		}
	}

	time.Sleep(2 * time.Second)
	if got := sender.countFor(eventMarker); got > maxDeliver {
		t.Fatalf("deliveries for this event = %d, want <= %d (MaxDeliver not honored)", got, maxDeliver)
	}
	cancel()
}
