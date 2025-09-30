package sitemap

import (
	"encoding/xml"
	"errors"
	"net/url"
	"time"

	"webgo/internal/config"
)

const sitemapNS = "http://www.sitemaps.org/schemas/sitemap/0.9"

// Build generates a sitemap XML document for the provided routes.
func Build(baseURL string, routes []config.Route, generated time.Time) ([]byte, error) {
	if baseURL == "" {
		return nil, ErrBaseURLRequired
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	entries := make([]urlEntry, 0, len(routes))

	for _, rt := range routes {
		ref, err := url.Parse(rt.Path)
		if err != nil {
			return nil, err
		}

		loc := base.ResolveReference(ref)

		entries = append(entries, urlEntry{
			Loc:     loc.String(),
			LastMod: generated.UTC().Format(time.RFC3339),
		})
	}

	doc := urlSet{
		XMLNS: sitemapNS,
		URLs:  entries,
	}

	return xml.MarshalIndent(doc, "", "  ")
}

// ErrBaseURLRequired indicates Build was called without a base URL.
var ErrBaseURLRequired = errors.New("base URL is required")

type urlSet struct {
	XMLName xml.Name   `xml:"urlset"`
	XMLNS   string     `xml:"xmlns,attr"`
	URLs    []urlEntry `xml:"url"`
}

type urlEntry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}
