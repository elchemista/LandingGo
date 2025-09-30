package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const manifestFile = "manifest.json"

// ManifestEntry describes an asset present in the packed output.
type ManifestEntry struct {
	Path    string    `json:"path"`
	SHA256  string    `json:"sha256"`
	Size    int64     `json:"size"`
	MIME    string    `json:"mime"`
	ModTime time.Time `json:"mod_time"`
}

// Manifest captures metadata for cache and ETag handling.
type Manifest struct {
	GeneratedAt time.Time                `json:"generated_at"`
	Files       map[string]ManifestEntry `json:"files"`
}

// LoadManifest reads and parses a manifest from the provided filesystem.
func LoadManifest(fsys fs.FS) (*Manifest, error) {
	if fsys == nil {
		return nil, errors.New("nil filesystem")
	}

	data, err := fs.ReadFile(fsys, manifestFile)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}

	if manifest.Files == nil {
		manifest.Files = make(map[string]ManifestEntry)
	}

	return &manifest, nil
}

// Cache stores rendered templates and asset bytes in memory.
type Cache struct {
	fs          fs.FS
	manifest    *Manifest
	defaultTime time.Time
	modTimeFn   func(string) (time.Time, error)
	assets      sync.Map // string -> *CachedAsset
}

// CachedAsset is the cached representation of a static asset.
type CachedAsset struct {
	Path         string
	Body         []byte
	ETag         string
	LastModified time.Time
	MIME         string
	Size         int64
}

// NewCache constructs a Cache backed by the provided filesystem.
func NewCache(fsys fs.FS, manifest *Manifest, defaultTime time.Time, modTime func(string) (time.Time, error)) *Cache {
	return &Cache{
		fs:          fsys,
		manifest:    manifest,
		defaultTime: defaultTime,
		modTimeFn:   modTime,
	}
}

// Get returns the cached asset, reading and caching it on the first request.
func (c *Cache) Get(path string) (*CachedAsset, error) {
	if c == nil {
		return nil, errors.New("cache is nil")
	}

	if path == "" {
		return nil, errors.New("path is empty")
	}

	if v, ok := c.assets.Load(path); ok {
		return v.(*CachedAsset), nil
	}

	body, err := fs.ReadFile(c.fs, path)
	if err != nil {
		return nil, err
	}

	meta := c.lookupMeta(path)

	asset := &CachedAsset{
		Path:         path,
		Body:         body,
		ETag:         meta.ETag,
		LastModified: meta.LastModified,
		MIME:         meta.MIME,
		Size:         int64(len(body)),
	}

	if asset.ETag == "" {
		asset.ETag = strongETag(body)
	}

	if asset.LastModified.IsZero() && c.modTimeFn != nil {
		if mt, err := c.modTimeFn(path); err == nil {
			asset.LastModified = mt.UTC()
		}
	}

	if asset.LastModified.IsZero() {
		asset.LastModified = c.defaultTime
	}

	if asset.MIME == "" {
		ext := strings.ToLower(filepath.Ext(path))
		if mt := mime.TypeByExtension(ext); mt != "" {
			asset.MIME = mt
		} else if mt := fallbackMIME(ext); mt != "" {
			asset.MIME = mt
		} else {
			asset.MIME = http.DetectContentType(body)
		}
	}

	if asset.MIME == "" || asset.MIME == "application/octet-stream" {
		if sniffed := http.DetectContentType(body); sniffed != "" && sniffed != "application/octet-stream" {
			asset.MIME = sniffed
		}
	}

	c.assets.Store(path, asset)

	return asset, nil
}

// Invalidate evicts a single asset from the cache.
func (c *Cache) Invalidate(path string) {
	if c == nil || path == "" {
		return
	}
	c.assets.Delete(path)
}

func (c *Cache) lookupMeta(path string) assetMeta {
	if c == nil || c.manifest == nil {
		return assetMeta{}
	}

	entry, ok := c.manifest.Files[path]
	if !ok {
		return assetMeta{}
	}

	lm := entry.ModTime
	if lm.IsZero() {
		lm = c.manifest.GeneratedAt
	}

	etag := entry.SHA256
	if etag != "" && !strings.HasPrefix(etag, "\"") {
		etag = fmt.Sprintf("\"%s\"", etag)
	}

	return assetMeta{
		ETag:         etag,
		LastModified: lm.UTC(),
		MIME:         entry.MIME,
	}
}

type assetMeta struct {
	ETag         string
	LastModified time.Time
	MIME         string
}

func strongETag(body []byte) string {
	sum := sha256.Sum256(body)
	return "\"" + hex.EncodeToString(sum[:]) + "\""
}

func fallbackMIME(ext string) string {
	switch ext {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".svg":
		return "image/svg+xml"
	case ".xml":
		return "application/xml"
	case ".txt":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

// ManifestFilename is the filename emitted by the asset packer.
const ManifestFilename = manifestFile
