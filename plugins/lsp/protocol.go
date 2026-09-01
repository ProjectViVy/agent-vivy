package lsp

import (
	"encoding/json"
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

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

// location is one LSP Location (definition/reference hit).
type location struct {
	URI   string `json:"uri"`
	Range span   `json:"range"`
}

type definitionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

type referenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

type referenceParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
	Context      referenceContext       `json:"context"`
}

// documentSymbol is the hierarchical documentSymbol reply shape.
type documentSymbol struct {
	Name     string           `json:"name"`
	Kind     int              `json:"kind"`
	Range    span             `json:"range"`
	Children []documentSymbol `json:"children,omitempty"`
}

// symbolInformation is the flat reply shape some servers use.
type symbolInformation struct {
	Name     string   `json:"name"`
	Kind     int      `json:"kind"`
	Location location `json:"location"`
}

// parseSymbols accepts both reply shapes: hierarchical documentSymbol
// arrays and flat symbolInformation arrays (heuristic: an element that
// carries a "location" key is the flat shape).
func parseSymbols(raw json.RawMessage) ([]documentSymbol, []symbolInformation, error) {
	var elements []json.RawMessage
	if err := json.Unmarshal(raw, &elements); err != nil {
		return nil, nil, err
	}
	for _, el := range elements {
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(el, &probe); err != nil {
			return nil, nil, err
		}
		if _, ok := probe["location"]; ok {
			var flat []symbolInformation
			if err := json.Unmarshal(raw, &flat); err != nil {
				return nil, nil, err
			}
			return nil, flat, nil
		}
		break
	}
	var tree []documentSymbol
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, nil, err
	}
	return tree, nil, nil
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
