package server

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/elchemista/LandingGo/internal/assets"
	"github.com/elchemista/LandingGo/internal/config"
	"github.com/elchemista/LandingGo/internal/contact"
	errorspkg "github.com/elchemista/LandingGo/internal/errors"
	"github.com/elchemista/LandingGo/internal/middleware"
	"github.com/elchemista/LandingGo/internal/pages"
	"github.com/elchemista/LandingGo/internal/robots"
	"github.com/elchemista/LandingGo/internal/router"
	"github.com/elchemista/LandingGo/internal/sitemap"
)

// Server represents the HTTP server runtime.
type Server struct {
	cfg    *config.Config
	source *assets.Source
	logger *slog.Logger
	dev    bool

	router  *router.Router
	handler http.Handler

	pageMgr    *pages.Manager
	assetCache *assets.Cache

	sitemap []byte
	robots  []byte

	contact contact.Sender

	pageCache  sync.Map // route path -> *pageEntry
	errorCache sync.Map // key -> []byte
}

// pageEntry caches rendered HTML and metadata.
type pageEntry struct {
	Body         []byte
	ETag         string
	LastModified time.Time
}

// New constructs a server instance.
func New(cfg *config.Config, src *assets.Source, logger *slog.Logger, dev bool) (*Server, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}
	if src == nil {
		return nil, errors.New("asset source is nil")
	}

	pagesFS, err := src.Sub("pages")
	if err != nil {
		return nil, fmt.Errorf("pages fs: %w", err)
	}

	pageMgr := pages.New(pagesFS, nil)

	assetCache := assets.NewCache(src.FS, src.Manifest, src.GeneratedAt, src.ModTime)

	routes := cfg.RoutesByPath()

	sitemapPayload, err := sitemap.Build(cfg.Site.BaseURL, routes, cfg.LoadedAt())
	if err != nil {
		return nil, fmt.Errorf("sitemap build: %w", err)
	}
	sitemapPayload = append([]byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"), sitemapPayload...)

	robotsPayload, err := robots.Build(cfg.Site.BaseURL, cfg.Site.RobotsPolicy)
	if err != nil {
		return nil, fmt.Errorf("robots build: %w", err)
	}
	robotsPayload = append(robotsPayload, '\n')

	var contactSender contact.Sender
	if cfg.Contact.Enabled() {
		contactSender = contact.NewService(cfg.Contact, nil)
	}

	srv := &Server{
		cfg:        cfg,
		source:     src,
		logger:     logger,
		dev:        dev,
		router:     router.New(),
		pageMgr:    pageMgr,
		assetCache: assetCache,
		sitemap:    sitemapPayload,
		robots:     robotsPayload,
		contact:    contactSender,
	}

	srv.registerRoutes(routes)

	srv.handler = middleware.Chain(
		http.HandlerFunc(srv.router.ServeHTTP),
		middleware.Recover(logger, srv.recoverHandler),
		middleware.WithRequestID("X-Request-Id"),
		middleware.Logging(logger),
		middleware.Gzip(-1),
	)

	return srv, nil
}

func (s *Server) registerRoutes(routes []config.Route) {
	s.router.Handle("/sitemap.xml", http.HandlerFunc(s.serveSitemap))
	s.router.Handle("/robots.txt", http.HandlerFunc(s.serveRobots))
	s.router.Handle("/healthz", http.HandlerFunc(s.serveHealth))
	s.router.HandlePrefix("/static/", http.HandlerFunc(s.serveStatic))

	for _, route := range routes {
		route := route

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.servePage(w, r, route)
		})

		if route.Path == "/contact" {
			handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					s.handleContactSubmit(w, r)
					return
				}
				s.servePage(w, r, route)
			})
		}

		s.router.Handle(route.Path, handler)
	}

	s.router.NotFound(http.HandlerFunc(s.serveNotFound))
}

// Handler exposes the server handler stack.
func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) servePage(w http.ResponseWriter, r *http.Request, route config.Route) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		s.writeStatus(w, http.StatusMethodNotAllowed)
		return
	}

	entry, err := s.loadPage(route)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("render page", "path", route.Path, "error", err)
		}
		s.serveError(w, r, http.StatusInternalServerError)
		return
	}

	s.applyCacheHeaders(w, entry.ETag, entry.LastModified)
	s.applyHTMLHeaders(w)
	s.applyRouteHeaders(w, route.Path)

	if isNotModified(r, entry.ETag, entry.LastModified) {
		s.writeStatus(w, http.StatusNotModified)
		return
	}

	if r.Method == http.MethodHead {
		s.writeStatus(w, http.StatusOK)
		return
	}

	s.writeStatus(w, http.StatusOK)
	_, _ = w.Write(entry.Body)
}

