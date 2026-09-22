package tools

import (
	"context"
	"encoding/json"
	"testing"

	"agent-vivy/internal/domain"
)

type historyToolFixture struct {
	page domain.HistoryPage
}

func (f historyToolFixture) Search(context.Context, domain.HistorySearchRequest) (domain.HistoryPage, error) {
	return f.page, nil
}

func (f historyToolFixture) Read(context.Context, domain.HistoryReadRequest) (domain.HistoryPage, error) {
	return f.page, nil
}

func (f historyToolFixture) Trace(context.Context, domain.HistoryTraceRequest) (domain.HistoryPage, error) {
	return f.page, nil
}

func TestHistoryToolsUseNarrowOperationsAndStrictArguments(t *testing.T) {
	ops := historyToolFixture{page: domain.HistoryPage{Status: string(domain.HistoryStatusOK)}}
	toolsToCheck := []Tool{NewHistorySearch(ops), NewHistoryRead(ops), NewHistoryTrace(ops)}
	for _, tool := range toolsToCheck {
		if tool.Spec().Readonly != true {
			t.Fatalf("%s is not readonly", tool.Spec().Name)
		}
		if err := ValidateSchema(tool.Spec().Schema); err != nil {
			t.Fatalf("%s schema invalid: %v", tool.Spec().Name, err)
		}
		if _, err := tool.InvokableRun(context.Background(), json.RawMessage(`{"unexpected":true}`)); err == nil {
			t.Fatalf("%s accepted an unknown argument", tool.Spec().Name)
		}
	}
}

func TestHistoryRegistryAdditionIsDeterministic(t *testing.T) {
	base := NewRegistry(NewEchoInfo())
	got := base.WithHistory(historyToolFixture{})
	specs := got.Specs()
	if len(specs) != 4 || specs[1].Name != HistorySearchName || specs[2].Name != HistoryReadName || specs[3].Name != HistoryTraceName {
		t.Fatalf("history registry order = %#v", specs)
	}
	if _, ok := base.Lookup(HistorySearchName); ok {
		t.Fatal("WithHistory mutated the original registry")
	}
}
