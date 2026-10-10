package reports

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	nb "agent-vivy/internal/notebookcontract"
	rc "agent-vivy/internal/reportcontract"
	controlaction "agent-vivy/sdk/port/controlaction"
)

const (
	ActionGenerate      = "vivy.reports.generate"
	ActionGet           = "vivy.reports.get"
	ActionCancel        = "vivy.reports.cancel"
	ActionSettingsRead  = "vivy.reports.settings.read"
	ActionSettingsWrite = "vivy.reports.settings.write"
)

// ActionIDs is the sealed control-action inventory the source catalog
// mirrors onto std/control-action@v1 Provides.
var ActionIDs = []string{ActionGenerate, ActionGet, ActionCancel, ActionSettingsRead, ActionSettingsWrite}

const (
	maxReportActionInput  = 64 << 10
	maxReportActionOutput = 256 << 10
)

type reportAction struct {
	definition controlaction.Definition
	invoke     func(context.Context, rc.ScopedActions, json.RawMessage) (any, error)
}

func (a reportAction) Definition() controlaction.Definition { return a.definition }

// Invoke reaches the owner-bound facade only through the sealed internal
// ActionHost extension. A lookalike host fails closed before any decode.
func (a reportAction) Invoke(ctx context.Context, host controlaction.Host, input json.RawMessage) (json.RawMessage, error) {
	privateHost, ok := host.(rc.ActionHost)
	if !ok || privateHost == nil {
		return nil, errors.New("reports: action facade unavailable")
	}
	if len(input) > maxReportActionInput {
		return reportOutcome(reportActionErr(&rc.Error{Code: rc.CodeInvalidRequest, Message: "input exceeds bound"}))
	}
	facade, err := privateHost.Reports()
	if err != nil {
		return reportOutcome(reportActionErr(err))
	}
	result, err := a.invoke(ctx, facade, input)
	if err != nil {
		return reportOutcome(reportActionErr(err))
	}
	return reportOutcome(reportActionOK(result))
}

// ActionProviders returns the sealed std/control-action@v1 inventory owned
// by vivy/reports.
func ActionProviders() []controlaction.Provider {
	return []controlaction.Provider{
		reportAction{definition: rdef(ActionGenerate, "Generate a bounded report", controlaction.EffectWrite,
			`{"type":"object","additionalProperties":false,"required":["period","window","operation_key"],"properties":{"period":{"enum":["daily","weekly","monthly"]},"window":{"enum":["current","completed"]},"operation_key":{"type":"string","minLength":1},"target":{"type":"object","additionalProperties":false,"properties":{"section_id":{"type":"string"},"entry_id":{"type":"string"}}}}}`),
			invoke: invokeReport(func(s rc.ScopedActions, in reportGenerateIn) (any, error) {
				return s.Generate(context.Background(), nb.OperationKeyed[rc.ReportRequest]{
					OperationKey: in.OperationKey,
					Request:      rc.ReportRequest{Period: rc.Period(in.Period), Window: rc.WindowSelector(in.Window), Target: in.Target},
				})
			})},
		reportAction{definition: rdef(ActionGet, "Read one report run", controlaction.EffectRead,
			`{"type":"object","additionalProperties":false,"required":["run_id"],"properties":{"run_id":{"type":"string","minLength":1}}}`),
			invoke: invokeReport(func(s rc.ScopedActions, in reportGetIn) (any, error) {
				return s.Get(context.Background(), in.RunID)
			})},
		reportAction{definition: rdef(ActionCancel, "Cancel an active report run", controlaction.EffectWrite,
			`{"type":"object","additionalProperties":false,"required":["run_id"],"properties":{"run_id":{"type":"string","minLength":1}}}`),
			invoke: invokeReport(func(s rc.ScopedActions, in reportGetIn) (any, error) {
				return nil, s.Cancel(context.Background(), in.RunID)
			})},
		reportAction{definition: rdef(ActionSettingsRead, "Read report settings", controlaction.EffectRead,
			`{"type":"object","additionalProperties":false,"required":["period"],"properties":{"period":{"enum":["daily","weekly","monthly"]}}}`),
			invoke: invokeReport(func(s rc.ScopedActions, in reportSettingsIn) (any, error) {
				return s.ReadSettings(context.Background(), rc.Period(in.Period))
			})},
		reportAction{definition: rdef(ActionSettingsWrite, "Replace report settings under revision CAS", controlaction.EffectWrite,
			`{"type":"object","additionalProperties":false,"required":["period","expected_revision","operation_key","timezone","section_id","enabled","schedule_expr"],"properties":{"period":{"enum":["daily","weekly","monthly"]},"expected_revision":{"type":"integer","minimum":1},"operation_key":{"type":"string","minLength":1},"timezone":{"type":"string","minLength":1},"section_id":{"type":"string","minLength":1},"provider":{"type":"string"},"model_id":{"type":"string"},"enabled":{"type":"boolean"},"schedule_expr":{"type":"string"}}}`),
			invoke: invokeReport(func(s rc.ScopedActions, in reportSettingsWriteIn) (any, error) {
				return s.WriteSettings(context.Background(), nb.OperationKeyed[rc.ReportSettingsWrite]{
					OperationKey: in.OperationKey,
					Request: rc.ReportSettingsWrite{
						Period: rc.Period(in.Period), ExpectedRevision: in.ExpectedRevision,
						Timezone: in.Timezone, SectionID: in.SectionID,
						Provider: in.Provider, ModelID: in.ModelID,
						Enabled: in.Enabled, ScheduleExpr: in.ScheduleExpr},
				})
			})},
	}
}

