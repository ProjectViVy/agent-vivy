package logging

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
)

// The redaction vocabulary is the kernel-wide D-010 depth: the same
// credential and email shapes are removed at the tool-result boundary
// (internal/tools RedactSensitive delegates here) and again at the slog
// handler layer, so a value that slipped past call-site discipline still
// never reaches the log sink. Patterns are deliberately conservative —
// only high-confidence token shapes and whole-value redaction for
// obviously sensitive attribute keys — so ordinary log fields (run ids,
// model names, file paths) pass through untouched.
var (
	secretPattern         = regexp.MustCompile(`(?i)\b(?:sk|rk|pk)-[A-Za-z0-9_-]{12,}\b|\bBearer\s+[A-Za-z0-9._-]{12,}`)
	emailPattern          = regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)
	keyValueSecretPattern = regexp.MustCompile(`(?i)\b([a-z0-9_-]*(?:token|secret|password|passwd|api_key|apikey|authorization|credential)[a-z0-9_-]*)(\s*[:=]\s*)(?:"[^"]*"|'[^']*'|[^\s,;]+)`)

	secretMarker = "[REDACTED_SECRET]"
	emailMarker  = "[REDACTED_EMAIL]"
	valueMarker  = "[REDACTED]"
)

// sensitiveKeyMarkers are substrings whose appearance in an attribute key
// marks the whole value as a credential ("bot_token", "api_key",
// "Authorization", "client_secret"). Substring matching on purpose: keys
// arrive prefixed by groups and suffixed by conventions.
var sensitiveKeyMarkers = []string{
	"token", "secret", "password", "passwd",
	"api_key", "apikey", "authorization", "credential",
}

// Redact removes common credential and email forms from s using the
// kernel-wide redaction markers.
func Redact(s string) string {
	s = secretPattern.ReplaceAllString(s, secretMarker)
	s = keyValueSecretPattern.ReplaceAllString(s, `${1}${2}`+valueMarker)
	return emailPattern.ReplaceAllString(s, emailMarker)
}

func sensitiveAttrKey(key string) bool {
	k := strings.ToLower(key)
	for _, marker := range sensitiveKeyMarkers {
		if strings.Contains(k, marker) {
			return true
		}
	}
	return false
}

// redactAttr redacts one attribute: sensitive keys lose their whole value,
// string values are pattern-redacted, and groups recurse into their
// members. Non-string leaves pass through untouched — the tool-result
// boundary owns structured payloads, and the handler layer must not
// re-render typed values.
func redactAttr(a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindGroup {
		members := a.Value.Group()
		out := make([]slog.Attr, len(members))
		for i, member := range members {
			out[i] = redactAttr(member)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(out...)}
	}
	if sensitiveAttrKey(a.Key) {
		return slog.String(a.Key, valueMarker)
	}
	if a.Value.Kind() == slog.KindString {
		return slog.String(a.Key, Redact(a.Value.String()))
	}
	return a
}

// redactingHandler wraps a slog.Handler and redacts every record before
// the inner handler formats it: the message and all string attributes go
// through Redact, sensitive-keyed attributes collapse to a marker. It is
// always on for both kernel sinks (Setup and SetupWorker) — defense in
// depth behind D-010 call-site discipline (LOGGING.md §5), with no config
// knob to switch the guard off.
type redactingHandler struct {
	inner slog.Handler
}

func newRedactingHandler(inner slog.Handler) slog.Handler {
	return redactingHandler{inner: inner}
}

func (h redactingHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h redactingHandler) Handle(ctx context.Context, r slog.Record) error {
	out := slog.NewRecord(r.Time, r.Level, Redact(r.Message), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		out.AddAttrs(redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, out)
}

func (h redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		out[i] = redactAttr(a)
	}
	return redactingHandler{inner: h.inner.WithAttrs(out)}
}

func (h redactingHandler) WithGroup(name string) slog.Handler {
	return redactingHandler{inner: h.inner.WithGroup(name)}
}
