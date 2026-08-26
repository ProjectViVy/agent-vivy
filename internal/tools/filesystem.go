package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"agent-vivy/internal/domain"
)

const (
	ListDirName     = "list_dir"
	ReadFileName    = "read_file"
	SearchFilesName = "search_files"
	WriteFileName   = "write_file"
	PatchName       = "patch"
)

// DirListRequest is the Vivy-owned listing contract. Depth applies only when
// Recursive is set; zero lets the backend pick its default. Ignored
// directories (.git, node_modules, ...) are listed but never traversed.
type DirListRequest struct {
	Path       string
	Recursive  bool
	Depth      int
	MaxEntries int
}

type DirEntry struct {
	Path       string `json:"path"`
	IsDir      bool   `json:"is_dir"`
	Size       int64  `json:"size,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
}

type DirListResult struct {
	Path      string     `json:"path"`
	Entries   []DirEntry `json:"entries"`
	Truncated bool       `json:"truncated"`
}

// FileReadRequest is the Vivy-owned read contract. The runtime filesystem
// adapter maps it to Eino's filesystem.Backend request types.
type FileReadRequest struct {
	Path      string
	StartLine int
	EndLine   int
	MaxBytes  int
}

type FileReadResult struct {
	Path       string `json:"path"`
	Content    string `json:"content,omitempty"`
	StartLine  int    `json:"start_line,omitempty"`
	EndLine    int    `json:"end_line,omitempty"`
	TotalLines int    `json:"total_lines,omitempty"`
	Bytes      int    `json:"bytes"`
	Binary     bool   `json:"binary"`
	Truncated  bool   `json:"truncated"`
}

type FileSearchRequest struct {
	Query         string
	Path          string
	Glob          string
	MaxResults    int
	MaxBytes      int
	CaseSensitive bool
}

type FileMatch struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type FileSearchResult struct {
	Matches      []FileMatch `json:"matches"`
	FilesScanned int         `json:"files_scanned"`
	Truncated    bool        `json:"truncated"`
}

type FileWriteRequest struct {
	Path          string
	Content       string
	CreateParents bool
}

type FilePatchRequest struct {
	Path       string
	OldString  string
	NewString  string
	ReplaceAll bool
}

type FileMutationResult struct {
	Path    string `json:"path"`
	Changed bool   `json:"changed"`
	Bytes   int    `json:"bytes"`
	SHA256  string `json:"sha256"`
	Diff    string `json:"diff,omitempty"`
}

// FileOperations is the non-Eino contract consumed by Vivy tools. Runtime
// implements it with an Eino filesystem.Backend adapter.
type FileOperations interface {
	ListDir(context.Context, domain.RunID, DirListRequest) (DirListResult, error)
	ReadFile(context.Context, domain.RunID, FileReadRequest) (FileReadResult, error)
	SearchFiles(context.Context, domain.RunID, FileSearchRequest) (FileSearchResult, error)
	WriteFile(context.Context, domain.RunID, FileWriteRequest) (FileMutationResult, error)
	PatchFile(context.Context, domain.RunID, FilePatchRequest) (FileMutationResult, error)
}

type fileMutationPreview interface {
	PrepareWriteFile(context.Context, domain.RunID, FileWriteRequest) (domain.ToolProposal, error)
	PreparePatchFile(context.Context, domain.RunID, FilePatchRequest) (domain.ToolProposal, error)
}

type listDirTool struct{ ops FileOperations }
type readFileTool struct{ ops FileOperations }
type searchFilesTool struct{ ops FileOperations }
type writeFileTool struct{ ops FileOperations }
type patchTool struct{ ops FileOperations }

func NewListDir(ops FileOperations) Tool     { return &listDirTool{ops: ops} }
func NewReadFile(ops FileOperations) Tool    { return &readFileTool{ops: ops} }
func NewSearchFiles(ops FileOperations) Tool { return &searchFilesTool{ops: ops} }
func NewWriteFile(ops FileOperations) Tool   { return &writeFileTool{ops: ops} }
func NewPatch(ops FileOperations) Tool       { return &patchTool{ops: ops} }

func (t *listDirTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: ListDirName, Description: "Lists directory entries in the current run workspace; optional bounded recursion for exploring an unfamiliar tree.", Readonly: true,
		Keywords: []string{"list", "directory", "dir", "ls", "explore", "workspace", "tree"},
		Params: map[string]domain.ToolParam{
			"path":        {Desc: "Workspace-relative directory path.", Required: true},
			"recursive":   {Desc: "Optional true/false; include nested entries below the directory.", Required: false},
			"depth":       {Desc: "Optional max levels below path when recursive.", Required: false},
			"max_entries": {Desc: "Optional entry limit.", Required: false},
		},
	}
}

func (t *listDirTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	var input struct {
		Path       string `json:"path"`
		Recursive  string `json:"recursive"`
		Depth      string `json:"depth"`
		MaxEntries string `json:"max_entries"`
	}
	if err := decodeStringArgs(args, &input); err != nil {
		return "", err
	}
	recursive, err := optionalBool("recursive", input.Recursive)
	if err != nil {
		return "", err
	}
	depth, err := optionalInt("depth", input.Depth)
	if err != nil {
		return "", err
	}
	maxEntries, err := optionalInt("max_entries", input.MaxEntries)
	if err != nil {
		return "", err
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: filesystem backend not wired")
	}
	result, err := t.ops.ListDir(ctx, RunIDFromContext(ctx), DirListRequest{Path: input.Path, Recursive: recursive, Depth: depth, MaxEntries: maxEntries})
	if err != nil {
		return "", err
	}
	return marshalToolResult(result)
}

func (t *readFileTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: ReadFileName, Description: "Reads a bounded text file from the current run workspace.", Readonly: true,
		Keywords: []string{"read", "file", "source", "cat"},
		Params: map[string]domain.ToolParam{
			"path":       {Desc: "Workspace-relative file path.", Required: true},
			"start_line": {Desc: "Optional 1-based first line.", Required: false},
			"end_line":   {Desc: "Optional inclusive last line.", Required: false},
			"max_bytes":  {Desc: "Optional maximum file bytes.", Required: false},
		},
	}
}

func (t *readFileTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	var input struct {
		Path      string `json:"path"`
		StartLine string `json:"start_line"`
		EndLine   string `json:"end_line"`
		MaxBytes  string `json:"max_bytes"`
	}
	if err := decodeStringArgs(args, &input); err != nil {
		return "", err
	}
	start, err := optionalInt("start_line", input.StartLine)
	if err != nil {
		return "", err
	}
	end, err := optionalInt("end_line", input.EndLine)
	if err != nil {
		return "", err
	}
	maxBytes, err := optionalInt("max_bytes", input.MaxBytes)
	if err != nil {
		return "", err
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: filesystem backend not wired")
	}
	result, err := t.ops.ReadFile(ctx, RunIDFromContext(ctx), FileReadRequest{Path: input.Path, StartLine: start, EndLine: end, MaxBytes: maxBytes})
	if err != nil {
		return "", err
	}
	return marshalToolResult(result)
}

func (t *searchFilesTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: SearchFilesName, Description: "Searches bounded workspace files for literal text.", Readonly: true,
		Keywords: []string{"search", "find", "grep", "files", "code"},
		Params: map[string]domain.ToolParam{
			"query":          {Desc: "Literal text to find.", Required: true},
			"path":           {Desc: "Optional workspace-relative directory or file.", Required: false},
			"glob":           {Desc: "Optional file glob filter.", Required: false},
			"max_results":    {Desc: "Optional result limit.", Required: false},
			"max_bytes":      {Desc: "Optional total result byte limit.", Required: false},
			"case_sensitive": {Desc: "Optional true/false case matching flag.", Required: false},
		},
	}
}

func (t *searchFilesTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	var input struct {
		Query         string `json:"query"`
		Path          string `json:"path"`
		Glob          string `json:"glob"`
		MaxResults    string `json:"max_results"`
		MaxBytes      string `json:"max_bytes"`
		CaseSensitive string `json:"case_sensitive"`
	}
	if err := decodeStringArgs(args, &input); err != nil {
		return "", err
	}
	maxResults, err := optionalInt("max_results", input.MaxResults)
	if err != nil {
		return "", err
	}
	maxBytes, err := optionalInt("max_bytes", input.MaxBytes)
	if err != nil {
		return "", err
	}
	caseSensitive, err := optionalBool("case_sensitive", input.CaseSensitive)
	if err != nil {
		return "", err
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: filesystem backend not wired")
	}
	result, err := t.ops.SearchFiles(ctx, RunIDFromContext(ctx), FileSearchRequest{
		Query: input.Query, Path: input.Path, Glob: input.Glob, MaxResults: maxResults, MaxBytes: maxBytes, CaseSensitive: caseSensitive,
	})
	if err != nil {
		return "", err
	}
	return marshalToolResult(result)
}

func (t *writeFileTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: WriteFileName, Description: "Writes a file in the current run workspace after approval.", Readonly: false,
		Keywords: []string{"write", "save", "create", "file"},
		Params: map[string]domain.ToolParam{
			"path":    {Desc: "Workspace-relative file path.", Required: true},
			"content": {Desc: "Complete replacement file content.", Required: true},
		},
	}
}

func (t *writeFileTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	var input struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := decodeStringArgs(args, &input); err != nil {
		return "", err
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: filesystem backend not wired")
	}
	result, err := t.ops.WriteFile(ctx, RunIDFromContext(ctx), FileWriteRequest{Path: input.Path, Content: input.Content, CreateParents: true})
	if err != nil {
		return "", err
	}
	return marshalToolResult(result)
}

func (t *writeFileTool) PrepareProposal(ctx context.Context, args json.RawMessage) (domain.ToolProposal, error) {
	var input struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := decodeStringArgs(args, &input); err != nil {
		return domain.ToolProposal{}, err
	}
	if planner, ok := t.ops.(fileMutationPreview); ok {
		return planner.PrepareWriteFile(ctx, RunIDFromContext(ctx), FileWriteRequest{Path: input.Path, Content: input.Content, CreateParents: true})
	}
	return domain.ToolProposal{Action: "write_file", Target: input.Path, Preview: fmt.Sprintf("write %d bytes", len(input.Content))}, nil
}

func (t *patchTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name: PatchName, Description: "Applies an exact string patch in the current run workspace after approval.", Readonly: false,
		Keywords: []string{"patch", "edit", "replace", "modify"},
		Params: map[string]domain.ToolParam{
			"path":        {Desc: "Workspace-relative file path.", Required: true},
			"old_string":  {Desc: "Exact existing text to replace.", Required: true},
			"new_string":  {Desc: "Replacement text.", Required: true},
			"replace_all": {Desc: "Optional true/false; otherwise the old text must be unique.", Required: false},
		},
	}
}

func (t *patchTool) InvokableRun(ctx context.Context, args json.RawMessage) (string, error) {
	var input struct {
		Path       string `json:"path"`
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
		ReplaceAll string `json:"replace_all"`
	}
	if err := decodeStringArgs(args, &input); err != nil {
		return "", err
	}
	replaceAll, err := optionalBool("replace_all", input.ReplaceAll)
	if err != nil {
		return "", err
	}
	if t.ops == nil {
		return "", fmt.Errorf("tools: filesystem backend not wired")
	}
	result, err := t.ops.PatchFile(ctx, RunIDFromContext(ctx), FilePatchRequest{Path: input.Path, OldString: input.OldString, NewString: input.NewString, ReplaceAll: replaceAll})
	if err != nil {
		return "", err
	}
	return marshalToolResult(result)
}

func (t *patchTool) PrepareProposal(ctx context.Context, args json.RawMessage) (domain.ToolProposal, error) {
	var input struct {
		Path       string `json:"path"`
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
		ReplaceAll string `json:"replace_all"`
	}
	if err := decodeStringArgs(args, &input); err != nil {
		return domain.ToolProposal{}, err
	}
	replaceAll, err := optionalBool("replace_all", input.ReplaceAll)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	if planner, ok := t.ops.(fileMutationPreview); ok {
		return planner.PreparePatchFile(ctx, RunIDFromContext(ctx), FilePatchRequest{Path: input.Path, OldString: input.OldString, NewString: input.NewString, ReplaceAll: replaceAll})
	}
	return domain.ToolProposal{Action: "patch", Target: input.Path, Preview: fmt.Sprintf("replace %d bytes with %d bytes", len(input.OldString), len(input.NewString))}, nil
}

func decodeStringArgs(args json.RawMessage, out any) error {
	dec := json.NewDecoder(strings.NewReader(string(args)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return &ArgError{Field: "args", Reason: err.Error()}
	}
	return nil
}

func optionalInt(field, value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0, &ArgError{Field: field, Reason: "must be a non-negative integer"}
	}
	return n, nil
}

func optionalBool(field, value string) (bool, error) {
	if strings.TrimSpace(value) == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(value)
	if err != nil {
		return false, &ArgError{Field: field, Reason: "must be true or false"}
	}
	return b, nil
}

func marshalToolResult(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("tools: marshal filesystem result: %w", err)
	}
	return string(data), nil
}
