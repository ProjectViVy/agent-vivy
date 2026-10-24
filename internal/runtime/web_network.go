package runtime

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/http/httpproxy"
)

// errPublicTargetDenied reports a target outside the public internet. It is a
// governance refusal, not a transport failure: no request reached the target.
var errPublicTargetDenied = errors.New("public fetch: target address is private or local")

// webNetwork is the shared outbound policy for the public web tools. The
// operator-configured proxy endpoint is the only private dial target allowed;
// every other connection and every proxied target must resolve to a public
// address.
type webNetwork struct {
	proxyFunc     func(*url.URL) (*url.URL, error)
	dialAllow     map[string]struct{}
	allowLoopback bool
	lookupIP      func(context.Context, string) ([]net.IPAddr, error)
}

func newWebNetworkFromEnv() *webNetwork {
	return newWebNetwork(httpproxy.FromEnvironment())
}

func newWebNetwork(cfg *httpproxy.Config) *webNetwork {
	proxyFunc := cfg.ProxyFunc()
	return &webNetwork{
		proxyFunc: proxyFunc,
		dialAllow: proxyDialAllow(cfg),
		lookupIP: func(ctx context.Context, host string) ([]net.IPAddr, error) {
			return net.DefaultResolver.LookupIPAddr(ctx, host)
		},
	}
}

// proxyDialAllow records the proxy endpoints the transport may dial. It reads
// the configured endpoints directly rather than asking the proxy function about
// a sample URL: NO_PROXY decides whether a request uses a proxy at all, not
// whether a configured endpoint is reachable, so probing a host that NO_PROXY
// matches would silently drop every endpoint and deny all proxied dials.
func proxyDialAllow(cfg *httpproxy.Config) map[string]struct{} {
	allow := map[string]struct{}{}
	if cfg == nil {
		return allow
	}
	for _, raw := range []string{cfg.HTTPProxy, cfg.HTTPSProxy} {
		if proxy, ok := parseProxyEndpoint(raw); ok {
			allow[proxy] = struct{}{}
		}
	}
	return allow
}

// parseProxyEndpoint normalizes a configured proxy into the host:port the
// transport dials. The scheme rule mirrors httpproxy.parseProxy: anything that
// is not a recognized proxy scheme is read as an HTTP proxy.
func parseProxyEndpoint(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || !isProxyScheme(u.Scheme) {
		if u, err = url.Parse("http://" + raw); err != nil {
			return "", false
		}
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "" {
		return "", false
	}
	port := u.Port()
	if port == "" {
		port = defaultProxyPort(u.Scheme)
	}
	return net.JoinHostPort(host, port), true
}

func isProxyScheme(scheme string) bool {
	switch strings.ToLower(scheme) {
	case "http", "https", "socks5", "socks5h":
		return true
	default:
		return false
	}
}

func defaultProxyPort(scheme string) string {
	switch strings.ToLower(scheme) {
	case "https":
		return "443"
	case "socks5", "socks5h":
		return "1080"
	default:
		return "80"
	}
}

func (n *webNetwork) transport() *http.Transport {
	return &http.Transport{
		Proxy:             n.proxy,
		DialContext:       n.dialContext,
		ForceAttemptHTTP2: true,
		MaxIdleConns:      8,
		IdleConnTimeout:   30 * time.Second,
	}
}

func (n *webNetwork) proxy(req *http.Request) (*url.URL, error) {
	if n == nil || n.proxyFunc == nil {
		return nil, nil
	}
	return n.proxyFunc(req.URL)
}

// testTransport dials directly and accepts loopback so httptest servers can
// exercise the pipeline. Production never installs it.
func (n *webNetwork) testTransport() *http.Transport {
	return &http.Transport{
		DialContext:       safeDialContext,
		ForceAttemptHTTP2: true,
	}
}

// dialContext is safeDialContext for the public web tools: the configured
// proxy endpoint is reachable, and every other address must be public. A
// private target fails at dial time even if DNS rebinding swaps the address
// after the preflight check.
func (n *webNetwork) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if n.isProxyAddress(host, port) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, address)
	}
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateOrLocalIP(ip) {
			return nil, errPublicTargetDenied
		}
	} else {
		ips, err := net.LookupIP(host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if isPrivateOrLocalIP(ip) {
				return nil, errPublicTargetDenied
			}
		}
	}
	return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(host, port))
}

func (n *webNetwork) isProxyAddress(host, port string) bool {
	if n == nil || len(n.dialAllow) == 0 {
		return false
	}
	key := net.JoinHostPort(strings.ToLower(strings.TrimSuffix(host, ".")), port)
	_, ok := n.dialAllow[key]
	return ok
}

// validateTarget rejects private and local targets before a request is sent.
// Proxied requests resolve the target locally as well: the proxy is trusted
// to connect, not to decide which hosts are public.
func (n *webNetwork) validateTarget(ctx context.Context, u *url.URL) error {
	if n == nil || n.allowLoopback || u == nil {
		return nil
	}
	host := strings.TrimSuffix(strings.TrimSpace(u.Hostname()), ".")
	if host == "" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateOrLocalIP(ip) {
			return errPublicTargetDenied
		}
		return nil
	}
	ips, err := n.lookupIP(ctx, host)
	if err != nil {
		return fmt.Errorf("public fetch: resolve %s: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("public fetch: resolve %s: no addresses", host)
	}
	for _, addr := range ips {
		if isPrivateOrLocalIP(addr.IP) {
			return errPublicTargetDenied
		}
	}
	return nil
}

// redirectCheck keeps every redirect hop on the same public, credential-free
// surface. A redirect to a private target is a fetch failure with unknown
// effects because the initial request already ran; it is not a not-executed
// refusal.
func (n *webNetwork) redirectCheck() func(*http.Request, []*http.Request) error {
	return func(next *http.Request, via []*http.Request) error {
		if len(via) >= maxFetchRedirects {
			return fmt.Errorf("public fetch: stopped after %d redirects", maxFetchRedirects)
		}
		if _, err := validatePublicURL(next.URL.String()); err != nil {
			return errors.New("public fetch: redirect leaves the credential-free internet surface")
		}
		if err := n.validateTarget(next.Context(), next.URL); err != nil {
			return fmt.Errorf("public fetch: redirect target is not public: %v", err)
		}
		return nil
	}
}
