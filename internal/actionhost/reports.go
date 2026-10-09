package actionhost

import (
	"context"

	nb "agent-vivy/internal/notebookcontract"
	rc "agent-vivy/internal/reportcontract"
	action "agent-vivy/sdk/port/controlaction"
)

const reportModuleOwner = "vivy/reports"

// reportActionHost is the sealed facade handed only to providers owned by
// vivy/reports. It binds scope + actor from the authenticated identity —
// never from provider JSON — like the notebook facade.
type reportActionHost struct {
	*providerHost
	bundle rc.Bundle
	scopes rc.ScopeResolver
}

func newReportActionHost(parent *providerHost, bundle rc.Bundle, scopes rc.ScopeResolver) *reportActionHost {
	return &reportActionHost{providerHost: parent, bundle: bundle, scopes: scopes}
}

// Reports returns the owner-bound facade. Run-bound invocations admit as
// agent work on the session's workspace scope; direct authenticated calls
// admit as human work on Home.
func (host *reportActionHost) Reports() (rc.ScopedActions, error) {
	if host == nil || host.providerHost == nil || host.bundle == nil || host.scopes == nil {
		return nil, &rc.Error{Code: rc.CodeCapabilityUnavailable, Message: "reports facade is not armed"}
	}
	if err := host.active(); err != nil {
		return nil, &rc.Error{Code: rc.CodeCapabilityUnavailable, Message: err.Error()}
	}
	identity := host.identity
	if identity.ID == "" {
		return nil, action.ErrUnauthenticated
	}
	var ac rc.AdmissionContext
	if identity.RunID != "" {
		scope, err := host.scopes.ForSession(context.Background(), identity.SessionID)
		if err != nil {
			return nil, err
		}
		ac = rc.AdmissionContext{Scope: scope, Actor: nb.Actor{Kind: nb.ActorAgent, Ref: "run:" + identity.RunID},
			Origin: nb.OriginAgent}
	} else {
		ac = rc.AdmissionContext{Scope: host.scopes.Home(), Actor: nb.Actor{Kind: nb.ActorHuman, Ref: "peer:" + identity.ID},
			Origin: nb.OriginHuman}
	}
	return &reportScopedActions{bundle: host.bundle, ac: ac}, nil
}

// reportScopedActions pins the admission context so the wire carries only
// the request; scope/actor/origin can never be forged through JSON.
type reportScopedActions struct {
	bundle rc.Bundle
	ac     rc.AdmissionContext
}

func (a *reportScopedActions) Generate(ctx context.Context, req nb.OperationKeyed[rc.ReportRequest]) (rc.ReportAdmission, error) {
	if req.OperationKey == "" {
		return rc.ReportAdmission{}, &rc.Error{Code: rc.CodeInvalidRequest, Message: "operation_key is required"}
	}
	req.Request.OperationKey = req.OperationKey
	return a.bundle.Service().StartReport(ctx, a.ac, req.Request)
}

func (a *reportScopedActions) Get(ctx context.Context, runID string) (rc.ReportResult, error) {
	return a.bundle.Service().GetReport(ctx, a.ac, runID)
}

func (a *reportScopedActions) Cancel(ctx context.Context, runID string) error {
	return a.bundle.Service().CancelReport(ctx, a.ac, runID)
}

func (a *reportScopedActions) ReadSettings(ctx context.Context, period rc.Period) (rc.ReportSettings, error) {
	return a.bundle.Service().ReadReportSettings(ctx, a.ac, period)
}
