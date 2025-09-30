package robots

import (
	"bytes"
	"errors"
	"net/url"
	"strings"
)

const defaultPolicy = "User-agent: *\nAllow: /"

// Build constructs a robots.txt payload using the provided optional policy.
// The sitemap URL is always appended (or rewritten) to ensure correctness.
func Build(baseURL, policy string) ([]byte, error) {
	if baseURL == "" {
		return nil, ErrBaseURLRequired
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	sitemapURL := u.ResolveReference(&url.URL{Path: "/sitemap.xml"}).String()

	if strings.TrimSpace(policy) == "" {
		policy = defaultPolicy
	}

	lines := splitPolicy(policy)

	seen := false
	for i, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "sitemap:") {
			lines[i] = "Sitemap: " + sitemapURL
			seen = true
		}
	}

	if !seen {
		lines = append(lines, "Sitemap: "+sitemapURL)
	}

	var buf bytes.Buffer
	for i, line := range lines {
		buf.WriteString(strings.TrimRight(line, "\r"))
		if i < len(lines)-1 {
			buf.WriteByte('\n')
		}
	}

	return buf.Bytes(), nil
}

// ErrBaseURLRequired is returned when the base URL is missing.
var ErrBaseURLRequired = errors.New("base URL is required")

func splitPolicy(policy string) []string {
	policy = strings.ReplaceAll(policy, "\r\n", "\n")
	policy = strings.ReplaceAll(policy, "\r", "\n")
	parts := strings.Split(policy, "\n")
	trimmed := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		trimmed = append(trimmed, p)
	}
	return trimmed
}
