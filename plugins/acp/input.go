package acp

import (
	"net/url"
	"path/filepath"
	"strings"

	acp "github.com/eino-contrib/acp"
)

// Prompt bounds (spec §10/§12.4): accumulated user text stays under 256 KiB
// and each reference field is independently bounded.
const (
	maxPromptTextBytes  = 256 << 10
	maxResourceFieldLen = 8 << 10
)

// normalizePrompt converts ACP content blocks into the turn/start payload.
// Text blocks concatenate in input order with a newline separator; resource
// links contribute a deterministic reference label at their position and
// safe local file: URIs additionally map to deduplicated context_paths
// relative to the durable session root. The adapter never opens, reads or
// fetches a referenced resource; image/audio/embedded blocks are rejected.
func normalizePrompt(root string, req acp.PromptRequest) (text string, contextPaths []string, err error) {
	var parts []string
	seen := map[string]bool{}
	size := 0

	appendText := func(s string) error {
		size += len(s)
		if size > maxPromptTextBytes {
			return rpcError(-32602, "prompt exceeds the text bound", "INVALID_INPUT")
		}
		parts = append(parts, s)
		return nil
	}

	for _, block := range req.Prompt {
		switch {
		case block.Text != nil:
			if err := appendText(block.Text.Text); err != nil {
				return "", nil, err
			}
		case block.ResourceLink != nil:
			label, rel, err := normalizeResourceLink(root, block.ResourceLink)
			if err != nil {
				return "", nil, err
			}
			if rel != "" && !seen[rel] {
				seen[rel] = true
				contextPaths = append(contextPaths, rel)
			}
			if err := appendText(label); err != nil {
				return "", nil, err
			}
		default:
			// image, audio, embedded resources and unknown/empty blocks are
			// not accepted in the restricted pilot.
			return "", nil, rpcError(-32602, "unsupported prompt content block", "INVALID_INPUT")
		}
	}

	text = strings.Join(parts, "\n")
	if strings.TrimSpace(text) == "" {
		return "", nil, rpcError(-32602, "prompt carries no usable content", "INVALID_INPUT")
	}
	// The newline separators join into the wire text; bound the joined form
	// so the per-block accounting cannot undercount the wire payload.
	if len(text) > maxPromptTextBytes {
		return "", nil, rpcError(-32602, "prompt exceeds the text bound", "INVALID_INPUT")
	}
	return text, contextPaths, nil
}

// normalizeResourceLink validates one resource link. It returns the user-text
// reference label and, for safe local file: URIs only, the session-relative
// context path. Non-file schemes remain pure references.
func normalizeResourceLink(root string, link *acp.ContentBlockResourceLink) (label, contextPath string, err error) {
	uri := link.URI
	name := link.Name
	if len(uri) > maxResourceFieldLen || len(name) > maxResourceFieldLen ||
		hasControlChar(uri) || hasControlChar(name) || strings.ContainsAny(name, "\r\n") {
		return "", "", rpcError(-32602, "resource link field out of bounds", "INVALID_INPUT")
	}
	u, perr := url.Parse(uri)
	if perr != nil || u.Scheme == "" || u.User != nil {
		return "", "", rpcError(-32602, "malformed or credential-bearing resource URI", "INVALID_INPUT")
	}

	if u.Scheme == "file" {
		rel, cerr := fileURIToContextPath(root, u)
		if cerr != nil {
			return "", "", cerr
		}
		contextPath = rel
	}

	// The label is deterministic user content; name and URI are never
	// treated as instructions.
	if name != "" {
		label = "[resource: " + name + "] " + uri
	} else {
		label = "[resource] " + uri
	}
	return label, contextPath, nil
}

// fileURIToContextPath converts one validated file: URI into a
// session-root-relative slash path. Query, fragment, remote authority,
// UNC/device forms, encoded separators and traversal are rejected outright —
// an unsafe file URI fails the prompt rather than degrading to a reference.
func fileURIToContextPath(root string, u *url.URL) (string, error) {
	reject := rpcError(-32602, "unsafe file URI", "INVALID_INPUT")
	if u.RawQuery != "" || u.Fragment != "" || u.ForceQuery {
		return "", reject
	}
	if u.Opaque != "" {
		return "", reject
	}
	host := strings.ToLower(u.Host)
	if host != "" && host != "localhost" {
		return "", reject
	}
	rawPath := u.EscapedPath()
	lower := strings.ToLower(rawPath)
	if strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") ||
		strings.Contains(lower, "%2e") || strings.Contains(lower, "%00") {
		return "", reject
	}
	decoded, derr := url.PathUnescape(rawPath)
	if derr != nil || decoded == "" {
		return "", reject
	}
	if hasControlChar(decoded) {
		return "", reject
	}
	if !filepath.IsAbs(decoded) {
		return "", reject
	}
	clean := filepath.Clean(decoded)
	for _, seg := range strings.Split(filepath.ToSlash(clean), "/") {
		if seg == ".." {
			return "", reject
		}
	}
	rel, rerr := filepath.Rel(root, clean)
	if rerr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", reject
	}
	return filepath.ToSlash(rel), nil
}
