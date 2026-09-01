package rpc

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newAccessLogRecorder(level slog.Level) (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: level}))
	return logger, &buf
}

func TestAccessLogRecordsStatusAndDuration(t *testing.T) {
	logger, buf := newAccessLogRecorder(slog.LevelDebug)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})
	srv := httptest.NewServer(AccessLogMiddleware(logger, next))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/rpc/bootstrap")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	line := buf.String()
	if !strings.Contains(line, "msg=\"http request\"") ||
		!strings.Contains(line, "method=GET") ||
		!strings.Contains(line, "path=/rpc/bootstrap") ||
		!strings.Contains(line, "status=201") ||
		!strings.Contains(line, "duration_ms=") {
		t.Fatalf("access line incomplete: %q", line)
	}
	if !strings.Contains(line, "level=INFO") {
		t.Fatalf("201 must log at info: %q", line)
	}
}

func TestAccessLogWarnsOnServerError(t *testing.T) {
	logger, buf := newAccessLogRecorder(slog.LevelInfo)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(AccessLogMiddleware(logger, next))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/rpc")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	line := buf.String()
	if !strings.Contains(line, "level=WARN") || !strings.Contains(line, "status=500") {
		t.Fatalf("500 must log at warn with status=500: %q", line)
	}
}

func TestAccessLogHealthzStaysBelowInfo(t *testing.T) {
	logger, buf := newAccessLogRecorder(slog.LevelInfo)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	srv := httptest.NewServer(AccessLogMiddleware(logger, next))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if buf.Len() != 0 {
		t.Fatalf("healthz must not log at info: %q", buf.String())
	}
}

// TestAccessLogWebSocketUpgradeLogs101 exercises the WebSocket upgrade
// path: the middleware must expose http.Hijacker to the handler (gorilla
// hijacks without writing the 101 itself) and log the negotiated status.
func TestAccessLogWebSocketUpgradeLogs101(t *testing.T) {
	logger, buf := newAccessLogRecorder(slog.LevelInfo)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("middleware must expose http.Hijacker to handlers")
		}
		if _, _, err := hj.Hijack(); err != nil {
			t.Fatalf("hijack: %v", err)
		}
	})
	srv := httptest.NewServer(AccessLogMiddleware(logger, next))
	defer srv.Close()
	// The hijacked connection is never answered, so the client-side error
	// is expected; the assertion is the logged upgrade status. The client
	// timeout keeps the doomed wait from hitting the transport's ~2min
	// default response deadline.
	client := &http.Client{Timeout: 2 * time.Second}
	_, _ = client.Get(srv.URL + "/rpc")
	if !strings.Contains(buf.String(), "status=101") {
		t.Fatalf("hijacked upgrade must log status=101: %q", buf.String())
	}
}
