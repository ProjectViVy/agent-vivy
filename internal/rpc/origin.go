package rpc

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// OriginPolicy permits same-origin requests and a configured set of exact
// browser origins for a separately hosted local UI.
type OriginPolicy struct {
	allowed map[string]struct{}
}

// NewOriginPolicy validates and normalizes the configured origins. The
// configuration layer additionally restricts them to loopback hosts; this
// lower-level policy remains reusable by transport tests and callers that
// provide their own deployment boundary.
func NewOriginPolicy(origins []string) (OriginPolicy, error) {
	policy := OriginPolicy{allowed: make(map[string]struct{}, len(origins))}
	for i, raw := range origins {
		origin, err := normalizeOrigin(raw)
		if err != nil {
			return OriginPolicy{}, fmt.Errorf("origin %d %q: %w", i, raw, err)
		}
		if _, exists := policy.allowed[origin]; exists {
			return OriginPolicy{}, fmt.Errorf("origin %q is duplicated", raw)
		}
		policy.allowed[origin] = struct{}{}
	}
	return policy, nil
}

// Allows reports whether a browser request may reach the control plane.
// Requests without Origin are retained for local CLI and test clients.
func (p OriginPolicy) Allows(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	normalized, err := normalizeOrigin(origin)
	if err != nil {
		return false
	}
	if normalized == requestOrigin(r) {
		return true
	}
	_, ok := p.allowed[normalized]
	return ok
}

// ApplyCORS emits the minimal response headers needed by a permitted
// cross-origin browser request. Callers must check Allows first.
func (p OriginPolicy) ApplyCORS(w http.ResponseWriter, r *http.Request) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return
	}
	w.Header().Add("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Origin", origin)
}

func requestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return strings.ToLower(scheme + "://" + r.Host)
}

func normalizeOrigin(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("must be an absolute HTTP(S) origin")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("must use http or https")
	}
	if parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return "", errors.New("must not include credentials, path, query, or fragment")
	}
	if parsed.Hostname() == "" {
		return "", errors.New("must include a host")
	}
	if port := parsed.Port(); port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return "", errors.New("contains an invalid port")
		}
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host), nil
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
