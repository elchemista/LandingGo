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
  <title>404 - Page not found</title>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="robots" content="noindex">
  <link rel="stylesheet" href="/static/app.css">
</head>
<body class="w-full h-screen">
  <div class="bg-gradient-to-r from-slate-200 to-gray-200 dark:from-gray-800 dark:to-gray-900 text-black dark:text-white">
    <div class="flex items-center justify-center min-h-screen px-2">
      <div class="text-center">
        <h1 class="text-9xl font-bold">404</h1>
        <p class="text-2xl font-medium mt-4">Oops! Page not found</p>
        <p class="mt-4 mb-8">The page you're looking for doesn't exist or has been moved.</p>
        <a href="/"
          class="px-6 py-3 bg-white font-bold rounded-full hover:bg-purple-100 transition duration-300 ease-in-out dark:bg-gray-700 dark:hover:bg-gray-600 dark:text-white">
          Go Home
        </a>
      </div>
    </div>
  </div>
</body>
</html>
`
	default500Source = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>500 - Internal Server Error</title>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="robots" content="noindex">
  <link rel="stylesheet" href="/static/app.css">
</head>
<body class="w-full h-screen">
  <div class="bg-gradient-to-r from-slate-200 to-gray-200 dark:from-gray-800 dark:to-gray-900 text-black dark:text-white">
    <div class="h-screen flex flex-col justify-center items-center">
      <h1 class="text-8xl font-extrabold text-red-500">500</h1>
      <p class="text-4xl font-medium text-gray-800">Internal Server Error</p>
      <p class="text-xl text-gray-800 mt-4">We apologize for the inconvenience. Please try again later.</p>
    </div>
  </div>
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
