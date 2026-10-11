package bundles

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

type AdmissionGate interface {
	BeginDrain(context.Context, string) error
	WaitIdle(context.Context, string) error
	EndDrain(string)
}
type PreparedOwners interface {
	View() bundlecontract.View
	Publish()
	Close(context.Context) error
}
type PrepareOwners func(context.Context, bundlecontract.View) (PreparedOwners, error)
type MCPRuntimeStatus struct {
	Name        string
	State       string
	Initialized bool
	ToolCount   int
}
type Config struct {
	StateRoot      string
	Limits         bundlecontract.Limits
	Capabilities   Capabilities
	Store          storage.BundleStore
	Gate           AdmissionGate
	PrepareOwners  PrepareOwners
	MCPStatuses    func() []MCPRuntimeStatus
	LeaseOwner     string
	GitEnvironment GitEnvironment
}
type PreviewRequest struct {
	OperationID string
	Expected    bundlecontract.CatalogIdentity
	Source      bundlecontract.SourceSpec
}
type PreparedSource struct {
	OperationID   string                          `json:"operation_id"`
	InstallID     string                          `json:"install_id"`
	ReleaseDigest string                          `json:"release_digest"`
	Source        bundlecontract.SourceLock       `json:"source"`
	Inventory     []bundlecontract.InventoryEntry `json:"inventory"`
	Manifest      bundlecontract.Manifest         `json:"manifest"`
	artifact      Artifact
	requested     bundlecontract.SourceSpec
}
type InstallRequest struct {
	OperationID           string
	Expected              bundlecontract.CatalogIdentity
	PreviewOperationID    string
	ExpectedReleaseDigest string
}
type PrepareRequest struct {
	OperationID string
	Expected    bundlecontract.CatalogIdentity
	Mutation    CatalogMutation
}
type CatalogMutation struct {
	Kind               string
	InstallID          string
	ReleaseDigest      string
	PreviewOperationID string
	ExpectedRevision   int64
}
type ExecutableConsentRequest struct {
	OperationID               string
	Expected                  bundlecontract.CatalogIdentity
	Ref                       bundlecontract.ComponentRef
	ExpectedRevision          int64
	ExpectedReleaseDigest     string
	ExpectedDeclarationDigest string
	Subject                   string
	Allowed                   bool
}
type BindingUpdateRequest struct {
	OperationID            string
	Expected               bundlecontract.CatalogIdentity
	ExpectedConfigRevision int64
	InstallID              string
	Kind                   bundlecontract.ComponentKind
	LocalID                string
	Bindings               []bundlecontract.BindingRef
	Subject                string
}
type HookConsentRequest = ExecutableConsentRequest
type InstallationInspection struct {
	Installation storage.BundleInstallation  `json:"installation"`
	Release      bundlecontract.ReleaseRef   `json:"release"`
	Releases     []bundlecontract.ReleaseRef `json:"releases"`
	Manifest     *bundlecontract.Manifest    `json:"manifest,omitempty"`
	Consents     []storage.BundleConsent     `json:"consents"`
	Executables  []ExecutableInspection      `json:"executables"`
	Bindings     []storage.ToolkitBinding    `json:"bindings"`
}
type ExecutableInspection struct {
	Kind               bundlecontract.ComponentKind `json:"kind"`
	LocalID            string                       `json:"local_id"`
	ReleaseDigest      string                       `json:"release_digest"`
	DeclarationDigest  string                       `json:"declaration_digest"`
	ConsentCurrent     bool                         `json:"consent_current"`
	ConsentAllowed     bool                         `json:"consent_allowed"`
	ConsentRevision    int64                        `json:"consent_revision"`
	MissingBindings    []string                     `json:"missing_bindings"`
	RuntimeState       string                       `json:"runtime_state,omitempty"`
	RuntimeInitialized bool                         `json:"runtime_initialized,omitempty"`
	RuntimeToolCount   int                          `json:"runtime_tool_count,omitempty"`
}
type ComponentPage struct {
	InstallID     string                       `json:"install_id"`
	Kind          bundlecontract.ComponentKind `json:"kind"`
	LocalID       string                       `json:"local_id"`
	ReleaseDigest string                       `json:"release_digest"`
	Digest        string                       `json:"digest"`
	Offset        int                          `json:"offset"`
	NextOffset    int                          `json:"next_offset"`
	TotalBytes    int                          `json:"total_bytes"`
	Content       string                       `json:"content"`
}
type PreparedCatalog struct {
	OperationID     string
	Expected        bundlecontract.CatalogIdentity
	CandidateDigest string
	InputDigest     string
	Operation       bundlecontract.Operation
	Changes         []bundlecontract.ComponentChange
	Releases        []bundlecontract.ReleaseRef
	newReleases     []bundlecontract.ReleaseRef
	owners          PreparedOwners
	installations   []storage.InstallationState
	mutation        CatalogMutation
}
type viewState struct {
	manager *Manager
	view    bundlecontract.View
	owners  PreparedOwners
	refs    int
	retired bool
	closed  bool
}
type Manager struct {
	cfg              Config
	mu               sync.Mutex
	gcMu             sync.Mutex
	current          *viewState
	previews         map[string]PreparedSource
	previewUse       map[string]string
	prepared         map[string]PreparedCatalog
	publishing       string
	publishInFlight  string
	recoveryRequired bool
	recoveryDrainID  string
	closed           bool
}
type ViewHandle struct {
	state   *viewState
	release func()
	once    sync.Once
}

func (h *ViewHandle) View() bundlecontract.View {
	if h == nil || h.state == nil {
		return bundlecontract.View{}
	}
	return h.state.view
}
func (h *ViewHandle) Release() {
	if h != nil {
		h.once.Do(func() {
			if h.release != nil {
				h.release()
			}
		})
	}
}

// Retain creates an independent lease over the same immutable projection.
// A child run must not depend on its parent's lease lifetime.
func (h *ViewHandle) Retain() *ViewHandle {
	if h == nil || h.state == nil || h.state.manager == nil {
		return nil
	}
	m := h.state.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || h.state.closed || h.state.refs <= 0 {
		return nil
	}
	h.state.refs++
	return m.handleLocked(h.state)
}

func (m *Manager) handleLocked(state *viewState) *ViewHandle {
	return &ViewHandle{state: state, release: func() {
		m.mu.Lock()
		var owners PreparedOwners
		if state.refs > 0 {
			state.refs--
		}
		if state.retired && state.refs == 0 && !state.closed {
			state.closed = true
			owners = state.owners
		}
		m.mu.Unlock()
		if owners != nil {
			_ = owners.Close(context.Background())
		}
	}}
}

// PinCurrent returns an independent lease over the currently published
// immutable projection. Management reads use this rather than keeping a
// pointer to a catalog that may be replaced during publication.
func (m *Manager) PinCurrent() (*ViewHandle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.current == nil || m.current.closed {
		return nil, errors.New("toolkits: current view unavailable")
	}
	m.current.refs++
	return m.handleLocked(m.current), nil
}

// ListInstallations returns the durable redacted installation projection.
// The source lock contains only the canonical origin metadata captured during
// acquisition; it never contains credential values.
func (m *Manager) ListInstallations(ctx context.Context) (storage.BundleState, error) {
	if m == nil || m.cfg.Store == nil {
		return storage.BundleState{}, errors.New("toolkits: manager unavailable")
	}
	return m.cfg.Store.ReadBundleState(ctx)
}

// InspectInstallation returns the bounded native manifest and exact current
// Hook/MCP consent rows without reading component bodies or secret values.
func (m *Manager) InspectInstallation(ctx context.Context, installID string) (InstallationInspection, error) {
	if m == nil || m.cfg.Store == nil || !bundlecontract.ValidInstallID(installID) {
		return InstallationInspection{}, bundlecontract.Invalid("install_id")
	}
	state, err := m.cfg.Store.ReadBundleState(ctx)
	if err != nil {
		return InstallationInspection{}, err
	}
	var installation storage.BundleInstallation
	found := false
	for _, item := range state.Installations {
		if item.InstallID == installID {
			installation, found = item, true
			break
		}
	}
	if !found {
		return InstallationInspection{}, errors.New("toolkits: not_found")
	}
	inspection := InstallationInspection{Installation: installation, Consents: []storage.BundleConsent{}, Executables: []ExecutableInspection{}, Bindings: []storage.ToolkitBinding{}, Releases: []bundlecontract.ReleaseRef{}}
	if installation.Tombstone {
		return inspection, nil
	}
	release, ok := findRelease(state.Releases, installID, installation.ReleaseDigest)
	if !ok {
		return InstallationInspection{}, errors.New("toolkits: artifact_missing")
	}
	artifact, err := OpenArtifact(ctx, m.cfg.StateRoot, release.ReleaseDigest)
	if err != nil {
		return InstallationInspection{}, err
	}
	all := Capabilities{Owners: map[bundlecontract.ComponentKind]bool{
		bundlecontract.Skill: true, bundlecontract.Mask: true, bundlecontract.Rule: true,
		bundlecontract.Hook: true, bundlecontract.MCP: true,
	}, Limits: m.cfg.Limits}
	if err := ValidateArtifact(ctx, artifact, all); err != nil {
		return InstallationInspection{}, err
	}
	manifest := artifact.Manifest()
	inspection.Release = release
	for _, candidate := range state.Releases {
		if candidate.InstallID == installID {
			inspection.Releases = append(inspection.Releases, candidate)
		}
	}
	inspection.Manifest = &manifest
	for _, binding := range state.Bindings {
		if binding.InstallID == installID {
			inspection.Bindings = append(inspection.Bindings, binding)
		}
	}
	for _, hook := range manifest.Components.Hooks {
		digest, err := operationDigest(hook)
		if err != nil {
			return InstallationInspection{}, err
		}
		inspection.Executables = append(inspection.Executables, executableInspection(bundlecontract.Hook, hook.ID, release.ReleaseDigest, digest, hookBindings(hook, componentBindingRefs(state.Bindings, installID, bundlecontract.Hook, hook.ID)), state.Catalog.ConfigRevision))
	}
	for _, server := range manifest.Components.MCP {
		digest, err := operationDigest(server)
		if err != nil {
			return InstallationInspection{}, err
		}
		entry := executableInspection(bundlecontract.MCP, server.ID, release.ReleaseDigest, digest, mcpBindings(server, componentBindingRefs(state.Bindings, installID, bundlecontract.MCP, server.ID)), state.Catalog.ConfigRevision)
		entry.RuntimeState = "inactive"
		if !m.cfg.Capabilities.Owners[bundlecontract.MCP] {
			entry.RuntimeState = "not-compiled"
		} else if m.cfg.MCPStatuses != nil {
			projectedID, idErr := bundlecontract.MCPInstanceID(installID, server.ID)
			if idErr == nil {
				for _, status := range m.cfg.MCPStatuses() {
					if status.Name == projectedID {
						entry.RuntimeState = status.State
						entry.RuntimeInitialized = status.Initialized
						entry.RuntimeToolCount = status.ToolCount
						break
					}
				}
			}
		}
		inspection.Executables = append(inspection.Executables, entry)
	}
	for _, ref := range appendExecutableRefs(installID, manifest) {
		consent, exists, err := m.cfg.Store.ReadBundleConsent(ctx, ref.InstallID, ref.Kind, ref.LocalID)
		if err != nil {
			return InstallationInspection{}, err
		}
		if exists {
			inspection.Consents = append(inspection.Consents, consent)
			for i := range inspection.Executables {
				if inspection.Executables[i].Kind == ref.Kind && inspection.Executables[i].LocalID == ref.LocalID {
					inspection.Executables[i].ConsentAllowed = consent.Allowed
					inspection.Executables[i].ConsentRevision = consent.Revision
					inspection.Executables[i].ConsentCurrent = consent.Allowed && consent.DeclarationDigest == consentDigestForInspection(installID, release.ReleaseDigest, inspection.Executables[i].DeclarationDigest, state.Catalog.ConfigRevision)
				}
			}
		}
	}
	return inspection, nil
}

func consentDigestForInspection(installID, releaseDigest, declarationDigest string, configRevision int64) string {
	digest, _ := bundlecontract.ExecutableConsentDigest(installID, releaseDigest, declarationDigest, configRevision)
	return digest
}

