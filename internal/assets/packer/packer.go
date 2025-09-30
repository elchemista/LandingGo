package packer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/elchemista/LandingGo/internal/assets"
	"github.com/elchemista/LandingGo/internal/config"
)

// Run executes the asset packing pipeline.
func Run(configPath, webDir, buildDir string) error {
	opts := options{
		configPath: configPath,
		webDir:     webDir,
		buildDir:   buildDir,
	}

	return opts.run()
}

type options struct {
	configPath string
	webDir     string
	buildDir   string
}

func (o *options) run() error {
	o.applyDefaults()

	cfg, err := config.Load(o.configPath)
	if err != nil {
		return err
	}

	pagesDir := filepath.Join(o.webDir, "pages")
	if err := cfg.Validate(func(name string) bool {
		_, err := os.Stat(filepath.Join(pagesDir, name))
		return err == nil
	}); err != nil {
		return err
	}

	publicDir := filepath.Join(o.buildDir, "public")
	if err := os.RemoveAll(publicDir); err != nil {
		return fmt.Errorf("clean build directory: %w", err)
	}
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		return fmt.Errorf("create build directory: %w", err)
	}

	now := time.Now().UTC()
	manifest := assets.Manifest{
		GeneratedAt: now,
		Files:       make(map[string]assets.ManifestEntry),
	}

	assetSet := make(map[string]struct{})
	pageSet := uniquePages(cfg)

	for _, page := range pageSet {
		src := filepath.Join(o.webDir, "pages", page)
		dst := filepath.Join(publicDir, "pages", page)

		info, err := os.Stat(src)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("stat page %s: %w", page, err)
		}

		if err := copyFile(src, dst); err != nil {
			return err
		}

		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read page %s: %w", page, err)
		}

		for _, asset := range collectAssets(data) {
			assetSet[asset] = struct{}{}
		}

		modTime := info.ModTime().UTC()
		addManifestEntry(&manifest, filepath.ToSlash(filepath.Join("pages", page)), data, modTime)
	}

	for assetPath := range assetSet {
		src := filepath.Join(o.webDir, filepath.FromSlash(assetPath))
		dst := filepath.Join(publicDir, filepath.FromSlash(assetPath))

		info, err := os.Stat(src)
		if err != nil {
			return fmt.Errorf("stat asset %s: %w", assetPath, err)
		}

		if err := copyFile(src, dst); err != nil {
			return err
		}

		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read asset %s: %w", assetPath, err)
		}

		addManifestEntry(&manifest, assetPath, data, info.ModTime().UTC())
	}

	manifestPath := filepath.Join(publicDir, assets.ManifestFilename)
	if err := writeManifest(manifestPath, &manifest); err != nil {
		return err
	}

	if err := writeEmbeddedFile(o.buildDir); err != nil {
		return err
	}

	return nil
}

func (o *options) applyDefaults() {
	if strings.TrimSpace(o.configPath) == "" {
		o.configPath = "config.example.json"
	}
	if strings.TrimSpace(o.webDir) == "" {
		o.webDir = "web"
	}
	if strings.TrimSpace(o.buildDir) == "" {
		o.buildDir = "build"
	}
}

func uniquePages(cfg *config.Config) []string {
	pages := make(map[string]struct{})
	for _, route := range cfg.Routes {
		pages[route.Page] = struct{}{}
	}

	// Ensure error overrides are included if present.
	for _, name := range []string{"404.html", "500.html"} {
		pages[name] = struct{}{}
	}

	list := make([]string, 0, len(pages))
	for page := range pages {
		if page == "" {
			continue
		}
		list = append(list, page)
	}

	sort.Strings(list)

	return list
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", dst, err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}

	return nil
}

func collectAssets(htmlBytes []byte) []string {
	node, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return nil
	}

	assets := make(map[string]struct{})

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			tag := strings.ToLower(n.Data)
			switch tag {
			case "link":
				ref := getAttr(n, "href")
				if ref != "" {
					if normalized, ok := normalizeAssetPath(ref); ok {
						assets[normalized] = struct{}{}
					}
				}
			case "script", "img", "source", "video", "audio", "track", "iframe", "image", "use":
				if ref := getAttr(n, "src"); ref != "" {
					if normalized, ok := normalizeAssetPath(ref); ok {
						assets[normalized] = struct{}{}
					}
				}
				if tag == "video" {
					if poster := getAttr(n, "poster"); poster != "" {
						if normalized, ok := normalizeAssetPath(poster); ok {
							assets[normalized] = struct{}{}
						}
					}
				}
				if srcset := getAttr(n, "srcset"); srcset != "" {
					for _, ref := range parseSrcSet(srcset) {
						if normalized, ok := normalizeAssetPath(ref); ok {
							assets[normalized] = struct{}{}
						}
					}
				}
			case "meta":
				if name := strings.ToLower(getAttr(n, "property")); name == "og:image" || name == "twitter:image" {
					if content := getAttr(n, "content"); content != "" {
						if normalized, ok := normalizeAssetPath(content); ok {
							assets[normalized] = struct{}{}
						}
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	walk(node)

	list := make([]string, 0, len(assets))
	for asset := range assets {
		list = append(list, asset)
	}

	sort.Strings(list)

	return list
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, key) {
			return strings.TrimSpace(attr.Val)
		}
	}
	return ""
}

func parseSrcSet(srcset string) []string {
	parts := strings.Split(srcset, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Fields(part)
		if len(fields) > 0 {
			out = append(out, fields[0])
		}
	}
	return out
}

func normalizeAssetPath(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false
	}

	lower := strings.ToLower(path)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "//") {
		return "", false
	}
	if strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "mailto:") {
		return "", false
	}
	if strings.HasPrefix(path, "#") {
		return "", false
	}

	if idx := strings.IndexAny(path, "?#"); idx >= 0 {
		path = path[:idx]
	}

	path = strings.TrimPrefix(path, "/")
	path = strings.TrimPrefix(path, "./")

	for strings.HasPrefix(path, "../") {
		path = strings.TrimPrefix(path, "../")
	}

	path = filepath.ToSlash(path)

	if !strings.HasPrefix(path, "static/") {
		return "", false
	}

	return path, true
}

func addManifestEntry(manifest *assets.Manifest, relativePath string, data []byte, modTime time.Time) {
	if manifest.Files == nil {
		manifest.Files = make(map[string]assets.ManifestEntry)
	}

	rel := filepath.ToSlash(relativePath)
	hash := sha256.Sum256(data)

	manifest.Files[rel] = assets.ManifestEntry{
		Path:    rel,
		SHA256:  hex.EncodeToString(hash[:]),
		Size:    int64(len(data)),
		MIME:    mimeType(rel),
		ModTime: modTime,
	}
}

func mimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".webp":
		return "image/webp"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	case ".xml":
		return "application/xml"
	default:
		return "application/octet-stream"
	}
}

func writeManifest(path string, manifest *assets.Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

func writeEmbeddedFile(buildDir string) error {
	content := `// Code generated by internal/assets/packer. DO NOT EDIT.

package build

import "embed"

//go:embed all:public
var FS embed.FS
`

	path := filepath.Join(buildDir, "embedded.go")

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write embedded.go: %w", err)
	}

	return nil
}
