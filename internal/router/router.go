package router

import "net/http"

// Router wires HTTP handlers without relying on ServeMux so custom 404 logic is possible.
type Router struct {
	exact    map[string]http.Handler
	prefixes []prefixHandler
	notFound http.Handler
}

type prefixHandler struct {
	prefix  string
	handler http.Handler
}

// New constructs a fresh Router.
func New() *Router {
	return &Router{
		exact: make(map[string]http.Handler),
	}
}

// Handle registers an exact path match.
func (r *Router) Handle(path string, handler http.Handler) {
	if path == "" || handler == nil {
		return
	}
	r.exact[path] = handler
}

// HandleFunc registers an exact path match via a function.
func (r *Router) HandleFunc(path string, fn http.HandlerFunc) {
	if fn == nil {
		return
	}
	r.Handle(path, http.HandlerFunc(fn))
}

// HandlePrefix registers a prefix match (e.g. for static assets).
func (r *Router) HandlePrefix(prefix string, handler http.Handler) {
	if prefix == "" || handler == nil {
		return
	}
	r.prefixes = append(r.prefixes, prefixHandler{prefix: prefix, handler: handler})
}

// NotFound sets the fallback handler.
func (r *Router) NotFound(handler http.Handler) {
	r.notFound = handler
}

// ServeHTTP satisfies http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if handler, ok := r.exact[req.URL.Path]; ok {
		handler.ServeHTTP(w, req)
		return
	}

	for _, ph := range r.prefixes {
		if ph.handler == nil {
			continue
		}
		if len(req.URL.Path) >= len(ph.prefix) && req.URL.Path[:len(ph.prefix)] == ph.prefix {
			ph.handler.ServeHTTP(w, req)
			return
		}
	}

	if r.notFound != nil {
		r.notFound.ServeHTTP(w, req)
		return
	}

	http.NotFound(w, req)
}