func executableInspection(kind bundlecontract.ComponentKind, localID, releaseDigest, declarationDigest string, missing []string, configRevision int64) ExecutableInspection {
	return ExecutableInspection{Kind: kind, LocalID: localID, ReleaseDigest: releaseDigest, DeclarationDigest: declarationDigest,
		MissingBindings: append([]string{}, missing...), ConsentCurrent: false, ConsentAllowed: false, ConsentRevision: 0}
}

func hookBindings(entry bundlecontract.HookEntry, refs []bundlecontract.BindingRef) []string {
	if applyHookBindings(&entry, refs) != nil {
		return []string{"invalid_binding"}
	}
	return missingEnvironmentBindings(entry.EnvFrom, "")
}

func mcpBindings(entry bundlecontract.MCPEntry, refs []bundlecontract.BindingRef) []string {
	if applyMCPBindings(&entry, refs) != nil {
		return []string{"invalid_binding"}
	}
	return missingEnvironmentBindings(entry.EnvFrom, entry.AuthEnv)
}

func missingEnvironmentBindings(envFrom map[string]string, authEnv string) []string {
	missing := []string{}
	if authEnv != "" {
		if value, ok := os.LookupEnv(authEnv); !ok || value == "" {
			missing = append(missing, "authEnv")
		}
	}
	children := make([]string, 0, len(envFrom))
	for child := range envFrom {
		children = append(children, child)
	}
	sort.Strings(children)
	for _, child := range children {
		reference := envFrom[child]
		if value, ok := os.LookupEnv(reference); !ok || value == "" {
			missing = append(missing, "envFrom:"+child)
		}
	}
	return missing
}

func appendExecutableRefs(installID string, manifest bundlecontract.Manifest) []bundlecontract.ComponentRef {
	refs := make([]bundlecontract.ComponentRef, 0, len(manifest.Components.Hooks)+len(manifest.Components.MCP))
	for _, hook := range manifest.Components.Hooks {
		refs = append(refs, bundlecontract.ComponentRef{InstallID: installID, Kind: bundlecontract.Hook, LocalID: hook.ID})
	}
	for _, server := range manifest.Components.MCP {
		refs = append(refs, bundlecontract.ComponentRef{InstallID: installID, Kind: bundlecontract.MCP, LocalID: server.ID})
	}
	return refs
}

func (m *Manager) GetOperation(ctx context.Context, operationID string) (bundlecontract.Operation, error) {
	if m == nil || m.cfg.Store == nil {
		return bundlecontract.Operation{}, errors.New("toolkits: manager unavailable")
	}
	if !bundlecontract.ValidInstallID(operationID) {
		return bundlecontract.Operation{}, bundlecontract.Invalid("operation_id")
	}
	return m.cfg.Store.GetBundleOperation(ctx, operationID)
}

// ReadComponent returns a bounded page of one inspected component's native
// body. The release digest is checked on every call so a stale detail view
// cannot silently switch to a newly published body.
func (m *Manager) ReadComponent(ctx context.Context, installID string, kind bundlecontract.ComponentKind, localID, expectedReleaseDigest string, offset, maxBytes int) (ComponentPage, error) {
	if m == nil || m.cfg.Store == nil || !bundlecontract.ValidInstallID(installID) || !bundlecontract.ValidLocalID(localID) ||
		!bundlecontract.ValidDigest(expectedReleaseDigest) || offset < 0 || maxBytes < 0 || maxBytes > 64<<10 {
		return ComponentPage{}, bundlecontract.Invalid("component_page")
	}
	if maxBytes == 0 {
		maxBytes = 32 << 10
	}
	state, err := m.cfg.Store.ReadBundleState(ctx)
	if err != nil {
		return ComponentPage{}, err
	}
	var installation *storage.BundleInstallation
	for i := range state.Installations {
		if state.Installations[i].InstallID == installID && !state.Installations[i].Tombstone {
			installation = &state.Installations[i]
			break
		}
	}
	if installation == nil {
		return ComponentPage{}, errors.New("toolkits: not_found")
	}
	if installation.ReleaseDigest != expectedReleaseDigest {
		return ComponentPage{}, errors.New("toolkits: revision_conflict")
	}
	release, ok := findRelease(state.Releases, installID, expectedReleaseDigest)
	if !ok {
		return ComponentPage{}, errors.New("toolkits: artifact_missing")
	}
	artifact, err := OpenArtifact(ctx, m.cfg.StateRoot, expectedReleaseDigest)
	if err != nil {
		return ComponentPage{}, err
	}
	manifest := artifact.Manifest()
	var body []byte
	switch kind {
	case bundlecontract.Skill:
		for _, item := range manifest.Components.Skills {
			if item.ID == localID {
				body, err = artifact.ReadDeclared(ctx, item.Path, m.cfg.Limits.SkillBytes)
				break
			}
		}
	case bundlecontract.Mask:
		for _, item := range manifest.Components.Masks {
			if item.ID == localID {
				body, err = artifact.ReadDeclared(ctx, item.Path, m.cfg.Limits.MaskBytes)
				break
			}
		}
	case bundlecontract.Rule:
		for _, item := range manifest.Components.Rules {
			if item.ID == localID {
				body, err = artifact.ReadDeclared(ctx, item.Path, m.cfg.Limits.RuleBytes)
				break
			}
		}
	case bundlecontract.Hook:
		for _, item := range manifest.Components.Hooks {
			if item.ID == localID {
				body, err = bundlecontract.CanonicalJSON(item)
				break
			}
		}
	case bundlecontract.MCP:
		for _, item := range manifest.Components.MCP {
			if item.ID == localID {
				body, err = bundlecontract.CanonicalJSON(item)
				break
			}
		}
	default:
		return ComponentPage{}, bundlecontract.Invalid("component_kind")
	}
	if err != nil {
		return ComponentPage{}, err
	}
	if body == nil {
		return ComponentPage{}, errors.New("toolkits: component_not_found")
	}
	if offset > len(body) || offset < len(body) && !utf8.RuneStart(body[offset]) {
		return ComponentPage{}, bundlecontract.Invalid("component_offset")
	}
	end := offset + maxBytes
	if end > len(body) {
		end = len(body)
	} else {
		for end > offset && end < len(body) && !utf8.RuneStart(body[end]) {
			end--
		}
	}
	if !utf8.Valid(body) {
		return ComponentPage{}, bundlecontract.Invalid("component_encoding")
	}
	pageDigest := sha256.Sum256(body)
	return ComponentPage{InstallID: installID, Kind: kind, LocalID: localID, ReleaseDigest: release.ReleaseDigest,
		Digest: hex.EncodeToString(pageDigest[:]), Offset: offset, NextOffset: end, TotalBytes: len(body), Content: string(body[offset:end])}, nil
}

// PreparedCandidate exposes only the exact candidate identity needed by the
// authenticated publish action; it never exposes owner or storage internals.
func (m *Manager) PreparedCandidate(operationID string) (string, bundlecontract.CatalogIdentity, bool) {
	if m == nil {
		return "", bundlecontract.CatalogIdentity{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.prepared[operationID]
	return p.CandidateDigest, p.Expected, ok
}

// CurrentView is the narrow immutable projection seam consumed by existing
// content owners that need an explicitly pinned management view.
func (m *Manager) CurrentView(ctx context.Context) (bundlecontract.View, func(), error) {
	if err := ctx.Err(); err != nil {
		return bundlecontract.View{}, nil, err
	}
	handle, err := m.PinCurrent()
	if err != nil {
		return bundlecontract.View{}, nil, err
	}
	return handle.View(), handle.Release, nil
}

// PinRun restores the exact durable release set admitted for runID. It never
// substitutes the current catalog when the run evidence is absent.
func (m *Manager) PinRun(ctx context.Context, runID domain.RunID) (*ViewHandle, error) {
	snapshot, err := m.cfg.Store.LoadRunBundleSnapshot(ctx, runID)
	if err != nil {
		return nil, err
	}
	handle, _, err := m.RestoreRun(ctx, snapshot)
	if err != nil {
		return nil, err
	}
	return handle, nil
}

// ReadSkillResource reads a captured Skill's declared supporting resource
// from the immutable artifact named by the captured View. It does not expose
// the artifact filesystem path to callers.
func (m *Manager) ReadSkillResource(ctx context.Context, view bundlecontract.View, skillID, relativePath string) (string, error) {
	record, ok := view.Lookup(bundlecontract.Skill, skillID)
	if !ok || !record.Available {
		return "", errors.New("toolkits: skill not found in captured view")
	}
	declared := false
	for _, path := range record.Resources {
		if path == relativePath {
			declared = true
			break
		}
	}
	if !declared {
		return "", artifactError("undeclared_resource", "path", nil)
	}
	artifact, err := OpenArtifact(ctx, m.cfg.StateRoot, record.Release.ReleaseDigest)
	if err != nil {
		return "", err
	}
	if err := ValidateArtifact(ctx, artifact, m.staticCapabilities()); err != nil {
		return "", err
	}
	content, err := artifact.ReadDeclared(ctx, relativePath, m.cfg.Limits.ResourceBytes)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
		return "", artifactError("invalid_resource", "utf8", nil)
	}
	return string(content), nil
}

// ResolveHookExecution resolves the explicit cwd policy from a captured Hook.
// Bundle cwd is confined to that release's verified files tree; workspace cwd
// is supplied by Runtime's already-resolved run workspace.
func (m *Manager) ResolveHookExecution(ctx context.Context, view bundlecontract.View, record bundlecontract.ComponentRecord, workspaceRoot string) (string, string, error) {
	if record.Ref.Kind != bundlecontract.Hook || record.Hook == nil || record.Hook.Event != "PreToolUse" || record.Release.ReleaseDigest == "" {
		return "", "", errors.New("toolkits: invalid captured Hook")
	}
	if _, ok := view.Lookup(bundlecontract.Hook, record.ProjectedID); !ok {
		return "", "", errors.New("toolkits: Hook is outside captured View")
	}
	switch record.Hook.Cwd {
	case "workspace":
		if strings.TrimSpace(workspaceRoot) == "" {
			return "", "", errors.New("toolkits: Hook workspace is unavailable")
		}
		info, err := os.Lstat(workspaceRoot)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", "", errors.New("toolkits: Hook workspace is not a safe directory")
		}
		root, err := filepath.EvalSymlinks(workspaceRoot)
		if err != nil {
			return "", "", err
		}
		return root, record.Release.ReleaseDigest, nil
	case "bundle":
		artifact, err := OpenArtifact(ctx, m.cfg.StateRoot, record.Release.ReleaseDigest)
		if err != nil {
			return "", "", err
		}
		if artifact.ReleaseDigest() != record.Release.ReleaseDigest {
			return "", "", errors.New("toolkits: Hook resource release does not match capture")
		}
		if err := ValidateArtifact(ctx, artifact, m.staticCapabilities()); err != nil {
			return "", "", err
		}
		return filepath.Join(artifact.directory, "files"), record.Release.ReleaseDigest, nil
	default:
		return "", "", errors.New("toolkits: Hook cwd policy is unsupported")
	}
}

