package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"webgo/internal/assets"
	"webgo/internal/config"
)

func TestServerHandlers(t *testing.T) {
	cfg, src := setupTestEnvironment(t)

	srv, err := New(cfg, src, nil, true)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	srv.router.Handle("/boom", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	t.Run("page", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/")
		if err != nil {
			t.Fatalf("get /: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
			t.Fatalf("unexpected content type: %s", ct)
		}

		if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=300" {
			t.Fatalf("unexpected cache-control: %s", cc)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Home") {
			t.Fatalf("expected body to contain title, got %q", body)
		}
	})

	t.Run("etag", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/static/app.css")
		if err != nil {
			t.Fatalf("get static: %v", err)
		}
		resp.Body.Close()

		etag := resp.Header.Get("ETag")
		if etag == "" {
			t.Fatalf("missing ETag in response")
		}

		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/static/app.css", nil)
		req.Header.Set("If-None-Match", etag)

		resp2, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("conditional get: %v", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusNotModified {
			t.Fatalf("expected 304, got %d", resp2.StatusCode)
		}
	})

	t.Run("not found", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/missing")
		if err != nil {
			t.Fatalf("get missing: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}

		if resp.Header.Get("Cache-Control") != "no-store, max-age=0" {
			t.Fatalf("expected no-store cache control")
		}
	})

	t.Run("panic", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/boom")
		if err != nil {
			t.Fatalf("get boom: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", resp.StatusCode)
		}

		if resp.Header.Get("Cache-Control") != "no-store, max-age=0" {
			t.Fatalf("expected no-store cache control on 500")
		}
	})

	t.Run("sitemap and robots", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/sitemap.xml")
		if err != nil {
			t.Fatalf("sitemap: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for sitemap, got %d", resp.StatusCode)
		}

		resp2, err := http.Get(ts.URL + "/robots.txt")
		if err != nil {
			t.Fatalf("robots: %v", err)
		}
		resp2.Body.Close()
		if resp2.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for robots, got %d", resp2.StatusCode)
		}
	})
}

func setupTestEnvironment(t *testing.T) (*config.Config, *assets.Source) {
	t.Helper()
	tdir := t.TempDir()
	webDir := filepath.Join(tdir, "web")

	mustWrite(t, filepath.Join(webDir, "pages", "home.html"), `<!doctype html><html><head><link rel="stylesheet" href="/static/app.css"></head><body><h1>Home</h1></body></html>`)
	mustWrite(t, filepath.Join(webDir, "static", "app.css"), "body { color: #000; }")

	cfg := &config.Config{
		Site:   config.Site{BaseURL: "https://example.test"},
		Routes: []config.Route{{Path: "/", Page: "home.html", Title: "Home"}},
	}
	cfg.WithLoadedTime(time.Now())

	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(webDir, "pages", name))
		return err == nil
	}

	if err := cfg.Validate(exists); err != nil {
		t.Fatalf("validate config: %v", err)
	}

	src, err := assets.NewDisk(webDir)
	if err != nil {
		t.Fatalf("new disk source: %v", err)
	}

	return cfg, src
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
