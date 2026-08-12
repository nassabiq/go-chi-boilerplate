package email

import (
	"fmt"
	htemplate "html/template"
	"strings"
	ttemplate "text/template"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/messaging"
)

// passwordChangedData feeds the password-changed security-notification
// templates. The HTML body is rendered by html/template (contextual escaping);
// the plain-text body by text/template.
type passwordChangedData struct {
	AppName    string
	Name       string
	OccurredAt string
}

const (
	passwordChangedSubject   = "Your password was changed"
	passwordChangedHTMLTmpl  = "templates/password_changed.html"
	passwordChangedTextTmpl  = "templates/password_changed.txt"
)

var (
	mustChangedHTMLTemplate = ttemplateMust(htemplate.ParseFS(templateFS, passwordChangedHTMLTmpl))
	mustChangedTextTemplate = ttemplate.Must(ttemplate.ParseFS(templateFS, passwordChangedTextTmpl))
)

// RenderPasswordChanged builds subject/text/HTML bodies for the security
// notification sent after a password change. Dynamic values are escaped
// contextually; no manual concatenation of event fields into markup happens
// here.
func RenderPasswordChanged(event messaging.PasswordChanged) (subject, text, htmlBody string) {
	name := strings.TrimSpace(event.Name)
	if name == "" {
		name = "there"
	}

	data := passwordChangedData{
		AppName:    appDisplayName,
		Name:       name,
		OccurredAt: event.OccurredAt.Format(time.RFC1123),
	}

	subject = passwordChangedSubject
	text = renderText(mustChangedTextTemplate,
		fmt.Sprintf("Hi %s,\n\nYour password was changed at %s.\n", data.Name, data.OccurredAt),
		data)
	htmlBody = renderHTML(mustChangedHTMLTemplate, data.OccurredAt, data)

	return subject, text, htmlBody
}
