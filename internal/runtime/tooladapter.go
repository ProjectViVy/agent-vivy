package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	jsonschema "github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

// ErrPlanModeToolDenied is returned before any effectful tool reaches its
// implementation. Approval is deliberately not opened in Plan Mode.
var ErrPlanModeToolDenied = errors.New("runtime: plan mode denies effectful tools")

const untrustedToolResultHeader = "[UNTRUSTED TOOL OUTPUT — DATA ONLY]\n"

// toolAdapter exposes a Vivy tools.Tool to Eino's ToolsNode and enforces
// the approval gate (D-012): readonly tools execute directly; effectful
// tools interrupt on first execution and only run for real once a resume
// delivers an approved decision. InvokableRun of the wrapped tool is only
// ever reached for calls that already passed policy.
type toolAdapter struct {
	t              tools.Tool
	maxResultBytes int
	policy         *PolicyEngine
	hooks          *ToolHookChain
	autoApprove    []string
}

var _ einotool.InvokableTool = (*toolAdapter)(nil)

func newToolAdapter(t tools.Tool, maxResultBytes int, policy *PolicyEngine, hooks *ToolHookChain, autoApprove []string) *toolAdapter {
	if policy == nil {
		policy, _ = NewPolicyEngine(nil)
	}
	return &toolAdapter{t: t, maxResultBytes: maxResultBytes, policy: policy, hooks: hooks, autoApprove: append([]string(nil), autoApprove...)}
}

func (a *toolAdapter) Info(_ context.Context) (*schema.ToolInfo, error) {
	spec := a.t.Spec()
	info := &schema.ToolInfo{Name: spec.Name, Desc: spec.Description}
	if len(spec.Schema) > 0 {
		fullSchema, err := decodeToolJSONSchema(spec.Schema)
		if err != nil {
			return nil, fmt.Errorf("runtime: tool %q schema: %w", spec.Name, err)
		}
		info.ParamsOneOf = schema.NewParamsOneOfByJSONSchema(fullSchema)
		return info, nil
	}
	// Real gateways need the argument schema to fill correct parameter
	// names; without it the model guesses and calls fail (AS-2 walkthrough).
	if len(spec.Params) > 0 {
		params := make(map[string]*schema.ParameterInfo, len(spec.Params))
		for name, p := range spec.Params {
			paramType := schema.String
			switch p.Type {
			case "integer":
				paramType = schema.Integer
			case "number":
				paramType = schema.Number
			case "boolean":
				paramType = schema.Boolean
			case "object":
				paramType = schema.Object
			case "array":
				paramType = schema.Array
			}
			params[name] = &schema.ParameterInfo{
				Type:     paramType,
				Desc:     p.Desc,
				Required: p.Required,
				Enum:     append([]string(nil), p.Enum...),
			}
		}
		info.ParamsOneOf = schema.NewParamsOneOfByParams(params)
	}
	return info, nil
}

// decodeToolJSONSchema uses Eino's official JSON Schema type while filling its
// Extras facility for keywords the pinned type does not model. Without this
// boundary adapter, json.Unmarshal silently drops extension/vocabulary
// keywords such as unevaluatedProperties before the model sees the schema.
func decodeToolJSONSchema(raw json.RawMessage) (*jsonschema.Schema, error) {
	var target jsonschema.Schema
	if err := json.Unmarshal(raw, &target); err != nil {
		return nil, err
	}
	if err := preserveToolSchemaExtras(raw, &target); err != nil {
		return nil, err
	}
	return &target, nil
}

