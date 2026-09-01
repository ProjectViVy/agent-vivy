package rpc

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// accessLogResponseWriter records the response status for the access line
// while forwarding Hijack and Flush so the WebSocket upgrade keeps working
// through the middleware (gorilla hijacks the connection without calling
// WriteHeader first).
type accessLogResponseWriter struct {
	http.ResponseWriter
	status   int
	written  bool
	hijacked bool
}

func (w *accessLogResponseWriter) WriteHeader(code int) {
	if !w.written {
		w.status = code
		w.written = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *accessLogResponseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.status = http.StatusOK
		w.written = true
	}
	return w.ResponseWriter.Write(b)
}

func (w *accessLogResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("rpc: response writer does not implement http.Hijacker")
	}
	w.hijacked = true
	return hijacker.Hijack()
}

func (w *accessLogResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// AccessLogMiddleware logs one structured line per gateway request (LOG-2,
// docs/architecture/LOGGING.md §7): method, path, status, duration_ms. A
// hijacked connection is the negotiated WebSocket upgrade (101). /healthz
// polls log at debug so container healthchecks do not flood the info
// channel; 5xx logs at warn (the request failed, the process continues).
func AccessLogMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &accessLogResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if rec.hijacked {
			rec.status = http.StatusSwitchingProtocols
		}
		level := slog.LevelInfo
		switch {
		case rec.status >= 500:
			level = slog.LevelWarn
		case r.URL.Path == "/healthz":
			level = slog.LevelDebug
		}
		logger.Log(r.Context(), level, "http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}
