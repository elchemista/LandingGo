# Landing Page Server

A production-focused Go landing-page server that compiles to a single self-contained binary. Routes, pages, and static assets are driven by a JSON configuration file, packed into the binary with `go:embed`, and served with performance-oriented HTTP defaults.

## Features

- Config-driven routing with automatic validation.
- Build-time asset packer that scans HTML for local `/static/...` references, copies required assets, and emits a manifest with SHA-256 hashes for ETag support.
- Contact form endpoint that submits to Mailgun using configuration-provided credentials.
- Single binary distribution with embedded pages, static files, sitemap, robots, and error fallbacks.
- Runtime caching for templates and static assets with conditional GET handling (`ETag`/`Last-Modified`).
- Middleware stack providing panic recovery, structured logging, request IDs, and transparent gzip compression.
- Automatic `/sitemap.xml`, `/robots.txt`, and `/healthz` endpoints.
- Overrideable `404` and `500` pages (served from `web/pages/404.html` or `500.html` when present).

## Project Layout

```
cmd/
  landing/        # binary entrypoint
  pack/           # asset packer CLI
internal/
  assets/         # FS helpers, cache, packer library
  config/         # JSON schema parsing & validation
  errors/         # Embedded default error pages
  log/            # slog helper
  middleware/     # HTTP middleware stack
  pages/          # template manager
  robots/         # robots.txt generation
  router/         # lightweight router
  server/         # HTTP server wiring
  sitemap/        # sitemap.xml generation
web/              # authoring source for pages/static assets
build/            # generated artefacts (public/, embedded.go)
```

## Development

```bash
npm install            # install frontend toolchain (once)
make assets           # builds Tailwind + bundled JS into web/static/
make dev              # runs the server with live files from ./web
make test             # runs unit/integration tests
```

The asset pipeline is managed by `esbuild` and Tailwind via `npm run build`. Source files live under `assets/src/` and emit compiled bundles into `web/static/`, which the Go packer then embeds.

## Production Build

```bash
make pack              # generates build/public/, manifest.json, embedded.go
make build             # pack + go build -o bin/landing
./bin/landing --addr :8080 --config config.example.json
```

Environment variables or CLI flags can override defaults:

- `--config` (env: `CONFIG`) path to configuration JSON.
- `--addr` (env: `ADDR` or `PORT`) listener address. Defaults to `:8080`.
- `--dev` (env: `DEV`) serve directly from disk.
- `--log-level` (env: `LOG_LEVEL`) one of `debug`, `info`, `warn`, `error`.

## Configuration Schema

See [`config.example.json`](./config.example.json) for a reference configuration. Pages are resolved relative to `web/pages`. Only `/static/...` assets referenced from those pages are bundled during `make pack`.

## Contact Form

The `/contact` page posts `name`, `email`, and `message` to `/contact`. In production the handler builds a Mailgun message with those fields and calls the official [`mailgun-go`](https://github.com/mailgun/mailgun-go) client to send the email.

Configuration lives under the top-level `contact` key:

```json
"contact": {
  "recipient": "owners@example.com",
  "from": "Landing Page <no-reply@example.com>",
  "subject": "New website enquiry",
  "mailgun": {
    "domain": "mg.example.com",
    "api_key": "key-your-mailgun-api-key"
  }
}
```

- `recipient` is the email address that receives contact messages.
- `from` is the Mailgun-verified sender shown to recipients (it can include a display name).
- `subject` is optional; it defaults to `New contact from <name>` if omitted.
- `mailgun.domain` is the Mailgun-supplied sending domain (e.g. `mg.example.com`).
- `mailgun.api_key` is the private API key with permission to send mail.

In production builds the server creates a Mailgun client with those credentials and queues the message. In `--dev` mode the contact handler remains active, but without a `contact` block POST requests return `503 Service Unavailable` so you can work without real credentials. Omit the `contact` block entirely to disable outbound email.

For deployments keep API keys out of version control—inject them via environment-specific config files or secret management tooling, then run `make pack && make build` to bake the configuration into the binary.

## Notes

- Regenerate assets (`make pack`) any time pages, static files, or the config changes before running `make build`.
- The generated `build/embedded.go` and `build/public/` should not be committed; they are ignored via `.gitignore`.
- Binary builds include all assets and can run on machines without access to the original `web/` directory.