type reportGenerateIn struct {
	Period       string        `json:"period"`
	Window       string        `json:"window"`
	OperationKey string        `json:"operation_key"`
	Target       *rc.TargetRef `json:"target,omitempty"`
}
type reportGetIn struct {
	RunID string `json:"run_id"`
}
type reportSettingsIn struct {
	Period string `json:"period"`
}
type reportSettingsWriteIn struct {
	Period           string `json:"period"`
	ExpectedRevision int64  `json:"expected_revision"`
	OperationKey     string `json:"operation_key"`
	Timezone         string `json:"timezone"`
	SectionID        string `json:"section_id"`
	Provider         string `json:"provider,omitempty"`
	ModelID          string `json:"model_id,omitempty"`
	Enabled          bool   `json:"enabled"`
	ScheduleExpr     string `json:"schedule_expr"`
}

func invokeReport[T any](fn func(rc.ScopedActions, T) (any, error)) func(context.Context, rc.ScopedActions, json.RawMessage) (any, error) {
	return func(ctx context.Context, s rc.ScopedActions, raw json.RawMessage) (any, error) {
		var in T
		dec := json.NewDecoder(bytesReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			return nil, &rc.Error{Code: rc.CodeInvalidRequest, Message: "invalid request payload: " + err.Error()}
		}
		return fn(s, in)
	}
}

type reportOutcomeObj struct {
	Status string              `json:"status"`
	Result any                 `json:"result,omitempty"`
	Error  *reportOutcomeError `json:"error,omitempty"`
}

type reportOutcomeError struct {
	Code      rc.Code `json:"code"`
	Message   string  `json:"message"`
	Retryable bool    `json:"retryable,omitempty"`
}

func reportActionErr(err error) reportOutcomeObj {
	obj := &reportOutcomeError{Code: rc.CodeStorageUnavailable, Message: err.Error(), Retryable: true}
	var re *rc.Error
	if errors.As(err, &re) {
		obj.Code, obj.Message = re.Code, re.Message
		obj.Retryable = false
	}
	return reportOutcomeObj{Status: "error", Error: obj}
}

func reportActionOK(v any) reportOutcomeObj { return reportOutcomeObj{Status: "ok", Result: v} }

func reportOutcome(o reportOutcomeObj) (json.RawMessage, error) { return json.Marshal(o) }

func rdef(id, description string, effect controlaction.Effect, input string) controlaction.Definition {
	return controlaction.Definition{
		ID: id, Owner: ID, ModuleID: ID, Description: description, Effect: effect,
		MaxInputBytes: maxReportActionInput, MaxOutputBytes: maxReportActionOutput,
		InputSchema: json.RawMessage(input), ResultSchema: reportOutcomeSchema,
	}
}

var reportOutcomeSchema = json.RawMessage(`{"type":"object","required":["status"],"properties":{"status":{"enum":["ok","error"]},"result":{},"error":{"type":"object","properties":{"code":{"type":"string"},"message":{"type":"string"},"retryable":{"type":"boolean"}}}}}`)

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
