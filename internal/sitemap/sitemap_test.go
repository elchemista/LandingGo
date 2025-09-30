package sitemap

import (
	"strings"
	"testing"
	"time"

	"github.com/elchemista/LandingGo/internal/config"
)

func TestBuildSitemap(t *testing.T) {
	routes := []config.Route{{Path: "/"}, {Path: "/about"}}

	data, err := Build("https://example.com", routes, time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC))
	if err != nil {
		t.Fatalf("build sitemap: %v", err)
	}

	xml := string(data)
	if !strings.Contains(xml, "https://example.com/about") {
		t.Fatalf("missing route URL in sitemap: %s", xml)
	}

	if !strings.Contains(xml, "2024-01-02T03:04:05Z") {
		t.Fatalf("missing lastmod timestamp: %s", xml)
	}
}
