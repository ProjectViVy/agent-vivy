package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

const (
	webFetchUserAgent   = "vivy-fetch/1.0"
	defaultFetchTimeout = 30 * time.Second
	minFetchTimeout     = 5 * time.Second
	maxFetchTimeout     = 120 * time.Second
	defaultDownloadTime = 300 * time.Second
	minDownloadTimeout  = 5 * time.Second
	maxDownloadTimeout  = 600 * time.Second
	maxFetchRedirects   = 5
	noisyHTMLSelector   = "script, style, noscript, iframe, svg, template, nav, header, footer, aside, form"
)

var markdownConverter = htmltomarkdown.NewConverter("", true, nil)

// EinoWebFetchBackend fetches public web text for the conversation. Unlike
// EinoHTTPBackend there is no host allowlist to fall back on, so the dialer
// refuses private and local addresses unconditionally.
type EinoWebFetchBackend struct {
	client       *http.Client
	sandbox      *SandboxManager
	maxBodyBytes int
}

var _ tools.WebFetchOperations = (*EinoWebFetchBackend)(nil)

func NewEinoWebFetchBackend(maxBodyBytes int, sandbox *SandboxManager) *EinoWebFetchBackend {
	if maxBodyBytes <= 0 || maxBodyBytes > 8<<20 {
		maxBodyBytes = defaultHTTPResponseBytes
	}
	return &EinoWebFetchBackend{
		client:       &http.Client{Transport: newPublicHTTPTransport(), Timeout: defaultFetchTimeout},
		sandbox:      sandbox,
		maxBodyBytes: maxBodyBytes,
	}
}

// allowLoopbackForTest lets httptest servers on 127.0.0.1 exercise the
// pipeline in tests; the production constructor never enables it.
func (b *EinoWebFetchBackend) allowLoopbackForTest() {
	b.client.Transport = &http.Transport{
		Proxy:             http.ProxyFromEnvironment,
		DialContext:       safeDialContext,
		ForceAttemptHTTP2: true,
	}
}

func newPublicHTTPTransport() *http.Transport {
	return &http.Transport{
		Proxy:             http.ProxyFromEnvironment,
		DialContext:       publicOnlyDialContext,
		ForceAttemptHTTP2: true,
		MaxIdleConns:      8,
		IdleConnTimeout:   30 * time.Second,
	}
}

// publicOnlyDialContext is safeDialContext without the explicit-localhost
// bypass: the web tools have no allowlist, so a private target must fail at
// dial time even if DNS rebinding swaps the address after validation.
func publicOnlyDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateOrLocalIP(ip) {
			return nil, errors.New("public fetch: target address is private or local")
		}
	} else {
		ips, err := net.LookupIP(host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if isPrivateOrLocalIP(ip) {
				return nil, errors.New("public fetch: resolved address is private or local")
			}
		}
	}
	return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(host, port))
}

// validatePublicURL enforces the shared web-tool surface: absolute HTTP(S),
// no embedded credentials, no credential-like query parameters.
func validatePublicURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return nil, errors.New("public fetch: URL must be an absolute HTTP(S) URL without credentials")
	}
	if queryHasCredential(u.Query()) {
		return nil, errors.New("public fetch: credential-like query parameters are denied")
	}
	return u, nil
}

func publicRedirectCheck() func(*http.Request, []*http.Request) error {
	return func(next *http.Request, via []*http.Request) error {
		if len(via) >= maxFetchRedirects {
			return fmt.Errorf("public fetch: stopped after %d redirects", maxFetchRedirects)
		}
		if _, err := validatePublicURL(next.URL.String()); err != nil {
			return errors.New("public fetch: redirect leaves the credential-free internet surface")
		}
		return nil
	}
}

func clampFetchTimeout(seconds int, fallback, min, max time.Duration) time.Duration {
	if seconds <= 0 {
		return fallback
	}
	d := time.Duration(seconds) * time.Second
	if d < min {
		return min
	}
	if d > max {
		return max
	}
	return d
}

