package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/bundles"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
)

type toolkitMCPPreparedOwners struct {
	view                   bundlecontract.View
	backend                *runtime.MCPBackend
	store                  storage.BundleStore
	currentUserMCPConfigs  func(context.Context) ([]runtime.MCPServerConfig, error)
	resourceBridgeCompiled bool
	configs                []runtime.MCPServerConfig
	afterPublish           func(context.Context, []runtime.MCPServerConfig) error
}

func (o *toolkitMCPPreparedOwners) View() bundlecontract.View { return o.view }
func (o *toolkitMCPPreparedOwners) Publish() {
	if o != nil && o.backend != nil {
		o.backend.ReplaceServers(o.configs)
	}
}
func (*toolkitMCPPreparedOwners) Close(context.Context) error { return nil }

func (o *toolkitMCPPreparedOwners) BeforeCommit(ctx context.Context) error {
	if o == nil || o.backend == nil || o.currentUserMCPConfigs == nil {
		return nil
	}
	userConfigs, err := o.currentUserMCPConfigs(ctx)
	if err != nil {
		return err
	}
	configs, err := mergeToolkitMCPConfigs(ctx, o.view, o.store, userConfigs, o.resourceBridgeCompiled)
	if err != nil {
		return err
	}
	o.configs = configs
	return nil
}

func (o *toolkitMCPPreparedOwners) AfterPublish(ctx context.Context) error {
	if o == nil || o.backend == nil || o.afterPublish == nil {
		return nil
	}
	return o.afterPublish(ctx, o.backend.ConfiguredServers())
}

