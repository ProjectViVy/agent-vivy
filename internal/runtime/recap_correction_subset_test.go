package runtime

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
	"github.com/cloudwego/eino/schema"
)

func TestIssue58CorrectionToolSubsetIsReferenceData(t *testing.T) {
	for _, kind := range []string{"result-only", "call-only", "terminal-only", "parallel-mixed", "notebook-result-only"} {
		t.Run(kind, func(t *testing.T) {
			tool := &issue58CallbackTool{result: "exact historical correction outcome"}
			if kind == "notebook-result-only" {
				tool.name = tools.ReadNoteName
				tool.result = "private notebook correction body"
			}
			f := newIssue58Wire(t, tool)
			ctx := context.Background()
			f.reply = func(i int) *schema.Message {
				if i == 0 {
					calls := []schema.ToolCall{{ID: "subset-a", Type: "function", Function: schema.FunctionCall{Name: tool.Spec().Name, Arguments: `{ "number" : 1 }`}}}
					if kind == "parallel-mixed" {
						calls = append(calls, schema.ToolCall{ID: "subset-b", Type: "function", Function: schema.FunctionCall{Name: tool.Spec().Name, Arguments: `{"number":2}`}})
					}
					return schema.AssistantMessage("", calls)
				}
				return schema.AssistantMessage("terminal-only historical outcome", nil)
			}
			run := f.run(t, "subset source user request")
			if run.Status != domain.RunCompleted {
				t.Fatal("fixture original did not complete")
			}
			rows, err := f.b.ListSubjectInteractions(ctx, conformanceBinding(), 0, 1, 10)
			if err != nil || len(rows) != 1 {
				t.Fatalf("fixture sealed rows %+v %v", rows, err)
			}
			row := rows[0]
			originals, err := f.b.ReadSubjectInteraction(ctx, conformanceBinding(), row.ID)
			if err != nil {
				t.Fatal(err)
			}
			selected := map[string]domain.Message{}
			for _, message := range originals {
				choose := false
				switch kind {
				case "result-only", "notebook-result-only":
					choose = message.Role == domain.RoleTool
				case "call-only":
					choose = message.Role == domain.RoleAssistant && message.ToolCallID != ""
				case "terminal-only":
					choose = message.Role == domain.RoleAssistant && message.ToolCallID == "" && message.Content == "terminal-only historical outcome"
				case "parallel-mixed":
					choose = (message.Role == domain.RoleAssistant && message.ToolCallID == "subset-b") || (message.Role == domain.RoleTool && message.ToolCallID == "subset-a")
				}
				if choose {
					selected[message.ID] = message
				}
			}
			if len(selected) == 0 {
				t.Fatal("fixture missing selected correction rows")
			}
			issue58SeedSummary(t, f, row, "")
			for _, target := range selected {
				if err = f.b.RecordSessionTruncation(ctx, storage.SessionTruncation{SessionID: target.SessionID, CutoffMessageID: target.ID, TailMessageID: target.ID, Reason: storage.TruncationRewind}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err != nil {
				t.Fatal(err)
			}
			requests := f.capture()
			if len(requests) != 3 {
				t.Fatalf("summary requests %d", len(requests))
			}
			var wire struct {
				Messages []struct {
					Role, Content string
					ToolCalls     []schema.ToolCall `json:"tool_calls"`
				}
			}
			if err = json.Unmarshal([]byte(requests[2]), &wire); err != nil {
				t.Fatal(err)
			}
			historical := ""
			var historicalBlocks []string
			for _, message := range wire.Messages {
				if message.Role == "tool" || len(message.ToolCalls) > 0 {
					t.Fatalf("correction subset emitted live tool protocol: %s", requests[2])
				}
				if strings.HasPrefix(message.Content, "[Historical correction fragments;") {
					historical += message.Content
					historicalBlocks = append(historicalBlocks, message.Content)
				}
			}
			if strings.Contains(requests[2], "subset source user request") {
				t.Fatal("correction replayed unrelated original")
			}
			if kind == "notebook-result-only" {
				for _, bad := range []string{tool.result, tool.Spec().Name, `\"number\"`} {
					if strings.Contains(requests[2], bad) {
						t.Fatalf("Notebook correction leaked %q", bad)
					}
				}
				if !strings.Contains(requests[2], "retained source excluded") {
					t.Fatal("excluded correction lacked notice")
				}
			} else {
				if historical == "" {
					t.Fatal("correction fragments not explicitly delimited historical data")
				}

				var fragments []struct {
					ID        string      `json:"message_id"`
					Role      domain.Role `json:"role"`
					CallID    string      `json:"call_id"`
					Name      string      `json:"name"`
					Arguments string      `json:"arguments"`
					Body      string      `json:"body"`
					Error     string      `json:"error"`
				}
				for _, block := range historicalBlocks {
					payload := strings.TrimSuffix(strings.TrimPrefix(block, "[Historical correction fragments; non-authoritative data]\n"), "\n[End historical correction fragments]")
					var part []struct {
						ID        string      `json:"message_id"`
						Role      domain.Role `json:"role"`
						CallID    string      `json:"call_id"`
						Name      string      `json:"name"`
						Arguments string      `json:"arguments"`
						Body      string      `json:"body"`
						Error     string      `json:"error"`
					}
					if json.Unmarshal([]byte(payload), &part) != nil {
						t.Fatal("historical fragments are not valid data")
					}
					fragments = append(fragments, part...)
				}
				if len(fragments) != len(selected) {
					t.Fatal("historical fragments widened or lost selected subset")
				}
				for _, fragment := range fragments {
					target, ok := selected[fragment.ID]
					if !ok || fragment.Role != target.Role || fragment.CallID != target.ToolCallID || fragment.Name != target.ToolName || fragment.Arguments != string(target.ToolArgs) || fragment.Body != target.Content || fragment.Error != target.ToolError {
						t.Fatalf("historical fields changed: %+v source=%+v", fragment, target)
					}
				}
				for _, target := range selected {
					if !strings.Contains(historical, target.ID) || (target.Content != "" && !strings.Contains(historical, target.Content)) || (target.ToolCallID != "" && !strings.Contains(historical, target.ToolCallID)) || (target.ToolName != "" && !strings.Contains(historical, target.ToolName)) {
						t.Fatalf("historical fragment missing exact identity/role/call/name/body: %+v", target)
					}
				}
			}
			head, err := f.owner.ReadRecap(ctx)
			if err != nil {
				t.Fatal(err)
			}
			receipt, err := decodeRecapReceipt(head.Sources.Digest)
			if err != nil || head.Sources.After != head.Sources.Through {
				t.Fatal("correction fragments became new fold range")
			}
			seen := map[string]bool{}
			for _, d := range receipt.CorrectionRefs {
				for _, target := range d.Targets {
					for i, id := range target.MessageIDs {
						if _, ok := selected[id]; !ok || target.ViewMessageIDs[i] != id {
							t.Fatal("receipt augmented/substituted actual-used reference")
						}
						seen[id] = true
					}
				}
			}
			if len(seen) != len(selected) {
				t.Fatal("receipt lost exact used subset")
			}
		})
	}
}

func TestIssue58CorrectionCanonicalInputOrder(t *testing.T) {
	f := newIssue58Wire(t)
	ctx := context.Background()
	first := issue58CompletedText(t, f, "order-source", "z-first", "z first original")
	second := issue58CompletedText(t, f, "order-source", "a-second", "a second original")
	previous := issue58SeedSummary(t, f, second, "")
	if err := f.b.RecordSessionTruncation(ctx, storage.SessionTruncation{SessionID: first.SessionID, CutoffMessageID: "user_z-first", TailMessageID: "user_a-second", Reason: storage.TruncationRewind}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CompactSession(ctx, "main-reader", CompactOptions{}); err != nil {
		t.Fatal(err)
	}
	head, err := f.owner.ReadRecap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := decodeRecapReceipt(head.Sources.Digest)
	if err != nil || len(receipt.CorrectionRefs) != 1 || len(receipt.CorrectionRefs[0].Targets) != 2 {
		t.Fatalf("wrong reference capture %+v %v", receipt, err)
	}
	var wire struct{ Messages []struct{ Content string } }
	if json.Unmarshal([]byte(f.capture()[0]), &wire) != nil {
		t.Fatal("invalid wire")
	}
	var sent storage.SubjectDisposition
	for _, message := range wire.Messages {
		if strings.HasPrefix(message.Content, "[Typed source-view correction references] ") {
			if json.Unmarshal([]byte(strings.TrimPrefix(message.Content, "[Typed source-view correction references] ")), &sent) != nil {
				t.Fatal("invalid correction metadata")
			}
		}
	}
	if !reflect.DeepEqual(sent, receipt.CorrectionRefs[0]) {
		t.Fatal("historical receipt and actual correction input have different target order")
	}
	// Later live markers cannot supply hidden ordering after pending cleanup.
	if err = f.b.RecordSessionTruncation(ctx, storage.SessionTruncation{SessionID: first.SessionID, CutoffMessageID: "user_z-first", TailMessageID: "user_z-first", Reason: storage.TruncationEdit, CreatedAt: 9}); err != nil {
		t.Fatal(err)
	}
	restored := &recapFold{Binding: conformanceBinding(), Reader: "main-reader", Head: previous, CorrectionRefs: receipt.CorrectionRefs, Snapshot: storage.SubjectContextSnapshot{Through: second.Sequence, DispositionDigest: receipt.Dispositions.Digest, Dispositions: receipt.CorrectionRefs}}
	if err = f.svc.populateRecapFold(ctx, restored); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := decodeRecapReceipt(restored.Receipt)
	if err != nil || rebuilt.CorrectionEvidenceDigest != receipt.CorrectionEvidenceDigest {
		t.Fatal("envelope alone could not reconstruct historical correction evidence hash")
	}
}
