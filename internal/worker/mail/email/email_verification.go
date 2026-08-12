package email

import (
	"fmt"
	htemplate "html/template"
	"strings"
	ttemplate "text/template"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/messaging"
)

// emailVerificationData feeds the email-verification templates.
type emailVerificationData struct {
	AppName       string
	Name          string
	VerificationURL string
	ExpiresAt     string
}

const (
	emailVerificationSubject   = "Verify your email"
	emailVerificationHTMLTmpl  = "templates/email_verification.html"
	emailVerificationTextTmpl  = "templates/email_verification.txt"
)

var (
	mustEmailVerificationHTMLTemplate = ttemplateMust(htemplate.ParseFS(templateFS, emailVerificationHTMLTmpl))
	mustEmailVerificationTextTemplate = ttemplate.Must(ttemplate.ParseFS(templateFS, emailVerificationTextTmpl))
)

// RenderEmailVerification builds subject/text/HTML bodies for the email
// verification email sent after registration or resend.
func RenderEmailVerification(event messaging.EmailVerificationRequested) (subject, text, htmlBody string) {
	name := strings.TrimSpace(event.Name)
	if name == "" {
		name = "there"
	}

	data := emailVerificationData{
		AppName:        appDisplayName,
		Name:           name,
		VerificationURL: event.VerificationURL,
		ExpiresAt:      event.ExpiresAt.Format(time.RFC1123),
	}

	subject = emailVerificationSubject
	text = renderText(mustEmailVerificationTextTemplate,
		fmt.Sprintf("Hi %s,\n\nVerify your email: %s\n", data.Name, data.VerificationURL),
		data)
	htmlBody = renderHTML(mustEmailVerificationHTMLTemplate, data.VerificationURL, data)

	return subject, text, htmlBody
}