package actionhost

import (
	"context"

	nb "agent-vivy/internal/notebookcontract"
	action "agent-vivy/sdk/port/controlaction"
)

const notebookModuleOwner = "vivy/notebook-core"

// notebookActionHost is the sealed facade handed only to providers owned by
// vivy/notebook-core. The public control-action Host interface does not grow
// a notebook method; like the mask and cognitive facades it wraps the
// invocation-local providerHost and binds scope/actor from the
// authenticated identity — never from provider JSON.
type notebookActionHost struct {
	*providerHost
	bundle nb.Bundle
	scopes nb.ScopeResolver
}

func newNotebookActionHost(parent *providerHost, bundle nb.Bundle, scopes nb.ScopeResolver) nb.ActionHost {
	return &notebookActionHost{providerHost: parent, bundle: bundle, scopes: scopes}
}

// Notebook returns the owner-bound facade. Admission origin is resolved at
// this trusted boundary: an invocation inside an active Run (identity.RunID)
// is agent work bound to that Session's canonical workspace scope; a direct
// authenticated UI/embedded/headless call is a human edit on Home. Unarmed or
// unauthenticated compositions fail closed.
func (host *notebookActionHost) Notebook() (nb.ScopedActions, error) {
	if host == nil || host.providerHost == nil || host.bundle == nil || host.scopes == nil {
		return nil, &nb.Error{Code: nb.CodeCapabilityUnavailable, Message: "notebook facade is not armed"}
	}
	if err := host.active(); err != nil {
		return nil, &nb.Error{Code: nb.CodeCapabilityUnavailable, Message: err.Error()}
	}
	identity := host.identity
	if identity.ID == "" {
		return nil, action.ErrUnauthenticated
	}
	if identity.RunID != "" {
		// Agent admission: bound to the run's session workspace scope. The
		// session ID is the authenticated one — a claimed session in JSON
		// never reaches this point.
		scope, err := host.scopes.ForSession(context.Background(), identity.SessionID)
		if err != nil {
			return nil, err
		}
		return host.bundle.Actions(scope, nb.Actor{Kind: nb.ActorAgent, Ref: "run:" + identity.RunID}), nil
	}
	return host.bundle.Actions(host.scopes.Home(), nb.Actor{Kind: nb.ActorHuman, Ref: "peer:" + identity.ID}), nil
}
