package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func toolOperationFixture(t *testing.T, _ string) (*Backend, domain.RunID) {
	slot, _ := workflowStepSlot(t)
	b := slot.Engine.(*Backend)
	ctx := context.Background()
	if err := b.CreateSession(ctx, domain.Session{ID: "operation-session", Title: "fixture", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	runID := domain.RunID("operation-run")
	if err := b.CreateRun(ctx, domain.Run{ID: runID, SessionID: "operation-session", Status: domain.RunActive, CreatedAt: 2}); err != nil {
		t.Fatal(err)
	}
	return b, runID
}
func testToolOperation(runID domain.RunID, id string) domain.ToolOperation {
	return domain.ToolOperation{RunID: runID, OperationID: id, ToolName: "write_note", RequestArguments: []byte(`{}`), MiddlewareInputArguments: []byte(`{}`), EffectiveArguments: []byte(`{}`)}
}
func TestToolOperationUnboundRowsCannotAuthorize(t *testing.T) {
	backend, runID := toolOperationFixture(t, "")
	ctx := context.Background()
	for _, state := range []string{"admitted", "claimed", "completed"} {
		for _, input := range []any{nil, []byte{}} {
			id := state + fmt.Sprintf("-%v", input)
			_, err := backend.db.ExecContext(ctx, `INSERT INTO tool_operations (run_id,operation_id,tool_name,request_arguments,middleware_input_arguments,effective_arguments,state,claim_owner,result,failure,created_at,updated_at) VALUES (?,?,?, ?,?,?,?, 'legacy-owner','legacy-result','legacy-failure',11,12)`, runID, id, "write_note", input, []byte(`{}`), []byte(`{}`), state)
			if err != nil {
				t.Fatal(err)
			}
			op, err := backend.GetToolOperation(ctx, runID, id)
			if err != nil || op.State != domain.ToolOperationState(state) || op.Result != "legacy-result" || op.ClaimOwner != "legacy-owner" || op.CreatedAt != 11 || op.UpdatedAt != 12 {
				t.Fatalf("diagnostic read=%+v %v", op, err)
			}
			admission := testToolOperation(runID, id)
			admission.ToolName = "changed-tool"
			if _, _, _, err := backend.AdmitToolOperation(ctx, admission); !errors.Is(err, storage.ErrSourceBindingUnavailable) {
				t.Fatalf("unbound admission %s=%v", state, err)
			}
			if _, _, _, err := backend.ClaimToolOperation(ctx, runID, id, "new-owner"); !errors.Is(err, storage.ErrSourceBindingUnavailable) {
				t.Fatalf("unbound claim %s=%v", state, err)
			}
			if _, _, err := backend.CompleteToolOperation(ctx, runID, id, "legacy-owner", domain.ToolOperationOutcome{Result: "legacy-result", Failure: "legacy-failure", Status: "fatal", Effects: "unknown"}); !errors.Is(err, storage.ErrSourceBindingUnavailable) {
				t.Fatalf("unbound completion %s=%v", state, err)
			}
		}
	}
}

func TestToolOperationCompletionJournalFailureRollsBack(t *testing.T) {
	b, runID := toolOperationFixture(t, "")
	ctx := context.Background()
	if _, _, _, err := b.AdmitToolOperation(ctx, testToolOperation(runID, "call")); err != nil {
		t.Fatal(err)
	}
	if _, acquired, _, err := b.ClaimToolOperation(ctx, runID, "call", "owner"); err != nil || !acquired {
		t.Fatalf("claim=%v %v", acquired, err)
	}
	_, err := b.db.ExecContext(ctx, `CREATE FUNCTION fail_operation_completion() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.type = 'tool.operation' AND (convert_from(NEW.payload,'UTF8')::json->>'state') = 'completed' THEN RAISE EXCEPTION 'completion write failed'; END IF; RETURN NEW; END $$; CREATE TRIGGER fail_operation_completion BEFORE INSERT ON run_events FOR EACH ROW EXECUTE FUNCTION fail_operation_completion()`)
	if err != nil {
		t.Fatal(err)
	}
	outcome := domain.ToolOperationOutcome{Result: "result", Failure: "diagnostic", Status: "recoverable", Reason: "command_failed", Effects: "unknown"}
	if _, _, err := b.CompleteToolOperation(ctx, runID, "call", "owner", outcome); err == nil {
		t.Fatal("Journal failure ignored")
	}
	op, err := b.GetToolOperation(ctx, runID, "call")
	if err != nil || op.State != domain.ToolOperationClaimed || op.Outcome() != (domain.ToolOperationOutcome{}) {
		t.Fatalf("partial completion=%+v %v", op, err)
	}
	if _, err := b.db.ExecContext(ctx, `DROP TRIGGER fail_operation_completion ON run_events`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.CompleteToolOperation(ctx, runID, "call", "owner", outcome); err != nil {
		t.Fatal(err)
	}
}
