package memory

import (
	"context"
	"math"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"agent-vivy/internal/config"
	"github.com/ProjectViVy/agent-vivy/bml"
	gardenmemory "github.com/ProjectViVy/laputa/garden/memory"
	"github.com/ProjectViVy/laputa/laputa/evolution"
)

func TestIssue58BMLExpansionBudgetBounds(t *testing.T) {
	ctx := context.Background()
	service, err := Open(ctx, config.Config{Storage: config.Storage{DataDir: t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	owner := bml.OwnershipBinding{SubjectID: "budget-subject", DomainID: "budget-domain", ScopeKind: "personal", DestinationID: "bml"}
	backend, err := NewBoundBackend(service, owner)
	if err != nil {
		t.Fatal(err)
	}
	scope := evolution.Scope{SubjectID: owner.SubjectID, Kind: evolution.ScopePersonal}
	body := strings.Repeat("界", 5001)
	receipt, err := backend.Mutate(ctx, gardenmemory.AuthorizedMutation{Scope: scope, DestinationID: owner.DestinationID, OperationID: "budget-create", PayloadDigest: "budget-digest", Operation: evolution.MutationCreate, RecordID: "budget-record", ExpectedAbsent: true, Body: body, Inference: evolution.InferenceObserved})
	if err != nil {
		t.Fatal(err)
	}
	original, err := service.home.ExportBoundSource(ctx, owner, receipt.TargetRef, int64(receipt.Revision))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		budget    int
		wantRunes int
		invalid   bool
	}{
		{"negative", -1, 0, true},
		{"minimum-int", math.MinInt, 0, true},
		{"zero-default", 0, 1000, false},
		{"positive", 7, 7, false},
		{"maximum", 4000, 4000, false},
		{"over-maximum", 4001, 0, true},
		{"maximum-int", math.MaxInt, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if p := recover(); p != nil {
					t.Errorf("budget %d must return an explicit result, panic: %v", tc.budget, p)
				}
			}()
			page, err := backend.Expand(ctx, gardenmemory.AuthorizedExpansion{Scopes: []evolution.Scope{scope}, CardID: receipt.TargetRef, ExpectedRevision: receipt.Revision, BudgetChars: tc.budget})
			if tc.invalid {
				if evolution.CodeOf(err) != evolution.ErrInvalidSchema {
					t.Fatalf("budget %d: code=%q err=%v", tc.budget, evolution.CodeOf(err), err)
				}
				if len(page.Items) != 0 {
					t.Fatalf("invalid budget disclosed fragments: %+v", page)
				}
				return
			}
			if err != nil || len(page.Items) != 1 {
				t.Fatalf("budget %d: page=%+v err=%v", tc.budget, page, err)
			}
			fragment := page.Items[0]
			if fragment.CardID != receipt.TargetRef || fragment.Revision != receipt.Revision || fragment.Excerpt != strings.Repeat("界", tc.wantRunes) || utf8.RuneCountInString(fragment.Excerpt) != tc.wantRunes || fragment.StartOffset != 0 || fragment.EndOffset != tc.wantRunes || !page.Truncated {
				t.Fatalf("budget %d returned incorrect native fragment: %+v", tc.budget, page)
			}
		})
	}
	retained, err := service.home.ExportBoundSource(ctx, owner, receipt.TargetRef, int64(receipt.Revision))
	if err != nil || !reflect.DeepEqual(retained, original) {
		t.Fatalf("expansion changed native record: %+v %v", retained, err)
	}
	status, err := backend.MutationStatus(ctx, receipt.OperationID)
	if err != nil || !reflect.DeepEqual(status, receipt) {
		t.Fatalf("expansion changed retained operation: %+v %v", status, err)
	}
}
