package errors

const (
	// Default404 is used when no custom 404 page is provided.
	Default404 = `<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><title>Page Not Found</title><meta name="robots" content="noindex"></head><body><h1>404 Not Found</h1><p>The requested page could not be found.</p></body></html>`

	// Default500 is used when no custom 500 page is provided.
	Default500 = `<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><title>Server Error</title><meta name="robots" content="noindex"></head><body><h1>500 Internal Server Error</h1><p>Something went wrong.</p></body></html>`
)
