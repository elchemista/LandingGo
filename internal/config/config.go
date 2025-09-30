package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Config represents the runtime configuration for the landing page server.
type Config struct {
	Site    Site                         `json:"site"`
	Routes  []Route                      `json:"routes"`
	Headers map[string]map[string]string `json:"headers"`
	Contact Contact                      `json:"contact"`

	loadedAt time.Time
	source   string
}

// Site contains global site metadata.
type Site struct {
	BaseURL      string `json:"base_url"`
	RobotsPolicy string `json:"robots_policy"`
}

// Contact describes contact-form delivery settings.
type Contact struct {
	Recipient string  `json:"recipient"`
	From      string  `json:"from"`
	Subject   string  `json:"subject"`
	Mailgun   Mailgun `json:"mailgun"`
}

// Mailgun holds credentials for Mailgun email delivery.
type Mailgun struct {
	Domain string `json:"domain"`
	APIKey string `json:"api_key"`
}

func (c *Contact) normalize() {
	if c == nil {
		return
	}
	c.Recipient = strings.TrimSpace(c.Recipient)
	c.From = strings.TrimSpace(c.From)
	c.Subject = strings.TrimSpace(c.Subject)
	c.Mailgun.Domain = strings.TrimSpace(c.Mailgun.Domain)
	c.Mailgun.APIKey = strings.TrimSpace(c.Mailgun.APIKey)
}

func (c Contact) Enabled() bool {
	return c.Recipient != "" && c.From != "" && c.Mailgun.Domain != "" && c.Mailgun.APIKey != ""
}

func (c Contact) isZero() bool {
	return c.Recipient == "" && c.From == "" && c.Subject == "" && c.Mailgun.Domain == "" && c.Mailgun.APIKey == ""
}

// Route maps an HTTP path to a template page.
type Route struct {
	Path  string `json:"path"`
	Page  string `json:"page"`
	Title string `json:"title"`
}

