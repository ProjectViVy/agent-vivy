package lsp

import (
	"path/filepath"
	"strings"
)

// Wire types for the LSP subset this plugin speaks. Positions use the
// LSP default UTF-16 code units; the tool reports them as received.

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type span struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type diagnostic struct {
	Range    span   `json:"range"`
	Severity int    `json:"severity,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

type publishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Version     *int         `json:"version,omitempty"`
	Diagnostics []diagnostic `json:"diagnostics"`
}

type textDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type versionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type didChangeParams struct {
	TextDocument   versionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
}

type initializeParams struct {
	ProcessID    *int     `json:"processId"`
	RootURI      string   `json:"rootUri"`
	Capabilities struct{} `json:"capabilities"`
}

// pathToURI converts a workspace-relative path into a file URI rooted at
// the run workspace.
func pathToURI(root, rel string) string {
	abs := filepath.Join(root, filepath.FromSlash(rel))
	return "file:///" + strings.TrimPrefix(filepath.ToSlash(abs), "/")
}

// uriToRel maps a file URI back to a workspace-relative path. URIs outside
// the workspace (or with a different drive-letter case on Windows) are
// reported verbatim instead of guessed.
func uriToRel(root, uri string) (string, bool) {
	rootURI := pathToURI(root, ".")
	prefix := rootURI + "/"
	if len(uri) > len(prefix)-1 && strings.EqualFold(uri[:len(prefix)], prefix) {
		return uri[len(prefix):], true
	}
	if strings.EqualFold(uri, rootURI) {
		return ".", true
	}
	return uri, false
}
