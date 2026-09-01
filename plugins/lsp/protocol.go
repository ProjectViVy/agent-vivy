package lsp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
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

// renameParams is the textDocument/rename request.
type renameParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
	NewName      string                 `json:"newName"`
}

// textEdit is one replacement inside a WorkspaceEdit. Range positions are
// UTF-16 code units (the LSP default).
type textEdit struct {
	Range   span   `json:"range"`
	NewText string `json:"newText"`
}

// workspaceEdit is the textDocument/rename reply (changes shape only;
// documentChanges requires a client capability this plugin does not
// declare, so servers keep to the simple form).
type workspaceEdit struct {
	Changes map[string][]textEdit `json:"changes"`
}

// utf16Offset maps an LSP position (line, UTF-16 character) onto a byte
// offset in content. A character beyond the line's length clamps to the
// end of the line; a line beyond the file clamps to EOF.
func utf16Offset(content string, pos position) int {
	lineStart := 0
	line := 0
	for line < pos.Line {
		nl := strings.IndexByte(content[lineStart:], '\n')
		if nl < 0 {
			return len(content)
		}
		lineStart += nl + 1
		line++
	}
	lineEnd := len(content)
	if nl := strings.IndexByte(content[lineStart:], '\n'); nl >= 0 {
		lineEnd = lineStart + nl
	}
	units := 0
	offset := lineStart
	for offset < lineEnd {
		if units >= pos.Character {
			break
		}
		r, size := utf8.DecodeRuneInString(content[offset:])
		if r >= 0x10000 {
			units += 2
		} else {
			units++
		}
		offset += size
	}
	return offset
}

// applyEdits folds a TextEdit list onto content. Edits are applied
// back-to-front so earlier offsets stay valid.
func applyEdits(content string, edits []textEdit) (string, error) {
	sorted := append([]textEdit(nil), edits...)
	sort.Slice(sorted, func(i, j int) bool {
		si, sj := sorted[i].Range.Start, sorted[j].Range.Start
		if si.Line != sj.Line {
			return si.Line > sj.Line
		}
		return si.Character > sj.Character
	})
	for _, e := range sorted {
		start := utf16Offset(content, e.Range.Start)
		end := utf16Offset(content, e.Range.End)
		if start > end {
			return "", fmt.Errorf("lsp: inverted edit range %d..%d", start, end)
		}
		content = content[:start] + e.NewText + content[end:]
	}
	return content, nil
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
