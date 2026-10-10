package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	laputaevolution "github.com/dashimaki/laputa/evolution"
	laputadiva "github.com/dashimaki/laputa/evolution/diva"

	"agent-vivy/internal/domain"
)

// This is a controller boundary unit, not a real-backend acceptance proof.
// The actual native graph produces the terminal outcome from a scripted
// Domain result; no run status, node event or watermark is manually seeded.
type unresolvedCognitiveDomain struct {
	*fakeCognitiveDomain
	status laputaevolution.EffectStatus
}

func (d *unresolvedCognitiveDomain) Apply(_ context.Context, e laputaevolution.Effect) (laputaevolution.EffectReceipt, error) {
	d.applyCalls++
	if d.status == "" {
		return laputaevolution.EffectReceipt{}, errors.New("fixture: effect reply lost")
	}
	return laputaevolution.EffectReceipt{OperationID: e.OperationID, PayloadDigest: e.PayloadDigest, Status: d.status}, nil
}

func TestCognitiveCompletedUnresolvedWindowDoesNotAdvance(t *testing.T) {
	for _, status := range []laputaevolution.EffectStatus{laputaevolution.StatusUnknown, laputaevolution.StatusRejected, ""} {
		t.Run(string(status), func(t *testing.T) {
			ctx := context.Background()
			d := &unresolvedCognitiveDomain{fakeCognitiveDomain: &fakeCognitiveDomain{batch: laputaevolution.EvidenceBatch{ActivityRevision: 3, Entries: []laputaevolution.Entry{{ID: "source", Body: "observed"}}}}, status: status}
			svc, backend := inofyExecService(t, cognitiveTestModel())
			svc.deps.Cognitive = cognitiveBinding(d, &fakeSource{high: 5}, backend.Snapshot(), func() int64 { return 1000 })
			svc.StartCognitiveLoop(ctx, time.Hour)
			t.Cleanup(svc.StopCognitiveLoop)
			if elig, err := svc.cognitiveAttempt(ctx, false); err != nil || !elig.Run {
				t.Fatalf("initial admission: %+v %v", elig, err)
			}
			runs := listWorkflowRuns(t, svc, backend)
			if len(runs) != 1 {
				t.Fatalf("initial workflow count=%d", len(runs))
			}
			waitForRunStatus(t, backend, runs[0].ID, domain.RunCompleted)
			details, err := svc.GetWorkflow(ctx, runs[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			var outcome laputadiva.Outcome
			if err := laputaevolution.DecodeStrictJSON([]byte(details.Outputs["outcome"]), &outcome); err != nil {
				t.Fatal(err)
			}
			if outcome.Status != laputadiva.OutcomePartial && outcome.Status != laputadiva.OutcomeRecoveryRequired {
				t.Fatalf("fixture produced a resolved outcome: %+v", outcome)
			}
			for _, manual := range []bool{false, true, false, true} {
				if elig, err := svc.cognitiveAttempt(ctx, manual); err != nil || elig.Run {
					t.Fatalf("unresolved admission manual=%v: %+v %v", manual, elig, err)
				}
				st, _, err := svc.loadCognitiveState(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if st.Watermark != 0 || st.PendingThrough != 5 || st.Blocked != cognitiveBlockUnknown || st.Attempt != 0 || st.ActiveRunID != string(runs[0].ID) {
					t.Fatalf("unresolved window was settled or retried: %+v", st)
				}
			}
			if len(listWorkflowRuns(t, svc, backend)) != 1 || d.applyCalls != 1 {
				t.Fatal("unresolved window created another workflow or effect")
			}
		})
	}
}

func TestCognitiveEffectfulFailureCannotMintNewAttempt(t *testing.T) {
	ctx := context.Background()
	d := &fakeCognitiveDomain{batch: laputaevolution.EvidenceBatch{ActivityRevision: 3, Entries: []laputaevolution.Entry{{ID: "source", Body: "observed"}}}}
	m := cognitiveTestModel()
	m.replies["stage=reflect"] = "malformed reflection reply"
	svc, backend := inofyExecService(t, m)
	svc.deps.Cognitive = cognitiveBinding(d, &fakeSource{high: 5}, backend.Snapshot(), func() int64 { return 1000 })
	svc.StartCognitiveLoop(ctx, time.Hour)
	t.Cleanup(svc.StopCognitiveLoop)
	if elig, err := svc.cognitiveAttempt(ctx, false); err != nil || !elig.Run {
		t.Fatalf("initial admission: %+v %v", elig, err)
	}
	runs := listWorkflowRuns(t, svc, backend)
	if len(runs) != 1 {
		t.Fatalf("initial workflow count=%d", len(runs))
	}
	waitForRunStatus(t, backend, runs[0].ID, domain.RunFailed)
	if len(d.applied) != 1 {
		t.Fatalf("fixture did not commit the preceding work effect: %d", len(d.applied))
	}
	for _, manual := range []bool{false, true, false, true} {
		if elig, err := svc.cognitiveAttempt(ctx, manual); err != nil || elig.Run {
			t.Fatalf("post-effect failure retried manual=%v: %+v %v", manual, elig, err)
		}
	}
	st, _, err := svc.loadCognitiveState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Watermark != 0 || st.ActiveRunID != string(runs[0].ID) || st.Blocked != cognitiveBlockUnknown || len(listWorkflowRuns(t, svc, backend)) != 1 || len(d.applied) != 1 {
		t.Fatalf("effectful failure lost its recovery identity: %+v", st)
	}
}