func preserveToolSchemaExtras(raw json.RawMessage, target *jsonschema.Schema) error {
	if target == nil || bytes.Equal(bytes.TrimSpace(raw), []byte("true")) || bytes.Equal(bytes.TrimSpace(raw), []byte("false")) {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	known := map[string]struct{}{
		"$schema": {}, "$id": {}, "$anchor": {}, "$ref": {}, "$dynamicRef": {}, "$defs": {}, "$comment": {},
		"allOf": {}, "anyOf": {}, "oneOf": {}, "not": {}, "if": {}, "then": {}, "else": {}, "dependentSchemas": {},
		"prefixItems": {}, "items": {}, "contains": {}, "properties": {}, "patternProperties": {}, "additionalProperties": {}, "propertyNames": {},
		"type": {}, "enum": {}, "const": {}, "multipleOf": {}, "maximum": {}, "exclusiveMaximum": {}, "minimum": {}, "exclusiveMinimum": {},
		"maxLength": {}, "minLength": {}, "pattern": {}, "maxItems": {}, "minItems": {}, "uniqueItems": {}, "maxContains": {}, "minContains": {},
		"maxProperties": {}, "minProperties": {}, "required": {}, "dependentRequired": {}, "format": {}, "contentEncoding": {}, "contentMediaType": {}, "contentSchema": {},
		"title": {}, "description": {}, "default": {}, "deprecated": {}, "readOnly": {}, "writeOnly": {}, "examples": {},
	}
	var extras map[string]any
	for key, value := range fields {
		if _, ok := known[key]; !ok {
			if extras == nil {
				extras = make(map[string]any)
			}
			extras[key] = json.RawMessage(append([]byte(nil), value...))
		}
	}
	if extras != nil {
		target.Extras = extras
	}
	for _, key := range []string{"enum", "const", "default", "examples"} {
		if value, ok := fields[key]; ok {
			decoded, err := decodeJSONWithNumbers(value)
			if err != nil {
				return err
			}
			switch key {
			case "enum":
				target.Enum, _ = decoded.([]any)
			case "const":
				target.Const = decoded
			case "default":
				target.Default = decoded
			case "examples":
				target.Examples, _ = decoded.([]any)
			}
		}
	}
	for key, value := range fields {
		switch key {
		case "$defs":
			var children map[string]json.RawMessage
			if err := json.Unmarshal(value, &children); err != nil {
				return err
			}
			if target.Definitions == nil {
				target.Definitions = jsonschema.Definitions{}
			}
			for name, child := range children {
				decoded, err := decodeToolJSONSchema(child)
				if err != nil {
					return err
				}
				target.Definitions[name] = decoded
			}
		case "properties":
			var children map[string]json.RawMessage
			if err := json.Unmarshal(value, &children); err != nil {
				return err
			}
			if target.Properties == nil {
				target.Properties = orderedmap.New[string, *jsonschema.Schema]()
			}
			for name, child := range children {
				decoded, err := decodeToolJSONSchema(child)
				if err != nil {
					return err
				}
				target.Properties.Set(name, decoded)
			}
		case "allOf", "anyOf", "oneOf", "prefixItems":
			var children []json.RawMessage
			if err := json.Unmarshal(value, &children); err != nil {
				return err
			}
			decoded := make([]*jsonschema.Schema, 0, len(children))
			for _, child := range children {
				item, err := decodeToolJSONSchema(child)
				if err != nil {
					return err
				}
				decoded = append(decoded, item)
			}
			switch key {
			case "allOf":
				target.AllOf = decoded
			case "anyOf":
				target.AnyOf = decoded
			case "oneOf":
				target.OneOf = decoded
			case "prefixItems":
				target.PrefixItems = decoded
			}
		case "dependentSchemas", "patternProperties":
			var children map[string]json.RawMessage
			if err := json.Unmarshal(value, &children); err != nil {
				return err
			}
			decoded := make(map[string]*jsonschema.Schema, len(children))
			for name, child := range children {
				item, err := decodeToolJSONSchema(child)
				if err != nil {
					return err
				}
				decoded[name] = item
			}
			if key == "dependentSchemas" {
				target.DependentSchemas = decoded
			} else {
				target.PatternProperties = decoded
			}
		case "items", "contains", "additionalProperties", "propertyNames", "contentSchema", "not", "if", "then", "else":
			decoded, err := decodeToolJSONSchema(value)
			if err != nil {
				return err
			}
			switch key {
			case "items":
				target.Items = decoded
			case "contains":
				target.Contains = decoded
			case "additionalProperties":
				target.AdditionalProperties = decoded
			case "propertyNames":
				target.PropertyNames = decoded
			case "contentSchema":
				target.ContentSchema = decoded
			case "not":
				target.Not = decoded
			case "if":
				target.If = decoded
			case "then":
				target.Then = decoded
			case "else":
				target.Else = decoded
			}
		}
	}
	return nil
}

func decodeJSONWithNumbers(raw json.RawMessage) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func (a *toolAdapter) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...einotool.Option) (string, error) {
	spec := a.t.Spec()
	if allowed, scoped := selectedToolSet(ctx); scoped {
		_, ok := allowed[spec.Name]
		// A skill_view mount extends the selected surface for the rest of
		// the run: hidden tools become callable once mounted.
		if !ok && tools.MountedToolsFromContext(ctx).Has(spec.Name) {
			ok = true
		}
		if !ok {
			return "", fmt.Errorf("runtime: tool %q is not selected for this request", spec.Name)
		}
	}
	if err := tools.ValidateArgs(spec, json.RawMessage(argumentsInJSON)); err != nil {
		return "", err
	}
	if err := tools.ValidateArgsSafety(spec, json.RawMessage(argumentsInJSON)); err != nil {
		return "", err
	}
	profile := policyProfile(ctx)
	evaluation, err := a.policy.Evaluate(profile, spec, []byte(argumentsInJSON))
	if err != nil {
		return "", err
	}
	emitGovernanceEvent(ctx, GovernanceEvent{
		Type: domain.EventPolicyEvaluated, ToolName: spec.Name, Decision: string(evaluation.Decision),
		Profile: profile, PolicyHash: evaluation.Snapshot.Hash, Reason: evaluation.Reason,
	})
	if evaluation.Decision == domain.PolicyDeny {
		if runMode(ctx) == domain.RunModePlan && !spec.Readonly {
			return "", fmt.Errorf("%w: %s", ErrPlanModeToolDenied, spec.Name)
		}
		return "", fmt.Errorf("%w: %s (%s)", ErrPolicyDenied, spec.Name, evaluation.Reason)
	}
	args := json.RawMessage(argumentsInJSON)
	if a.hooks != nil {
		args, err = a.hooks.PreToolUse(ctx, ToolHookCall{
			RunID: contextRunID(ctx), ToolName: spec.Name, Arguments: args, Profile: profile,
		})
		if err != nil {
			return "", err
		}
		if err := tools.ValidateArgs(spec, args); err != nil {
			return "", err
		}
		if err := tools.ValidateArgsSafety(spec, args); err != nil {
			return "", err
		}
		// A hook rewrite is untrusted input. The policy must see the final
		// arguments before the tool can observe them.
		if string(args) != argumentsInJSON {
			evaluation, err = a.policy.Evaluate(profile, spec, args)
			if err != nil {
				return "", err
			}
			emitGovernanceEvent(ctx, GovernanceEvent{
				Type: domain.EventPolicyEvaluated, ToolName: spec.Name, Decision: string(evaluation.Decision),
				Profile: profile, PolicyHash: evaluation.Snapshot.Hash, Reason: "post-hook argument rewrite: " + evaluation.Reason,
			})
			if evaluation.Decision != domain.PolicyAllow {
				if evaluation.Decision == domain.PolicyDeny {
					return "", fmt.Errorf("%w: rewritten arguments for %s", ErrPolicyDenied, spec.Name)
				}
				return "", fmt.Errorf("%w: rewritten arguments for %s require a fresh approval", ErrPolicyDenied, spec.Name)
			}
		}
	}

	middlewareApprovalClasses := []string(nil)
	args, evaluation, middlewareApprovalClasses, err = a.applyGovernedMiddleware(ctx, spec, args, profile, evaluation)
	if err != nil {
		return "", err
	}
	middlewareRequiresApproval := len(middlewareApprovalClasses) != 0

	// Per-call tiering: a classifier-aware tool (bash) can deny outright or
	// run safe read-only invocations without an interrupt under the 'auto'
	// approval policy. The check runs on the final arguments, after hooks and
	// public Middleware, and regardless of the profile decision so the deny
	// table holds even under full-auto profiles.
	if classifier, ok := a.t.(tools.InvocationClassifier); ok {
		class, findings, err := classifier.ClassifyInvocation(args)
		if err != nil {
			return "", err
		}
		if class == tools.InvocationDenied {
			reason := strings.Join(findings, "; ")
			if reason == "" {
				reason = "deny-table match"
			}
			return "", fmt.Errorf("%w: %s (%s)", ErrPolicyDenied, spec.Name, reason)
		}
		if class == tools.InvocationSafe && evaluation.Decision == domain.PolicyPrompt && approvalPolicy(ctx) == domain.ApprovalPolicyAuto && !middlewareRequiresApproval {
			emitGovernanceEvent(ctx, GovernanceEvent{
				Type: domain.EventPolicyEvaluated, ToolName: spec.Name, Decision: string(domain.PolicyAllow),
				Profile: profile, PolicyHash: evaluation.Snapshot.Hash, Reason: "safe read-only invocation auto-approved",
			})
			return a.run(ctx, string(args))
		}
	}
	if spec.Interaction == domain.ToolInteractionQuestion {
		isTarget, hasData, answer := einotool.GetResumeContext[string](ctx)
		if isTarget && hasData {
			return answer, nil
		}
		return "", einotool.Interrupt(ctx, "user answer required for "+spec.Name)
	}
	if evaluation.Decision == domain.PolicyPrompt || middlewareRequiresApproval {
		if middlewareRequiresApproval {
			wasInterrupted, _, _ := einotool.GetInterruptState[any](ctx)
			if !wasInterrupted {
				return "", einotool.Interrupt(ctx, "middleware approval required for "+spec.Name)
			}
			isTarget, hasData, decision := einotool.GetResumeContext[string](ctx)
			if !isTarget || !hasData {
				return "", einotool.Interrupt(ctx, "still waiting for middleware approval of "+spec.Name)
			}
			if decision == domain.ApprovalDenied {
				return spec.Name + " was denied by the user and did not run; continue without it.", nil
			}
		} else {
			approvalEval := a.policy.EvaluateApprovalPolicy(approvalPolicy(ctx), spec, a.autoApprove)
			if approvalEval.AutoApprove {
				return a.run(ctx, string(args))
			}
			if !approvalEval.ShouldAsk {
				return "", fmt.Errorf("%w: %s (%s)", ErrPolicyDenied, spec.Name, approvalEval.Reason)
			}
			wasInterrupted, _, _ := einotool.GetInterruptState[any](ctx)
			if !wasInterrupted {
				// First execution: pause the run so the service can surface
				// tool.approval_required over a durable checkpoint (D-029).
				return "", einotool.Interrupt(ctx, "approval required for "+spec.Name)
			}
			isTarget, hasData, decision := einotool.GetResumeContext[string](ctx)
			if !isTarget {
				// A sibling interrupt resumed first; keep waiting.
				return "", einotool.Interrupt(ctx, "still waiting for approval of "+spec.Name)
			}
			if hasData && decision == domain.ApprovalDenied {
				return spec.Name + " was denied by the user and did not run; continue without it.", nil
			}
		}
	}
	return a.run(ctx, string(args))
}

