package domain

import "testing"

func TestPermissionPresetBundle(t *testing.T) {
	cases := []struct {
		preset PermissionPreset
		mode   SandboxMode
		policy ApprovalPolicy
	}{
		{PermissionPresetCautious, SandboxModeReadOnly, ApprovalPolicyAsk},
		{PermissionPresetSmart, SandboxModeWorkspaceWrite, ApprovalPolicyAsk},
		{PermissionPresetTrusted, SandboxModeDangerFullAccess, ApprovalPolicyAuto},
	}
	for _, tc := range cases {
		mode, policy, ok := tc.preset.Bundle()
		if !ok || mode != tc.mode || policy != tc.policy {
			t.Fatalf("%s bundle = %s/%s ok=%v", tc.preset, mode, policy, ok)
		}
		if got := PermissionPresetOf(mode, policy); got != tc.preset {
			t.Fatalf("round trip %s -> %s", tc.preset, got)
		}
	}
	if _, _, ok := PermissionPresetCustom.Bundle(); ok {
		t.Fatal("custom must not be a switch target")
	}
	if PermissionPresetOf(SandboxModeReadOnly, ApprovalPolicyAuto) != PermissionPresetCustom {
		t.Fatal("unmatched knobs must be custom")
	}
}

func TestSessionEffectiveSandboxDefaults(t *testing.T) {
	s := Session{}
	mode, policy := s.EffectiveSandbox()
	if mode != SandboxModeWorkspaceWrite || policy != ApprovalPolicyAsk {
		t.Fatalf("empty session = %s/%s", mode, policy)
	}
	if s.PermissionPreset() != PermissionPresetSmart {
		t.Fatalf("empty session preset = %s", s.PermissionPreset())
	}
}