// Load reads the provided JSON configuration file and validates it.
func Load(path string) (*Config, error) {
	if path == "" {
		return nil, errors.New("config path is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg, err := Parse(data)
	if err != nil {
		return nil, err
	}

	cfg.source = path
	cfg.loadedAt = time.Now().UTC()

	return cfg, nil
}

// Parse constructs a Config from raw JSON bytes.
func Parse(data []byte) (*Config, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	if err := cfg.normalize(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// normalize applies canonical formatting to the configuration.
func (c *Config) normalize() error {
	if c.Headers == nil {
		c.Headers = make(map[string]map[string]string)
	}

	normalized := make(map[string]map[string]string, len(c.Headers))

	// Apply deterministic header key casing (Title-Case).
	for path, hdrs := range c.Headers {
		if hdrs == nil {
			continue
		}
		clean := make(map[string]string, len(hdrs))
		for key, val := range hdrs {
			clean[canonicalHeaderKey(key)] = strings.TrimSpace(val)
		}
		normalized[cleanPath(path)] = clean
	}

	c.Headers = normalized
	c.Contact.normalize()

	return nil
}

func canonicalHeaderKey(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	parts := strings.Split(s, "-")
	for i, part := range parts {
		parts[i] = titleCase(part)
	}

	return strings.Join(parts, "-")
}

func titleCase(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return s
	}

	runes := []rune(s)
	runes[0] = toUpperRune(runes[0])
	return string(runes)
}

func toUpperRune(r rune) rune {
	if 'a' <= r && r <= 'z' {
		return r - ('a' - 'A')
	}
	return r
}

// LoadedAt returns the time the config was read from disk.
func (c *Config) LoadedAt() time.Time {
	return c.loadedAt
}

// Source returns the backing config path.
func (c *Config) Source() string {
	return c.source
}

// Validate ensures the configuration is internally consistent and that required
// assets exist relative to the provided filesystem accessor. The fsExists
// callback should return true when the requested page exists.
func (c *Config) Validate(fsExists func(name string) bool) error {
	if c == nil {
		return errors.New("config is nil")
	}

	if fsExists == nil {
		return errors.New("fsExists is nil")
	}

	if err := c.validateSite(); err != nil {
		return err
	}

	if len(c.Routes) == 0 {
		return errors.New("config.routes must contain at least one entry")
	}

	seenPaths := make(map[string]struct{}, len(c.Routes))
	contactRoute := false

	for i := range c.Routes {
		rt := &c.Routes[i]

		if rt.Path == "" {
			return fmt.Errorf("route %d: path is required", i)
		}

		rt.Path = cleanPath(rt.Path)

		if _, ok := seenPaths[rt.Path]; ok {
			return fmt.Errorf("duplicate route path %q", rt.Path)
		}
		seenPaths[rt.Path] = struct{}{}

		if rt.Page == "" {
			return fmt.Errorf("route %s: page is required", rt.Path)
		}

		rt.Page = filepath.ToSlash(rt.Page)

		if strings.Contains(rt.Page, "..") {
			return fmt.Errorf("route %s: page must not contain '..'", rt.Path)
		}

		if !fsExists(rt.Page) {
			return fmt.Errorf("route %s: page %q not found", rt.Path, rt.Page)
		}

		if rt.Title == "" {
			rt.Title = defaultTitleFromPage(rt.Page)
		}

		if rt.Path == "/contact" {
			contactRoute = true
		}
	}

	if err := c.validateContact(contactRoute); err != nil {
		return err
	}

	return nil
}

func (c *Config) validateContact(contactRoute bool) error {
	contact := c.Contact
	if contact.isZero() {
		return nil
	}

	if contact.Recipient == "" || contact.From == "" || contact.Mailgun.Domain == "" {
		return errors.New("contact configuration is incomplete")
	}

	if !contactRoute {
		return errors.New("contact route '/contact' must be defined when contact configuration is provided")
	}

	if !strings.Contains(contact.Recipient, "@") {
		return errors.New("contact.recipient must be a valid email address")
	}

	if !strings.Contains(contact.From, "@") {
		return errors.New("contact.from must be a valid email address")
	}

	if strings.Contains(contact.Mailgun.Domain, "://") {
		return errors.New("contact.mailgun.domain must not include a URL scheme")
	}

	return nil
}

func (c *Config) validateSite() error {
	if c.Site.BaseURL == "" {
		return errors.New("site.base_url is required")
	}

	u, err := url.Parse(c.Site.BaseURL)
	if err != nil {
		return fmt.Errorf("site.base_url: %w", err)
	}

	if u.Scheme == "" || u.Host == "" {
		return errors.New("site.base_url must include scheme and host")
	}

	return nil
}

// HeaderDirectives returns the configured headers for a specific route path.
func (c *Config) HeaderDirectives(path string) map[string]string {
	if c == nil || c.Headers == nil {
		return nil
	}

	path = cleanPath(path)

	headers := c.Headers[path]

	if len(headers) == 0 {
		return nil
	}

	copy := make(map[string]string, len(headers))
	for k, v := range headers {
		copy[k] = v
	}

	return copy
}

// cleanPath ensures deterministic path representation.
func cleanPath(p string) string {
	if p == "" {
		return p
	}

	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}

	return p
}

func defaultTitleFromPage(page string) string {
	base := filepath.Base(page)
	if idx := strings.LastIndexByte(base, '.'); idx > 0 {
		base = base[:idx]
	}

	base = strings.ReplaceAll(base, "-", " ")
	base = strings.ReplaceAll(base, "_", " ")
	return titleCase(base)
}

// WithLoadedTime updates the loadedAt timestamp. Useful for tests.
func (c *Config) WithLoadedTime(t time.Time) {
	if c != nil {
		c.loadedAt = t
	}
}

// WithSource sets the configuration source identifier for diagnostics.
func (c *Config) WithSource(src string) {
	if c != nil {
		c.source = src
	}
}

// RoutesByPath returns a copy of routes sorted by path for deterministic output.
func (c *Config) RoutesByPath() []Route {
	if c == nil {
		return nil
	}

	clone := make([]Route, len(c.Routes))
	copy(clone, c.Routes)

	sort.Slice(clone, func(i, j int) bool {
		return clone[i].Path < clone[j].Path
	})

	return clone
}
