package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
)

type stubCommandOps struct {
	last    CommandRequest
	runID   domain.RunID
	calls   int
	failErr error
}

func (s *stubCommandOps) Execute(_ context.Context, runID domain.RunID, request CommandRequest) (CommandResult, error) {
	s.calls++
	s.last = request
	s.runID = runID
	if s.failErr != nil {
		return CommandResult{}, s.failErr
	}
	return CommandResult{Command: "bash", Stdout: "ok", ExitCode: 0, Untrusted: true}, nil
}

func TestBashToolExpandsToShellInvocation(t *testing.T) {
	ops := &stubCommandOps{}
	tool := NewBash(ops)
	result, err := tool.InvokableRun(context.Background(), []byte(`{"command":"echo hi && ls","cwd":"sub","timeout_ms":250}`))
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !strings.Contains(result, `"stdout":"ok"`) {
		t.Fatalf("result = %q, want marshaled CommandResult with stdout", result)
	}
	if ops.calls != 1 {
		t.Fatalf("ops calls = %d, want 1", ops.calls)
	}
	if ops.last.Command != "bash" || len(ops.last.Args) != 2 || ops.last.Args[0] != "-c" || ops.last.Args[1] != "echo hi && ls" {
		t.Fatalf("expanded request = %+v, want bash -c script", ops.last)
	}
	if ops.last.Cwd != "sub" || ops.last.TimeoutMS != 250 {
		t.Fatalf("cwd/timeout not propagated: %+v", ops.last)
	}
	if ops.runID != "" {
		t.Fatalf("run id from empty context = %q, want empty", ops.runID)
	}
}

func TestBashToolSurfacesDenyTableAndSyntaxErrors(t *testing.T) {
	ops := &stubCommandOps{}
	tool := NewBash(ops)
	if _, err := tool.InvokableRun(context.Background(), []byte(`{"command":"rm -rf /"}`)); err == nil || !strings.Contains(err.Error(), "deny-table") {
		t.Fatalf("denied script error = %v, want deny-table rejection", err)
	}
	if _, err := tool.InvokableRun(context.Background(), []byte(`{"command":":(){ :|:& };:"}`)); err == nil || !strings.Contains(err.Error(), "deny-table") {
		t.Fatalf("fork bomb error = %v, want deny-table rejection", err)
	}
	if _, err := tool.InvokableRun(context.Background(), []byte(`{"command":"echo \"unterminated"}`)); err == nil {
		t.Fatal("syntax error accepted")
	} else {
		var argErr *ArgError
		if !asArgError(err, &argErr) || argErr.Field != "command" {
			t.Fatalf("syntax error = %v, want ArgError on command", err)
		}
	}
	if ops.calls != 0 {
		t.Fatalf("denied/syntax scripts reached the backend %d times, want 0", ops.calls)
	}
	if _, err := tool.InvokableRun(context.Background(), []byte(`{"command":"   "}`)); err == nil || !strings.Contains(err.Error(), "is required") {
		t.Fatalf("empty command error = %v, want required", err)
	}
}

func TestBashToolProposalCarriesClassification(t *testing.T) {
	tool := NewBash(&stubCommandOps{})
	proposal, err := tool.(ProposalProvider).PrepareProposal(context.Background(), []byte(`{"command":"touch marker.txt"}`))
	if err != nil {
		t.Fatalf("PrepareProposal: %v", err)
	}
	if proposal.Action != BashName {
		t.Fatalf("proposal action = %q, want %q", proposal.Action, BashName)
	}
	if proposal.Preview != "touch marker.txt" || proposal.Target != "touch marker.txt" {
		t.Fatalf("proposal preview/target = %q/%q, want the script", proposal.Preview, proposal.Target)
	}
	if len(proposal.RiskFindings) == 0 {
		t.Fatal("proposal carries no risk findings")
	}
	var request CommandRequest
	if err := json.Unmarshal(proposal.Data, &request); err != nil {
		t.Fatalf("proposal data: %v", err)
	}
	if request.Command != "bash" || request.Args[1] != "touch marker.txt" {
		t.Fatalf("proposal data request = %+v, want bash expansion", request)
	}
	if _, err := tool.(ProposalProvider).PrepareProposal(context.Background(), []byte(`{"command":"rm -rf /"}`)); err == nil {
		t.Fatal("denied script produced a proposal instead of an error")
	}
}

func TestBashToolClassifyInvocation(t *testing.T) {
	tool := NewBash(&stubCommandOps{})
	classifier, ok := tool.(InvocationClassifier)
	if !ok {
		t.Fatal("bash tool does not implement InvocationClassifier")
	}
	class, _, err := classifier.ClassifyInvocation([]byte(`{"command":"git status"}`))
	if err != nil || class != InvocationSafe {
		t.Fatalf("git status class = %d err %v, want safe", class, err)
	}
	class, findings, err := classifier.ClassifyInvocation([]byte(`{"command":"shutdown"}`))
	if err != nil || class != InvocationDenied || len(findings) == 0 {
		t.Fatalf("shutdown class = %d findings %v err %v, want denied with findings", class, findings, err)
	}
	if _, _, err := classifier.ClassifyInvocation([]byte(`{}`)); err == nil {
		t.Fatal("missing command classified without error")
	}
}

func TestBashToolWithoutBackendFailsClosed(t *testing.T) {
	tool := NewBash(nil)
	if _, err := tool.InvokableRun(context.Background(), []byte(`{"command":"ls"}`)); err == nil || !strings.Contains(err.Error(), "backend not wired") {
		t.Fatalf("nil backend error = %v, want backend not wired", err)
	}
}

func TestValidateArgsSafetyExemptsBashCommandSyntax(t *testing.T) {
	bashSpec := NewBash(nil).Spec()
	if err := ValidateArgs(bashSpec, []byte(`{"command":"a && b | c"}`)); err != nil {
		t.Fatalf("bash args rejected by ValidateArgs: %v", err)
	}
	if err := ValidateArgsSafety(bashSpec, []byte(`{"command":"a && b | c"}`)); err != nil {
		t.Fatalf("bash shell syntax rejected by safety guard: %v", err)
	}
	if err := ValidateArgsSafety(bashSpec, []byte(`{"command":"a\u0000b"}`)); err == nil {
		t.Fatal("bash NUL byte accepted by safety guard")
	}
	executeSpec := NewExecute(nil).Spec()
	if err := ValidateArgsSafety(executeSpec, []byte(`{"command":"a && b"}`)); err == nil {
		t.Fatal("execute shell syntax accepted by safety guard")
	}
}

func asArgError(err error, target **ArgError) bool {
	if argErr, ok := err.(*ArgError); ok {
		*target = argErr
		return true
	}
	return false
}