// Fetch implements tools.WebFetchOperations.
func (b *EinoWebFetchBackend) Fetch(ctx context.Context, _ domain.RunID, input tools.WebFetchRequest) (tools.WebFetchResult, error) {
	format := strings.ToLower(strings.TrimSpace(input.Format))
	if format == "" {
		format = "markdown"
	}
	u, err := validatePublicURL(input.URL)
	if err != nil {
		return tools.WebFetchResult{}, err
	}
	if b.sandbox != nil {
		if err := b.sandbox.CheckNetwork(u.String()); err != nil {
			return tools.WebFetchResult{}, fmt.Errorf("sandbox: %w", err)
		}
	}
	client := *b.client
	client.Timeout = clampFetchTimeout(input.TimeoutSeconds, defaultFetchTimeout, minFetchTimeout, maxFetchTimeout)
	client.CheckRedirect = publicRedirectCheck()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return tools.WebFetchResult{}, fmt.Errorf("web fetch: build request: %w", err)
	}
	req.Header.Set("User-Agent", webFetchUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json;q=0.9,text/*;q=0.8,*/*;q=0.5")
	resp, err := client.Do(req)
	if err != nil {
		return tools.WebFetchResult{}, fmt.Errorf("web fetch: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(b.maxBodyBytes)+1))
	if err != nil {
		return tools.WebFetchResult{}, fmt.Errorf("web fetch: read response: %w", err)
	}
	truncated := len(body) > b.maxBodyBytes
	if truncated {
		body = body[:b.maxBodyBytes]
	}
	contentType := resp.Header.Get("Content-Type")
	media := mediaTypeOf(contentType)
	if !isTextMedia(media) {
		return tools.WebFetchResult{}, fmt.Errorf("web fetch: content type %q is not readable text; use download for binary content", contentType)
	}
	if !utf8.Valid(body) {
		return tools.WebFetchResult{}, errors.New("web fetch: response is not valid UTF-8 text")
	}
	// Error pages often carry readable context (rate limits, auth hints), so
	// a non-2xx is a bounded result rather than an error; the same caps and
	// the untrusted-output header still apply.
	var content, outFormat string
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		content, outFormat = strings.TrimSpace(string(body)), "text"
	} else {
		content, outFormat, err = convertFetchedContent(media, format, body)
		if err != nil {
			return tools.WebFetchResult{}, err
		}
	}
	if truncated {
		content += fmt.Sprintf("\n\n[Content truncated to %d bytes]", b.maxBodyBytes)
	}
	return tools.WebFetchResult{
		URL:         resp.Request.URL.String(),
		StatusCode:  resp.StatusCode,
		ContentType: contentType,
		Format:      outFormat,
		Content:     content,
		Truncated:   truncated,
		Bytes:       len(content),
		Untrusted:   true,
	}, nil
}

func mediaTypeOf(contentType string) string {
	media := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return strings.TrimSpace(media)
}

func isTextMedia(media string) bool {
	switch {
	case media == "":
		return true
	case strings.HasPrefix(media, "text/"):
		return true
	case media == "text/html" || media == "application/xhtml+xml":
		return true
	case media == "application/json" || strings.HasSuffix(media, "+json"):
		return true
	case media == "application/xml" || strings.HasSuffix(media, "+xml"):
		return true
	default:
		return false
	}
}

func convertFetchedContent(media, format string, body []byte) (string, string, error) {
	switch {
	case media == "application/json" || strings.HasSuffix(media, "+json"):
		return formatJSONBody(body)
	case media == "text/html" || media == "application/xhtml+xml":
		return convertHTMLContent(body, format)
	default:
		return string(body), "text", nil
	}
}

func formatJSONBody(body []byte) (string, string, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, body, "", "  "); err != nil {
		// The Content-Type claimed JSON but the body is not; pass it through.
		return string(body), "text", nil
	}
	return buf.String(), "text", nil
}

func convertHTMLContent(body []byte, format string) (string, string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("web fetch: parse html: %w", err)
	}
	doc.Find(noisyHTMLSelector).Remove()
	selection := doc.Find("body")
	if selection.Length() == 0 {
		selection = doc.Selection
	}
	switch format {
	case "html":
		bodyHTML, err := selection.Html()
		if err != nil {
			return "", "", fmt.Errorf("web fetch: extract body: %w", err)
		}
		bodyHTML = strings.TrimSpace(bodyHTML)
		if bodyHTML == "" {
			return "", "", errors.New("web fetch: page has no body content")
		}
		return bodyHTML, "html", nil
	case "text":
		return strings.Join(strings.Fields(selection.Text()), " "), "text", nil
	default:
		bodyHTML, err := selection.Html()
		if err != nil {
			return "", "", fmt.Errorf("web fetch: extract body: %w", err)
		}
		markdown, err := markdownConverter.ConvertString(bodyHTML)
		if err != nil {
			return "", "", fmt.Errorf("web fetch: convert html to markdown: %w", err)
		}
		markdown = strings.TrimSpace(markdown)
		if markdown == "" {
			return "", "", errors.New("web fetch: page has no readable content")
		}
		return markdown, "markdown", nil
	}
}
