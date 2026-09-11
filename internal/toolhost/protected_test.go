package toolhost

import (
	"errors"
	"testing"

	porttool "agent-vivy/sdk/port/tool"
)

func TestHostRejectsCollisionWithProtectedToolID(t *testing.T) {
	provider := testToolProvider{def: porttool.Definition{ID: "core.safe"}}
	_, err := New(Config{
		ProtectedIDs: []string{"core.safe"},
		Static: []StaticBinding{
			{OwnerID: "vivy/core", Provider: provider, Host: testModuleHost{id: "vivy/core"}, Trust: TrustCore},
			{OwnerID: "acme/plugin", Provider: provider, Host: testModuleHost{id: "acme/plugin"}, Trust: TrustPublic},
		},
	})
	if !errors.Is(err, ErrProtectedToolID) {
		t.Fatalf("New protected collision error = %v, want ErrProtectedToolID", err)
	}
}

func TestHostRejectsPublicClaimOnProtectedIDWhenCoreIsOmitted(t *testing.T) {
	provider := testToolProvider{def: porttool.Definition{ID: "core.safe"}}
	_, err := New(Config{
		ProtectedIDs: []string{"core.safe"},
		Static: []StaticBinding{{
			OwnerID:  "acme/plugin",
			Provider: provider,
			Host:     testModuleHost{id: "acme/plugin"},
			Trust:    TrustPublic,
		}},
	})
	if !errors.Is(err, ErrProtectedToolID) {
		t.Fatalf("New public protected claim error = %v, want ErrProtectedToolID", err)
	}
}

func TestHostReportsProtectedIdentity(t *testing.T) {
	provider := testToolProvider{def: porttool.Definition{ID: "core.safe"}}
	host, err := New(Config{
		ProtectedIDs: []string{"core.safe"},
		Static: []StaticBinding{
			{OwnerID: "vivy/core", Provider: provider, Host: testModuleHost{id: "vivy/core"}, Trust: TrustCore},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !host.IsProtected("core.safe") {
		t.Fatal("core.safe should be protected")
	}
	if host.IsProtected("plugin.other") {
		t.Fatal("plugin.other should not be protected")
	}
}
