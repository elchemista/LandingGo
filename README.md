# Landing Page Server in One Binary

A production-focused Go landing-page server that compiles to a single self-contained binary. Routes, pages, and static assets are driven by a JSON configuration file, packed into the binary with `go:embed`, and served with performance-oriented HTTP defaults.

## Why this exists

Clients often ask me for a **simple landing page**. I *could* spin it up on Deno Deploy / Vercel / Netlify with Svelte or Next.js—and I’ve done that plenty. But after a few projects, I kept migrating those sites to something **simpler and faster**: a tiny, memory-efficient **Go web server**.

I run it as a **single binary**, drop it on a **$5 VPS** or a **Fly.io** container, and… that’s it. No Node runtime, no framework updates, no build pipeline surprises, no cold starts. Just **predictable performance**, tiny memory, and fewer moving parts I need to babysit.

### TL;DR

* **Simplicity wins:** one binary, no complex runtime or adapter stack.
* **Speed & reliability:** server-rendered HTML, assets embedded, zero cold start.
* **Low cost:** runs comfortably on a $5 droplet or the smallest Fly.io machine.
* **Less to maintain:** fewer dependencies = fewer breakages.
* **Portability:** copy the binary anywhere; same behavior, same perf.

### When this approach shines

* You need a **fast, stable landing page** with a contact form, sitemap/robots, and basic routes.
* You want to **set it and forget it**—no constant framework churn.
* You care about **TTFB and tiny memory** more than heavy client-side interactivity.

### When not to use it

* You need a complex SPA, heavy client hydration, or a large UI ecosystem (then Svelte/Next can be a better fit).

For my use case—and many client landing pages—**Go is simpler**, **cheaper**, and **faster**. So I built this project to make that path easy and repeatable.


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
    "api_key": ""
  }
}
```

- `recipient` is the email address that receives contact messages.
- `from` is the Mailgun-verified sender shown to recipients (it can include a display name).
- `subject` is optional; it defaults to `New contact from <name>` if omitted.
- `mailgun.domain` is the Mailgun-supplied sending domain (e.g. `mg.example.com`).
- `mailgun.api_key` can be left blank; the server reads the private key from the `MAILGUN_API_KEY` environment variable at runtime.

In production builds the server creates a Mailgun client using the configuration plus the `MAILGUN_API_KEY` environment variable; set that secret via Fly.io or your process supervisor. In `--dev` mode the contact handler remains active, but without a `contact` block POST requests return `503 Service Unavailable` so you can work without real credentials. Omit the `contact` block entirely to disable outbound email.

For deployments keep API keys out of version control—inject them via environment-specific config files or secret management tooling, then run `make pack && make build` to bake the configuration into the binary.

## Notes

- Regenerate assets (`make pack`) any time pages, static files, or the config changes before running `make build`.
- The generated `build/embedded.go` and `build/public/` should not be committed; they are ignored via `.gitignore`.
- Binary builds include all assets and can run on machines without access to the original `web/` directory.