func (a *toolAdapter) run(ctx context.Context, argumentsInJSON string) (string, error) {
	result, err := a.invoke(ctx, argumentsInJSON)
	if err != nil {
		return "", err
	}
	result = untrustedToolResultHeader + tools.RedactSensitive(result)
	// A multimodal parts envelope must reach normalizeEnhancedResult
	// intact: byte compaction would corrupt it into unparseable JSON, so
	// the budget is applied per part there instead. Media parts are sized
	// at their source (e.g. the read_file image cap).
	if isToolPartsEnvelope(strings.TrimPrefix(result, untrustedToolResultHeader)) {
		return result, nil
	}
	return compactToolResult(result, a.maxResultBytes), nil
}

// invoke is the shared runtime-owned execution seam for direct tools. It
// performs the actual Tool call, post hooks, and mount accounting, but leaves
// result framing to the caller. The governed shell uses this seam so it can
// remove the raw command/cwd fields before the bounded untrusted result is
// persisted, while model tools continue through run unchanged.
func (a *toolAdapter) invoke(ctx context.Context, argumentsInJSON string) (string, error) {
	// The runtime run identity is copied into the tools package context at the
	// Eino boundary so workspace-backed tools cannot fall back to a host path.
	toolCtx := tools.WithRunID(ctx, contextRunID(ctx))
	toolCtx = tools.WithSessionID(toolCtx, contextSessionID(ctx))
	toolCtx = tools.WithWorkspaceID(toolCtx, contextWorkspaceID(ctx))
	mountsBefore := tools.MountedToolsFromContext(ctx).Mounted()
	result, err := a.t.InvokableRun(toolCtx, json.RawMessage(argumentsInJSON))
	if a.hooks != nil {
		a.hooks.PostToolUse(ctx, ToolHookCall{
			RunID: contextRunID(ctx), ToolName: a.t.Spec().Name, Arguments: json.RawMessage(argumentsInJSON), Profile: policyProfile(ctx),
		}, tools.RedactSensitive(result), err)
	}
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "proposal stale") || strings.Contains(strings.ToLower(err.Error()), "target changed after human review") {
			tools.ReportProposalStale(ctx, err.Error())
		}
		return "", err
	}
	emitToolMounts(ctx, a.t.Spec().Name, mountsBefore)
	return result, nil
}

