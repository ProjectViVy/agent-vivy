package tools

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/domain"
)

type recordingFileOps struct {
	listRunID   domain.RunID
	readRunID   domain.RunID
	searchRunID domain.RunID
	writeRunID  domain.RunID
	patchRunID  domain.RunID
	listReq     DirListRequest
	readReq     FileReadRequest
	searchReq   FileSearchRequest
	writeReq    FileWriteRequest
	patchReq    FilePatchRequest
}

func (r *recordingFileOps) ListDir(_ context.Context, runID domain.RunID, req DirListRequest) (DirListResult, error) {
	r.listRunID, r.listReq = runID, req
	return DirListResult{Path: req.Path}, nil
}

func (r *recordingFileOps) ReadFile(_ context.Context, runID domain.RunID, req FileReadRequest) (FileReadResult, error) {
	r.readRunID, r.readReq = runID, req
	return FileReadResult{Path: req.Path, Content: "ok"}, nil
}

func (r *recordingFileOps) SearchFiles(_ context.Context, runID domain.RunID, req FileSearchRequest) (FileSearchResult, error) {
	r.searchRunID, r.searchReq = runID, req
	return FileSearchResult{FilesScanned: 1}, nil
}

func (r *recordingFileOps) WriteFile(_ context.Context, runID domain.RunID, req FileWriteRequest) (FileMutationResult, error) {
	r.writeRunID, r.writeReq = runID, req
	return FileMutationResult{Path: req.Path, Changed: true}, nil
}

func (r *recordingFileOps) PatchFile(_ context.Context, runID domain.RunID, req FilePatchRequest) (FileMutationResult, error) {
	r.patchRunID, r.patchReq = runID, req
	return FileMutationResult{Path: req.Path, Changed: true}, nil
}

func TestFilesystemToolsForwardTypedArgumentsAndRunIdentity(t *testing.T) {
	ops := &recordingFileOps{}
	ctx := WithRunID(context.Background(), domain.RunID("run_tool_test"))

	list, err := NewListDir(ops).InvokableRun(ctx, json.RawMessage(`{"path":"src","recursive":"true","depth":"3","max_entries":"50"}`))
	if err != nil {
		t.Fatalf("list tool: %v", err)
	}
	var listResult DirListResult
	if err := json.Unmarshal([]byte(list), &listResult); err != nil {
		t.Fatalf("decode list result: %v", err)
	}
	if listResult.Path != "src" {
		t.Fatalf("list result = %s", list)
	}
	if ops.listRunID != "run_tool_test" || ops.listReq.Path != "src" || !ops.listReq.Recursive || ops.listReq.Depth != 3 || ops.listReq.MaxEntries != 50 {
		t.Fatalf("list forwarding = %q/%+v", ops.listRunID, ops.listReq)
	}

	read, err := NewReadFile(ops).InvokableRun(ctx, json.RawMessage(`{"path":"src/main.go","start_line":"2","end_line":"4","max_bytes":"100"}`))
	if err != nil {
		t.Fatalf("read tool: %v", err)
	}
	var readResult FileReadResult
	if err := json.Unmarshal([]byte(read), &readResult); err != nil {
		t.Fatalf("decode read result: %v", err)
	}
	if readResult.Path != "src/main.go" || readResult.Content != "ok" {
		t.Fatalf("read result = %s", read)
	}
	if ops.readRunID != "run_tool_test" || ops.readReq.StartLine != 2 || ops.readReq.EndLine != 4 || ops.readReq.MaxBytes != 100 {
		t.Fatalf("read forwarding = %q/%+v", ops.readRunID, ops.readReq)
	}

	if _, err := NewSearchFiles(ops).InvokableRun(ctx, json.RawMessage(`{"query":"needle","glob":"**/*.go","max_results":"3","max_bytes":"200","case_sensitive":"true"}`)); err != nil {
		t.Fatalf("search tool: %v", err)
	}
	if ops.searchRunID != "run_tool_test" || ops.searchReq.Query != "needle" || ops.searchReq.MaxResults != 3 || !ops.searchReq.CaseSensitive {
		t.Fatalf("search forwarding = %q/%+v", ops.searchRunID, ops.searchReq)
	}

	if _, err := NewWriteFile(ops).InvokableRun(ctx, json.RawMessage(`{"path":"notes/today.md","content":"hello"}`)); err != nil {
		t.Fatalf("write tool: %v", err)
	}
	if ops.writeRunID != "run_tool_test" || !ops.writeReq.CreateParents || ops.writeReq.Content != "hello" {
		t.Fatalf("write forwarding = %q/%+v", ops.writeRunID, ops.writeReq)
	}

	if _, err := NewPatch(ops).InvokableRun(ctx, json.RawMessage(`{"path":"notes/today.md","old_string":"hello","new_string":"updated","replace_all":"true"}`)); err != nil {
		t.Fatalf("patch tool: %v", err)
	}
	if ops.patchRunID != "run_tool_test" || !ops.patchReq.ReplaceAll || ops.patchReq.OldString != "hello" {
		t.Fatalf("patch forwarding = %q/%+v", ops.patchRunID, ops.patchReq)
	}
}

func TestFilesystemToolArgumentErrorsAndSpecs(t *testing.T) {
	ops := &recordingFileOps{}
	if _, err := NewReadFile(ops).InvokableRun(context.Background(), json.RawMessage(`{"path":"a.txt","start_line":"nope"}`)); err == nil {
		t.Fatal("invalid integer should fail")
	}
	if _, err := NewSearchFiles(ops).InvokableRun(context.Background(), json.RawMessage(`{"query":"x","case_sensitive":"maybe"}`)); err == nil {
		t.Fatal("invalid boolean should fail")
	}
	if _, err := NewListDir(ops).InvokableRun(context.Background(), json.RawMessage(`{"path":"src","recursive":"maybe"}`)); err == nil {
		t.Fatal("invalid list boolean should fail")
	}
	if _, err := NewPatch(ops).InvokableRun(context.Background(), json.RawMessage(`{"path":"a.txt","old_string":"x","unknown":"y"}`)); err == nil {
		t.Fatal("unknown argument should fail")
	}

	for _, tool := range []Tool{NewListDir(ops), NewReadFile(ops), NewSearchFiles(ops), NewWriteFile(ops), NewPatch(ops)} {
		spec := tool.Spec()
		if spec.Name == "" || len(spec.Params) == 0 {
			t.Fatalf("invalid filesystem spec: %+v", spec)
		}
		if spec.Readonly && (spec.Name == WriteFileName || spec.Name == PatchName) {
			t.Fatalf("mutation tool marked readonly: %+v", spec)
		}
	}
}
