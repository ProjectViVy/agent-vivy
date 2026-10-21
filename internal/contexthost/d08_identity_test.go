package contexthost

import (
	"context"
	"testing"

	"agent-vivy/sdk/port/contextsource"
)

func TestContextViewHasIndependentQueryIdentity(t *testing.T) {
	host, err := New(Config{Sources: []contextsource.Provider{fixtureSource{id: "source", page: contextsource.NewPage([]contextsource.Candidate{{ContentID: "item", MediaType: "text/plain", Content: "same"}}, "")}}})
	if err != nil {
		t.Fatal(err)
	}
	first, err := host.Query(context.Background(), Request{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := host.Query(context.Background(), Request{})
	if err != nil {
		t.Fatal(err)
	}
	if first.View.ID == "" || first.View.ID == second.View.ID {
		t.Fatalf("queries share a content-derived View ID: %q / %q", first.View.ID, second.View.ID)
	}
}
