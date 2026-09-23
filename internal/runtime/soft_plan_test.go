package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"agent-vivy/internal/tools"
)

func TestSoftPlanPreservesIndependentEffectfulPolicy(t *testing.T) {
	for _, tc := range []struct {
		name        string
		profile     domain.PolicyProfile
		wantCalls   int
		wantRefusal string
	}{
		{name: "writable policy remains writable", profile: domain.PolicyProfileFullAuto, wantCalls: 1},
		{name: "read-only policy remains restrictive", profile: domain.PolicyProfileReadOnly, wantRefusal: "did not run"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode, profile, collaboration, version, err := normalizeRunOptions(
				domain.RunModeNormal, tc.profile, domain.CollaborationModePlan, domain.CollaborationVersion)
			if err != nil {
				t.Fatalf("normalize soft Plan options: %v", err)
			}
			if mode != domain.RunModeNormal || profile != tc.profile || collaboration != domain.CollaborationModePlan || version != domain.CollaborationVersion {
				t.Fatalf("soft Plan rewrote independent run options: %q %q %q %d", mode, profile, collaboration, version)
			}
			tool := &planCountingTool{}
			adapter := newToolAdapter(tool, 0, nil, nil, nil)
			ctx := withPolicyProfile(withRunMode(context.Background(), mode), profile)
			result, err := adapter.InvokableRun(ctx, `{"value":"draft"}`)
			if err != nil {
				t.Fatalf("effectful tool invocation: %v", err)
			}
			if tool.calls != tc.wantCalls {
				t.Fatalf("effectful calls = %d, want %d (result %q)", tool.calls, tc.wantCalls, result)
			}
			if tc.wantRefusal != "" && !strings.Contains(result, tc.wantRefusal) {
				t.Fatalf("refusal result = %q, want %q", result, tc.wantRefusal)
			}
		})
	}
}

func TestPlanGuidanceUsesEmbeddedMarkdownSource(t *testing.T) {
	guidance := promptAsset("plan.md")
	if guidance == "" {
		t.Fatal("embedded plan.md guidance is empty")
	}
	if guidance != planGuidanceText {
		t.Fatalf("runtime guidance differs from embedded plan.md: %q != %q", planGuidanceText, guidance)
	}
	if !strings.Contains(composeRunPreamble(time.Unix(0, 0), "", true, domain.FaceWeb, domain.CollaborationModePlan), guidance) {
		t.Fatal("per-run composition omitted embedded Plan guidance")
	}
	messages := reconcilePlanGuidance(nil, true)
	if len(messages) != 1 || messages[0].Content != guidance {
		t.Fatalf("middleware guidance = %+v, want one message from plan.md", messages)
	}
}

func TestLegacyPlanShellResumeRestoresHardRestriction(t *testing.T) {
	ctx := context.Background()
	backend, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "legacy-plan-shell.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	const runID = domain.RunID("legacy-plan-shell")
	if err := backend.CreateRun(ctx, domain.Run{ID: runID, SessionID: "legacy-plan-shell-session", Status: domain.RunActive, CreatedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("create run: %v", err)
	}
	mapper := newEventMapper(runID, 64<<10)
	started := mapper.build(domain.EventRunStarted, payloadRunStarted{
		Mode: string(domain.RunModePlan), PolicyProfile: string(domain.PolicyProfileFullAuto),
		Face: string(domain.FaceTui), SandboxMode: string(domain.SandboxModeWorkspaceWrite),
		ApprovalPolicy: string(domain.ApprovalPolicyAuto),
	})
	required := mapper.build(domain.EventToolApprovalRequired, payloadToolApprovalRequired{
		ApprovalID: "legacy-approval", ToolName: tools.BashName, Mode: string(domain.RunModePlan),
		PolicyProfile: string(domain.PolicyProfileFullAuto), Face: string(domain.FaceTui),
		SandboxMode: string(domain.SandboxModeWorkspaceWrite), ApprovalPolicy: string(domain.ApprovalPolicyAuto),
	})
	if _, err := backend.Append(ctx, storage.Commit{RunID: runID, Events: []domain.RunEvent{started, required}}); err != nil {
		t.Fatalf("append legacy events: %v", err)
	}
	svc := &Service{deps: ServiceDeps{Journal: backend}, defaultProfile: domain.PolicyProfileDefault}
	_, _, resumedMode, _, resumedProfile, _, _, _ := svc.approvalDetails(ctx, runID)
	if resumedMode != domain.RunModePlan || resumedProfile != domain.PolicyProfilePlan {
		t.Fatalf("recovered runtime tool mode/profile = %q/%q, want plan/plan for legacy hard restriction", resumedMode, resumedProfile)
	}
	runtimeTool := &planCountingTool{}
	runtimeAdapter := newToolAdapter(runtimeTool, 0, nil, nil, nil)
	result, err := runtimeAdapter.InvokableRun(withPolicyProfile(withRunMode(ctx, resumedMode), resumedProfile), `{"value":"draft"}`)
	if err != nil || runtimeTool.calls != 0 || !strings.Contains(result, "plan mode") {
		t.Fatalf("legacy resumed runtime tool result = %q, err %v, calls %d; want pre-invocation hard denial", result, err, runtimeTool.calls)
	}
	profile, _, mode, _, _, _ := svc.shellRecoveryMetadata(ctx, runID, domain.Approval{ApprovalPolicy: string(domain.ApprovalPolicyAuto)})
	if mode != domain.RunModePlan || profile != domain.PolicyProfilePlan {
		t.Fatalf("recovered shell mode/profile = %q/%q, want plan/plan for legacy hard restriction", mode, profile)
	}
	tool := &planCountingTool{}
	adapter := newToolAdapter(tool, 0, nil, nil, nil)
	runCtx := withPolicyProfile(withRunMode(ctx, mode), profile)
	_, err = svc.authorizeShell(runCtx, adapter, json.RawMessage(`{"value":"draft"}`), false)
	if err == nil || tool.calls != 0 {
		t.Fatalf("legacy shell authorization = %v with %d tool calls, want denial before invocation", err, tool.calls)
	}
}
