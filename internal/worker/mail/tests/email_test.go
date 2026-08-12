package tests

import (
	"html"
	"strings"
	"testing"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/messaging"
	email "github.com/nassabiq/go-chi-boilerplate/internal/worker/mail/email"
)

func testEvent() messaging.PasswordResetRequested {
	return messaging.PasswordResetRequested{
		EventID:    "evt-1",
		UserID:     42,
		Email:      "user@example.com",
		Name:       "Ada <Lovelace>",
		ResetURL:   "https://app.example.com/reset?token=raw-secret-token&x=1",
		ExpiresAt:  time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC),
		OccurredAt: time.Now(),
	}
}

func TestRenderPasswordResetText(t *testing.T) {
	event := testEvent()
	_, text, _ := email.RenderPasswordReset(event)

	for _, want := range []string{
		"reset",
		event.ResetURL,
		"Fri, 02 Jan 2026 03:04:00 UTC",
		"ignore this email",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text body missing %q", want)
		}
	}
}

func TestRenderPasswordResetHTML(t *testing.T) {
	event := testEvent()
	subject, _, body := email.RenderPasswordReset(event)

	if subject != "Reset your password" {
		t.Errorf("subject = %q", subject)
	}

	for _, want := range []string{
		`<a href="` + html.EscapeString(event.ResetURL) + `"`,
		"Fri, 02 Jan 2026 03:04:00 UTC",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("html body missing %q", want)
		}
	}
}

func TestRenderPasswordResetHTMLEscapesName(t *testing.T) {
	event := testEvent()
	event.Name = `Ada <script>alert("x")</script>`
	_, _, body := email.RenderPasswordReset(event)

	if strings.Contains(body, "<script>") {
		t.Error("unescaped <script> found in HTML body")
	}
	if !strings.Contains(body, "Ada &lt;script&gt;") {
		t.Error("name not escaped in HTML body")
	}
}

func TestRenderPasswordResetHTMLEscapesURL(t *testing.T) {
	event := testEvent()
	event.ResetURL = `https://app.example.com/reset?a=" onclick="evil`
	_, _, body := email.RenderPasswordReset(event)

	if strings.Contains(body, `" onclick="evil`) {
		t.Error("unescaped quote in href; URL not escaped")
	}
	if !strings.Contains(body, "a=&#34;") && !strings.Contains(body, "%22") {
		t.Error("expected escaped quotes in HTML body")
	}
}

func TestRenderPasswordResetNoSecretMaterial(t *testing.T) {
	event := testEvent()
	event.Name = "token-hash-value password-hash"

	subject, text, html := email.RenderPasswordReset(event)

	for _, body := range []string{subject, text, html} {
		for _, bad := range []string{"token_hash", "access_token", "refresh_token", "Bearer "} {
			if strings.Contains(body, bad) {
				t.Errorf("body contains sensitive material %q", bad)
			}
		}
	}
}

func TestRenderPasswordResetEmptyNameFallback(t *testing.T) {
	event := testEvent()
	event.Name = ""
	_, text, body := email.RenderPasswordReset(event)

	if !strings.Contains(text, "Hi there,") || !strings.Contains(body, "<strong>there</strong>") {
		t.Error("empty name should fall back to greeting without name injection")
	}
}
