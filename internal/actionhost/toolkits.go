package actionhost

import (
	"context"
	"errors"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/bundles"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/toolkitcontract"
)

const toolkitModuleOwner = "vivy/toolkits"

// toolkitActionHost is private to the sealed T1 owner. The generic action
// Host and public SDK Port receive no Manager or storage capability.
type toolkitActionHost struct {
	*providerHost
	manager      toolkitcontract.Manager
	capabilities toolkitcontract.Capabilities
}

func newToolkitActionHost(parent *providerHost, manager toolkitcontract.Manager, capabilities toolkitcontract.Capabilities) toolkitcontract.ActionHost {
	return &toolkitActionHost{providerHost: parent, manager: manager, capabilities: cloneToolkitCapabilities(capabilities)}
}

func cloneToolkitCapabilities(in toolkitcontract.Capabilities) toolkitcontract.Capabilities {
	out := toolkitcontract.Capabilities{Owners: make(map[bundlecontract.ComponentKind]bool, len(in.Owners))}
	for kind, available := range in.Owners {
		out.Owners[kind] = available
	}
	return out
}

func (host *toolkitActionHost) Capabilities() toolkitcontract.Capabilities {
	return cloneToolkitCapabilities(host.capabilities)
}
func (host *toolkitActionHost) RunBound() bool {
	return host == nil || host.providerHost == nil || host.identity.RunID != ""
}
func (host *toolkitActionHost) HumanSubject() (string, error) {
	if host == nil || host.providerHost == nil {
		return "", toolkitcontract.NewError(toolkitcontract.CodeAuthorizationDenied, nil)
	}
	if err := host.active(); err != nil {
		return "", toolkitcontract.NewError(toolkitcontract.CodeUnavailable, err)
	}
	if host.identity.ID == "" || host.identity.RunID != "" {
		return "", toolkitcontract.NewError(toolkitcontract.CodeAuthorizationDenied, nil)
	}
	return host.identity.ID, nil
}
func (host *toolkitActionHost) managerContext(ctx context.Context) (context.Context, func(), error) {
	if host == nil || host.providerHost == nil {
		return nil, func() {}, toolkitcontract.NewError(toolkitcontract.CodeUnavailable, nil)
	}
	if err := host.active(); err != nil {
		return nil, func() {}, toolkitcontract.NewError(toolkitcontract.CodeUnavailable, err)
	}
	if host.manager == nil {
		return nil, func() {}, toolkitcontract.NewError(toolkitcontract.CodeUnavailable, errors.New("manager unavailable"))
	}
	managed, release := host.scopedContext(ctx)
	return managed, release, nil
}
func (host *toolkitActionHost) ListInstallations(ctx context.Context) (storage.BundleState, error) {
	managed, release, err := host.managerContext(ctx)
	if err != nil {
		return storage.BundleState{}, err
	}
	defer release()
	return host.manager.ListInstallations(managed)
}
func (host *toolkitActionHost) InspectInstallation(ctx context.Context, id string) (bundles.InstallationInspection, error) {
	managed, release, err := host.managerContext(ctx)
	if err != nil {
		return bundles.InstallationInspection{}, err
	}
	defer release()
	return host.manager.InspectInstallation(managed, id)
}
func (host *toolkitActionHost) ReadComponent(ctx context.Context, id string, kind bundlecontract.ComponentKind, localID, digest string, offset, maxBytes int) (bundles.ComponentPage, error) {
	managed, release, err := host.managerContext(ctx)
	if err != nil {
		return bundles.ComponentPage{}, err
	}
	defer release()
	return host.manager.ReadComponent(managed, id, kind, localID, digest, offset, maxBytes)
}
func (host *toolkitActionHost) GetOperation(ctx context.Context, id string) (bundlecontract.Operation, error) {
	managed, release, err := host.managerContext(ctx)
	if err != nil {
		return bundlecontract.Operation{}, err
	}
	defer release()
	return host.manager.GetOperation(managed, id)
}
func (host *toolkitActionHost) Preview(ctx context.Context, in bundles.PreviewRequest) (bundles.PreparedSource, error) {
	managed, release, err := host.managerContext(ctx)
	if err != nil {
		return bundles.PreparedSource{}, err
	}
	defer release()
	return host.manager.Preview(managed, in)
}
func (host *toolkitActionHost) Install(ctx context.Context, in bundles.InstallRequest) (bundlecontract.Operation, error) {
	managed, release, err := host.managerContext(ctx)
	if err != nil {
		return bundlecontract.Operation{}, err
	}
	defer release()
	return host.manager.Install(managed, in)
}
func (host *toolkitActionHost) Prepare(ctx context.Context, in bundles.PrepareRequest) (bundles.PreparedCatalog, error) {
	managed, release, err := host.managerContext(ctx)
	if err != nil {
		return bundles.PreparedCatalog{}, err
	}
	defer release()
	return host.manager.Prepare(managed, in)
}
func (host *toolkitActionHost) PreparedCandidate(id string) (string, bundlecontract.CatalogIdentity, bool) {
	if host == nil || host.providerHost == nil || host.active() != nil || host.manager == nil {
		return "", bundlecontract.CatalogIdentity{}, false
	}
	return host.manager.PreparedCandidate(id)
}
func (host *toolkitActionHost) Publish(ctx context.Context, id string) (bundlecontract.Operation, error) {
	managed, release, err := host.managerContext(ctx)
	if err != nil {
		return bundlecontract.Operation{}, err
	}
	defer release()
	return host.manager.Publish(managed, id)
}
func (host *toolkitActionHost) CancelOperation(ctx context.Context, id string, revision int64) (bundlecontract.Operation, error) {
	managed, release, err := host.managerContext(ctx)
	if err != nil {
		return bundlecontract.Operation{}, err
	}
	defer release()
	return host.manager.CancelOperation(managed, id, revision)
}
func (host *toolkitActionHost) CancelPreview(id string) bool {
	if host == nil || host.providerHost == nil || host.active() != nil || host.manager == nil {
		return false
	}
	return host.manager.CancelPreview(id)
}
func (host *toolkitActionHost) SetExecutableConsent(ctx context.Context, in bundles.ExecutableConsentRequest) (bundlecontract.Operation, error) {
	if _, err := host.HumanSubject(); err != nil {
		return bundlecontract.Operation{}, err
	}
	managed, release, err := host.managerContext(ctx)
	if err != nil {
		return bundlecontract.Operation{}, err
	}
	defer release()
	return host.manager.SetExecutableConsent(managed, in)
}
func (host *toolkitActionHost) SetBindings(ctx context.Context, in bundles.BindingUpdateRequest) (bundlecontract.Operation, error) {
	managed, release, err := host.managerContext(ctx)
	if err != nil {
		return bundlecontract.Operation{}, err
	}
	defer release()
	return host.manager.SetBindings(managed, in)
}

var _ toolkitcontract.ActionHost = (*toolkitActionHost)(nil)
