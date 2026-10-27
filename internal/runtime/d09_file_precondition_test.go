package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"agent-vivy/internal/tools"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func TestD09ReviewedFileBytesAndExistence(t *testing.T) {
	str := func(s string) *string { return &s }
	for _, fixture := range []struct {
		name          string
		before, after *string
		conflict      bool
	}{
		{"unchanged", str("AAA"), str("AAA"), false},
		{"same-length-stale", str("AAA"), str("BBB"), true},
		{"missing-to-empty", nil, str(""), true},
		{"empty-to-missing", str(""), nil, true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			backend, workspace, runID := newFilesystemTestBackend(t)
			path := filepath.Join(workspace.Path, "review.txt")
			if fixture.before != nil {
				if err := os.WriteFile(path, []byte(*fixture.before), 0600); err != nil {
					t.Fatal(err)
				}
			}
			info, _ := os.Stat(path)
			request := tools.FileWriteRequest{Path: "review.txt", Content: "approved"}
			proposal, err := backend.PrepareWriteFile(context.Background(), runID, request)
			if err != nil {
				t.Fatal(err)
			}
			if fixture.after == nil {
				if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(path, []byte(*fixture.after), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if info != nil && fixture.after != nil {
				if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
			}
			ctx := tools.WithProposalData(context.Background(), proposal.Data)
			_, err = backend.WriteFile(ctx, runID, request)
			if fixture.conflict && err == nil {
				t.Fatal("reviewed target changed but write succeeded")
			}
			if !fixture.conflict && err != nil {
				t.Fatalf("unchanged reviewed write: %v", err)
			}
			if fixture.conflict {
				got, readErr := os.ReadFile(path)
				if fixture.after == nil {
					if !errors.Is(readErr, os.ErrNotExist) {
						t.Fatalf("missing target changed: %v", readErr)
					}
				} else if readErr != nil || string(got) != *fixture.after {
					t.Fatalf("stale write changed target: %v", readErr)
				}
			}
		})
	}
}

func TestD09ReviewedBeforeBytesAreBounded(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	backend.maxFileBytes = 8
	if err := os.WriteFile(filepath.Join(workspace.Path, "review.txt"), []byte("too much before state"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.PrepareWriteFile(context.Background(), runID, tools.FileWriteRequest{Path: "review.txt", Content: "new"}); err == nil {
		t.Fatal("oversized private before-state admitted")
	}
}

func TestD09DownloadBeforeStateAndInFlightRace(t *testing.T) {
	backend, workspace, runID := newDownloadTestBackend(t, 1<<20)
	target := filepath.Join(workspace, "download.txt")
	var requests atomic.Int32
	var changeDuringRequest atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		if changeDuringRequest.Load() {
			if err := os.WriteFile(target, []byte("BBB"), 0600); err != nil {
				t.Error(err)
			}
		}
		_, _ = w.Write([]byte("downloaded"))
	}))
	defer server.Close()
	request := tools.DownloadRequest{URL: server.URL, Path: "download.txt"}
	for _, during := range []bool{false, true} {
		if err := os.WriteFile(target, []byte("AAA"), 0600); err != nil {
			t.Fatal(err)
		}
		proposal, err := backend.PrepareDownload(context.Background(), runID, request)
		if err != nil {
			t.Fatal(err)
		}
		if during {
			changeDuringRequest.Store(true)
		} else {
			if err := os.WriteFile(target, []byte("BBB"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		previous := requests.Load()
		_, err = backend.Download(tools.WithProposalData(context.Background(), proposal.Data), runID, request)
		if err == nil {
			t.Fatalf("stale download committed (in-flight=%v)", during)
		}
		if !during && requests.Load() != previous {
			t.Fatal("already stale download touched network")
		}
		got, readErr := os.ReadFile(target)
		if readErr != nil || string(got) != "BBB" {
			t.Fatalf("stale download changed target: %v", readErr)
		}
		leftovers, err := filepath.Glob(filepath.Join(workspace, ".vivy-download-*"))
		if err != nil || len(leftovers) != 0 {
			t.Fatalf("download scratch leaked: %v %v", leftovers, err)
		}
	}
}

func TestD09PrivateBeforeStatePreservesAuthorizedSensitiveEdit(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	if err := os.WriteFile(filepath.Join(workspace.Path, "public.pem"), []byte("LOCAL_VALUE=old"), 0600); err != nil {
		t.Fatal(err)
	}
	request := tools.FileWriteRequest{Path: "public.pem", Content: "LOCAL_VALUE=new"}
	proposal, err := backend.PrepareWriteFile(context.Background(), runID, request)
	if err != nil {
		t.Fatalf("authorized private file review: %v", err)
	}
	if _, err := backend.WriteFile(tools.WithProposalData(context.Background(), proposal.Data), runID, request); err != nil {
		t.Fatalf("authorized private file edit: %v", err)
	}
}

func TestD09BeforeBindingKeepsCauseWithoutPrivateBytes(t *testing.T) {
	for _, data := range []json.RawMessage{
		json.RawMessage(`{"format":"private-canary"`),
		json.RawMessage(`{"format":"vivy.file-before/v1","path":"review.txt","exists":false,"content":null} {"private-canary"`),
	} {
		ctx := tools.WithProposalData(context.Background(), data)
		err := checkReviewedFileBefore(ctx, "review.txt", fileBeforeState{}, 1024)
		if err == nil || !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("private parser cause lost: %v", err)
		}
		if strings.Contains(err.Error(), "private-canary") {
			t.Fatal("private data leaked through binding failure")
		}
	}

}

func TestD09MissingReviewedBeforeStateIsUnavailable(t *testing.T) {
	for _, data := range []json.RawMessage{nil, json.RawMessage(`{"format":"vivy.file-before/v1","path":"review.txt","exists":true}`), json.RawMessage(`{"format":"vivy.file-before/v1","path":"review.txt","exists":true,"content":null}`)} {
		err := checkReviewedFileBefore(tools.WithProposalData(context.Background(), data), "review.txt", fileBeforeState{exists: true, content: []byte{}}, 1024)
		if err == nil {
			t.Fatal("missing approved bytes accepted")
		}
	}
}
func TestD09NativeDispatchScopesPrivateBeforeState(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	request := tools.FileWriteRequest{Path: "approved.txt", Content: "approved"}
	proposal, err := backend.PrepareWriteFile(context.Background(), runID, request)
	if err != nil {
		t.Fatal(err)
	}
	tl := newContractTool(func(ctx context.Context, args json.RawMessage) (string, error) {
		var req tools.FileWriteRequest
		if err := json.Unmarshal(args, &req); err != nil {
			return "", err
		}
		_, err := backend.WriteFile(ctx, runID, req)
		return "written", err
	})
	tl.spec.Params = tools.NewWriteFile(backend).Spec().Params
	node, err := compose.NewToolNode(context.Background(), &compose.ToolsNodeConfig{Tools: []einotool.BaseTool{newToolAdapter(tl, 0, nil, nil)}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := withApprovedToolCall(withApprovedToolArguments(tools.WithProposalData(context.Background(), proposal.Data), json.RawMessage(`{}`)), "approved")
	for _, call := range []struct{ id, path string }{{"approved", "approved.txt"}, {"fresh", "fresh.txt"}} {
		input := &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{ID: call.id, Type: "function", Function: schema.FunctionCall{Name: contractToolName, Arguments: `{"path":"` + call.path + `","content":"approved"}`}}}}
		if _, err := node.Invoke(ctx, input); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(filepath.Join(workspace.Path, call.path)); err != nil || string(got) != "approved" {
			t.Fatalf("%s did not write: %v", call.id, err)
		}
	}
}
func TestD09ReviewedWriteCanReplaceBinary(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	path := filepath.Join(workspace.Path, "binary.dat")
	if err := os.WriteFile(path, []byte{0, 1, 2}, 0600); err != nil {
		t.Fatal(err)
	}
	request := tools.FileWriteRequest{Path: "binary.dat", Content: "text"}
	proposal, err := backend.PrepareWriteFile(context.Background(), runID, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backend.WriteFile(tools.WithProposalData(context.Background(), proposal.Data), runID, request); err != nil {
		t.Fatal(err)
	}
}