func NewManager(cfg Config, store storage.BundleStore) (*Manager, error) {
	if store == nil || cfg.StateRoot == "" {
		return nil, errors.New("toolkits: invalid manager configuration")
	}
	cfg.Store = store
	if cfg.Limits.ManifestBytes == 0 {
		cfg.Limits = bundlecontract.DefaultLimits()
	}
	if cfg.Capabilities.Limits.ManifestBytes == 0 {
		cfg.Capabilities.Limits = cfg.Limits
	}
	m := &Manager{cfg: cfg, previews: map[string]PreparedSource{}, previewUse: map[string]string{}, prepared: map[string]PreparedCatalog{}}
	initial, err := bundlecontract.NewView(bundlecontract.CatalogIdentity{}, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	m.current = &viewState{manager: m, view: initial}
	return m, nil
}
func operationDigest(v any) (string, error) {
	b, err := bundlecontract.CanonicalJSON(v)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}
func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
func (m *Manager) Preview(ctx context.Context, r PreviewRequest) (PreparedSource, error) {
	if r.OperationID == "" {
		id, e := newID()
		if e != nil {
			return PreparedSource{}, e
		}
		r.OperationID = id
	}
	m.mu.Lock()
	closed := m.closed
	m.mu.Unlock()
	if closed {
		return PreparedSource{}, errors.New("toolkits: manager closed")
	}
	state, e := m.cfg.Store.ReadBundleState(ctx)
	if e != nil {
		return PreparedSource{}, e
	}
	expected := r.Expected
	if expected == (bundlecontract.CatalogIdentity{}) {
		expected = state.Catalog
	}
	if expected != state.Catalog {
		return PreparedSource{}, errors.New("toolkits: revision_conflict")
	}
	m.mu.Lock()
	if prior, ok := m.previews[r.OperationID]; ok {
		m.mu.Unlock()
		if prior.requested == r.Source {
			return prior, nil
		}
		return PreparedSource{}, errors.New("toolkits: revision_conflict")
	}
	m.mu.Unlock()
	m.gcMu.Lock()
	defer m.gcMu.Unlock()
	installID, e := newID()
	if e != nil {
		return PreparedSource{}, e
	}
	artifact, e := Acquire(ctx, AcquireRequest{OperationID: r.OperationID, Source: r.Source, StateRoot: m.cfg.StateRoot, Limits: m.cfg.Limits, GitEnvironment: m.cfg.GitEnvironment})
	if e != nil {
		return PreparedSource{}, e
	}
	staticCaps := m.staticCapabilities()
	if e = ValidateArtifact(ctx, artifact, staticCaps); e != nil {
		return PreparedSource{}, e
	}
	out := PreparedSource{OperationID: r.OperationID, InstallID: installID, ReleaseDigest: artifact.ReleaseDigest(), Source: artifact.SourceLock(), Inventory: artifact.Inventory(), Manifest: artifact.Manifest(), artifact: artifact, requested: r.Source}
	current, e := m.cfg.Store.ReadBundleState(ctx)
	if e != nil {
		return PreparedSource{}, e
	}
	if current.Catalog != expected {
		return PreparedSource{}, errors.New("toolkits: revision_conflict")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return PreparedSource{}, errors.New("toolkits: manager closed")
	}
	if prior, exists := m.previews[r.OperationID]; exists && prior.ReleaseDigest != out.ReleaseDigest {
		return PreparedSource{}, errors.New("toolkits: revision_conflict")
	}
	m.previews[r.OperationID] = out
	return out, nil
}

func (m *Manager) staticCapabilities() Capabilities {
	return m.cfg.Capabilities
}

func (m *Manager) Install(ctx context.Context, r InstallRequest) (bundlecontract.Operation, error) {
	if r.OperationID == "" {
		id, err := newID()
		if err != nil {
			return bundlecontract.Operation{}, err
		}
		r.OperationID = id
	}
	m.mu.Lock()
	p, ok := m.previews[r.PreviewOperationID]
	usedBy := m.previewUse[r.PreviewOperationID]
	busy := m.publishing != ""
	closed := m.closed
	if ok && usedBy == "" && !busy && !closed {
		m.previewUse[r.PreviewOperationID] = r.OperationID
	}
	m.mu.Unlock()
	if closed {
		return bundlecontract.Operation{}, errors.New("toolkits: manager closed")
	}
	if busy {
		return bundlecontract.Operation{}, errors.New("toolkits: catalog_update_pending")
	}
	if !ok || p.ReleaseDigest != r.ExpectedReleaseDigest || (usedBy != "" && usedBy != r.OperationID) {
		return bundlecontract.Operation{}, errors.New("toolkits: preview_stale")
	}
	state, err := m.cfg.Store.ReadBundleState(ctx)
	if err != nil {
		m.releasePreviewReservation(r.PreviewOperationID, r.OperationID)
		return bundlecontract.Operation{}, err
	}
	if r.Expected == (bundlecontract.CatalogIdentity{}) {
		r.Expected = state.Catalog
	}
	if r.Expected != state.Catalog {
		m.releasePreviewReservation(r.PreviewOperationID, r.OperationID)
		return bundlecontract.Operation{}, errors.New("toolkits: revision_conflict")
	}
	artifact, err := OpenArtifact(ctx, m.cfg.StateRoot, p.ReleaseDigest)
	if err != nil {
		m.releasePreviewReservation(r.PreviewOperationID, r.OperationID)
		return bundlecontract.Operation{}, err
	}
	if err := ValidateArtifact(ctx, artifact, m.staticCapabilities()); err != nil {
		m.releasePreviewReservation(r.PreviewOperationID, r.OperationID)
		return bundlecontract.Operation{}, err
	}
	manifest := artifact.Manifest()
	request := struct {
		Expected                 bundlecontract.CatalogIdentity
		Preview, Install, Digest string
	}{r.Expected, p.OperationID, p.InstallID, p.ReleaseDigest}
	input, e := operationDigest(request)
	if e != nil {
		m.releasePreviewReservation(r.PreviewOperationID, r.OperationID)
		return bundlecontract.Operation{}, e
	}
	op, e := m.cfg.Store.SaveBundleInstallation(ctx, storage.InstallCommit{OperationID: r.OperationID, InputDigest: input, Expected: r.Expected, Installation: storage.BundleInstallation{InstallID: p.InstallID, BundleID: manifest.ID, State: "disabled", Source: p.Source, ReleaseDigest: p.ReleaseDigest}, Release: bundlecontract.ReleaseRef{InstallID: p.InstallID, BundleID: manifest.ID, ReleaseDigest: p.ReleaseDigest, ReleaseSequence: 1}, NowMS: time.Now().UnixMilli()})
	if e == nil && op.State == "committed" {
		m.mu.Lock()
		m.previewUse[p.OperationID] = r.OperationID
		m.mu.Unlock()
	} else if e == nil {
		m.releasePreviewReservation(p.OperationID, r.OperationID)
	}
	return op, e
}

func (m *Manager) releasePreviewReservation(previewID, operationID string) {
	m.mu.Lock()
	if m.previewUse[previewID] == operationID {
		delete(m.previewUse, previewID)
	}
	m.mu.Unlock()
}

func (m *Manager) CancelPreview(operationID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.previewUse[operationID] != "" {
		return false
	}
	if _, ok := m.previews[operationID]; !ok {
		return false
	}
	delete(m.previews, operationID)
	return true
}
func (m *Manager) Prepare(ctx context.Context, r PrepareRequest) (result PreparedCatalog, retErr error) {
	if r.OperationID == "" {
		var e error
		r.OperationID, e = newID()
		if e != nil {
			return PreparedCatalog{}, e
		}
	}
	if !bundlecontract.ValidInstallID(r.OperationID) {
		return PreparedCatalog{}, bundlecontract.Invalid("operation_id")
	}
	m.mu.Lock()
	if m.closed || m.publishing != "" {
		m.mu.Unlock()
		return PreparedCatalog{}, errors.New("toolkits: catalog_update_pending")
	}
	m.publishing = r.OperationID
	m.mu.Unlock()
	m.gcMu.Lock()
	defer m.gcMu.Unlock()
	operationStarted := false
	defer func() {
		if retErr != nil {
			if operationStarted {
				if op, e := m.cfg.Store.GetBundleOperation(ctx, r.OperationID); e == nil && op.State == "pending" {
					_, _ = m.cfg.Store.CancelBundleOperation(ctx, storage.CancelOperation{ID: r.OperationID, ExpectedRevision: op.Revision})
				}
			}
			m.mu.Lock()
			if m.publishing == r.OperationID {
				m.publishing = ""
			}
			m.mu.Unlock()
		}
	}()
	state, err := m.cfg.Store.ReadBundleState(ctx)
	if err != nil {
		return PreparedCatalog{}, err
	}
	if r.Expected == (bundlecontract.CatalogIdentity{}) {
		r.Expected = state.Catalog
	}
	if r.Expected != state.Catalog {
		return PreparedCatalog{}, errors.New("toolkits: revision_conflict")
	}
	installs := append([]storage.BundleInstallation{}, state.Installations...)
	byID := map[string]int{}
	for i, x := range installs {
		byID[x.InstallID] = i
	}
	var next []bundlecontract.ReleaseRef
	for _, x := range installs {
		if x.State == "enabled" && !x.Tombstone {
			if rr, ok := findRelease(state.Releases, x.InstallID, x.ReleaseDigest); ok {
				next = append(next, rr)
			} else {
				return PreparedCatalog{}, errors.New("toolkits: artifact_missing")
			}
		}
	}
	var added *bundlecontract.ReleaseRef
	var candidate *Artifact
	switch r.Mutation.Kind {
	case "enable", "disable", "uninstall":
		i, ok := byID[r.Mutation.InstallID]
		if !ok || installs[i].Tombstone {
			return PreparedCatalog{}, errors.New("toolkits: not_found")
		}
		if r.Mutation.Kind == "enable" && installs[i].State != "disabled" ||
			r.Mutation.Kind == "disable" && installs[i].State != "enabled" ||
			r.Mutation.Kind == "uninstall" && installs[i].State != "disabled" {
			return PreparedCatalog{}, errors.New("toolkits: invalid_installation_state")
		}
		if r.Mutation.ExpectedRevision != 0 && r.Mutation.ExpectedRevision != installs[i].Revision {
			return PreparedCatalog{}, errors.New("toolkits: revision_conflict")
		}
		switch r.Mutation.Kind {
		case "enable":
			installs[i].State = "enabled"
		case "disable":
			installs[i].State = "disabled"
		case "uninstall":
			installs[i].State = "orphaned"
			installs[i].Tombstone = true
		}
		filtered := next[:0]
		for _, active := range next {
			if active.InstallID != installs[i].InstallID {
				filtered = append(filtered, active)
			}
		}
		next = filtered
		if r.Mutation.Kind == "enable" {
			rr, ok := findRelease(state.Releases, installs[i].InstallID, installs[i].ReleaseDigest)
			if !ok {
				return PreparedCatalog{}, errors.New("toolkits: artifact_missing")
			}
			next = append(next, rr)
		}
	case "update", "rollback", "rebind":
		i, ok := byID[r.Mutation.InstallID]
		if !ok || installs[i].Tombstone {
			return PreparedCatalog{}, errors.New("toolkits: not_found")
		}
		if r.Mutation.ExpectedRevision != 0 && r.Mutation.ExpectedRevision != installs[i].Revision {
			return PreparedCatalog{}, errors.New("toolkits: revision_conflict")
		}
		var a Artifact
		var source bundlecontract.SourceLock
		if r.Mutation.Kind == "rollback" {
			target, exists := findRelease(state.Releases, installs[i].InstallID, r.Mutation.ReleaseDigest)
			if !exists || target.BundleID != installs[i].BundleID {
				return PreparedCatalog{}, errors.New("toolkits: release_not_found")
			}
			var openErr error
			a, openErr = OpenArtifact(ctx, m.cfg.StateRoot, target.ReleaseDigest)
			if openErr != nil {
				return PreparedCatalog{}, openErr
			}
			// The durable installation source tracks the currently configured
			// origin. Older release rows intentionally contain content identity,
			// not a duplicate mutable source configuration.
			source = installs[i].Source
		} else {
			m.mu.Lock()
			p, exists := m.previews[r.Mutation.PreviewOperationID]
			usedBy := m.previewUse[r.Mutation.PreviewOperationID]
			m.mu.Unlock()
			if !exists || p.ReleaseDigest != r.Mutation.ReleaseDigest || usedBy != "" {
				return PreparedCatalog{}, errors.New("toolkits: preview_stale")
			}
			a, err = OpenArtifact(ctx, m.cfg.StateRoot, p.ReleaseDigest)
			if err != nil {
				return PreparedCatalog{}, err
			}
			source = p.Source
			if r.Mutation.Kind == "update" && (source.Kind != installs[i].Source.Kind || source.Origin != installs[i].Source.Origin) {
				return PreparedCatalog{}, errors.New("toolkits: source_rebind_required")
			}
			if r.Mutation.Kind == "rebind" && source.Kind == installs[i].Source.Kind && source.Origin == installs[i].Source.Origin {
				return PreparedCatalog{}, errors.New("toolkits: source_unchanged")
			}
		}
		manifest := a.Manifest()
		if manifest.ID != installs[i].BundleID {
			return PreparedCatalog{}, errors.New("toolkits: bundle_id_conflict")
		}
		if err := ValidateArtifact(ctx, a, m.cfg.Capabilities); err != nil {
			return PreparedCatalog{}, err
		}
		candidate = &a
		filtered := next[:0]
		for _, active := range next {
			if active.InstallID != installs[i].InstallID {
				filtered = append(filtered, active)
			}
		}
		next = filtered
		var seq int64
		for _, rr := range state.Releases {
			if rr.InstallID == installs[i].InstallID && rr.ReleaseSequence > seq {
				seq = rr.ReleaseSequence
			}
		}
		ref := bundlecontract.ReleaseRef{InstallID: installs[i].InstallID, BundleID: manifest.ID, ReleaseDigest: a.ReleaseDigest(), ReleaseSequence: seq + 1}
		added = &ref
		installs[i].ReleaseDigest = ref.ReleaseDigest
		installs[i].Revision++
		if r.Mutation.Kind != "rollback" {
			installs[i].Source = source
		}
		if installs[i].State == "enabled" {
			next = append(next, ref)
		}
	default:
		return PreparedCatalog{}, errors.New("toolkits: invalid_mutation")
	}
	sort.Slice(next, func(i, j int) bool {
		if next[i].InstallID != next[j].InstallID {
			return next[i].InstallID < next[j].InstallID
		}
		return next[i].ReleaseSequence < next[j].ReleaseSequence
	})
	catDigest, err := catalogDigest(state.Catalog.ConfigRevision, state.Catalog.GenerationID, next, state.Bindings)
	if err != nil {
		return PreparedCatalog{}, err
	}
	candidateIdentity := bundlecontract.CatalogIdentity{Epoch: state.Catalog.Epoch + 1, ConfigRevision: state.Catalog.ConfigRevision, EffectiveDigest: catDigest, GenerationID: state.Catalog.GenerationID}
	view, err := m.buildView(ctx, candidateIdentity, next, installs, candidate, state.Bindings)
	if err != nil {
		return PreparedCatalog{}, err
	}
	owners := PreparedOwners(noopOwners{view: view})
	if m.cfg.PrepareOwners != nil {
		owners, err = m.cfg.PrepareOwners(ctx, view)
		if err != nil {
			return PreparedCatalog{}, err
		}
		if owners == nil {
			return PreparedCatalog{}, errors.New("toolkits: owner preparation returned nil")
		}
	}
	input := struct {
		Expected bundlecontract.CatalogIdentity
		Mutation CatalogMutation
		Releases []bundlecontract.ReleaseRef
	}{r.Expected, r.Mutation, next}
	dig, err := operationDigest(input)
	if err != nil {
		_ = owners.Close(ctx)
		return PreparedCatalog{}, err
	}
	if r.OperationID == "" {
		r.OperationID, err = newID()
		if err != nil {
			_ = owners.Close(ctx)
			return PreparedCatalog{}, err
		}
	}
	changes := []storage.InstallationState{}
	for _, x := range installs {
		if before, ok := byID[x.InstallID]; ok && x != state.Installations[before] {
			changes = append(changes, storage.InstallationState{InstallID: x.InstallID, State: x.State, ReleaseDigest: x.ReleaseDigest, Revision: state.Installations[before].Revision, Tombstone: x.Tombstone})
		}
	}
	newReleases := []bundlecontract.ReleaseRef{}
	if added != nil {
		newReleases = append(newReleases, *added)
	}
	componentChanges := []bundlecontract.ComponentChange{}
	if candidate != nil && (r.Mutation.Kind == "update" || r.Mutation.Kind == "rollback" || r.Mutation.Kind == "rebind") {
		currentInstall := state.Installations[byID[r.Mutation.InstallID]]
		oldArtifact, openErr := OpenArtifact(ctx, m.cfg.StateRoot, currentInstall.ReleaseDigest)
		if openErr != nil {
			_ = owners.Close(ctx)
			return PreparedCatalog{}, openErr
		}
		componentChanges, err = diffArtifacts(ctx, oldArtifact, *candidate, m.cfg.Limits)
		if err != nil {
			_ = owners.Close(ctx)
			return PreparedCatalog{}, err
		}
	}
	installChanges := make([]bundlecontract.InstallationChange, 0, len(changes))
	for _, change := range changes {
		installChanges = append(installChanges, bundlecontract.InstallationChange{InstallID: change.InstallID, State: change.State, ReleaseDigest: change.ReleaseDigest, Revision: change.Revision, Tombstone: change.Tombstone})
	}
	durable := &bundlecontract.PreparedOperation{InputDigest: dig, Expected: r.Expected, CandidateDigest: catDigest, Releases: append([]bundlecontract.ReleaseRef(nil), next...), NewReleases: newReleases, Installations: installChanges, Changes: append([]bundlecontract.ComponentChange(nil), componentChanges...), MutationKind: r.Mutation.Kind, MutationInstallID: r.Mutation.InstallID, MutationReleaseDigest: r.Mutation.ReleaseDigest, MutationExpectedRevision: r.Mutation.ExpectedRevision}
	op, err := m.cfg.Store.BeginBundleOperation(ctx, storage.BeginOperation{ID: r.OperationID, InputDigest: dig, Kind: "publish", ExpectedEpoch: r.Expected.Epoch, Prepared: durable})
	if err != nil {
		_ = owners.Close(ctx)
		return PreparedCatalog{}, err
	}
	operationStarted = true
	if op.State != "pending" {
		_ = owners.Close(ctx)
		return PreparedCatalog{}, errors.New("toolkits: operation_not_pending")
	}
	out := PreparedCatalog{OperationID: r.OperationID, Expected: r.Expected, CandidateDigest: catDigest, InputDigest: dig, Operation: op, Changes: append([]bundlecontract.ComponentChange(nil), componentChanges...), Releases: next, owners: owners, installations: changes, newReleases: newReleases, mutation: r.Mutation}
	if owners.View().Identity() != candidateIdentity {
		_ = owners.Close(ctx)
		return PreparedCatalog{}, errors.New("toolkits: prepared_owner_epoch_mismatch")
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		_ = owners.Close(ctx)
		return PreparedCatalog{}, errors.New("toolkits: manager closed")
	}
	m.prepared[r.OperationID] = out
	m.mu.Unlock()
	return out, nil
}
func findRelease(releases []bundlecontract.ReleaseRef, install, digest string) (bundlecontract.ReleaseRef, bool) {
	var latest bundlecontract.ReleaseRef
	found := false
	for _, r := range releases {
		if r.InstallID == install && r.ReleaseDigest == digest && (!found || r.ReleaseSequence > latest.ReleaseSequence) {
			latest = r
			found = true
		}
	}
	return latest, found
}

type noopOwners struct{ view bundlecontract.View }

func (n noopOwners) View() bundlecontract.View { return n.view }
func (noopOwners) Publish()                    {}
func (noopOwners) Close(context.Context) error { return nil }
func catalogDigest(configRevision int64, generationID string, releases []bundlecontract.ReleaseRef, bindings []storage.ToolkitBinding) (string, error) {
	ordered := append([]storage.ToolkitBinding(nil), bindings...)
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if a.InstallID != b.InstallID {
			return a.InstallID < b.InstallID
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.LocalID != b.LocalID {
			return a.LocalID < b.LocalID
		}
		return a.Name < b.Name
	})
	return operationDigest(struct {
		ConfigRevision int64
		GenerationID   string
		Releases       []bundlecontract.ReleaseRef
		Bindings       []storage.ToolkitBinding
	}{configRevision, generationID, releases, ordered})
}

