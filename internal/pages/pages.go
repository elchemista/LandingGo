package pages

import (
	"bytes"
	"html/template"
	"io/fs"
	"sync"
)

// Manager handles template parsing and rendering.
type Manager struct {
	fs        fs.FS
	funcs     template.FuncMap
	templates sync.Map // string -> *template.Template
}

// New constructs a Manager for the provided filesystem containing page templates.
func New(fsys fs.FS, funcs template.FuncMap) *Manager {
	if funcs == nil {
		funcs = template.FuncMap{}
	}

	return &Manager{
		fs:    fsys,
		funcs: funcs,
	}
}

// PageData provides the minimum templating context.
type PageData struct {
	Title      string
	BaseURL    string
	NowRFC3339 string
	RoutePath  string
	Extra      map[string]any
}

// Render executes the named template with the provided data.
func (m *Manager) Render(name string, data PageData) ([]byte, error) {
	tmpl, err := m.template(name)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Exists reports whether the template file exists.
func (m *Manager) Exists(name string) bool {
	if m == nil || name == "" {
		return false
	}
	_, err := fs.Stat(m.fs, name)
	return err == nil
}

// Invalidate releases a template from the cache (useful in dev hot-reload).
func (m *Manager) Invalidate(name string) {
	if m == nil || name == "" {
		return
	}
	m.templates.Delete(name)
}

func (m *Manager) template(name string) (*template.Template, error) {
	if m == nil {
		return nil, fs.ErrNotExist
	}

	if v, ok := m.templates.Load(name); ok {
		return v.(*template.Template), nil
	}

	src, err := fs.ReadFile(m.fs, name)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New(name).
		Funcs(m.funcs).
		Option("missingkey=zero").
		Parse(string(src))
	if err != nil {
		return nil, err
	}

	m.templates.Store(name, tmpl)
	return tmpl, nil
}