func (s *Server) handleContactSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, HEAD, POST")
		s.writeStatus(w, http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid form data"})
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	message := strings.TrimSpace(r.FormValue("message"))

	if name == "" || email == "" || message == "" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name, email, and message are required"})
		return
	}

	if s.contact == nil || !s.contact.Enabled() {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "contact form disabled"})
		return
	}

	err := s.contact.Send(r.Context(), contact.Message{
		Name:  name,
		Email: email,
		Body:  message,
	})
	if err != nil {
		if s.logger != nil {
			s.logger.Error("contact send", "error", err)
		}
		s.writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to send message"})
		return
	}

	s.writeJSON(w, http.StatusAccepted, map[string]string{"status": "sent"})
}

func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		s.writeStatus(w, http.StatusMethodNotAllowed)
		return
	}

	relPath := strings.TrimPrefix(r.URL.Path, "/")

	asset, err := s.assetCache.Get(relPath)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("static asset", "asset", relPath, "error", err)
		}
		s.serveNotFound(w, r)
		return
	}

	header := w.Header()
	header.Set("Content-Type", asset.MIME)
	header.Set("Cache-Control", "public, max-age=31536000, immutable")
	header.Set("Content-Length", fmt.Sprintf("%d", asset.Size))

	s.applyCacheHeaders(w, asset.ETag, asset.LastModified)

	if isNotModified(r, asset.ETag, asset.LastModified) {
		s.writeStatus(w, http.StatusNotModified)
		return
	}

	if r.Method == http.MethodHead {
		s.writeStatus(w, http.StatusOK)
		return
	}

	s.writeStatus(w, http.StatusOK)
	_, _ = w.Write(asset.Body)
}

func (s *Server) serveSitemap(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	header.Set("Content-Type", "application/xml")
	header.Set("Cache-Control", "public, max-age=300")
	header.Set("Content-Length", fmt.Sprintf("%d", len(s.sitemap)))

	if r.Method == http.MethodHead {
		s.writeStatus(w, http.StatusOK)
		return
	}

	s.writeStatus(w, http.StatusOK)
	_, _ = w.Write(s.sitemap)
}

func (s *Server) serveRobots(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	header.Set("Content-Type", "text/plain; charset=utf-8")
	header.Set("Cache-Control", "public, max-age=300")
	header.Set("Content-Length", fmt.Sprintf("%d", len(s.robots)))

	if r.Method == http.MethodHead {
		s.writeStatus(w, http.StatusOK)
		return
	}

	s.writeStatus(w, http.StatusOK)
	_, _ = w.Write(s.robots)
}

func (s *Server) serveHealth(w http.ResponseWriter, r *http.Request) {
	health := []byte(`{"status":"ok"}`)
	header := w.Header()
	header.Set("Content-Type", "application/json")
	header.Set("Cache-Control", "no-store, max-age=0")
	header.Set("Content-Length", fmt.Sprintf("%d", len(health)))

	if r.Method == http.MethodHead {
		s.writeStatus(w, http.StatusOK)
		return
	}

	s.writeStatus(w, http.StatusOK)
	_, _ = w.Write(health)
}

func (s *Server) serveNotFound(w http.ResponseWriter, r *http.Request) {
	s.writeErrorPage(w, r, "404.html", errorspkg.Default404, http.StatusNotFound)
}

func (s *Server) serveError(w http.ResponseWriter, r *http.Request, status int) {
	if status == http.StatusInternalServerError {
		s.writeErrorPage(w, r, "500.html", errorspkg.Default500, status)
		return
	}

	s.writeErrorPage(w, r, "404.html", errorspkg.Default404, status)
}

func (s *Server) recoverHandler(w http.ResponseWriter, r *http.Request, rec any) {
	s.serveError(w, r, http.StatusInternalServerError)
}

