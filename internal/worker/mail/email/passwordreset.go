package email

import (
	"fmt"
	htemplate "html/template"
	"strings"
	ttemplate "text/template"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/messaging"
)

// passwordResetData feeds the password-reset email templates. The HTML body is
// rendered by html/template (contextual escaping); the plain-text body by
// text/template.
type passwordResetData struct {
	AppName   string
	Name      string
	ResetURL  string
	ExpiresAt string
}

const (
	passwordResetSubject  = "Reset your password"
	passwordResetHTMLTmpl = "templates/password_reset.html"
	passwordResetTextTmpl = "templates/password_reset.txt"
)

var (
	mustHTMLTemplate = ttemplateMust(htemplate.ParseFS(templateFS, passwordResetHTMLTmpl))
	mustTextTemplate = ttemplate.Must(ttemplate.ParseFS(templateFS, passwordResetTextTmpl))
)

// RenderPasswordReset builds subject/text/HTML bodies from the embedded
// templates. Dynamic values are escaped contextually; no manual concatenation
// of event fields into markup happens here.
func RenderPasswordReset(event messaging.PasswordResetRequested) (subject, text, htmlBody string) {
	name := strings.TrimSpace(event.Name)
	if name == "" {
		name = "there"
	}

	data := passwordResetData{
		AppName:   appDisplayName,
		Name:      name,
		ResetURL:  event.ResetURL,
		ExpiresAt: event.ExpiresAt.Format(time.RFC1123),
	}

	subject = passwordResetSubject
	text = renderText(mustTextTemplate,
		fmt.Sprintf("Hi %s,\n\nReset your password: %s\n", data.Name, data.ResetURL),
		data)
	htmlBody = renderHTML(mustHTMLTemplate, data.ResetURL, data)

	return subject, text, htmlBody
}
