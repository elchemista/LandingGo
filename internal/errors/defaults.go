package errors

import (
	"bytes"
	"html/template"
	"strings"

	"github.com/elchemista/LandingGo/internal/pages"
)

const (
	default404Source = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Not Found</title>
  <meta name="robots" content="noindex">
  <link rel="stylesheet" href="/static/app.css">
</head>
<body class="space-y-6">
  <h1 class="text-3xl font-bold text-rose-600">Custom 404</h1>
  <p>No content lives at <code>{{.RoutePath}}</code>.</p>
  <a class="nav-link" href="/">Return home</a>
</body>
</html>
`
	default500Source = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Server Error</title>
  <meta name="robots" content="noindex">
  <link rel="stylesheet" href="/static/app.css">
</head>
<body class="space-y-6">
  <h1 class="text-3xl font-bold text-rose-600">Custom 500</h1>
  <p>We hit a snag processing <code>{{.RoutePath}}</code>. Please try again later.</p>
</body>
</html>
`
)

var (
	default404Template = parseTemplate("404.html", default404Source)
	default500Template = parseTemplate("500.html", default500Source)
)

const (
	fallback404 = `<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><title>Page Not Found</title><meta name="robots" content="noindex"></head><body><h1>404 Not Found</h1><p>The requested page could not be found.</p></body></html>`
	fallback500 = `<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><title>Server Error</title><meta name="robots" content="noindex"></head><body><h1>500 Internal Server Error</h1><p>Something went wrong.</p></body></html>`
)

// Default404 renders the embedded 404 template using the provided page data.
func Default404(data pages.PageData) []byte {
	return renderTemplate(default404Template, data, fallback404)
}

// Default500 renders the embedded 500 template using the provided page data.
func Default500(data pages.PageData) []byte {
	return renderTemplate(default500Template, data, fallback500)
}

func renderTemplate(tmpl *template.Template, data pages.PageData, fallback string) []byte {
	if tmpl == nil {
		return []byte(fallback)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return []byte(fallback)
	}

	return buf.Bytes()
}

func parseTemplate(name, src string) *template.Template {
	if strings.TrimSpace(src) == "" {
		return nil
	}

	tmpl, err := template.New(name).
		Option("missingkey=zero").
		Parse(src)
	if err != nil {
		return nil
	}

	return tmpl
}
