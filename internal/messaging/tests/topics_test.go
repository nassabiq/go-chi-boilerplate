package tests

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/messaging"
)

func TestTopicsAreDomainFirstAndVersioned(t *testing.T) {
	if messaging.TopicPasswordResetRequested != "auth.password-reset.requested.v1" {
		t.Fatalf("unexpected reset topic: %q", messaging.TopicPasswordResetRequested)
	}
	if messaging.TopicPasswordChanged != "auth.password-changed.v1" {
		t.Fatalf("unexpected changed topic: %q", messaging.TopicPasswordChanged)
	}
	if !strings.HasPrefix(messaging.TopicPasswordResetRequested, "auth.") {
		t.Fatalf("topic must be domain-first (auth.*): %q", messaging.TopicPasswordResetRequested)
	}
}

func TestPasswordResetRequestedContract(t *testing.T) {
	now := time.Now()
	event := messaging.PasswordResetRequested{
		EventID:    "evt-1",
		UserID:     42,
		Email:      "user@example.com",
		Name:       "User",
		ResetURL:   "https://app.example.com/reset/raw-token-xyz",
		ExpiresAt:  now,
		OccurredAt: now,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := string(data)

	// Required fields present.
	for _, want := range []string{"event_id", "user_id", "email", "name", "reset_url", "expires_at", "occurred_at"} {
		if !strings.Contains(raw, `"`+want+`"`) {
			t.Fatalf("event json missing field %q: %s", want, raw)
		}
	}

	// Sensitive material must never appear in the contract.
	for _, forbidden := range []string{"password", "password_hash", "access_token", "refresh_token", "token_hash"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("event must not contain %q: %s", forbidden, raw)
		}
	}

	// Round-trip preserves content (ignore the monotonic clock component that
	// JSON does not serialize).
	var decoded messaging.PasswordResetRequested
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.EventID != event.EventID ||
		decoded.UserID != event.UserID ||
		decoded.Email != event.Email ||
		decoded.Name != event.Name ||
		decoded.ResetURL != event.ResetURL ||
		!decoded.ExpiresAt.Equal(event.ExpiresAt) ||
		!decoded.OccurredAt.Equal(event.OccurredAt) {
		t.Fatalf("round-trip mismatch: %+v != %+v", decoded, event)
	}
}

func TestPasswordChangedContractHasNoSecret(t *testing.T) {
	event := messaging.PasswordChanged{
		EventID:    "evt-2",
		UserID:     7,
		Email:      "u@example.com",
		Name:       "U",
		OccurredAt: time.Now(),
	}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := string(data)
	for _, forbidden := range []string{"password", "password_hash", "access_token", "refresh_token", "reset_url", "token_hash"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("password-changed event must not contain %q: %s", forbidden, raw)
		}
	}
}
