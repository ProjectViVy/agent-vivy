package actionhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	action "agent-vivy/sdk/port/controlaction"
)

const cognitiveModuleOwner = "vivy/diva-cognitive"

// cognitiveActionHost is deliberately private, like the mask facade. The
// ordinary control-action Host remains the only public provider facade; this
// wrapper is created only for the compiler-bound T1 cognitive owner and
// carries no raw Garden capability. Session binding is enforced here, at the
// trusted boundary, so a forged session_id inside provider JSON can never
// reach the armed dispatcher.
type cognitiveActionHost struct {
	*providerHost
	resolve      cognitivecontract.DispatcherProvider
	sessionCheck func(context.Context, domain.SessionID) error
}

func newCognitiveActionHost(parent *providerHost, resolve cognitivecontract.DispatcherProvider, check func(context.Context, domain.SessionID) error) cognitivecontract.ActionHost {
	return &cognitiveActionHost{providerHost: parent, resolve: resolve, sessionCheck: check}
}

// Cognitive hands the armed dispatcher to the sealed cognitive providers,
// wrapped in the session guard bound to this invocation's authenticated
// identity. An unarmed composition fails closed.
func (host *cognitiveActionHost) Cognitive() (cognitivecontract.Dispatcher, error) {
	if host == nil || host.providerHost == nil {
		return nil, cognitivecontract.ErrUnarmed
	}
	if err := host.active(); err != nil {
		return nil, err
	}
	if host.resolve == nil {
		return nil, cognitivecontract.ErrUnarmed
	}
	dispatcher := host.resolve.Dispatcher()
	if dispatcher == nil {
		return nil, cognitivecontract.ErrUnarmed
	}
	return &sessionGuardDispatcher{
		session: host.identity.SessionID,
		check:   host.sessionCheck,
		next:    dispatcher,
	}, nil
}

// sessionGuardDispatcher admits a dispatch only when the input's session_id
// equals the session the transport bound to this peer AND that session still
// resolves in the host-side session registry. The claimed session is never
// trusted on its own.
type sessionGuardDispatcher struct {
	session string
	check   func(context.Context, domain.SessionID) error
	next    cognitivecontract.Dispatcher
}

type sessionClaim struct {
	SessionID string `json:"session_id"`
}

func (d *sessionGuardDispatcher) Invoke(ctx context.Context, actionID string, input json.RawMessage) (json.RawMessage, error) {
	var claim sessionClaim
	if err := json.Unmarshal(input, &claim); err != nil {
		return nil, fmt.Errorf("%w: malformed cognitive input", action.ErrInvalidInput)
	}
	if claim.SessionID == "" || d.session == "" || claim.SessionID != d.session {
		return nil, fmt.Errorf("%w: session is not bound to caller", action.ErrGrantDenied)
	}
	if d.check == nil {
		return nil, cognitivecontract.ErrUnarmed
	}
	if err := d.check(ctx, domain.SessionID(claim.SessionID)); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("%w: session is not bound to caller", action.ErrGrantDenied)
		}
		return nil, err
	}
	return d.next.Invoke(ctx, actionID, input)
}
