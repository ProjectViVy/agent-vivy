package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/toolhost"
)

type middlewareRuntimeTool struct {
	calls         int
	rewrite       json.RawMessage
	approvalClass string
}

func (tool *middlewareRuntimeTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name:     "middleware_write",
		Readonly: false,
		Params: map[string]domain.ToolParam{
			"value": {Type: "string", Required: true},
		},
	}
}

func (tool *middlewareRuntimeTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	tool.calls++
	return "executed", nil
}

func (tool *middlewareRuntimeTool) ApplyPreTool(ctx context.Context, arguments json.RawMessage, revalidate toolhost.RevalidateFunc) (toolhost.MiddlewareResult, error) {
	result := toolhost.MiddlewareResult{ToolID: tool.Spec().Name, Arguments: append(json.RawMessage(nil), arguments...)}
	if len(tool.rewrite) != 0 {
		result.Arguments = append(json.RawMessage(nil), tool.rewrite...)
		if revalidate != nil {
			if err := revalidate(ctx, toolhost.Request{ID: tool.Spec().Name, Args: result.Arguments}); err != nil {
				return toolhost.MiddlewareResult{}, err
			}
		}
	}
	if tool.approvalClass != "" {
		result.ApprovalClasses = []string{tool.approvalClass}
	}
	return result, nil
}

func TestToolAdapterRechecksPolicyAfterPublicMiddlewareRewrite(t *testing.T) {
	policy, err := NewPolicyEngine(map[domain.PolicyProfile]PolicyDefinition{
		domain.PolicyProfileDefault: {
			Default: domain.PolicyAllow,
			Rules: []PolicyRule{{
				Tool:     "middleware_write",
				Field:    "value",
				Equals:   "danger",
				Decision: domain.PolicyDeny,
				Reason:   "rewritten value denied",
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tool := &middlewareRuntimeTool{rewrite: json.RawMessage(`{"value":"danger"}`)}
	adapter := newToolAdapter(tool, 0, policy, nil, nil)
	_, err = adapter.InvokableRun(context.Background(), `{"value":"safe"}`)
	if !errors.Is(err, ErrPolicyDenied) {
		t.Fatalf("middleware rewrite error = %v, want ErrPolicyDenied", err)
	}
	if tool.calls != 0 {
		t.Fatalf("denied rewritten tool calls = %d, want 0", tool.calls)
	}
}

func TestToolAdapterMiddlewareRequireApprovalBlocksPermissivePolicy(t *testing.T) {
	policy, err := NewPolicyEngine(map[domain.PolicyProfile]PolicyDefinition{
		domain.PolicyProfileDefault: {Default: domain.PolicyAllow},
	})
	if err != nil {
		t.Fatal(err)
	}
	tool := &middlewareRuntimeTool{approvalClass: "external-effect"}
	adapter := newToolAdapter(tool, 0, policy, nil, []string{tool.Spec().Name})
	if _, err := adapter.InvokableRun(context.Background(), `{"value":"safe"}`); err == nil {
		t.Fatal("middleware RequireApproval must interrupt instead of executing")
	}
	if tool.calls != 0 {
		t.Fatalf("middleware approval tool calls = %d, want 0", tool.calls)
	}
}
