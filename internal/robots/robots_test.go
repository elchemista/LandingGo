package robots

import (
	"strings"
	"testing"
)

func TestBuildDefaultPolicy(t *testing.T) {
	data, err := Build("https://example.com", "")
	if err != nil {
		t.Fatalf("build robots: %v", err)
	}

	text := string(data)
	if !strings.Contains(text, "Allow: /") {
		t.Fatalf("expected Allow directive, got %q", text)
	}

	if !strings.Contains(text, "Sitemap: https://example.com/sitemap.xml") {
		t.Fatalf("expected sitemap line, got %q", text)
	}
}

func TestBuildOverridesSitemap(t *testing.T) {
	policy := "User-agent: *\nDisallow: /tmp\nSitemap: https://old.example/sitemap.xml"
	data, err := Build("https://example.com", policy)
	if err != nil {
		t.Fatalf("build robots: %v", err)
	}

	if strings.Count(string(data), "Sitemap:") != 1 {
		t.Fatalf("expected single sitemap line: %q", data)
	}

	if !strings.Contains(string(data), "https://example.com/sitemap.xml") {
		t.Fatalf("sitemap URL not rewritten: %q", data)
	}
}
