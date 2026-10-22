package actionhost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	action "agent-vivy/sdk/port/controlaction"
)

func TestActionHostAuditClampsDiagnosticSizes(t *testing.T) {
	recorder := &auditRecorder{}
	host := &Host{deps: Deps{Audit: recorder}}
	if err := host.recordAudit(context.Background(), nil, nil, "example.action.bounds", maxAuditPayload+1, -1, AuditOutcomeDenied, "decision", errors.New("private-cause"), time.Now()); err != nil {
		t.Fatal(err)
	}
	records := recorder.snapshot()
	if len(records) != 1 || records[0].InputBytes != maxAuditPayload || records[0].OutputBytes != 0 || records[0].Error != "controlaction: action failed" {
		t.Fatalf("audit bounds or safe error classification lost: %+v", records)
	}
}

func TestActionHostAuditsBoundedMetadataWithoutPayloads(t *testing.T) {
	const privateInput = "private-action-input-canary"
	const privateOutput = "private-action-output-canary"
	input := json.RawMessage(`{"value":"` + privateInput + `"}`)
	output := json.RawMessage(`{"ok":true,"value":"` + privateOutput + `"}`)
	for _, scenario := range []string{"success", "denied", "provider failure", "invalid output", "oversized input"} {
		t.Run(scenario, func(t *testing.T) {
			definition := testDefinition("example.action.metadata", action.EffectWrite)
			definition.ResultSchema = json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"},"value":{"type":"string"}},"required":["ok","value"],"additionalProperties":false}`)
			calls := 0
			provider := providerFunc{definition: definition, invoke: func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) {
				calls++
				if scenario == "provider failure" {
					return nil, errors.New(privateOutput)
				}
				if scenario == "invalid output" {
					return json.RawMessage(privateOutput), nil
				}
				return output, nil
			}}
			recorder := &auditRecorder{}
			deps := readyDeps(provider)
			deps.Audit = recorder
			wantErr, wantOutcome, wantCalls, wantCount := error(nil), AuditOutcomeSucceeded, 1, 2
			wantOutputBytes := len(output)
			switch scenario {
			case "denied":
				deps.Authorize = func(context.Context, Identity, action.Definition, json.RawMessage) error {
					return errors.New(privateInput)
				}
				wantErr, wantOutcome, wantCalls, wantCount = ErrAuthorizationUnavailable, AuditOutcomeDenied, 0, 1
				wantOutputBytes = 0
			case "provider failure":
				wantErr, wantOutcome, wantOutputBytes = ErrProviderFailed, AuditOutcomeFailed, 0
			case "invalid output":
				wantErr, wantOutcome, wantOutputBytes = ErrInvalidOutput, AuditOutcomeFailed, len(privateOutput)
			case "oversized input":
				deps.MaxInputBytes = len(input) - 1
				wantErr, wantOutcome, wantCalls, wantCount = ErrInvalidInput, AuditOutcomeDenied, 0, 1
				wantOutputBytes = 0
			}
			host, err := New(deps)
			if err != nil {
				t.Fatal(err)
			}
			defer host.Close()
			_, err = host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, input)
			if !errors.Is(err, wantErr) || calls != wantCalls {
				t.Fatalf("Invoke() error=%v calls=%d, want %v/%d", err, calls, wantErr, wantCalls)
			}
			records := recorder.snapshot()
			if len(records) != wantCount {
				t.Fatalf("audit count=%d, want %d", len(records), wantCount)
			}
			last := records[len(records)-1]
			if last.Outcome != wantOutcome || last.InputBytes != len(input) || last.OutputBytes != wantOutputBytes || last.Effect != action.EffectWrite || last.Event != "action.write" || last.CallerID != "operator" || last.ModuleID != testModule || last.ActionID != definition.ID || last.InvocationID == "" {
				t.Fatalf("audit lost bounded decision metadata: %+v", last)
			}
			if wantCount == 2 && (records[0].Phase != "pre" || records[0].Outcome != AuditOutcomeStarted || records[0].OutputBytes != 0 || last.Phase != "completion" || last.InvocationID != records[0].InvocationID) {
				t.Fatalf("effect admission/completion mismatch: %+v", records)
			}
			if wantCount == 1 && last.Phase != "decision" {
				t.Fatalf("denial phase=%q, want decision", last.Phase)
			}
			var logs bytes.Buffer
			sink := SlogAuditSink{Logger: slog.New(slog.NewJSONHandler(&logs, nil))}
			for _, record := range records {
				encoded, err := json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				if len(encoded) > 2048 || record.DurationMS < 0 || len(record.Error) > maxAuditError {
					t.Fatalf("audit exceeded bounded metadata contract: %s", encoded)
				}
				if err := sink.Record(context.Background(), record); err != nil {
					t.Fatal(err)
				}
				for _, forbidden := range []string{privateInput, privateOutput, "SHA256", "digest"} {
					if strings.Contains(string(encoded), forbidden) {
						t.Fatalf("private or retired metadata %q in record: %s", forbidden, encoded)
					}
				}
			}
			for _, forbidden := range []string{privateInput, privateOutput, "sha256", "digest"} {
				if strings.Contains(logs.String(), forbidden) {
					t.Fatalf("private or retired metadata %q in slog: %s", forbidden, logs.String())
				}
			}
			if !strings.Contains(logs.String(), `"input_bytes":`) || !strings.Contains(logs.String(), `"outcome":"`+string(wantOutcome)+`"`) {
				t.Fatalf("slog omitted decision metadata: %s", logs.String())
			}
		})
	}
}

func TestActionHostAuditSinkFailuresRemainClosed(t *testing.T) {
	for _, phase := range []string{"pre", "completion"} {
		for _, failure := range []string{"error", "panic"} {
			t.Run(phase+"/"+failure, func(t *testing.T) {
				calls, records := 0, 0
				definition := testDefinition("example.action.audit-failure", action.EffectExternalEffect)
				provider := providerFunc{definition: definition, invoke: func(context.Context, action.Host, json.RawMessage) (json.RawMessage, error) {
					calls++
					return json.RawMessage(`{"ok":true}`), nil
				}}
				deps := readyDeps(provider)
				deps.Audit = AuditSinkFunc(func(ctx context.Context, record AuditRecord) error {
					records++
					if _, ok := ctx.Deadline(); !ok {
						t.Error("audit sink has no deadline")
					}
					if record.Phase != phase {
						return nil
					}
					if failure == "panic" {
						panic("private-audit-cause")
					}
					return errors.New("private-audit-cause")
				})
				host, err := New(deps)
				if err != nil {
					t.Fatal(err)
				}
				defer host.Close()
				result, err := host.Invoke(context.Background(), readyCaller(), testModule, definition.ID, json.RawMessage(`{"value":"x"}`))
				wantCalls := 0
				if phase == "completion" {
					wantCalls = 1
				}
				if !errors.Is(err, ErrAuditUnavailable) || result != nil || calls != wantCalls || records != wantCalls+1 || strings.Contains(err.Error(), "private-audit-cause") {
					t.Fatalf("audit failure escaped boundary: result=%s err=%v calls=%d records=%d", result, err, calls, records)
				}
			})
		}
	}
}
