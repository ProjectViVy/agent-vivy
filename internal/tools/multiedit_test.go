package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
)

type recordingMultiOps struct {
	req   FileMultiEditRequest
	runID domain.RunID
	calls int
}

func (r *recordingMultiOps) MultiPatchFile(_ context.Context, runID domain.RunID, req FileMultiEditRequest) (FileMutationResult, error) {
	r.calls++
	r.runID, r.req = runID, req
	return FileMutationResult{Path: req.Path, Changed: true}, nil
}

func TestMultiEditToolDecodesEditsInOrder(t *testing.T) {
	ops := &recordingMultiOps{}
	ctx := WithRunID(context.Background(), domain.RunID("run_multi_test"))
	result, err := NewMultiEdit(ops).InvokableRun(ctx, json.RawMessage(`{"path":"src/a.go","edits":[`+
		`{"old_string":"one","new_string":"ONE"},`+
		`{"old_string":"two","new_string":"TWO","replace_all":"true"}]}`))
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !strings.Contains(result, `"changed":true`) {
		t.Fatalf("result = %q, want marshaled FileMutationResult", result)
	}
	if ops.calls != 1 || ops.runID != "run_multi_test" || ops.req.Path != "src/a.go" {
		t.Fatalf("forwarding = %d/%q/%+v", ops.calls, ops.runID, ops.req)
	}
	if len(ops.req.Edits) != 2 {
		t.Fatalf("edits = %+v, want 2", ops.req.Edits)
	}
	if ops.req.Edits[0].OldString != "one" || ops.req.Edits[0].NewString != "ONE" || ops.req.Edits[0].ReplaceAll {
		t.Fatalf("edit 0 = %+v", ops.req.Edits[0])
	}
	if ops.req.Edits[1].OldString != "two" || !ops.req.Edits[1].ReplaceAll {
		t.Fatalf("edit 1 = %+v", ops.req.Edits[1])
	}
}

func TestMultiEditToolValidatesEdits(t *testing.T) {
	ops := &recordingMultiOps{}
	tool := NewMultiEdit(ops)
	cases := []struct {
		args string
		want string
	}{
		{`{"path":"a"}`, `"edits"`},
		{`{"path":"a","edits":[]}`, `"edits"`},
		{`{"path":"a","edits":[{"new_string":"x"}]}`, `edits[0].old_string`},
		{`{"path":"a","edits":[{"old_string":"x","new_string":"x"}]}`, `must differ`},
		{`{"path":"a","edits":[{"old_string":"x","new_string":"y","replace_all":"maybe"}]}`, `must be true or false`},
	}
	for _, tc := range cases {
		if _, err := tool.InvokableRun(context.Background(), json.RawMessage(tc.args)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("args %s error = %v, want %s", tc.args, err, tc.want)
		}
	}
	if ops.calls != 0 {
		t.Fatalf("backend called %d times during validation failures", ops.calls)
	}
}

func TestMultiEditToolSpecMarksMutating(t *testing.T) {
	spec := NewMultiEdit(&recordingMultiOps{}).Spec()
	if spec.Name != MultiEditName || spec.Readonly {
		t.Fatalf("spec = %+v, want mutating multiedit", spec)
	}
	if !spec.Params["path"].Required || !spec.Params["edits"].Required {
		t.Fatalf("params = %+v, want path and edits required", spec.Params)
	}
}

type multiPreviewStub struct {
	recordingMultiOps
	proposal domain.ToolProposal
}

func (s *multiPreviewStub) PrepareMultiPatchFile(_ context.Context, _ domain.RunID, req FileMultiEditRequest) (domain.ToolProposal, error) {
	s.proposal = domain.ToolProposal{Action: MultiEditName, Target: req.Path, Preview: "stub preview"}
	return s.proposal, nil
}

func TestMultiEditToolProposalUsesPreviewer(t *testing.T) {
	stub := &multiPreviewStub{}
	stub.req.Path = "a.txt"
	proposal, err := NewMultiEdit(stub).(ProposalProvider).PrepareProposal(context.Background(), json.RawMessage(`{"path":"a.txt","edits":[{"old_string":"x","new_string":"y"}]}`))
	if err != nil {
		t.Fatalf("PrepareProposal: %v", err)
	}
	if proposal.Action != MultiEditName || proposal.Preview != "stub preview" {
		t.Fatalf("proposal = %+v, want previewer output", proposal)
	}

	fallback, err := NewMultiEdit(&recordingMultiOps{}).(ProposalProvider).PrepareProposal(context.Background(), json.RawMessage(`{"path":"a.txt","edits":[{"old_string":"x","new_string":"y"},{"old_string":"p","new_string":"q"}]}`))
	if err != nil {
		t.Fatalf("fallback proposal: %v", err)
	}
	if !strings.Contains(fallback.Preview, "apply 2 edits") {
		t.Fatalf("fallback proposal = %+v, want edit count preview", fallback)
	}
}
