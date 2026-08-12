//go:build integration
// +build integration

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/messaging"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/nats"
	"github.com/nats-io/nats.go/jetstream"
)

// TestNATSAppEventsStream verifies the real NATS client creates the APP_EVENTS
// stream capturing auth.> and that the publisher delivers a domain event that a
// consumer can read back. It requires a reachable NATS server (the bundled
// docker-compose provides one) and skips otherwise.
func TestNATSAppEventsStream(t *testing.T) {
	cfg := &config.Config{
		NATS: config.NATSConfig{
			URL: "nats://localhost:4222",
		},
	}

	client, err := nats.New(cfg)
	if err != nil {
		t.Skipf("NATS unavailable, skipping: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	strm, err := client.JetStream.Stream(ctx, nats.AppEventsStream)
	if err != nil {
		t.Fatalf("stream %s not found: %v", nats.AppEventsStream, err)
	}
	info, err := strm.Info(ctx)
	if err != nil {
		t.Fatalf("stream info: %v", err)
	}
	if len(info.Config.Subjects) == 0 || info.Config.Subjects[0] != nats.AuthSubjectWildcard {
		t.Fatalf("stream does not capture %s: %+v", nats.AuthSubjectWildcard, info.Config.Subjects)
	}

	consumerName := fmt.Sprintf("test-nats-%d", time.Now().UnixNano())
	cons, err := client.JetStream.CreateOrUpdateConsumer(ctx, nats.AppEventsStream, jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: messaging.TopicPasswordResetRequested,
		DeliverPolicy: jetstream.DeliverNewPolicy,
	})
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}
	t.Cleanup(func() {
		_ = client.JetStream.DeleteConsumer(ctx, nats.AppEventsStream, consumerName)
	})

	pub := messaging.NewPublisher(client)
	eventID := fmt.Sprintf("evt-nats-%d", time.Now().UnixNano())
	event := messaging.PasswordResetRequested{
		EventID:    eventID,
		UserID:     1,
		Email:      "nats@example.com",
		Name:       "Nats",
		ResetURL:   "https://app.example.com/reset/raw-token",
		ExpiresAt:  time.Now().Add(time.Hour),
		OccurredAt: time.Now(),
	}

	if err := pub.PublishPasswordResetRequested(ctx, event); err != nil {
		t.Fatalf("publish: %v", err)
	}

	msgs, err := cons.Messages()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	var msg jetstream.Msg
	type recv struct {
		msg jetstream.Msg
		err error
	}
	ch := make(chan recv, 1)
	go func() {
		m, e := msgs.Next()
		ch <- recv{m, e}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("receive: %v", r.err)
		}
		msg = r.msg
	case <-time.After(15 * time.Second):
		t.Fatalf("timed out waiting for published event")
	}

	if msg.Subject() != messaging.TopicPasswordResetRequested {
		t.Fatalf("unexpected subject: %q", msg.Subject())
	}

	var got messaging.PasswordResetRequested
	if err := json.Unmarshal(msg.Data(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.EventID != eventID || got.Email != event.Email || got.ResetURL != event.ResetURL {
		t.Fatalf("event mismatch: %+v", got)
	}
	if err := msg.Ack(); err != nil {
		t.Fatalf("ack: %v", err)
	}
}