func (m *Manager) buildView(ctx context.Context, id bundlecontract.CatalogIdentity, releases []bundlecontract.ReleaseRef, installs []storage.BundleInstallation, candidate *Artifact, bindings []storage.ToolkitBinding) (bundlecontract.View, error) {
	records := []bundlecontract.ComponentRecord{}
	_ = installs // release references are authoritative; install rows provide lifecycle metadata only.
	for _, rr := range releases {
		var artifact Artifact
		if candidate != nil && candidate.ReleaseDigest() == rr.ReleaseDigest {
			artifact = *candidate
		} else {
			var err error
			artifact, err = OpenArtifact(ctx, m.cfg.StateRoot, rr.ReleaseDigest)
			if err != nil {
				return bundlecontract.View{}, err
			}
		}
		if err := ValidateArtifact(ctx, artifact, m.cfg.Capabilities); err != nil {
			return bundlecontract.View{}, err
		}
		manifest := artifact.Manifest()
		display := manifest.ID
		for _, s := range manifest.Components.Skills {
			b, e := artifact.ReadDeclared(ctx, s.Path, m.cfg.Limits.SkillBytes)
			if e != nil {
				return bundlecontract.View{}, e
			}
			skill, e := DecodeNativeSkill(b)
			if e != nil {
				return bundlecontract.View{}, e
			}
			hostID, _ := bundlecontract.SkillID(rr.InstallID, s.ID)
			resources := []string{}
			for _, p := range manifest.Resources {
				resources = append(resources, p)
			}
			records = append(records, bundlecontract.ComponentRecord{Ref: bundlecontract.ComponentRef{InstallID: rr.InstallID, Kind: bundlecontract.Skill, LocalID: s.ID}, ProjectedID: hostID, Release: rr, ContentDigest: rr.ReleaseDigest, DisplayRef: display + ":" + s.ID, Name: skill.Name, Description: skill.Description, Body: skill.Body, Resources: resources, Available: true})
		}
		for _, s := range manifest.Components.Masks {
			b, e := artifact.ReadDeclared(ctx, s.Path, m.cfg.Limits.MaskBytes)
			if e != nil {
				return bundlecontract.View{}, e
			}
			id, _ := bundlecontract.MaskID(rr.InstallID, s.ID)
			refs := []bundlecontract.ComponentRef{}
			for _, local := range s.RecommendedSkills {
				refs = append(refs, bundlecontract.ComponentRef{InstallID: rr.InstallID, Kind: bundlecontract.Skill, LocalID: local})
			}
			records = append(records, bundlecontract.ComponentRecord{Ref: bundlecontract.ComponentRef{InstallID: rr.InstallID, Kind: bundlecontract.Mask, LocalID: s.ID}, ProjectedID: id, Release: rr, ContentDigest: rr.ReleaseDigest, DisplayRef: display + ":" + s.ID, Name: s.Name, Description: s.Description, Body: string(b), RecommendedSkills: refs, Available: true})
		}
		for _, s := range manifest.Components.Rules {
			b, e := artifact.ReadDeclared(ctx, s.Path, m.cfg.Limits.RuleBytes)
			if e != nil {
				return bundlecontract.View{}, e
			}
			id, _ := bundlecontract.SkillID(rr.InstallID, s.ID)
			records = append(records, bundlecontract.ComponentRecord{Ref: bundlecontract.ComponentRef{InstallID: rr.InstallID, Kind: bundlecontract.Rule, LocalID: s.ID}, ProjectedID: id, Release: rr, ContentDigest: rr.ReleaseDigest, DisplayRef: display + ":" + s.ID, Body: string(b), Faces: s.Faces, Available: true})
		}
		for _, s := range manifest.Components.Hooks {
			d, _ := operationDigest(s)
			id, _ := bundlecontract.SkillID(rr.InstallID, s.ID)
			hook := s
			refs := componentBindingRefs(bindings, rr.InstallID, bundlecontract.Hook, s.ID)
			if err := applyHookBindings(&hook, refs); err != nil {
				return bundlecontract.View{}, err
			}
			records = append(records, bundlecontract.ComponentRecord{Ref: bundlecontract.ComponentRef{InstallID: rr.InstallID, Kind: bundlecontract.Hook, LocalID: s.ID}, ProjectedID: id, Release: rr, ContentDigest: rr.ReleaseDigest, DisplayRef: display + ":" + s.ID, Hook: &hook, Bindings: refs, ExecutableDigest: d, Available: false, Status: "unarmed"})
		}
		for _, s := range manifest.Components.MCP {
			d, _ := operationDigest(s)
			id, _ := bundlecontract.MCPInstanceID(rr.InstallID, s.ID)
			server := s
			refs := componentBindingRefs(bindings, rr.InstallID, bundlecontract.MCP, s.ID)
			if err := applyMCPBindings(&server, refs); err != nil {
				return bundlecontract.View{}, err
			}
			records = append(records, bundlecontract.ComponentRecord{Ref: bundlecontract.ComponentRef{InstallID: rr.InstallID, Kind: bundlecontract.MCP, LocalID: s.ID}, ProjectedID: id, Release: rr, ContentDigest: rr.ReleaseDigest, DisplayRef: display + ":" + s.ID, MCP: &server, Bindings: refs, ExecutableDigest: d, Available: false, Status: "unauthorized"})
		}
	}
	return bundlecontract.NewView(id, releases, records, nil)
}

