package runtime

import (
	"context"
	"errors"

	"agent-vivy/sdk/port/contextsource"
)

var ErrContextSourceUnauthorized = errors.New("runtime: Context Source scope not admitted")

type contextSourceAdmissionKey struct{}

func withContextSourceAdmission(ctx context.Context, tenant, session, workspace string) context.Context {
	return context.WithValue(ctx, contextSourceAdmissionKey{}, contextsource.Scope{TenantID: tenant, SessionID: session, WorkspaceID: workspace})
}

// AuthorizeContextSourceRequest checks the private Runtime query capability.
// Request identities alone cannot authorize reads from an App-owned source.
func AuthorizeContextSourceRequest(ctx context.Context, request contextsource.Request) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	scope, ok := ctx.Value(contextSourceAdmissionKey{}).(contextsource.Scope)
	if !ok || request.SessionID == "" || scope.TenantID != request.TenantID || scope.SessionID != request.SessionID || scope.WorkspaceID != request.WorkspaceID {
		return ErrContextSourceUnauthorized
	}
	return nil
}
