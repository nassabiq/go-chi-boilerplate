package email

import (
	"fmt"
	htemplate "html/template"
	"strings"
	ttemplate "text/template"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/messaging"
)

// emailChangeData feeds the email-change verification templates. The NEW email
// is confirmed via the link; the trusted email is only swapped after the user
// clicks it.
type emailChangeData struct {
	AppName        string
	Name           string
	OldEmail       string
	NewEmail       string
	VerificationURL string
	ExpiresAt      string
}

const (
	emailChangeSubject   = "Confirm your new email address"
	emailChangeHTMLTmpl  = "templates/email_change.html"
	emailChangeTextTmpl  = "templates/email_change.txt"
)

var (
	mustEmailChangeHTMLTemplate = ttemplateMust(htemplate.ParseFS(templateFS, emailChangeHTMLTmpl))
	mustEmailChangeTextTemplate = ttemplate.Must(ttemplate.ParseFS(templateFS, emailChangeTextTmpl))
)

// RenderEmailChange builds subject/text/HTML bodies for the new-email
// verification email sent after an authenticated email-change request.
func RenderEmailChange(event messaging.EmailChangeRequested) (subject, text, htmlBody string) {
	name := strings.TrimSpace(event.Name)
	if name == "" {
		name = "there"
	}

	data := emailChangeData{
		AppName:        appDisplayName,
		Name:           name,
		OldEmail:       event.OldEmail,
		NewEmail:       event.NewEmail,
		VerificationURL: event.VerificationURL,
		ExpiresAt:      event.ExpiresAt.Format(time.RFC1123),
	}

	subject = emailChangeSubject
	text = renderText(mustEmailChangeTextTemplate,
		fmt.Sprintf("Hi %s,\n\nConfirm your new email: %s\n", data.Name, data.VerificationURL),
		data)
	htmlBody = renderHTML(mustEmailChangeHTMLTemplate, data.VerificationURL, data)

	return subject, text, htmlBody
}