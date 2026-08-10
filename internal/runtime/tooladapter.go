package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

// ErrPlanModeToolDenied is returned before any effectful tool reaches its
// implementation. Approval is deliberately not opened in Plan Mode.
var ErrPlanModeToolDenied = errors.New("runtime: plan mode denies effectful tools")

// toolAdapter exposes a Vivy tools.Tool to Eino's ToolsNode and enforces
// the approval gate (D-012): readonly tools execute directly; effectful
// tools interrupt on first execution and only run for real once a resume
// delivers an approved decision. InvokableRun of the wrapped tool is only
// ever reached for calls that already passed policy.
type toolAdapter struct {
	t              tools.Tool
	maxResultBytes int
}

var _ einotool.InvokableTool = (*toolAdapter)(nil)

func newToolAdapter(t tools.Tool, maxResultBytes int) *toolAdapter {
	return &toolAdapter{t: t, maxResultBytes: maxResultBytes}
}

func (a *toolAdapter) Info(_ context.Context) (*schema.ToolInfo, error) {
	spec := a.t.Spec()
	info := &schema.ToolInfo{Name: spec.Name, Desc: spec.Description}
	// Real gateways need the argument schema to fill correct parameter
	// names; without it the model guesses and calls fail (AS-2 walkthrough).
	if len(spec.Params) > 0 {
		params := make(map[string]*schema.ParameterInfo, len(spec.Params))
		for name, p := range spec.Params {
			params[name] = &schema.ParameterInfo{
				Type:     schema.String,
				Desc:     p.Desc,
				Required: p.Required,
			}
		}
		info.ParamsOneOf = schema.NewParamsOneOfByParams(params)
	}
	return info, nil
}

func (a *toolAdapter) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...einotool.Option) (string, error) {
	spec := a.t.Spec()
	if allowed, scoped := selectedToolSet(ctx); scoped {
		if _, ok := allowed[spec.Name]; !ok {
			return "", fmt.Errorf("runtime: tool %q is not selected for this request", spec.Name)
		}
	}
	if runMode(ctx) == domain.RunModePlan && !spec.Readonly {
		return "", fmt.Errorf("%w: %s", ErrPlanModeToolDenied, spec.Name)
	}
	if err := tools.ValidateArgs(spec, json.RawMessage(argumentsInJSON)); err != nil {
		return "", err
	}
	if spec.Readonly {
		return a.run(ctx, argumentsInJSON)
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
		// A plain tool result lets the model continue and close the run
		// without executing the effectful call.
		return spec.Name + " was denied by the user and did not run; continue without it.", nil
	}
	return a.run(ctx, argumentsInJSON)
}

func (a *toolAdapter) run(ctx context.Context, argumentsInJSON string) (string, error) {
	result, err := a.t.InvokableRun(ctx, json.RawMessage(argumentsInJSON))
	if err != nil {
		return "", err
	}
	return compactToolResult(result, a.maxResultBytes), nil
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
