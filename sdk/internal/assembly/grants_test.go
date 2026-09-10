package assembly

import (
	"reflect"
	"strings"
	"testing"

	"agent-vivy/sdk/module"
)

func TestCalculateEffectiveGrantsAcceptsAllApprovedGrantNames(t *testing.T) {
	grants := []module.Grant{
		module.GrantFSRead,
		module.GrantFSWrite,
		module.GrantChannelPoll,
		module.GrantChannelWebhook,
		module.GrantChannelListen,
		module.GrantChannelA2A,
		module.GrantSecretRead,
		module.GrantProcSpawn,
		module.GrantTTY,
		module.GrantArgv,
		module.GrantRPCClient,
		module.GrantNetClient,
	}
	approvals := make([]GrantApproval, 0, len(grants))
	for _, grant := range grants {
		approval := GrantApproval{Module: "fixture/internal", Name: grant}
		if grant == module.GrantNetClient {
			approval.Constraints = map[string][]string{"hosts": {"api.example.com"}, "schemes": {"https"}}
		}
		approvals = append(approvals, approval)
	}

	got, err := calculateEffectiveGrants("fixture/internal", grants, grants, TrustT1, approvals)
	if err != nil {
		t.Fatalf("calculateEffectiveGrants() error = %v", err)
	}
	want := []EffectiveGrant{
		{Name: module.GrantArgv, Constraints: map[string][]string{}},
		{Name: module.GrantChannelA2A, Constraints: map[string][]string{}},
		{Name: module.GrantChannelListen, Constraints: map[string][]string{}},
		{Name: module.GrantChannelPoll, Constraints: map[string][]string{}},
		{Name: module.GrantChannelWebhook, Constraints: map[string][]string{}},
		{Name: module.GrantFSRead, Constraints: map[string][]string{}},
		{Name: module.GrantFSWrite, Constraints: map[string][]string{}},
		{Name: module.GrantNetClient, Constraints: map[string][]string{"hosts": {"api.example.com"}, "schemes": {"https"}}},
		{Name: module.GrantProcSpawn, Constraints: map[string][]string{}},
		{Name: module.GrantRPCClient, Constraints: map[string][]string{}},
		{Name: module.GrantSecretRead, Constraints: map[string][]string{}},
		{Name: module.GrantTTY, Constraints: map[string][]string{}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("effective grants = %#v, want %#v", got, want)
	}
}

func TestCalculateEffectiveGrantsFailsClosed(t *testing.T) {
	tests := []struct {
		name      string
		requested []module.Grant
		allowed   []module.Grant
		trust     Trust
		approvals []GrantApproval
		wantError string
	}{
		{
			name:      "unknown grant",
			requested: []module.Grant{"root.everything"},
			allowed:   []module.Grant{"root.everything"},
			trust:     TrustT1,
			approvals: []GrantApproval{{Module: "fixture/module", Name: "root.everything"}},
			wantError: "unknown requested grant",
		},
		{
			name:      "not allowed by port",
			requested: []module.Grant{module.GrantFSWrite},
			trust:     TrustT1,
			approvals: []GrantApproval{{Module: "fixture/module", Name: module.GrantFSWrite}},
			wantError: "grant fs.write is not allowed by selected Ports for fixture/module",
		},
		{
			name:      "implicit default denial",
			requested: []module.Grant{module.GrantNetClient},
			allowed:   []module.Grant{module.GrantNetClient},
			trust:     TrustT2,
			wantError: "grant net.client is not approved for fixture/module",
		},
		{
			name:      "t2 process needs evidence",
			requested: []module.Grant{module.GrantProcSpawn},
			allowed:   []module.Grant{module.GrantProcSpawn},
			trust:     TrustT2,
			approvals: []GrantApproval{{Module: "fixture/module", Name: module.GrantProcSpawn}},
			wantError: "grant proc.spawn exceeds T2 Trust ceiling for fixture/module without conformance evidence",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := calculateEffectiveGrants("fixture/module", test.requested, test.allowed, test.trust, test.approvals)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("calculateEffectiveGrants() error = %v, want substring %q", err, test.wantError)
			}
		})
	}
}

func TestCalculateEffectiveGrantsCanonicalizesConstraints(t *testing.T) {
	approval := GrantApproval{
		Module: "fixture/module",
		Name:   module.GrantNetClient,
		Constraints: map[string][]string{
			"schemes": {"https", "https"},
			"ports":   {"443"},
			"hosts":   {"z.example.com", "a.example.com"},
		},
	}
	got, err := calculateEffectiveGrants(
		"fixture/module",
		[]module.Grant{module.GrantNetClient},
		[]module.Grant{module.GrantNetClient},
		TrustT2,
		[]GrantApproval{approval},
	)
	if err != nil {
		t.Fatalf("calculateEffectiveGrants() error = %v", err)
	}
	want := []EffectiveGrant{{
		Name: module.GrantNetClient,
		Constraints: map[string][]string{
			"hosts":   {"a.example.com", "z.example.com"},
			"ports":   {"443"},
			"schemes": {"https"},
		},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("effective grants = %#v, want %#v", got, want)
	}
}

func TestCalculateEffectiveGrantsRejectsSecretAndEnvironmentMaterial(t *testing.T) {
	for _, constraints := range []map[string][]string{
		{"token": {"plain-text-secret"}},
		{"hosts": {"${API_HOST}"}},
		{"hosts": {"env:API_HOST"}},
	} {
		_, err := calculateEffectiveGrants(
			"fixture/module",
			[]module.Grant{module.GrantNetClient},
			[]module.Grant{module.GrantNetClient},
			TrustT2,
			[]GrantApproval{{Module: "fixture/module", Name: module.GrantNetClient, Constraints: constraints}},
		)
		if err == nil || !strings.Contains(err.Error(), "forbidden Secret or environment material") {
			t.Fatalf("calculateEffectiveGrants() error = %v, want material rejection for %#v", err, constraints)
		}
	}
}
