package email

import (
	"embed"
	"fmt"
	htemplate "html/template"
	"strings"
	ttemplate "text/template"
)

//go:embed templates/*.html templates/*.txt
var templateFS embed.FS

// appDisplayName is the product name injected into email templates.
const appDisplayName = "Boilerplate"

// ttemplateMust panics if an embedded template fails to parse at startup.
// A bad template is a programming error, not a runtime condition, so failing
// fast keeps the worker from starting with a broken email pipeline.
func ttemplateMust(t *htemplate.Template, err error) *htemplate.Template {
	if err != nil {
		panic(fmt.Sprintf("mail worker: parse embedded template: %v", err))
	}
	return t
}

// renderText executes a text/template into a string, falling back to a minimal
// hand-built body if execution somehow fails (it should not: templates are
// parsed at startup).
func renderText(tmpl *ttemplate.Template, fallback string, data any) string {
	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		return fallback
	}
	return b.String()
}

// renderHTML executes an html/template into a string, falling back to the
// given minimal content if execution fails.
func renderHTML(tmpl *htemplate.Template, fallback string, data any) string {
	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		return fallback
	}
	return b.String()
}
