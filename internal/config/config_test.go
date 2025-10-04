package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidateSuccess(t *testing.T) {
	cfg := &Config{
		Site:   Site{BaseURL: "http://localhost:8080"},
		Routes: []Route{{Path: "/", Page: "home.html", Title: "Home"}},
		Headers: map[string]map[string]string{
			"/": {"cache-control": "public, max-age=60"},
		},
	}

	cfg.WithLoadedTime(time.Now())

	if err := cfg.normalize(); err != nil {
		t.Fatalf("normalize: %v", err)
	}

	if err := cfg.Validate(func(name string) bool { return name == "home.html" }); err != nil {
		t.Fatalf("expected validation to pass, got %v", err)
	}

	headers := cfg.HeaderDirectives("/")
	if headers["Cache-Control"] != "public, max-age=60" {
		t.Fatalf("header normalization failed: %+v", headers)
	}
}

func TestValidateDuplicateRoute(t *testing.T) {
	cfg := &Config{
		Site:   Site{BaseURL: "http://localhost:8080"},
		Routes: []Route{{Path: "/", Page: "home.html"}, {Path: "/", Page: "about.html"}},
	}

	err := cfg.Validate(func(string) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate route error, got %v", err)
	}
}

func TestValidateMissingPage(t *testing.T) {
	cfg := &Config{
		Site:   Site{BaseURL: "http://localhost:8080"},
		Routes: []Route{{Path: "/", Page: "missing.html"}},
	}

	err := cfg.Validate(func(string) bool { return false })
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected missing page error, got %v", err)
	}
}

func TestValidateBaseURL(t *testing.T) {
	cfg := &Config{
		Site:   Site{BaseURL: "//bad"},
		Routes: []Route{{Path: "/", Page: "home.html"}},
	}

	err := cfg.Validate(func(string) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "site.base_url") {
		t.Fatalf("expected base_url error, got %v", err)
	}
}

func TestValidateContactWithoutRouteAllowed(t *testing.T) {
	cfg := &Config{
		Site:   Site{BaseURL: "http://localhost:8080"},
		Routes: []Route{{Path: "/", Page: "home.html"}},
		Contact: Contact{
			Recipient: "owner@example.com",
			From:      "no-reply@example.com",
			Mailgun:   Mailgun{Domain: "mg.example.com", APIKey: "abc"},
		},
	}
	cfg.WithLoadedTime(time.Now())
	_ = cfg.normalize()

	if err := cfg.Validate(func(string) bool { return true }); err != nil {
		t.Fatalf("expected validation to succeed without contact route, got %v", err)
	}
}

func TestValidateContactIncomplete(t *testing.T) {
	cfg := &Config{
		Site:   Site{BaseURL: "http://localhost:8080"},
		Routes: []Route{{Path: "/", Page: "home.html"}, {Path: "/contact", Page: "contact.html"}},
		Contact: Contact{
			Recipient: "owner@example.com",
			Mailgun:   Mailgun{Domain: "mg.example.com", APIKey: ""},
		},
	}
	cfg.WithLoadedTime(time.Now())
	_ = cfg.normalize()

	err := cfg.Validate(func(name string) bool { return name == "home.html" || name == "contact.html" })
	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("expected incomplete contact error, got %v", err)
	}
}

func TestValidateContactMissingAPIKeyAllowed(t *testing.T) {
	cfg := &Config{
		Site:   Site{BaseURL: "http://localhost:8080"},
		Routes: []Route{{Path: "/", Page: "home.html"}, {Path: "/contact", Page: "contact.html"}},
		Contact: Contact{
			Recipient: "owner@example.com",
			From:      "no-reply@example.com",
			Mailgun:   Mailgun{Domain: "mg.example.com", APIKey: ""},
		},
	}
	cfg.WithLoadedTime(time.Now())
	_ = cfg.normalize()

	if err := cfg.Validate(func(name string) bool { return name == "home.html" || name == "contact.html" }); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateContactSuccess(t *testing.T) {
	cfg := &Config{
		Site:   Site{BaseURL: "http://localhost:8080"},
		Routes: []Route{{Path: "/", Page: "home.html"}, {Path: "/contact", Page: "contact.html"}},
		Contact: Contact{
			Recipient: "owner@example.com",
			From:      "no-reply@example.com",
			Mailgun:   Mailgun{Domain: "mg.example.com", APIKey: "abc"},
		},
	}
	cfg.WithLoadedTime(time.Now())
	_ = cfg.normalize()

	if err := cfg.Validate(func(name string) bool { return name == "home.html" || name == "contact.html" }); err != nil {
		t.Fatalf("expected contact validation success, got %v", err)
	}
}
