package packer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"webgo/internal/assets"
)

func TestCollectAssets(t *testing.T) {
	html := []byte(`<!doctype html><html><head>
<link rel="stylesheet" href="/static/app.css">
<link rel="icon" href="/static/favicon.ico">
<link rel="canonical" href="https://example.com/">
</head><body>
<img src="/static/img/logo.png" srcset="/static/img/logo.png 1x, https://cdn.example.com/logo@2x.png 2x">
<script src="/static/app.js"></script>
<video src="/static/video.mp4" poster="/static/poster.jpg"></video>
</body></html>`)

	assets := collectAssets(html)
	expected := []string{
		"static/app.css",
		"static/app.js",
		"static/favicon.ico",
		"static/img/logo.png",
		"static/poster.jpg",
		"static/video.mp4",
	}

	if len(assets) != len(expected) {
		t.Fatalf("expected %d assets, got %d: %#v", len(expected), len(assets), assets)
	}

	for i, asset := range assets {
		if asset != expected[i] {
			t.Fatalf("asset mismatch at %d: want %s got %s", i, expected[i], asset)
		}
	}
}

func TestRunGeneratesManifestAndEmbed(t *testing.T) {
	tdir := t.TempDir()
	webDir := filepath.Join(tdir, "web")
	buildDir := filepath.Join(tdir, "build")

	mustMkdir(t, filepath.Join(webDir, "pages"))
	mustMkdir(t, filepath.Join(webDir, "static"))

	writeFile(t, filepath.Join(webDir, "pages", "home.html"), `<!doctype html><html><head><link rel="stylesheet" href="/static/app.css"></head><body><img src="/static/img.png"></body></html>`)
	writeFile(t, filepath.Join(webDir, "static", "app.css"), "body{}")
	writeFile(t, filepath.Join(webDir, "static", "img.png"), "PNG")

	configPath := filepath.Join(tdir, "config.json")
	writeFile(t, configPath, `{
  "site": {"base_url": "https://example.com"},
  "routes": [{"path": "/", "page": "home.html", "title": "Home"}]
}`)

	if err := Run(configPath, webDir, buildDir); err != nil {
		t.Fatalf("packer run: %v", err)
	}

	manifestPath := filepath.Join(buildDir, "public", assets.ManifestFilename)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var manifest assets.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}

	if len(manifest.Files) != 3 {
		t.Fatalf("expected 3 manifest entries, got %d", len(manifest.Files))
	}

	if _, ok := manifest.Files["pages/home.html"]; !ok {
		t.Fatalf("manifest missing page entry: %+v", manifest.Files)
	}
	if _, ok := manifest.Files["static/app.css"]; !ok {
		t.Fatalf("manifest missing css entry")
	}
	if _, ok := manifest.Files["static/img.png"]; !ok {
		t.Fatalf("manifest missing img entry")
	}

	if _, err := os.Stat(filepath.Join(buildDir, "embedded.go")); err != nil {
		t.Fatalf("embedded.go not generated: %v", err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