// emitToolMounts journals tools newly mounted during a successful
// invocation (TT-3): the model.request event records only the active
// baseline, so the mount delta is its own audit event. The diff is
// generic — any tool that activates hidden tools gets journaled, not
// just skill_view.
func emitToolMounts(ctx context.Context, toolName string, before []string) {
	mounts := tools.MountedToolsFromContext(ctx)
	if mounts == nil {
		return
	}
	seen := make(map[string]struct{}, len(before))
	for _, name := range before {
		seen[name] = struct{}{}
	}
	var added []string
	for _, name := range mounts.Mounted() {
		if _, ok := seen[name]; !ok {
			added = append(added, name)
		}
	}
	if len(added) == 0 {
		return
	}
	emitGovernanceEvent(ctx, GovernanceEvent{
		Type: domain.EventToolMounted, ToolName: toolName, MountedTools: added,
	})
}

func isToolPartsEnvelope(result string) bool {
	var envelope rawToolEnvelope
	return json.Unmarshal([]byte(result), &envelope) == nil && len(envelope.Parts) > 0
}

// compactToolResult keeps a bounded head and tail around an explicit
// tombstone. The original result remains available to the tool's own durable
// audit/event path only when that path chooses to retain it; the model never
// receives an unbounded tool result.
func compactToolResult(result string, budget int) string {
	if budget <= 0 || len(result) <= budget {
		return result
	}
	marker := fmt.Sprintf("\n[tool output collapsed: %d bytes removed]\n", len(result)-budget)
	if len(marker) >= budget {
		return truncateUTF8(marker, budget)
	}
	available := budget - len(marker)
	headBudget := available / 2
	tailBudget := available - headBudget
	return takePrefixUTF8(result, headBudget) + marker + takeSuffixUTF8(result, tailBudget)
}

func truncateUTF8(value string, budget int) string {
	return takePrefixUTF8(value, budget)
}

func takePrefixUTF8(value string, budget int) string {
	if budget <= 0 {
		return ""
	}
	used := 0
	for _, r := range value {
		size := utf8.RuneLen(r)
		if used+size > budget {
			break
		}
		used += size
	}
	return value[:used]
}

func takeSuffixUTF8(value string, budget int) string {
	if budget <= 0 {
		return ""
	}
	used := 0
	start := len(value)
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(value[:start])
		if used+size > budget {
			break
		}
		used += size
		start -= size
		if r == utf8.RuneError && size == 0 {
			break
		}
	}
	return value[start:]
}
