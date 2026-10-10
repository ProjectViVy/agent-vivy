package divacognitive

import (
	"context"
	"errors"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/sdk/port/contextsource"
)

func TestRecallSourceRequiresSingleOwnedBinding(t *testing.T) {
	source := NewRecallSource()
	if _, err := source.Query(context.Background(), contextsource.Request{Query: "fact", SessionID: "s"}); !errors.Is(err, cognitivecontract.ErrUnarmed) {
		t.Fatalf("unarmed source: %v", err)
	}
	if err := source.BindCognitiveContext(nil, func(context.Context, contextsource.Request) error { return nil }); err == nil {
		t.Fatal("nil owner accepted")
	}
	ctx, bundle := openBundle(t)
	if err := source.BindCognitiveContext(bundle, nil); err == nil {
		t.Fatal("missing scope admission accepted")
	}
	denied := errors.New("synthetic denied scope")
	if err := source.BindCognitiveContext(bundle, func(context.Context, contextsource.Request) error { return denied }); err != nil {
		t.Fatal(err)
	}
	if _, err := source.Query(ctx, contextsource.Request{Query: "fact", SessionID: "s"}); !errors.Is(err, denied) {
		t.Fatalf("scope refusal bypassed: %v", err)
	}
	if err := source.BindCognitiveContext(bundle, func(context.Context, contextsource.Request) error { return nil }); err == nil {
		t.Fatal("source binding replaced")
	}
}
