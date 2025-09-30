package middleware

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// keyRequestID is used to stash the request ID in the context.
type keyRequestID struct{}

// Chain applies middleware in order.
func Chain(h http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i](h)
	}
	return h
}

// WithRequestID attaches a request ID to the context and response headers.
func WithRequestID(header string) func(http.Handler) http.Handler {
	if header == "" {
		header = "X-Request-Id"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := r.Header.Get(header)
			if reqID == "" {
				reqID = randomID()
			}
			ctx := context.WithValue(r.Context(), keyRequestID{}, reqID)

			w.Header().Set(header, reqID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequestIDFromContext extracts the request ID if present.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(keyRequestID{}).(string); ok {
		return v
	}
	return ""
}

// Recover wraps handlers with panic recovery and structured logging.
func Recover(logger *slog.Logger, onError func(http.ResponseWriter, *http.Request, any)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					if logger != nil {
						logger.Error("panic recovered", "error", rec, "path", r.URL.Path, "method", r.Method, "request_id", RequestIDFromContext(r.Context()))
					}
					if onError != nil {
						onError(w, r, rec)
					}
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// Logging provides basic structured request logging.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	skip := func(path string) bool {
		return strings.HasPrefix(path, "/static/")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skip(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			recorder := &responseRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(recorder, r)

			logger.Info("request completed",
				"ip", clientIP(r),
				"method", r.Method,
				"path", r.URL.Path,
				"status", recorder.status,
				"duration", time.Since(start),
				"request_id", RequestIDFromContext(r.Context()),
			)
		})
	}
}

// Gzip compresses response bodies when the client supports it.
func Gzip(level int) func(http.Handler) http.Handler {
	if level < gzip.HuffmanOnly || level > gzip.BestCompression {
		level = gzip.DefaultCompression
	}

	pool := sync.Pool{
		New: func() any {
			w, _ := gzip.NewWriterLevel(io.Discard, level)
			return w
		},
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodHead || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			gzw := &gzipResponseWriter{ResponseWriter: w, pool: &pool, compress: true}

			defer func() {
				if rec := recover(); rec != nil {
					gzw.DisableCompression()
					panic(rec)
				}
				gzw.Close()
			}()

			next.ServeHTTP(gzw, r)
		})
	}
}

// responseRecorder captures status codes for logging.
type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (rw *responseRecorder) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseRecorder) Write(p []byte) (int, error) {
	return rw.ResponseWriter.Write(p)
}

func (rw *responseRecorder) DisableCompression() {
	if disabler, ok := rw.ResponseWriter.(interface{ DisableCompression() }); ok {
		disabler.DisableCompression()
	}
}

func (rw *responseRecorder) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (rw *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

func (rw *responseRecorder) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := rw.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000")))
	}
	return hex.EncodeToString(b[:])
}

func clientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		return strings.TrimSpace(parts[0])
	}
	if real := r.Header.Get("X-Real-IP"); real != "" {
		return strings.TrimSpace(real)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type gzipResponseWriter struct {
	http.ResponseWriter
	pool        *sync.Pool
	writer      *gzip.Writer
	wroteHeader bool
	compress    bool
}

func (g *gzipResponseWriter) ensureWriter() {
	if !g.compress {
		return
	}
	if g.writer != nil {
		return
	}
	gw := g.pool.Get().(*gzip.Writer)
	gw.Reset(g.ResponseWriter)
	g.writer = gw
	header := g.Header()
	header.Del("Content-Length")
	header.Set("Content-Encoding", "gzip")
	header.Add("Vary", "Accept-Encoding")
}

func (g *gzipResponseWriter) WriteHeader(code int) {
	if code >= 400 {
		g.DisableCompression()
	}
	g.wroteHeader = true
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipResponseWriter) Write(p []byte) (int, error) {
	if !g.compress {
		if !g.wroteHeader {
			g.WriteHeader(http.StatusOK)
		}
		return g.ResponseWriter.Write(p)
	}
	if g.writer == nil {
		g.ensureWriter()
	}
	if !g.wroteHeader {
		g.WriteHeader(http.StatusOK)
	}
	return g.writer.Write(p)
}

func (g *gzipResponseWriter) Close() {
	if g.writer == nil {
		return
	}
	_ = g.writer.Close()
	g.pool.Put(g.writer)
	g.writer = nil
}

func (g *gzipResponseWriter) DisableCompression() {
	if !g.compress {
		return
	}
	g.compress = false
	if g.writer != nil {
		g.writer.Reset(io.Discard)
		_ = g.writer.Close()
		g.pool.Put(g.writer)
		g.writer = nil
	}
	header := g.Header()
	header.Del("Content-Encoding")
	header.Del("Content-Length")
}

func (g *gzipResponseWriter) Flush() {
	if g.writer != nil {
		_ = g.writer.Flush()
	}
	if flusher, ok := g.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (g *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := g.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

func (g *gzipResponseWriter) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := g.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}
