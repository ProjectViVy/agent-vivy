package notebookcontract

import (
	"errors"
	"fmt"
	"testing"
)

func TestRequestDigestIsCanonical(t *testing.T) {
	req := SaveEntryRequest{EntryID: "e1", ExpectedVersion: 3, BaseRevisionID: "r2", Title: "t", Markdown: "m"}
	d1, err := RequestDigest("vivy.notebook.entries.save", req)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	d2, err := RequestDigest("vivy.notebook.entries.save", req)
	if err != nil {
		t.Fatalf("digest2: %v", err)
	}
	if d1 != d2 || len(d1) != 64 {
		t.Fatalf("digest not stable hex64: %q vs %q", d1, d2)
	}
	changed, err := RequestDigest("vivy.notebook.entries.save", SaveEntryRequest{EntryID: "e1", ExpectedVersion: 3, BaseRevisionID: "r2", Title: "t2", Markdown: "m"})
	if err != nil {
		t.Fatalf("changed digest: %v", err)
	}
	if changed == d1 {
		t.Fatal("digest ignored a semantic field change")
	}
	otherKind, err := RequestDigest("vivy.notebook.entries.create", req)
	if err != nil {
		t.Fatalf("kind digest: %v", err)
	}
	if otherKind == d1 {
		t.Fatal("digest ignored the operation kind")
	}
}

func TestErrorCodesAndMetadata(t *testing.T) {
	err := &Error{Code: CodeRevisionConflict, Message: "stale", CurrentVersion: 9, CurrentRevisionID: "rev-9"}
	if !errors.Is(err, ErrRevisionConflict) {
		t.Fatal("errors.Is did not match by code")
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatal("errors.Is matched a different code")
	}
	if v, r := CurrentOf(fmt.Errorf("wrap: %w", err)); v != 9 || r != "rev-9" {
		t.Fatalf("CurrentOf = %d,%q", v, r)
	}
	if got := CodeOf(err); got != CodeRevisionConflict {
		t.Fatalf("CodeOf = %q", got)
	}
	if got := CodeOf(errors.New("boom")); got != CodeStorageUnavailable {
		t.Fatalf("unknown error code = %q, want storage_unavailable", got)
	}
}

func TestActorOriginMapping(t *testing.T) {
	if o, err := ActorHuman.OriginFor(); err != nil || o != OriginHuman {
		t.Fatalf("human origin = %q err=%v", o, err)
	}
	if o, err := ActorAgent.OriginFor(); err != nil || o != OriginAgent {
		t.Fatalf("agent origin = %q err=%v", o, err)
	}
	if _, err := ActorWorkflow.OriginFor(); err == nil {
		t.Fatal("workflow origin reached the ordinary write path; generated writes belong to GeneratedWriter")
	}
	if ActorKind("generated").Valid() {
		t.Fatal("client-supplied 'generated' actor kind accepted")
	}
}

func TestScopeAndRoleHelpers(t *testing.T) {
	if WorkspaceScope("ws-1") == HomeScopeID {
		t.Fatal("workspace scope collided with home")
	}
	for _, role := range SystemRoles {
		if RoleSectionID(role) == "" || RoleDefaultTitle(role) == "" {
			t.Fatalf("role %q lacks deterministic id/title", role)
		}
	}
	if len(SystemRoles) != 4 {
		t.Fatalf("seed roles = %v, want 4", SystemRoles)
	}
}