func prepareToolkitMCPOwners(ctx context.Context, view bundlecontract.View, store storage.BundleStore,
	backend *runtime.MCPBackend, currentUserMCPConfigs func(context.Context) ([]runtime.MCPServerConfig, error), resourceBridgeCompiled bool,
) (bundles.PreparedOwners, error) {
	if backend == nil {
		return noopToolkitOwners{view: view}, nil
	}
	var userConfigs []runtime.MCPServerConfig
	if currentUserMCPConfigs != nil {
		var err error
		userConfigs, err = currentUserMCPConfigs(ctx)
		if err != nil {
			return nil, err
		}
	}
	merged, err := mergeToolkitMCPConfigs(ctx, view, store, userConfigs, resourceBridgeCompiled)
	if err != nil {
		return nil, err
	}
	return &toolkitMCPPreparedOwners{
		view: view, backend: backend, store: store, currentUserMCPConfigs: currentUserMCPConfigs,
		resourceBridgeCompiled: resourceBridgeCompiled, configs: merged,
	}, nil
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

func validateToolkitMCPDiscovery(ctx context.Context, manager *bundles.Manager, store storage.BundleStore, config runtime.MCPServerConfig) error {
	if manager == nil || store == nil || config.ToolkitRef == nil || config.Enabled == nil || !*config.Enabled {
		return runtime.ErrBundleMCPNotCaptured
	}
	ref := *config.ToolkitRef
	if ref.Kind != bundlecontract.MCP || config.ToolkitConsentDigest == "" || config.ToolkitConsentRevision <= 0 {
		return runtime.ErrBundleMCPNotCaptured
	}
	handle, err := manager.PinCurrent()
	if err != nil {
		return err
	}
	defer handle.Release()
	view := handle.View()
	identity := view.Identity()
	if config.ToolkitCatalogEpoch != identity.Epoch {
		return runtime.ErrBundleMCPNotCaptured
	}
	state, err := manager.ListInstallations(ctx)
	if err != nil {
		return err
	}
	if state.Catalog != identity {
		return errors.New("toolkits: catalog_recovery_required")
	}
	projectedID, err := bundlecontract.MCPInstanceID(ref.InstallID, ref.LocalID)
	if err != nil {
		return runtime.ErrBundleMCPNotCaptured
	}
	if config.Name != projectedID {
		return runtime.ErrBundleMCPNotCaptured
	}
	record, found := view.Lookup(bundlecontract.MCP, projectedID)
	if !found || record.Ref != ref || record.MCP == nil || record.Release.ReleaseDigest != config.ToolkitReleaseDigest ||
		record.ExecutableDigest != config.ToolkitDeclarationDigest {
		return runtime.ErrBundleMCPNotCaptured
	}
	consent, found, err := store.ReadBundleConsent(ctx, ref.InstallID, ref.Kind, ref.LocalID)
	if err != nil {
		return err
	}
	if !found || !consent.Allowed || consent.Revision != config.ToolkitConsentRevision ||
		consent.DeclarationDigest != config.ToolkitConsentDigest {
		return runtime.ErrBundleMCPNotCaptured
	}
	expectedConsent, err := bundlecontract.ExecutableConsentDigest(ref.InstallID, record.Release.ReleaseDigest,
		record.ExecutableDigest, identity.ConfigRevision)
	if err != nil || expectedConsent != consent.DeclarationDigest {
		return runtime.ErrBundleMCPNotCaptured
	}
	return manager.CheckLive(ctx, bundlecontract.CaptureCheck{Catalog: identity, Executables: []bundlecontract.ExecutableRef{{
		ComponentRef: ref, DeclarationDigest: record.ExecutableDigest,
		ConsentDigest: consent.DeclarationDigest, ConsentRevision: consent.Revision,
	}}})
}

type toolkitSettingsDrain interface {
	BeginDrain(context.Context, string) error
	WaitCatalogIdle(context.Context, string) error
	EndDrain(string)
}

func beginToolkitDrainWhenAvailable(ctx context.Context, operationID string, begin func(context.Context, string) error) error {
	for {
		err := begin(ctx, operationID)
		if !errors.Is(err, runtime.ErrCatalogUpdatePending) {
			return err
		}
		// Settings and Toolkit publications share one exclusive Runtime gate.
		// Wait for the current owner instead of dropping a settings update.
		timer := time.NewTimer(40 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func replaceMCPServersWithDrain(service toolkitSettingsDrain, backend *runtime.MCPBackend,
	buildConfigs func(context.Context) ([]runtime.MCPServerConfig, error), afterReplace func([]runtime.MCPServerConfig),
) (func(), error) {
	if service == nil || backend == nil {
		return nil, runtime.ErrBundleDrainUnavailable
	}
	if buildConfigs == nil {
		return nil, errors.New("app: MCP settings config builder is unavailable")
	}
	operationID := fmt.Sprintf("settings-mcp-%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := beginToolkitDrainWhenAvailable(ctx, operationID, service.BeginDrain); err != nil {
		return nil, err
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { service.EndDrain(operationID) }) }
	draining := true
	defer func() {
		if draining {
			release()
		}
	}()
	if err := service.WaitCatalogIdle(ctx, operationID); err != nil {
		return nil, err
	}
	// Read settings and pin the active Toolkit view only after acquiring the
	// gate; either may have changed while this settings update was waiting.
	configs, err := buildConfigs(ctx)
	if err != nil {
		return nil, err
	}
	if !sameMCPRuntimeConfigs(configs, backend.ConfiguredServers()) {
		backend.ReplaceServers(configs)
	}
	if afterReplace != nil {
		afterReplace(backend.ConfiguredServers())
	}
	draining = false
	return release, nil
}

type noopToolkitOwners struct{ view bundlecontract.View }

func (o noopToolkitOwners) View() bundlecontract.View { return o.view }
func (noopToolkitOwners) Publish()                    {}
func (noopToolkitOwners) Close(context.Context) error { return nil }

// toolkitDrainGate adapts the established Runtime Service to the coordinator
// publication protocol without coupling the bundle manager to Runtime.
type toolkitDrainGate struct{ service func() *runtime.Service }

func (g toolkitDrainGate) BeginDrain(ctx context.Context, operationID string) error {
	return beginToolkitDrainWhenAvailable(ctx, operationID, func(ctx context.Context, id string) error {
		if g.service == nil || g.service() == nil {
			return runtime.ErrBundleDrainUnavailable
		}
		return g.service().BeginDrain(ctx, id)
	})
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