func (s *Server) writeErrorPage(w http.ResponseWriter, r *http.Request, pageName, fallback string, status int) {
	var body []byte
	if cached, ok := s.errorCache.Load(pageName); ok {
		body = cached.([]byte)
	} else if s.pageMgr.Exists(pageName) {
		data, err := s.pageMgr.Render(pageName, s.basePageData(status, r.URL.Path))
		if err == nil {
			body = data
			s.errorCache.Store(pageName, body)
		}
	}

	if body == nil {
		body = []byte(fallback)
	}

	header := w.Header()
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("Cache-Control", "no-store, max-age=0")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (s *Server) basePageData(status int, path string) pages.PageData {
	return pages.PageData{
		Title:      fmt.Sprintf("%d", status),
		BaseURL:    s.cfg.Site.BaseURL,
		NowRFC3339: s.cfg.LoadedAt().Format(time.RFC3339),
		RoutePath:  path,
	}
}

func (s *Server) loadPage(route config.Route) (*pageEntry, error) {
	if entry, ok := s.pageCache.Load(route.Path); ok {
		return entry.(*pageEntry), nil
	}

	body, err := s.pageMgr.Render(route.Page, pages.PageData{
		Title:      route.Title,
		BaseURL:    s.cfg.Site.BaseURL,
		NowRFC3339: s.cfg.LoadedAt().Format(time.RFC3339),
		RoutePath:  route.Path,
	})
	if err != nil {
		return nil, err
	}

	entry := &pageEntry{Body: body}

	if s.source.Manifest != nil {
		manifestPath := filepath.ToSlash(filepath.Join("pages", route.Page))
		if meta, ok := s.source.Manifest.Files[manifestPath]; ok {
			entry.ETag = ensureQuoted(meta.SHA256)
			if !meta.ModTime.IsZero() {
				entry.LastModified = meta.ModTime.UTC()
			}
		}
	}

	if entry.ETag == "" {
		entry.ETag = computeETag(body)
	}
	if entry.LastModified.IsZero() {
		if mt, err := s.source.ModTime(filepath.ToSlash(filepath.Join("pages", route.Page))); err == nil {
			entry.LastModified = mt.UTC()
		} else {
			entry.LastModified = s.source.GeneratedAt
		}
	}

	s.pageCache.Store(route.Path, entry)

	return entry, nil
}

func (s *Server) applyCacheHeaders(w http.ResponseWriter, etag string, lastModified time.Time) {
	header := w.Header()
	if etag != "" {
		header.Set("ETag", etag)
	}
	if !lastModified.IsZero() {
		header.Set("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
	}
}

func (s *Server) applyHTMLHeaders(w http.ResponseWriter) {
	header := w.Header()
	header.Set("Content-Type", "text/html; charset=utf-8")
	if header.Get("Cache-Control") == "" {
		header.Set("Cache-Control", "public, max-age=300")
	}
}

func (s *Server) applyRouteHeaders(w http.ResponseWriter, path string) {
	header := w.Header()
	for key, val := range s.cfg.HeaderDirectives(path) {
		header.Set(key, val)
	}
}

func (s *Server) writeStatus(w http.ResponseWriter, status int) {
	if status == http.StatusNotModified {
		w.WriteHeader(status)
		return
	}

	if status == http.StatusOK {
		return
	}

	w.WriteHeader(status)
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		status = http.StatusInternalServerError
		data = []byte(`{"error":"internal error"}`)
	}

	header := w.Header()
	header.Set("Content-Type", "application/json")
	header.Set("Cache-Control", "no-store, max-age=0")

	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func ensureQuoted(hash string) string {
	if hash == "" {
		return ""
	}
	if strings.HasPrefix(hash, "\"") {
		return hash
	}
	return fmt.Sprintf("\"%s\"", hash)
}

func computeETag(body []byte) string {
	sum := sha256.Sum256(body)
	return fmt.Sprintf("\"%x\"", sum[:])
}

func isNotModified(r *http.Request, etag string, lastModified time.Time) bool {
	if etag != "" {
		if inm := r.Header.Get("If-None-Match"); inm != "" {
			for _, candidate := range strings.Split(inm, ",") {
				candidate = strings.TrimSpace(candidate)
				if candidate == etag || candidate == "*" {
					return true
				}
			}
		}
	}

	if !lastModified.IsZero() {
		if ims := r.Header.Get("If-Modified-Since"); ims != "" {
			if ts, err := time.Parse(http.TimeFormat, ims); err == nil {
				if !lastModified.After(ts) {
					return true
				}
			}
		}
	}

	return false
}
