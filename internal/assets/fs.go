package assets

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// SourceKind identifies whether assets are served from disk or embedded data.
type SourceKind int

const (
	// SourceEmbedded represents assets served from the generated embedded FS.
	SourceEmbedded SourceKind = iota
	// SourceDisk represents assets served directly from disk (dev mode).
	SourceDisk
)

// Source exposes file-system helpers for embedded and disk assets.
type Source struct {
	FS          fs.FS
	kind        SourceKind
	root        string
	Manifest    *Manifest
	GeneratedAt time.Time
}

// NewEmbedded constructs a Source from an embedded filesystem.
func NewEmbedded(fsys fs.FS) (*Source, error) {
	if fsys == nil {
		return nil, errors.New("embedded filesystem is nil")
	}

	manifest, err := LoadManifest(fsys)
	if err != nil {
		return nil, err
	}

	gen := manifest.GeneratedAt
	if gen.IsZero() {
		gen = time.Now().UTC()
	}

	return &Source{
		FS:          fsys,
		kind:        SourceEmbedded,
		Manifest:    manifest,
		GeneratedAt: gen,
	}, nil
}

// NewDisk constructs a Source from a directory on disk.
func NewDisk(root string) (*Source, error) {
	if root == "" {
		return nil, errors.New("disk root is empty")
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, errors.New("disk root must be a directory")
	}

	return &Source{
		FS:          os.DirFS(root),
		kind:        SourceDisk,
		root:        root,
		GeneratedAt: time.Now().UTC(),
	}, nil
}

// Kind returns the source type.
func (s *Source) Kind() SourceKind {
	if s == nil {
		return SourceDisk
	}
	return s.kind
}

// Exists reports whether the specified relative path exists.
func (s *Source) Exists(name string) bool {
	if s == nil || name == "" {
		return false
	}

	if s.kind == SourceDisk {
		_, err := os.Stat(filepath.Join(s.root, name))
		return err == nil
	}

	_, err := fs.Stat(s.FS, name)
	return err == nil
}

// PagesExists reports whether the page file is present beneath pages/.
func (s *Source) PageExists(page string) bool {
	if page == "" {
		return false
	}
	return s.Exists(filepath.Join("pages", page))
}

// StaticExists reports whether the static asset exists beneath static/.
func (s *Source) StaticExists(path string) bool {
	if path == "" {
		return false
	}
	return s.Exists(filepath.Join("static", path))
}

// ModTime returns the best-effort modification time for a file.
func (s *Source) ModTime(name string) (time.Time, error) {
	if s == nil {
		return time.Time{}, errors.New("source is nil")
	}

	if s.kind == SourceDisk {
		info, err := os.Stat(filepath.Join(s.root, name))
		if err != nil {
			return time.Time{}, err
		}
		return info.ModTime().UTC(), nil
	}

	if s.Manifest != nil {
		if entry, ok := s.Manifest.Files[name]; ok {
			if !entry.ModTime.IsZero() {
				return entry.ModTime.UTC(), nil
			}
			return s.Manifest.GeneratedAt.UTC(), nil
		}
	}

	return s.GeneratedAt, nil
}

// Sub returns a view into a nested directory within the source.
func (s *Source) Sub(dir string) (fs.FS, error) {
	if s == nil {
		return nil, errors.New("source is nil")
	}

	return fs.Sub(s.FS, dir)
}

// Root returns the disk root (dev mode) or empty string for embedded.
func (s *Source) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}