func componentBindingRefs(bindings []storage.ToolkitBinding, installID string, kind bundlecontract.ComponentKind, localID string) []bundlecontract.BindingRef {
	refs := []bundlecontract.BindingRef{}
	for _, binding := range bindings {
		if binding.InstallID == installID && binding.Kind == string(kind) && binding.LocalID == localID {
			refs = append(refs, bundlecontract.BindingRef{Name: binding.Name, Reference: binding.Reference})
		}
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Name < refs[j].Name })
	return refs
}

type artifactComponentDigest struct {
	kind, localID, digest string
}

func diffArtifacts(ctx context.Context, before, after Artifact, limits bundlecontract.Limits) ([]bundlecontract.ComponentChange, error) {
	oldComponents, err := artifactComponentDigests(ctx, before, limits)
	if err != nil {
		return nil, err
	}
	newComponents, err := artifactComponentDigests(ctx, after, limits)
	if err != nil {
		return nil, err
	}
	keys := map[string]bool{}
	for key := range oldComponents {
		keys[key] = true
	}
	for key := range newComponents {
		keys[key] = true
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	changes := make([]bundlecontract.ComponentChange, 0)
	for _, key := range ordered {
		oldValue, oldOK := oldComponents[key]
		newValue, newOK := newComponents[key]
		if oldOK && newOK && oldValue.digest == newValue.digest {
			continue
		}
		value := newValue
		change := "changed"
		if !oldOK {
			change = "added"
		}
		if !newOK {
			value = oldValue
			change = "removed"
		}
		item := bundlecontract.ComponentChange{Kind: value.kind, LocalID: value.localID, Change: change, RequiresFreshConsent: value.kind == string(bundlecontract.Hook) || value.kind == string(bundlecontract.MCP)}
		if oldOK {
			item.BeforeDigest = oldValue.digest
		}
		if newOK {
			item.AfterDigest = newValue.digest
		}
		changes = append(changes, item)
	}
	return changes, nil
}

func artifactComponentDigests(ctx context.Context, artifact Artifact, limits bundlecontract.Limits) (map[string]artifactComponentDigest, error) {
	manifest := artifact.Manifest()
	out := map[string]artifactComponentDigest{}
	add := func(kind, id string, body []byte) {
		sum := sha256.Sum256(body)
		item := artifactComponentDigest{kind: kind, localID: id, digest: hex.EncodeToString(sum[:])}
		out[kind+"/"+id] = item
	}
	read := func(kind, id, path string, max int64) error {
		body, err := artifact.ReadDeclared(ctx, path, max)
		if err != nil {
			return err
		}
		add(kind, id, body)
		return nil
	}
	for _, entry := range manifest.Components.Skills {
		if err := read(string(bundlecontract.Skill), entry.ID, entry.Path, limits.SkillBytes); err != nil {
			return nil, err
		}
	}
	for _, entry := range manifest.Components.Masks {
		if err := read(string(bundlecontract.Mask), entry.ID, entry.Path, limits.MaskBytes); err != nil {
			return nil, err
		}
	}
	for _, entry := range manifest.Components.Rules {
		if err := read(string(bundlecontract.Rule), entry.ID, entry.Path, limits.RuleBytes); err != nil {
			return nil, err
		}
	}
	for _, entry := range manifest.Components.Hooks {
		body, err := bundlecontract.CanonicalJSON(entry)
		if err != nil {
			return nil, err
		}
		add(string(bundlecontract.Hook), entry.ID, body)
	}
	for _, entry := range manifest.Components.MCP {
		body, err := bundlecontract.CanonicalJSON(entry)
		if err != nil {
			return nil, err
		}
		add(string(bundlecontract.MCP), entry.ID, body)
	}
	for _, path := range manifest.Resources {
		if err := read("resource", path, path, limits.ResourceBytes); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func applyHookBindings(entry *bundlecontract.HookEntry, refs []bundlecontract.BindingRef) error {
	for _, ref := range refs {
		if !strings.HasPrefix(ref.Name, "envFrom:") {
			return errors.New("toolkits: invalid stored Hook binding")
		}
		child := strings.TrimPrefix(ref.Name, "envFrom:")
		if _, exists := entry.EnvFrom[child]; !exists {
			return errors.New("toolkits: stored Hook binding no longer matches declaration")
		}
		entry.EnvFrom[child] = ref.Reference
	}
	return nil
}

func applyMCPBindings(entry *bundlecontract.MCPEntry, refs []bundlecontract.BindingRef) error {
	for _, ref := range refs {
		switch {
		case ref.Name == "authEnv":
			if entry.AuthEnv == "" {
				return errors.New("toolkits: stored MCP binding no longer matches declaration")
			}
			entry.AuthEnv = ref.Reference
		case strings.HasPrefix(ref.Name, "envFrom:"):
			child := strings.TrimPrefix(ref.Name, "envFrom:")
			if _, exists := entry.EnvFrom[child]; !exists {
				return errors.New("toolkits: stored MCP binding no longer matches declaration")
			}
			entry.EnvFrom[child] = ref.Reference
		default:
			return errors.New("toolkits: invalid stored MCP binding")
		}
	}
	return nil
}
func (m *Manager) Publish(ctx context.Context, id string) (bundlecontract.Operation, error) {
	m.mu.Lock()
	p, ok := m.prepared[id]
	if !ok || m.publishing != id || m.recoveryRequired || m.publishInFlight != "" {
		m.mu.Unlock()
		return bundlecontract.Operation{}, errors.New("toolkits: publish_in_progress")
	}
	m.publishInFlight = id
	gate := m.cfg.Gate
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		if m.publishInFlight == id {
			m.publishInFlight = ""
		}
		m.mu.Unlock()
	}()
	if gate == nil {
		m.discardPrepared(ctx, id, p)
		return bundlecontract.Operation{}, errors.New("toolkits: admission_gate_unavailable")
	}
	if err := gate.BeginDrain(ctx, id); err != nil {
		m.discardPrepared(ctx, id, p)
		return bundlecontract.Operation{}, err
	}
	draining := true
	defer func() {
		if draining {
			gate.EndDrain(id)
		}
	}()
	if err := gate.WaitIdle(ctx, id); err != nil {
		m.discardPrepared(ctx, id, p)
		return bundlecontract.Operation{}, err
	}
	state, err := m.cfg.Store.ReadBundleState(ctx)
	if err != nil {
		m.discardPrepared(ctx, id, p)
		return bundlecontract.Operation{}, err
	}
	removed, err := m.removedMasks(ctx, p, state)
	if err != nil {
		m.discardPrepared(ctx, id, p)
		return bundlecontract.Operation{}, err
	}
	op, err := m.cfg.Store.CommitBundleActivation(ctx, storage.ActivationCommit{OperationID: id, InputDigest: mustDigestForOperation(p), LeaseOwner: m.cfg.LeaseOwner, Expected: p.Expected, EffectiveDigest: p.CandidateDigest, GenerationID: p.owners.View().Identity().GenerationID, Installations: p.installations, Releases: p.newReleases, RemovedMasks: removed, NowMS: time.Now().UnixMilli()})
	if err != nil {
		latest, stateErr := m.cfg.Store.ReadBundleState(ctx)
		if stateErr == nil && latest.Catalog == p.Expected {
			m.discardPrepared(ctx, id, p)
			return op, err
		}
		if stateErr == nil && latest.Catalog.Epoch == p.Expected.Epoch+1 && latest.Catalog.EffectiveDigest == p.CandidateDigest {
			receipt, receiptErr := m.cfg.Store.GetBundleOperation(ctx, id)
			if receiptErr == nil && receipt.State == "committed" && receipt.CommittedEpoch == latest.Catalog.Epoch {
				op = receipt
				err = nil
			} else {
				stateErr = errors.Join(errors.New("toolkits: committed_state_receipt_mismatch"), receiptErr)
			}
		}
		if err != nil {
			m.mu.Lock()
			m.recoveryRequired = true
			m.recoveryDrainID = id
			m.mu.Unlock()
			draining = false // Recover must expose the durable epoch before admission resumes.
			return bundlecontract.Operation{}, errors.Join(errors.New("toolkits: publication_recovery_required"), err, stateErr)
		}
	}
	// The admission gate stays closed until Core Storage and every owner expose
	// the same committed epoch. PreparedOwners.Publish is intentionally infallible.
	m.mu.Lock()
	if m.closed {
		m.recoveryRequired = true
		m.recoveryDrainID = id
		m.mu.Unlock()
		draining = false
		return bundlecontract.Operation{}, errors.New("toolkits: publication_recovery_required")
	}
	view := p.owners.View()
	p.owners.Publish()
	old := m.current
	m.current = &viewState{manager: m, view: view, owners: p.owners}
	delete(m.prepared, id)
	m.publishing = ""
	m.recoveryRequired = false
	m.recoveryDrainID = ""
	m.mu.Unlock()
	m.retire(ctx, old)
	gate.EndDrain(id)
	draining = false
	return op, nil
}
func mustDigestForOperation(p PreparedCatalog) string {
	if p.InputDigest != "" {
		return p.InputDigest
	}
	d, _ := operationDigest(struct {
		Expected bundlecontract.CatalogIdentity
		Mutation CatalogMutation
		Releases []bundlecontract.ReleaseRef
	}{p.Expected, p.mutation, p.Releases})
	return d
}
func (m *Manager) removedMasks(ctx context.Context, p PreparedCatalog, state storage.BundleState) ([]string, error) {
	if p.mutation.Kind != "disable" && p.mutation.Kind != "uninstall" && p.mutation.Kind != "update" && p.mutation.Kind != "rollback" && p.mutation.Kind != "rebind" {
		return nil, nil
	}
	var installation *storage.BundleInstallation
	for i := range state.Installations {
		if state.Installations[i].InstallID == p.mutation.InstallID {
			installation = &state.Installations[i]
			break
		}
	}
	if installation == nil {
		return nil, errors.New("toolkits: not_found")
	}
	oldArtifact, err := OpenArtifact(ctx, m.cfg.StateRoot, installation.ReleaseDigest)
	if err != nil {
		return nil, err
	}
	oldIDs := map[string]struct{}{}
	for _, mask := range oldArtifact.Manifest().Components.Masks {
		id, err := bundlecontract.MaskID(installation.InstallID, mask.ID)
		if err != nil {
			return nil, err
		}
		oldIDs[id] = struct{}{}
	}
	var newDigest string
	for _, change := range p.installations {
		if change.InstallID == installation.InstallID {
			newDigest = change.ReleaseDigest
			break
		}
	}
	if newDigest == "" && p.mutation.Kind != "disable" && p.mutation.Kind != "uninstall" {
		newDigest = installation.ReleaseDigest
	}
	if newDigest != "" {
		newArtifact, err := OpenArtifact(ctx, m.cfg.StateRoot, newDigest)
		if err != nil {
			return nil, err
		}
		for _, mask := range newArtifact.Manifest().Components.Masks {
			id, err := bundlecontract.MaskID(installation.InstallID, mask.ID)
			if err != nil {
				return nil, err
			}
			delete(oldIDs, id)
		}
	}
	removed := make([]string, 0, len(oldIDs))
	for id := range oldIDs {
		removed = append(removed, id)
	}
	sort.Strings(removed)
	return removed, nil
}
func (m *Manager) discardPrepared(ctx context.Context, id string, p PreparedCatalog) {
	m.mu.Lock()
	_, owned := m.prepared[id]
	if owned {
		delete(m.prepared, id)
	}
	if m.publishing == id {
		m.publishing = ""
	}
	if m.publishInFlight == id {
		m.publishInFlight = ""
	}
	m.mu.Unlock()
	if !owned {
		return
	}
	_ = p.owners.Close(ctx)
	if op, err := m.cfg.Store.GetBundleOperation(ctx, id); err == nil && op.State == "pending" {
		_, _ = m.cfg.Store.CancelBundleOperation(ctx, storage.CancelOperation{ID: id, ExpectedRevision: op.Revision})
	}
}

func (m *Manager) CancelOperation(ctx context.Context, id string, expectedRevision int64) (bundlecontract.Operation, error) {
	op, err := m.cfg.Store.CancelBundleOperation(ctx, storage.CancelOperation{ID: id, ExpectedRevision: expectedRevision})
	if err != nil {
		return op, err
	}
	m.mu.Lock()
	p, hasPrepared := m.prepared[id]
	if hasPrepared {
		delete(m.prepared, id)
	}
	if m.publishing == id {
		m.publishing = ""
	}
	m.mu.Unlock()
	if hasPrepared {
		if err := p.owners.Close(ctx); err != nil {
			return op, err
		}
	}
	return op, nil
}

func (m *Manager) AcquireView(context.Context) (*ViewHandle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.current == nil {
		return nil, errors.New("toolkits: manager unavailable")
	}
	state := m.current
	state.refs++
	return m.handleLocked(state), nil
}

// CaptureRun pins the published view and creates the immutable evidence plus
// optimistic catalog/revocation CAS for one new run admission.
func (m *Manager) CaptureRun(ctx context.Context, runID domain.RunID) (*ViewHandle, storage.RunBundleSnapshot, bundlecontract.CaptureCheck, error) {
	if runID == "" {
		return nil, storage.RunBundleSnapshot{}, bundlecontract.CaptureCheck{}, errors.New("toolkits: run id is required")
	}
	handle, err := m.AcquireView(ctx)
	if err != nil {
		return nil, storage.RunBundleSnapshot{}, bundlecontract.CaptureCheck{}, err
	}
	state, err := m.cfg.Store.ReadBundleState(ctx)
	if err != nil {
		handle.Release()
		return nil, storage.RunBundleSnapshot{}, bundlecontract.CaptureCheck{}, err
	}
	if handle.View().Identity() != state.Catalog {
		handle.Release()
		return nil, storage.RunBundleSnapshot{}, bundlecontract.CaptureCheck{}, errors.New("toolkits: catalog_recovery_required")
	}
	payload := bundlecontract.SnapshotPayload(handle.View(), state.RevocationRevision)
	for index := range payload.Components {
		component := &payload.Components[index]
		if component.Ref.Kind != bundlecontract.Hook && component.Ref.Kind != bundlecontract.MCP {
			continue
		}
		consent, found, consentErr := m.cfg.Store.ReadBundleConsent(ctx, component.Ref.InstallID, component.Ref.Kind, component.Ref.LocalID)
		if consentErr != nil {
			handle.Release()
			return nil, storage.RunBundleSnapshot{}, bundlecontract.CaptureCheck{}, fmt.Errorf("toolkits: read Hook consent: %w", consentErr)
		}
		if found {
			component.ConsentPresent = true
			component.ConsentDigest = consent.DeclarationDigest
			component.ConsentAllowed = consent.Allowed
			component.ConsentRevision = consent.Revision
		}
	}
	encoded, err := bundlecontract.EncodeRunSnapshotPayload(payload)
	if err != nil {
		handle.Release()
		return nil, storage.RunBundleSnapshot{}, bundlecontract.CaptureCheck{}, err
	}
	sum := sha256.Sum256(encoded)
	snapshot := storage.RunBundleSnapshot{
		RunID: runID, SchemaVersion: 1, Payload: encoded, PayloadSHA256: hex.EncodeToString(sum[:]),
		Releases: payload.Releases,
	}
	check := bundlecontract.CaptureCheckFor(payload)
	return handle, snapshot, check, nil
}

// SetBindings replaces one manifest-declared component's host environment
// references. It drains admitted work and publishes the config revision with
// the already-prepared owner projection. Reference names, never values, are
// persisted; the revision change makes executable consents stale.
func (m *Manager) SetBindings(ctx context.Context, request BindingUpdateRequest) (bundlecontract.Operation, error) {
	if m == nil || m.cfg.Store == nil || !bundlecontract.ValidInstallID(request.OperationID) ||
		!bundlecontract.ValidInstallID(request.InstallID) || !bundlecontract.ValidLocalID(request.LocalID) ||
		(request.Kind != bundlecontract.Hook && request.Kind != bundlecontract.MCP) || strings.TrimSpace(request.Subject) == "" ||
		request.ExpectedConfigRevision < 0 || request.Expected.ConfigRevision != request.ExpectedConfigRevision {
		return bundlecontract.Operation{}, bundlecontract.Invalid("binding_update")
	}
	if m.cfg.Gate == nil {
		return bundlecontract.Operation{}, errors.New("toolkits: catalog drain unavailable")
	}
	m.gcMu.Lock()
	defer m.gcMu.Unlock()
	m.mu.Lock()
	if m.closed || m.publishing != "" {
		m.mu.Unlock()
		return bundlecontract.Operation{}, errors.New("toolkits: catalog_update_pending")
	}
	m.publishing = request.OperationID
	m.mu.Unlock()
	clearPublishing := true
	defer func() {
		if clearPublishing {
			m.mu.Lock()
			if m.publishing == request.OperationID {
				m.publishing = ""
			}
			m.mu.Unlock()
		}
	}()
	if err := m.cfg.Gate.BeginDrain(ctx, request.OperationID); err != nil {
		return bundlecontract.Operation{}, err
	}
	draining := true
	defer func() {
		if draining {
			m.cfg.Gate.EndDrain(request.OperationID)
		}
	}()
	if err := m.cfg.Gate.WaitIdle(ctx, request.OperationID); err != nil {
		return bundlecontract.Operation{}, err
	}
	state, err := m.cfg.Store.ReadBundleState(ctx)
	if err != nil {
		return bundlecontract.Operation{}, err
	}
	if state.Catalog != request.Expected {
		return bundlecontract.Operation{}, errors.New("toolkits: revision_conflict")
	}
	var installation *storage.BundleInstallation
	for i := range state.Installations {
		if state.Installations[i].InstallID == request.InstallID && !state.Installations[i].Tombstone {
			installation = &state.Installations[i]
			break
		}
	}
	if installation == nil {
		return bundlecontract.Operation{}, errors.New("toolkits: not_found")
	}
	artifact, err := OpenArtifact(ctx, m.cfg.StateRoot, installation.ReleaseDigest)
	if err != nil {
		return bundlecontract.Operation{}, err
	}
	manifest := artifact.Manifest()
	declared := map[string]bool{}
	if request.Kind == bundlecontract.Hook {
		for _, entry := range manifest.Components.Hooks {
			if entry.ID == request.LocalID {
				for child := range entry.EnvFrom {
					declared["envFrom:"+child] = true
				}
			}
		}
	} else {
		for _, entry := range manifest.Components.MCP {
			if entry.ID == request.LocalID {
				if entry.AuthEnv != "" {
					declared["authEnv"] = true
				}
				for child := range entry.EnvFrom {
					declared["envFrom:"+child] = true
				}
			}
		}
	}
	if len(declared) == 0 {
		return bundlecontract.Operation{}, errors.New("toolkits: component_binding_not_found")
	}
	refs := append([]bundlecontract.BindingRef(nil), request.Bindings...)
	sort.Slice(refs, func(i, j int) bool { return refs[i].Name < refs[j].Name })
	seen := map[string]bool{}
	for _, binding := range refs {
		if !declared[binding.Name] || !bundlecontract.ValidBindingName(binding.Name) || !bundlecontract.ValidBindingReference(binding.Reference) || seen[binding.Name] {
			return bundlecontract.Operation{}, bundlecontract.Invalid("binding_reference")
		}
		seen[binding.Name] = true
	}
	bindings := make([]storage.ToolkitBinding, 0, len(state.Bindings)+len(refs))
	for _, old := range state.Bindings {
		if old.InstallID != request.InstallID || old.Kind != string(request.Kind) || old.LocalID != request.LocalID {
			bindings = append(bindings, old)
		}
	}
	for _, binding := range refs {
		bindings = append(bindings, storage.ToolkitBinding{InstallID: request.InstallID, Kind: string(request.Kind), LocalID: request.LocalID, Name: binding.Name, Reference: binding.Reference})
	}
	active, err := m.AcquireView(ctx)
	if err != nil {
		return bundlecontract.Operation{}, err
	}
	current := active.View()
	active.Release()
	if current.Identity() != request.Expected {
		return bundlecontract.Operation{}, errors.New("toolkits: catalog_recovery_required")
	}
	newRevision := request.Expected.ConfigRevision + 1
	effectiveDigest, err := catalogDigest(newRevision, request.Expected.GenerationID, current.Releases(), bindings)
	if err != nil {
		return bundlecontract.Operation{}, err
	}
	identity := bundlecontract.CatalogIdentity{Epoch: request.Expected.Epoch, ConfigRevision: newRevision, EffectiveDigest: effectiveDigest, GenerationID: request.Expected.GenerationID}
	view, err := m.buildView(ctx, identity, current.Releases(), state.Installations, nil, bindings)
	if err != nil {
		return bundlecontract.Operation{}, err
	}
	owners := PreparedOwners(noopOwners{view: view})
	if m.cfg.PrepareOwners != nil {
		owners, err = m.cfg.PrepareOwners(ctx, view)
		if err != nil {
			return bundlecontract.Operation{}, err
		}
		if owners == nil || owners.View().Identity() != identity {
			if owners != nil {
				_ = owners.Close(ctx)
			}
			return bundlecontract.Operation{}, errors.New("toolkits: prepared_owner_epoch_mismatch")
		}
	}
	inputDigest, err := operationDigest(struct {
		Expected  bundlecontract.CatalogIdentity
		InstallID string
		Kind      bundlecontract.ComponentKind
		LocalID   string
		Bindings  []bundlecontract.BindingRef
		Subject   string
	}{request.Expected, request.InstallID, request.Kind, request.LocalID, refs, request.Subject})
	if err != nil {
		_ = owners.Close(ctx)
		return bundlecontract.Operation{}, err
	}
	commitBindings := make([]storage.ToolkitBinding, 0, len(refs))
	for _, binding := range refs {
		commitBindings = append(commitBindings, storage.ToolkitBinding{InstallID: request.InstallID, Kind: string(request.Kind), LocalID: request.LocalID, Name: binding.Name, Reference: binding.Reference})
	}
	op, err := m.cfg.Store.CommitBundleBindings(ctx, storage.BindingCommit{
		OperationID: request.OperationID, InputDigest: inputDigest, LeaseOwner: m.cfg.LeaseOwner, Subject: request.Subject,
		Expected: request.Expected, InstallID: request.InstallID, Kind: string(request.Kind), LocalID: request.LocalID,
		Bindings: commitBindings, EffectiveDigest: effectiveDigest, NowMS: time.Now().UnixMilli(),
	})
	if err != nil {
		_ = owners.Close(ctx)
		return bundlecontract.Operation{}, err
	}
	m.mu.Lock()
	if m.closed {
		m.recoveryRequired = true
		m.recoveryDrainID = request.OperationID
		m.mu.Unlock()
		_ = owners.Close(ctx)
		clearPublishing = false
		draining = false
		return bundlecontract.Operation{}, errors.New("toolkits: publication_recovery_required")
	}
	old := m.current
	owners.Publish()
	m.current = &viewState{manager: m, view: owners.View(), owners: owners}
	m.publishing = ""
	m.mu.Unlock()
	clearPublishing = false
	m.retire(ctx, old)
	m.cfg.Gate.EndDrain(request.OperationID)
	draining = false
	return op, nil
}

// SetHookConsent arms or disarms one exact PreToolUse declaration.
func (m *Manager) SetHookConsent(ctx context.Context, request HookConsentRequest) (bundlecontract.Operation, error) {
	if request.Ref.Kind != bundlecontract.Hook {
		return bundlecontract.Operation{}, errors.New("toolkits: invalid Hook consent request")
	}
	return m.SetExecutableConsent(ctx, request)
}

// SetExecutableConsent authorizes one exact Hook or MCP declaration. The
// stored digest binds installation, release, declaration, and local config
// revision; changing any of them requires a new human action.
func (m *Manager) SetExecutableConsent(ctx context.Context, request ExecutableConsentRequest) (bundlecontract.Operation, error) {
	if m == nil || (request.Ref.Kind != bundlecontract.Hook && request.Ref.Kind != bundlecontract.MCP) ||
		request.Ref.InstallID == "" || request.Ref.LocalID == "" || strings.TrimSpace(request.Subject) == "" {
		return bundlecontract.Operation{}, errors.New("toolkits: invalid executable consent request")
	}
	if !m.cfg.Capabilities.Owners[request.Ref.Kind] {
		return bundlecontract.Operation{}, errors.New("toolkits: executable capability unavailable")
	}
	if request.OperationID == "" {
		id, err := newID()
		if err != nil {
			return bundlecontract.Operation{}, err
		}
		request.OperationID = id
	}
	state, err := m.cfg.Store.ReadBundleState(ctx)
	if err != nil {
		return bundlecontract.Operation{}, err
	}
	if request.Expected == (bundlecontract.CatalogIdentity{}) {
		request.Expected = state.Catalog
	}
	if request.Expected != state.Catalog {
		return bundlecontract.Operation{}, errors.New("toolkits: revision_conflict")
	}
	var installed storage.BundleInstallation
	installedFound := false
	for _, candidate := range state.Installations {
		if candidate.InstallID == request.Ref.InstallID {
			installed, installedFound = candidate, true
			break
		}
	}
	if !installedFound || installed.Tombstone {
		return bundlecontract.Operation{}, errors.New("toolkits: not_found")
	}
	inspection, err := m.InspectInstallation(ctx, request.Ref.InstallID)
	if err != nil {
		return bundlecontract.Operation{}, err
	}
	if inspection.Installation.Tombstone || inspection.Installation.ReleaseDigest != installed.ReleaseDigest {
		return bundlecontract.Operation{}, errors.New("toolkits: revision_conflict")
	}
	var executable *ExecutableInspection
	for index := range inspection.Executables {
		candidate := &inspection.Executables[index]
		if candidate.Kind == request.Ref.Kind && candidate.LocalID == request.Ref.LocalID {
			executable = candidate
			break
		}
	}
	if executable == nil || executable.DeclarationDigest == "" {
		return bundlecontract.Operation{}, errors.New("toolkits: executable declaration unavailable")
	}
	if request.Ref.Kind == bundlecontract.Hook {
		isPreToolUse := false
		if inspection.Manifest != nil {
			for _, hook := range inspection.Manifest.Components.Hooks {
				if hook.ID == request.Ref.LocalID && hook.Event == "PreToolUse" {
					isPreToolUse = true
					break
				}
			}
		}
		if !isPreToolUse {
			return bundlecontract.Operation{}, errors.New("toolkits: executable declaration unavailable")
		}
	}
	if (request.ExpectedReleaseDigest != "" && request.ExpectedReleaseDigest != executable.ReleaseDigest) ||
		(request.ExpectedDeclarationDigest != "" && request.ExpectedDeclarationDigest != executable.DeclarationDigest) {
		return bundlecontract.Operation{}, errors.New("toolkits: revision_conflict")
	}
	consentDigest, err := bundlecontract.ExecutableConsentDigest(request.Ref.InstallID, executable.ReleaseDigest, executable.DeclarationDigest, state.Catalog.ConfigRevision)
	if err != nil {
		return bundlecontract.Operation{}, err
	}
	currentConsent, found, err := m.cfg.Store.ReadBundleConsent(ctx, request.Ref.InstallID, request.Ref.Kind, request.Ref.LocalID)
	if err != nil {
		return bundlecontract.Operation{}, err
	}
	currentRevision := int64(0)
	if found {
		currentRevision = currentConsent.Revision
	}
	if currentRevision != request.ExpectedRevision {
		return bundlecontract.Operation{}, errors.New("toolkits: revision_conflict")
	}
	inputDigest, err := operationDigest(struct {
		Expected         bundlecontract.CatalogIdentity
		Ref              bundlecontract.ComponentRef
		ConsentDigest    string
		ExpectedRevision int64
		Subject          string
		Allowed          bool
	}{request.Expected, request.Ref, consentDigest, request.ExpectedRevision, request.Subject, request.Allowed})
	if err != nil {
		return bundlecontract.Operation{}, err
	}
	return m.cfg.Store.CommitBundleConsent(ctx, storage.ConsentCommit{
		OperationID: request.OperationID, InputDigest: inputDigest, LeaseOwner: m.cfg.LeaseOwner,
		InstallID: request.Ref.InstallID, Kind: string(request.Ref.Kind), LocalID: request.Ref.LocalID,
		DeclarationDigest: consentDigest, Subject: request.Subject,
		ExpectedEpoch: request.Expected.Epoch, ExpectedConfigRevision: request.Expected.ConfigRevision, ExpectedRevision: request.ExpectedRevision,
		Allowed: request.Allowed, NowMS: time.Now().UnixMilli(),
	})
}
func (m *Manager) retire(ctx context.Context, s *viewState) {
	if s == nil {
		return
	}
	m.mu.Lock()
	s.retired = true
	var owners PreparedOwners
	if s.refs == 0 && !s.closed {
		s.closed = true
		owners = s.owners
	}
	m.mu.Unlock()
	if owners != nil {
		_ = owners.Close(ctx)
	}
}
func (m *Manager) CheckLive(ctx context.Context, c bundlecontract.CaptureCheck) error {
	return m.cfg.Store.CheckBundleRevocations(ctx, c)
}

// RestoreRun reconstructs the exact immutable View referenced by a durable
// snapshot. It never substitutes the currently published catalog. Archived
// releases must still be present and pass the current owner capability gate.
func (m *Manager) RestoreRun(ctx context.Context, snapshot storage.RunBundleSnapshot) (*ViewHandle, bundlecontract.CaptureCheck, error) {
	payload, err := storage.ValidateRunBundleSnapshot(snapshot)
	if err != nil {
		return nil, bundlecontract.CaptureCheck{}, err
	}
	state, err := m.cfg.Store.ReadBundleState(ctx)
	if err != nil {
		return nil, bundlecontract.CaptureCheck{}, err
	}
	for _, expected := range payload.Releases {
		found := false
		for _, stored := range state.Releases {
			if stored == expected {
				found = true
				break
			}
		}
		if !found {
			return nil, bundlecontract.CaptureCheck{}, errors.New("toolkits: run snapshot release is unavailable")
		}
	}
	check := bundlecontract.CaptureCheckFor(payload)
	if err := m.CheckLive(ctx, check); err != nil {
		return nil, bundlecontract.CaptureCheck{}, err
	}
	snapshotBindings := make([]storage.ToolkitBinding, 0)
	for _, component := range payload.Components {
		for _, binding := range component.Bindings {
			snapshotBindings = append(snapshotBindings, storage.ToolkitBinding{InstallID: component.Ref.InstallID, Kind: string(component.Ref.Kind), LocalID: component.Ref.LocalID, Name: binding.Name, Reference: binding.Reference})
		}
	}
	view, err := m.buildView(ctx, payload.Catalog, payload.Releases, state.Installations, nil, snapshotBindings)
	if err != nil {
		return nil, bundlecontract.CaptureCheck{}, err
	}
	owners := PreparedOwners(noopOwners{view: view})
	if m.cfg.PrepareOwners != nil {
		owners, err = m.cfg.PrepareOwners(ctx, view)
		if err != nil {
			return nil, bundlecontract.CaptureCheck{}, err
		}
		if owners == nil || owners.View().Identity() != payload.Catalog {
			if owners != nil {
				_ = owners.Close(ctx)
			}
			return nil, bundlecontract.CaptureCheck{}, errors.New("toolkits: restored owner projection does not match snapshot")
		}
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		_ = owners.Close(ctx)
		return nil, bundlecontract.CaptureCheck{}, errors.New("toolkits: manager closed")
	}
	viewState := &viewState{manager: m, view: owners.View(), owners: owners, refs: 1, retired: true}
	// Restored projections are scoped to this run and must not publish into
	// the process-wide owner set.
	m.mu.Unlock()
	return m.handleLocked(viewState), check, nil
}
func (m *Manager) Recover(ctx context.Context) error {
	m.gcMu.Lock()
	defer m.gcMu.Unlock()
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return errors.New("toolkits: manager closed")
	}
	if m.publishing != "" && !m.recoveryRequired {
		m.mu.Unlock()
		return errors.New("toolkits: catalog_update_pending")
	}
	m.mu.Unlock()
	state, err := m.cfg.Store.ReadBundleState(ctx)
	if err != nil {
		return err
	}
	active := []bundlecontract.ReleaseRef{}
	for _, install := range state.Installations {
		if install.State == "enabled" && !install.Tombstone {
			r, ok := findRelease(state.Releases, install.InstallID, install.ReleaseDigest)
			if !ok {
				return errors.New("toolkits: recovery_release_missing")
			}
			active = append(active, r)
		}
	}
	view, err := m.buildView(ctx, state.Catalog, active, state.Installations, nil, state.Bindings)
	if err != nil {
		return fmt.Errorf("toolkits: recovery: %w", err)
	}
	owners := PreparedOwners(noopOwners{view: view})
	if m.cfg.PrepareOwners != nil {
		owners, err = m.cfg.PrepareOwners(ctx, view)
		if err != nil {
			return err
		}
		if owners == nil {
			return errors.New("toolkits: recovery_owner_missing")
		}
	}
	if owners.View().Identity() != state.Catalog {
		_ = owners.Close(ctx)
		return errors.New("toolkits: recovery_owner_epoch_mismatch")
	}
	var pending *PreparedCatalog
	if state.Pending != nil && state.Pending.Kind == "publish" && state.Pending.State == "pending" {
		if state.Pending.Prepared == nil {
			// A pending row without a candidate receipt never passed the Manager's
			// prepare phase. It is safe to cancel; a committed candidate always
			// persists its receipt transactionally with the pending operation.
			if _, cancelErr := m.cfg.Store.CancelBundleOperation(ctx, storage.CancelOperation{ID: state.Pending.ID, ExpectedRevision: state.Pending.Revision}); cancelErr != nil {
				_ = owners.Close(ctx)
				return fmt.Errorf("toolkits: unprepared operation recovery: %w", cancelErr)
			}
		} else {
			restored, restoreErr := m.restorePrepared(ctx, state, *state.Pending)
			if restoreErr != nil {
				_ = owners.Close(ctx)
				return fmt.Errorf("toolkits: pending operation recovery: %w", restoreErr)
			}
			pending = &restored
		}
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		_ = owners.Close(ctx)
		return errors.New("toolkits: manager closed")
	}
	prepared := m.prepared
	m.prepared = map[string]PreparedCatalog{}
	if pending != nil {
		m.prepared[pending.OperationID] = *pending
		m.publishing = pending.OperationID
	} else {
		m.publishing = ""
	}
	owners.Publish()
	old := m.current
	m.current = &viewState{manager: m, view: owners.View(), owners: owners}
	drainID := m.recoveryDrainID
	m.recoveryDrainID = ""
	m.recoveryRequired = false
	m.publishInFlight = ""
	m.mu.Unlock()
	m.retire(ctx, old)
	var cleanupErrs []error
	for _, candidate := range prepared {
		if closeErr := candidate.owners.Close(ctx); closeErr != nil {
			cleanupErrs = append(cleanupErrs, closeErr)
		}
	}
	if drainID != "" && m.cfg.Gate != nil {
		m.cfg.Gate.EndDrain(drainID)
	}
	return errors.Join(cleanupErrs...)
}

func (m *Manager) restorePrepared(ctx context.Context, state storage.BundleState, operation bundlecontract.Operation) (PreparedCatalog, error) {
	if operation.Prepared == nil || operation.Prepared.InputDigest == "" || operation.Prepared.Expected != state.Catalog || operation.ExpectedEpoch != state.Catalog.Epoch || !bundlecontract.ValidDigest(operation.Prepared.CandidateDigest) {
		return PreparedCatalog{}, errors.New("pending operation has no valid prepared receipt")
	}
	durable := operation.Prepared
	if len(durable.Releases) > 256 || len(durable.Installations) > 256 || len(durable.NewReleases) > 256 {
		return PreparedCatalog{}, errors.New("pending operation receipt exceeds bounds")
	}
	calculated, err := operationDigest(struct {
		ConfigRevision int64
		GenerationID   string
		Releases       []bundlecontract.ReleaseRef
	}{state.Catalog.ConfigRevision, state.Catalog.GenerationID, durable.Releases})
	if err != nil || calculated != durable.CandidateDigest {
		return PreparedCatalog{}, errors.New("pending candidate digest mismatch")
	}
	installs := append([]storage.BundleInstallation(nil), state.Installations...)
	changes := make([]storage.InstallationState, 0, len(durable.Installations))
	for _, change := range durable.Installations {
		found := false
		for i := range installs {
			if installs[i].InstallID != change.InstallID {
				continue
			}
			if installs[i].Revision != change.Revision || installs[i].Tombstone {
				return PreparedCatalog{}, errors.New("pending installation revision mismatch")
			}
			installs[i].State = change.State
			installs[i].ReleaseDigest = change.ReleaseDigest
			installs[i].Tombstone = change.Tombstone
			installs[i].Revision++
			found = true
			break
		}
		if !found {
			return PreparedCatalog{}, errors.New("pending installation is missing")
		}
		changes = append(changes, storage.InstallationState{InstallID: change.InstallID, State: change.State, ReleaseDigest: change.ReleaseDigest, Revision: change.Revision, Tombstone: change.Tombstone})
	}
	candidateIdentity := bundlecontract.CatalogIdentity{Epoch: state.Catalog.Epoch + 1, ConfigRevision: state.Catalog.ConfigRevision, EffectiveDigest: durable.CandidateDigest, GenerationID: state.Catalog.GenerationID}
	view, err := m.buildView(ctx, candidateIdentity, durable.Releases, installs, nil, state.Bindings)
	if err != nil {
		return PreparedCatalog{}, err
	}
	owners := PreparedOwners(noopOwners{view: view})
	if m.cfg.PrepareOwners != nil {
		owners, err = m.cfg.PrepareOwners(ctx, view)
		if err != nil {
			return PreparedCatalog{}, err
		}
		if owners == nil {
			return PreparedCatalog{}, errors.New("pending owner preparation returned nil")
		}
	}
	if owners.View().Identity() != candidateIdentity {
		_ = owners.Close(ctx)
		return PreparedCatalog{}, errors.New("pending owner epoch mismatch")
	}
	mutation := CatalogMutation{Kind: durable.MutationKind, InstallID: durable.MutationInstallID, ReleaseDigest: durable.MutationReleaseDigest, ExpectedRevision: durable.MutationExpectedRevision}
	return PreparedCatalog{OperationID: operation.ID, Expected: durable.Expected, CandidateDigest: durable.CandidateDigest, InputDigest: durable.InputDigest, Operation: operation, Changes: append([]bundlecontract.ComponentChange(nil), durable.Changes...), Releases: append([]bundlecontract.ReleaseRef(nil), durable.Releases...), newReleases: append([]bundlecontract.ReleaseRef(nil), durable.NewReleases...), owners: owners, installations: changes, mutation: mutation}, nil
}
func (m *Manager) Close(ctx context.Context) error {
	m.mu.Lock()
	m.closed = true
	state := m.current
	m.current = nil
	prepared := m.prepared
	m.prepared = map[string]PreparedCatalog{}
	m.previews = map[string]PreparedSource{}
	m.previewUse = map[string]string{}
	publishing := m.publishing
	m.recoveryDrainID = ""
	m.publishing = ""
	m.publishInFlight = ""
	m.mu.Unlock()
	var errs []error
	m.retire(ctx, state)
	for id, p := range prepared {
		if id == publishing {
			continue
		}
		if err := p.owners.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

type ArtifactGCResult struct {
	Quarantined []string
	Deleted     []string
	Retained    []string
}

func validObjectDigest(name string) bool {
	raw, err := hex.DecodeString(name)
	return err == nil && len(raw) == sha256.Size && hex.EncodeToString(raw) == name
}

func readRootEntries(root *os.Root) ([]os.DirEntry, error) {
	f, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.ReadDir(-1)
}

func (m *Manager) gcRetainedDigests() (map[string]struct{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, errors.New("toolkits: manager closed")
	}
	if m.publishing != "" || m.recoveryRequired {
		return nil, errors.New("toolkits: catalog_update_pending")
	}
	retained := map[string]struct{}{}
	for _, preview := range m.previews {
		retained[preview.ReleaseDigest] = struct{}{}
	}
	for _, prepared := range m.prepared {
		for _, release := range prepared.Releases {
			retained[release.ReleaseDigest] = struct{}{}
		}
		for _, release := range prepared.newReleases {
			retained[release.ReleaseDigest] = struct{}{}
		}
	}
	return retained, nil
}

func (m *Manager) gcLiveDigests(ctx context.Context) (map[string]struct{}, error) {
	if err := m.cfg.Store.CheckBundleLease(ctx, m.cfg.LeaseOwner); err != nil {
		return nil, err
	}
	reachability, err := m.cfg.Store.ReadBundleReachability(ctx)
	if err != nil {
		return nil, err
	}
	live := make(map[string]struct{}, len(reachability.Digests))
	for _, digest := range reachability.Digests {
		live[digest] = struct{}{}
	}
	retained, err := m.gcRetainedDigests()
	if err != nil {
		return nil, err
	}
	for digest := range retained {
		live[digest] = struct{}{}
	}
	return live, nil
}

// GarbageCollect is an explicit maintenance operation. It is serialized with
// acquisition/publication and requires the organism lease. Unreachable objects
// are moved to quarantine, then reachability and lease ownership are checked
// again immediately before deletion.
func (m *Manager) GarbageCollect(ctx context.Context) (ArtifactGCResult, error) {
	m.gcMu.Lock()
	defer m.gcMu.Unlock()
	result := ArtifactGCResult{Quarantined: []string{}, Deleted: []string{}, Retained: []string{}}
	live, err := m.gcLiveDigests(ctx)
	if err != nil {
		return result, err
	}
	stateRoot, err := os.OpenRoot(m.cfg.StateRoot)
	if err != nil {
		return result, err
	}
	defer stateRoot.Close()
	if err := stateRoot.MkdirAll("content-bundles", 0700); err != nil {
		return result, err
	}
	baseInfo, err := stateRoot.Lstat("content-bundles")
	if err != nil || !baseInfo.IsDir() || baseInfo.Mode()&os.ModeSymlink != 0 {
		return result, errors.Join(errors.New("toolkits: unsafe_content_root"), err)
	}
	base, err := stateRoot.OpenRoot("content-bundles")
	if err != nil {
		return result, err
	}
	defer base.Close()
	for _, name := range []string{"objects", "quarantine", "staging"} {
		if err := base.MkdirAll(name, 0700); err != nil {
			return result, err
		}
		info, err := base.Lstat(name)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return result, errors.Join(fmt.Errorf("toolkits: unsafe_content_%s", name), err)
		}
	}
	objects, err := base.OpenRoot("objects")
	if err != nil {
		return result, err
	}
	defer objects.Close()
	quarantine, err := base.OpenRoot("quarantine")
	if err != nil {
		return result, err
	}
	defer quarantine.Close()
	staging, err := base.OpenRoot("staging")
	if err != nil {
		return result, err
	}
	defer staging.Close()

	// Finish or restore work interrupted after quarantine rename.
	entries, err := readRootEntries(quarantine)
	if err != nil {
		return result, err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		parts := strings.SplitN(entry.Name(), ".", 2)
		digest := parts[0]
		if !validObjectDigest(digest) {
			continue
		}
		info, err := quarantine.Lstat(entry.Name())
		if err != nil {
			return result, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return result, errors.New("toolkits: unsafe_quarantine_entry")
		}
		live, err = m.gcLiveDigests(ctx)
		if err != nil {
			return result, err
		}
		if _, retained := live[digest]; retained {
			if _, objectErr := objects.Lstat(digest); errors.Is(objectErr, os.ErrNotExist) {
				if err := base.Rename("quarantine/"+entry.Name(), "objects/"+digest); err != nil {
					return result, err
				}
			} else if objectErr == nil {
				if err := quarantine.RemoveAll(entry.Name()); err != nil {
					return result, err
				}
			} else {
				return result, objectErr
			}
			continue
		}
		if err := m.cfg.Store.CheckBundleLease(ctx, m.cfg.LeaseOwner); err != nil {
			return result, err
		}
		latest, err := m.gcLiveDigests(ctx)
		if err != nil {
			return result, err
		}
		if _, retained := latest[digest]; retained {
			if _, objectErr := objects.Lstat(digest); errors.Is(objectErr, os.ErrNotExist) {
				if err := base.Rename("quarantine/"+entry.Name(), "objects/"+digest); err != nil {
					return result, err
				}
			} else if objectErr == nil {
				if err := quarantine.RemoveAll(entry.Name()); err != nil {
					return result, err
				}
			} else {
				return result, objectErr
			}
			continue
		}
		if err := quarantine.RemoveAll(entry.Name()); err != nil {
			return result, err
		}
		result.Deleted = append(result.Deleted, digest)
	}

	// Staging exists only during an in-process acquisition, which is excluded
	// by gcMu. The single-owner lease excludes another process doing the same.
	staged, err := readRootEntries(staging)
	if err != nil {
		return result, err
	}
	for _, entry := range staged {
		info, err := staging.Lstat(entry.Name())
		if err != nil {
			return result, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !bundlecontract.ValidInstallID(entry.Name()) {
			continue
		}
		if err := staging.RemoveAll(entry.Name()); err != nil {
			return result, err
		}
	}

	entries, err = readRootEntries(objects)
	if err != nil {
		return result, err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		digest := entry.Name()
		if !validObjectDigest(digest) {
			continue
		}
		info, err := objects.Lstat(digest)
		if err != nil {
			return result, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return result, errors.New("toolkits: unsafe_object_entry")
		}
		if _, retained := live[digest]; retained {
			result.Retained = append(result.Retained, digest)
			continue
		}
		if err := m.cfg.Store.CheckBundleLease(ctx, m.cfg.LeaseOwner); err != nil {
			return result, err
		}
		name, err := newID()
		if err != nil {
			return result, err
		}
		quarantineName := digest + "." + name
		if err := base.Rename("objects/"+digest, "quarantine/"+quarantineName); err != nil {
			return result, err
		}
		result.Quarantined = append(result.Quarantined, digest)
		latest, err := m.gcLiveDigests(ctx)
		if err != nil {
			return result, err
		}
		if _, retained := latest[digest]; retained {
			if _, objectErr := objects.Lstat(digest); errors.Is(objectErr, os.ErrNotExist) {
				if err := base.Rename("quarantine/"+quarantineName, "objects/"+digest); err != nil {
					return result, err
				}
			} else if objectErr == nil {
				if err := quarantine.RemoveAll(quarantineName); err != nil {
					return result, err
				}
			} else {
				return result, objectErr
			}
			continue
		}
		if err := m.cfg.Store.CheckBundleLease(ctx, m.cfg.LeaseOwner); err != nil {
			return result, err
		}
		latest, err = m.gcLiveDigests(ctx)
		if err != nil {
			return result, err
		}
		if _, retained := latest[digest]; retained {
			if err := base.Rename("quarantine/"+quarantineName, "objects/"+digest); err != nil {
				return result, err
			}
			continue
		}
		if err := quarantine.RemoveAll(quarantineName); err != nil {
			return result, err
		}
		result.Deleted = append(result.Deleted, digest)
	}
	sort.Strings(result.Quarantined)
	sort.Strings(result.Deleted)
	sort.Strings(result.Retained)
	return result, nil
}

func (m *Manager) stateRoot(digest string) string {
	return filepath.Join(m.cfg.StateRoot, "content-bundles", "objects", digest)
}
