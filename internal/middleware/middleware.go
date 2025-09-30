package middleware

import (
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

			gw := pool.Get().(*gzip.Writer)
			gw.Reset(w)

			h := w.Header()
			h.Del("Content-Length")
			h.Set("Content-Encoding", "gzip")
			h.Add("Vary", "Accept-Encoding")

			recorder := &responseRecorder{ResponseWriter: w, status: http.StatusOK, writer: gw, compressed: true}
			recorder.closeFn = func() {
				gw.Close()
				pool.Put(gw)
			}

			defer func() {
				if rec := recover(); rec != nil {
					recorder.Close()
					panic(rec)
				}
				recorder.Close()
			}()

			next.ServeHTTP(recorder, r)
		})
	}
}

// responseRecorder captures status and optionally wraps the writer.
type responseRecorder struct {
	http.ResponseWriter
	status          int
	writer          io.Writer
	closeFn         func()
	compressed      bool
	headerSanitized bool
}

func (rw *responseRecorder) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseRecorder) Write(p []byte) (int, error) {
	if rw.writer != nil {
		if rw.compressed && !rw.headerSanitized {
			rw.headerSanitized = true
			header := rw.Header()
			header.Del("Content-Length")
		}
		return rw.writer.Write(p)
	}
	return rw.ResponseWriter.Write(p)
}

func (rw *responseRecorder) Close() {
	if rw.closeFn != nil {
		rw.closeFn()
		rw.closeFn = nil
	}
}

func (rw *responseRecorder) DisableCompression() {
	if !rw.compressed {
		return
	}
	rw.compressed = false
	rw.Close()
	rw.writer = nil
	header := rw.Header()
	header.Del("Content-Encoding")
	header.Del("Content-Length")
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
