package tools

import (
	"context"
	"strings"
	"testing"
)

type stubGrepOps struct {
	grepReq GrepRequest
	globReq GlobRequest
	grepRes GrepResult
	globRes GlobResult
	greps   int
	globs   int
}

func (s *stubGrepOps) Grep(_ context.Context, req GrepRequest) (GrepResult, error) {
	s.greps++
	s.grepReq = req
	return s.grepRes, nil
}

func (s *stubGrepOps) Glob(_ context.Context, req GlobRequest) (GlobResult, error) {
	s.globs++
	s.globReq = req
	return s.globRes, nil
}

func TestGrepToolDecodesRequestAndMarshalsResult(t *testing.T) {
	ops := &stubGrepOps{grepRes: GrepResult{Matches: []GrepMatch{{Path: "a.go", Line: 3, Content: "needle here"}}}}
	tool := NewGrep(ops)
	result, err := tool.InvokableRun(context.Background(), []byte(`{"pattern":"needle","path":"sub","include":"*.go"}`))
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !strings.Contains(result, `"path":"a.go"`) || !strings.Contains(result, `"line":3`) || !strings.Contains(result, `"content":"needle here"`) {
		t.Fatalf("result = %q, want marshaled GrepResult", result)
	}
	if ops.greps != 1 {
		t.Fatalf("ops calls = %d, want 1", ops.greps)
	}
	if ops.grepReq.Pattern != "needle" || ops.grepReq.Path != "sub" || ops.grepReq.Include != "*.go" {
		t.Fatalf("decoded request = %+v", ops.grepReq)
	}
}

func TestGrepToolRequiresPattern(t *testing.T) {
	tool := NewGrep(&stubGrepOps{})
	if _, err := tool.InvokableRun(context.Background(), []byte(`{}`)); err == nil || !strings.Contains(err.Error(), `"pattern"`) {
		t.Fatalf("empty args error = %v, want pattern ArgError", err)
	}
	if _, err := tool.(ProposalProvider).PrepareProposal(context.Background(), []byte(`{"path":"."}`)); err == nil || !strings.Contains(err.Error(), `"pattern"`) {
		t.Fatalf("proposal error = %v, want pattern ArgError", err)
	}
}

func TestGlobToolDecodesRequestAndMarshalsResult(t *testing.T) {
	ops := &stubGrepOps{globRes: GlobResult{Files: []GlobFileInfo{{Path: "a.go", Size: 12, ModifiedAt: "2026-08-31T00:00:00Z"}}, Truncated: true}}
	tool := NewGlob(ops)
	result, err := tool.InvokableRun(context.Background(), []byte(`{"pattern":"**/*.go"}`))
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !strings.Contains(result, `"path":"a.go"`) || !strings.Contains(result, `"truncated":true`) {
		t.Fatalf("result = %q, want marshaled GlobResult", result)
	}
	if ops.globs != 1 || ops.globReq.Pattern != "**/*.go" || ops.globReq.Path != "" {
		t.Fatalf("decoded request = %+v (calls %d)", ops.globReq, ops.globs)
	}
}

func TestGlobToolRequiresPattern(t *testing.T) {
	tool := NewGlob(&stubGrepOps{})
	if _, err := tool.InvokableRun(context.Background(), []byte(`{"path":"x"}`)); err == nil || !strings.Contains(err.Error(), `"pattern"`) {
		t.Fatalf("empty args error = %v, want pattern ArgError", err)
	}
}

func TestGrepGlobSpecsMarkReadonly(t *testing.T) {
	grepSpec := NewGrep(&stubGrepOps{}).Spec()
	if grepSpec.Name != GrepName || !grepSpec.Readonly {
		t.Fatalf("grep spec = %+v, want readonly grep", grepSpec)
	}
	if !grepSpec.Params["pattern"].Required {
		t.Fatalf("grep pattern param = %+v, want required", grepSpec.Params["pattern"])
	}
	globSpec := NewGlob(&stubGrepOps{}).Spec()
	if globSpec.Name != GlobName || !globSpec.Readonly {
		t.Fatalf("glob spec = %+v, want readonly glob", globSpec)
	}
	if !globSpec.Params["pattern"].Required {
		t.Fatalf("glob pattern param = %+v, want required", globSpec.Params["pattern"])
	}
}

func TestGrepGlobProposalsPreview(t *testing.T) {
	proposal, err := NewGrep(&stubGrepOps{}).(ProposalProvider).PrepareProposal(context.Background(), []byte(`{"pattern":"x"}`))
	if err != nil {
		t.Fatalf("grep proposal: %v", err)
	}
	if proposal.Action != GrepName || proposal.Target != "" || !strings.Contains(proposal.Preview, `grep "x" in .`) {
		t.Fatalf("grep proposal = %+v", proposal)
	}
	globProposal, err := NewGlob(&stubGrepOps{}).(ProposalProvider).PrepareProposal(context.Background(), []byte(`{"pattern":"**"}`))
	if err != nil {
		t.Fatalf("glob proposal: %v", err)
	}
	if globProposal.Action != GlobName || !strings.Contains(globProposal.Preview, `glob "**" in .`) {
		t.Fatalf("glob proposal = %+v", globProposal)
	}
}
