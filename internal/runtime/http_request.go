package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

const (
	defaultHTTPResponseBytes = 1 << 20
	defaultHTTPTimeout       = 10 * time.Second
	maxHTTPTimeout           = 120 * time.Second
)

type HTTPBackend struct {
	mu           sync.RWMutex
	client       *http.Client
	sandbox      *SandboxManager
	allowedHosts []string
	maxBodyBytes int
}

var _ tools.HTTPOperations = (*HTTPBackend)(nil)

func NewHTTPBackend(allowedHosts []string, maxBodyBytes int, timeoutSeconds int, sandbox *SandboxManager) *HTTPBackend {
	if maxBodyBytes <= 0 || maxBodyBytes > 8<<20 {
		maxBodyBytes = defaultHTTPResponseBytes
	}
	hosts := make([]string, 0, len(allowedHosts))
	for _, host := range allowedHosts {
		if normalized := normalizeAllowedHost(host); normalized != "" {
			hosts = append(hosts, normalized)
		}
	}
	transport := &http.Transport{
		Proxy:             http.ProxyFromEnvironment,
		DialContext:       safeDialContext,
		ForceAttemptHTTP2: true,
		MaxIdleConns:      8,
		IdleConnTimeout:   30 * time.Second,
	}
	return &HTTPBackend{
		client:       &http.Client{Transport: transport, Timeout: httpTimeout(timeoutSeconds)},
		sandbox:      sandbox,
		allowedHosts: hosts,
		maxBodyBytes: maxBodyBytes,
	}
}

// httpTimeout clamps an operator-supplied seconds value: 0 (or negative)
// keeps the 10s default and values beyond the 120s ceiling are clamped, so
// a typo can neither disable the timeout nor stall a run for minutes.
func httpTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultHTTPTimeout
	}
	if seconds > 120 {
		return maxHTTPTimeout
	}
	return time.Duration(seconds) * time.Second
}

// SetConfig live-applies the operator-managed allowlist and request timeout
// (settings.yaml http overlay). Hosts are normalized the same way the
// constructor normalizes them; nil keeps the current list.
func (b *HTTPBackend) SetConfig(allowedHosts []string, timeoutSeconds int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if allowedHosts != nil {
		hosts := make([]string, 0, len(allowedHosts))
		for _, host := range allowedHosts {
			if normalized := normalizeAllowedHost(host); normalized != "" {
				hosts = append(hosts, normalized)
			}
		}
		b.allowedHosts = hosts
	}
	if b.client != nil {
		b.client.Timeout = httpTimeout(timeoutSeconds)
	}
}

func (b *HTTPBackend) Request(ctx context.Context, _ domain.RunID, input tools.HTTPRequest) (tools.HTTPResponse, error) {
	method := strings.ToUpper(strings.TrimSpace(input.Method))
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet && method != http.MethodHead {
		return tools.HTTPResponse{}, errors.New("http request: only GET and HEAD are permitted; external writes are denied")
	}
	u, err := url.Parse(strings.TrimSpace(input.URL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return tools.HTTPResponse{}, errors.New("http request: URL must be an absolute HTTP(S) URL without credentials")
	}

	// Sandbox network policy check (D-021)
	if b.sandbox != nil {
		if err := b.sandbox.CheckNetwork(u.String()); err != nil {
			return tools.HTTPResponse{}, fmt.Errorf("sandbox: %w", err)
		}
	}

	if !b.hostAllowed(u.Hostname()) {
		return tools.HTTPResponse{}, fmt.Errorf("http request: host %q is not allowlisted", u.Hostname())
	}
	if queryHasCredential(u.Query()) {
		return tools.HTTPResponse{}, errors.New("http request: credential-like query parameters are denied")
	}
	if err := validateRequestHeaders(input.Headers); err != nil {
		return tools.HTTPResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return tools.HTTPResponse{}, fmt.Errorf("http request: build request: %w", err)
	}
	for key, value := range input.Headers {
		req.Header.Set(key, value)
	}
	b.mu.RLock()
	client := *b.client
	b.mu.RUnlock()
	client.CheckRedirect = func(next *http.Request, _ []*http.Request) error {
		if next.URL.User != nil || next.URL.Host != u.Host || !b.hostAllowed(next.URL.Hostname()) || queryHasCredential(next.URL.Query()) {
			return errors.New("http request: redirect leaves the allowlisted, credential-free surface")
		}
		return validateRequestHeaders(headerMap(next.Header))
	}
	resp, err := client.Do(req)
	if err != nil {
		return tools.HTTPResponse{}, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(b.maxBodyBytes)+1))
	if err != nil {
		return tools.HTTPResponse{}, fmt.Errorf("http request: read response: %w", err)
	}
	if len(body) > b.maxBodyBytes {
		return tools.HTTPResponse{}, fmt.Errorf("http request: response exceeds %d byte limit", b.maxBodyBytes)
	}
	if !utf8.Valid(body) {
		return tools.HTTPResponse{}, errors.New("http request: binary response is not supported by the text tool")
	}
	return tools.HTTPResponse{
		StatusCode:  resp.StatusCode,
		URL:         resp.Request.URL.String(),
		Headers:     safeResponseHeaders(resp.Header),
		Body:        string(body),
		ContentType: resp.Header.Get("Content-Type"),
		Untrusted:   true,
	}, nil
}

func (b *HTTPBackend) hostAllowed(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, allowed := range b.allowedHosts {
		if strings.HasPrefix(allowed, "*.") {
			base := strings.TrimPrefix(allowed, "*.")
			if host != base && strings.HasSuffix(host, "."+base) {
				return true
			}
			continue
		}
		if host == allowed {
			return true
		}
	}
	return false
}
func normalizeAllowedHost(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Hostname() != "" {
		value = parsed.Hostname()
	}
	return strings.TrimSuffix(value, ".")
}

func queryHasCredential(values url.Values) bool {
	for key := range values {
		switch strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), ".", "_")) {
		case "token", "access_token", "api_key", "apikey", "key", "password", "passwd", "secret", "credential", "authorization":
			return true
		}
	}
	return false
}

func validateRequestHeaders(headers map[string]string) error {
	for key := range headers {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "authorization", "proxy-authorization", "cookie", "set-cookie", "x-api-key", "x-auth-token":
			return fmt.Errorf("http request: credential-bearing header %q is denied", key)
		}
	}
	return nil
}

func safeResponseHeaders(headers http.Header) map[string]string {
	result := make(map[string]string, 2)
	for _, key := range []string{"Content-Type", "Content-Length"} {
		if value := headers.Get(key); value != "" {
			result[strings.ToLower(key)] = value
		}
	}
	return result
}

func headerMap(headers http.Header) map[string]string {
	result := make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return result
}

func safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if !isExplicitLocalHost(host) {
		ips, err := net.LookupIP(host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if isPrivateOrLocalIP(ip) {
				return nil, errors.New("http request: resolved address is private or local")
			}
		}
	}
	return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(host, port))
}

func isExplicitLocalHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func isPrivateOrLocalIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
