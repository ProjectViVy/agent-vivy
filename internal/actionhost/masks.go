package actionhost

import (
	"context"
	"errors"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/maskcontract"
)

const maskModuleOwner = "vivy/masks"

// maskActionHost is deliberately private. The ordinary control-action Host
// remains the only public provider facade; this wrapper is created only for
// the compiler-bound T1 mask owner and carries no raw storage capability.
type maskActionHost struct {
	*providerHost
	manager maskcontract.Manager
}

func newMaskActionHost(parent *providerHost, manager maskcontract.Manager) maskcontract.ActionHost {
	return &maskActionHost{providerHost: parent, manager: manager}
}

func (host *maskActionHost) managerReady() error {
	if host == nil || host.providerHost == nil {
		return maskcontract.NewError(maskcontract.CodeUnavailable, errors.New("mask action host is unavailable"))
	}
	if err := host.active(); err != nil {
		return err
	}
	if host.manager == nil {
		return maskcontract.NewError(maskcontract.CodeUnavailable, errors.New("mask manager is unavailable"))
	}
	return nil
}

// managerContext never trusts a Provider-supplied context as the authority.
// scopedContext roots the call in the Host-owned deadline/cancellation while
// still observing a child context the Provider uses for its own work.
func (host *maskActionHost) managerContext(ctx context.Context) (context.Context, func(), error) {
	if err := host.managerReady(); err != nil {
		return nil, func() {}, err
	}
	managed, release := host.scopedContext(ctx)
	return managed, release, nil
}

func (host *maskActionHost) sessionID(id domain.SessionID) (domain.SessionID, error) {
	if err := host.managerReady(); err != nil {
		return "", err
	}
	if id == "" || host.identity.SessionID == "" || string(id) != host.identity.SessionID {
		return "", maskcontract.NewError(maskcontract.CodeAuthorizationDenied, errors.New("session is not bound to caller"))
	}
	if err := maskcontract.ValidateSessionID(id); err != nil {
		return "", maskcontract.NewError(maskcontract.CodeInvalidMask, err)
	}
	return id, nil
}

func (host *maskActionHost) ListMasks(ctx context.Context, in maskcontract.ListRequest) (maskcontract.Page, error) {
	managed, release, err := host.managerContext(ctx)
	if err != nil {
		return maskcontract.Page{}, err
	}
	defer release()
	return host.manager.ListMasks(managed, in)
}

func (host *maskActionHost) GetMask(ctx context.Context, in maskcontract.GetRequest) (maskcontract.Definition, error) {
	managed, release, err := host.managerContext(ctx)
	if err != nil {
		return maskcontract.Definition{}, err
	}
	defer release()
	return host.manager.GetMask(managed, in)
}

func (host *maskActionHost) CreateMask(ctx context.Context, in maskcontract.CreateRequest) (maskcontract.Definition, error) {
	managed, release, err := host.managerContext(ctx)
	if err != nil {
		return maskcontract.Definition{}, err
	}
	defer release()
	return host.manager.CreateMask(managed, in)
}

func (host *maskActionHost) UpdateMask(ctx context.Context, in maskcontract.UpdateRequest) (maskcontract.Definition, error) {
	managed, release, err := host.managerContext(ctx)
	if err != nil {
		return maskcontract.Definition{}, err
	}
	defer release()
	return host.manager.UpdateMask(managed, in)
}

func (host *maskActionHost) DeleteMask(ctx context.Context, in maskcontract.DeleteRequest) (maskcontract.DeleteResult, error) {
	managed, release, err := host.managerContext(ctx)
	if err != nil {
		return maskcontract.DeleteResult{}, err
	}
	defer release()
	return host.manager.DeleteMask(managed, in)
}

func (host *maskActionHost) GetMaskSelection(ctx context.Context, in maskcontract.SelectionRequest) (maskcontract.SelectionView, error) {
	managed, release, err := host.managerContext(ctx)
	if err != nil {
		return maskcontract.SelectionView{}, err
	}
	defer release()
	sessionID, err := host.sessionID(in.SessionID)
	if err != nil {
		return maskcontract.SelectionView{}, err
	}
	in.SessionID = sessionID
	return host.manager.GetMaskSelection(managed, in)
}

func (host *maskActionHost) SetMaskSelection(ctx context.Context, in maskcontract.SetSelectionRequest) (maskcontract.SelectionView, error) {
	managed, release, err := host.managerContext(ctx)
	if err != nil {
		return maskcontract.SelectionView{}, err
	}
	defer release()
	sessionID, err := host.sessionID(in.SessionID)
	if err != nil {
		return maskcontract.SelectionView{}, err
	}
	in.SessionID = sessionID
	return host.manager.SetMaskSelection(managed, in)
}
