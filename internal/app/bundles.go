package app

import (
	"context"
	"fmt"
	"time"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/bundles"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
)

type toolkitMCPPreparedOwners struct {
	view    bundlecontract.View
	backend *runtime.MCPBackend
	configs []runtime.MCPServerConfig
}

func (o *toolkitMCPPreparedOwners) View() bundlecontract.View { return o.view }
func (o *toolkitMCPPreparedOwners) Publish() {
	if o != nil && o.backend != nil {
		o.backend.ReplaceServers(o.configs)
	}
}
func (*toolkitMCPPreparedOwners) Close(context.Context) error { return nil }

func prepareToolkitMCPOwners(ctx context.Context, view bundlecontract.View, store storage.BundleStore,
	backend *runtime.MCPBackend, userConfigs []runtime.MCPServerConfig, resourceBridgeCompiled bool,
) (bundles.PreparedOwners, error) {
	if backend == nil {
		return noopToolkitOwners{view: view}, nil
	}
	merged, err := mergeToolkitMCPConfigs(ctx, view, store, userConfigs, resourceBridgeCompiled)
	if err != nil {
		return nil, err
	}
	return &toolkitMCPPreparedOwners{view: view, backend: backend, configs: merged}, nil
}

func mergeToolkitMCPConfigs(ctx context.Context, view bundlecontract.View, store storage.BundleStore,
	userConfigs []runtime.MCPServerConfig, resourceBridgeCompiled bool,
) ([]runtime.MCPServerConfig, error) {
	consents := make(map[bundlecontract.ComponentRef]runtime.BundleMCPConsent)
	for _, record := range view.Components() {
		if record.Ref.Kind != bundlecontract.MCP {
			continue
		}
		consent, found, err := store.ReadBundleConsent(ctx, record.Ref.InstallID, record.Ref.Kind, record.Ref.LocalID)
		if err != nil {
			return nil, err
		}
		if found {
			consents[record.Ref] = runtime.BundleMCPConsent{
				Allowed: consent.Allowed, Digest: consent.DeclarationDigest, Revision: consent.Revision,
			}
		}
	}
	bundled, err := runtime.PrepareBundleMCPConfigs(view, consents, nil, resourceBridgeCompiled)
	if err != nil {
		return nil, err
	}
	return runtime.MergeMCPConfigs(userConfigs, bundled)
}

func replaceMCPServersWithDrain(service *runtime.Service, backend *runtime.MCPBackend, configs []runtime.MCPServerConfig) error {
	if service == nil || backend == nil {
		return runtime.ErrBundleDrainUnavailable
	}
	operationID := fmt.Sprintf("settings-mcp-%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := service.BeginDrain(ctx, operationID); err != nil {
		return err
	}
	defer service.EndDrain(operationID)
	if err := service.WaitCatalogIdle(ctx, operationID); err != nil {
		return err
	}
	backend.ReplaceServers(configs)
	return nil
}

type noopToolkitOwners struct{ view bundlecontract.View }

func (o noopToolkitOwners) View() bundlecontract.View { return o.view }
func (noopToolkitOwners) Publish()                    {}
func (noopToolkitOwners) Close(context.Context) error { return nil }

// toolkitDrainGate adapts the established Runtime Service to the coordinator
// publication protocol without coupling the bundle manager to Runtime.
type toolkitDrainGate struct{ service func() *runtime.Service }

func (g toolkitDrainGate) BeginDrain(ctx context.Context, operationID string) error {
	if g.service == nil || g.service() == nil {
		return runtime.ErrBundleDrainUnavailable
	}
	return g.service().BeginDrain(ctx, operationID)
}
func (g toolkitDrainGate) WaitIdle(ctx context.Context, operationID string) error {
	if g.service == nil || g.service() == nil {
		return runtime.ErrBundleDrainUnavailable
	}
	return g.service().WaitCatalogIdle(ctx, operationID)
}
func (g toolkitDrainGate) EndDrain(operationID string) {
	if g.service != nil && g.service() != nil {
		g.service().EndDrain(operationID)
	}
}

// toolkitRuntimeBridge converts the Manager's concrete handle return into the
// Runtime-owned narrow interface. A captured View remains pinned until Runtime
// reaches its terminal cleanup path.
type toolkitRuntimeBridge struct{ manager *bundles.Manager }

type toolkitViewHandle struct{ handle *bundles.ViewHandle }

func (h toolkitViewHandle) View() bundlecontract.View {
	if h.handle == nil {
		return bundlecontract.View{}
	}
	return h.handle.View()
}
func (h toolkitViewHandle) Retain() runtime.ToolkitViewHandle {
	if h.handle == nil {
		return nil
	}
	retained := h.handle.Retain()
	if retained == nil {
		return nil
	}
	return toolkitViewHandle{handle: retained}
}
func (h toolkitViewHandle) Release() {
	if h.handle != nil {
		h.handle.Release()
	}
}

func (b toolkitRuntimeBridge) CaptureRun(ctx context.Context, runID domain.RunID) (runtime.ToolkitViewHandle, storage.RunBundleSnapshot, bundlecontract.CaptureCheck, error) {
	handle, snapshot, check, err := b.manager.CaptureRun(ctx, runID)
	if err != nil {
		return nil, snapshot, check, err
	}
	return toolkitViewHandle{handle: handle}, snapshot, check, nil
}
func (b toolkitRuntimeBridge) RestoreRun(ctx context.Context, snapshot storage.RunBundleSnapshot) (runtime.ToolkitViewHandle, bundlecontract.CaptureCheck, error) {
	handle, check, err := b.manager.RestoreRun(ctx, snapshot)
	if err != nil {
		return nil, check, err
	}
	return toolkitViewHandle{handle: handle}, check, nil
}
func (b toolkitRuntimeBridge) CheckLive(ctx context.Context, check bundlecontract.CaptureCheck) error {
	return b.manager.CheckLive(ctx, check)
}

var _ bundles.AdmissionGate = toolkitDrainGate{}
var _ runtime.ToolkitRuntime = toolkitRuntimeBridge{}
