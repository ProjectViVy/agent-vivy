package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// ToolkitViewHandle pins one immutable owner projection for a run. Runtime
// carries the value through context and releases the owner lease at terminal.
type ToolkitViewHandle interface {
	View() bundlecontract.View
	Retain() ToolkitViewHandle
	Release()
}

// ToolkitRuntime is the narrow run-time seam over the coordinator. It returns
// a durable capture token and never exposes mutable owner state.
type ToolkitRuntime interface {
	CaptureRun(context.Context, domain.RunID) (ToolkitViewHandle, storage.RunBundleSnapshot, bundlecontract.CaptureCheck, error)
	RestoreRun(context.Context, storage.RunBundleSnapshot) (ToolkitViewHandle, bundlecontract.CaptureCheck, error)
	CheckLive(context.Context, bundlecontract.CaptureCheck) error
}

type bundleViewContextKey struct{}
type bundleCaptureContextKey struct{}

func withBundleView(ctx context.Context, view bundlecontract.View) context.Context {
	return context.WithValue(ctx, bundleViewContextKey{}, view)
}

func bundleView(ctx context.Context) (bundlecontract.View, bool) {
	view, ok := ctx.Value(bundleViewContextKey{}).(bundlecontract.View)
	return view, ok
}

func withBundleCapture(ctx context.Context, check bundlecontract.CaptureCheck) context.Context {
	check.Executables = append([]bundlecontract.ExecutableRef(nil), check.Executables...)
	check.Consents = append([]bundlecontract.ConsentRef(nil), check.Consents...)
	return context.WithValue(ctx, bundleCaptureContextKey{}, check)
}

func bundleCapture(ctx context.Context) (bundlecontract.CaptureCheck, bool) {
	if ctx == nil {
		return bundlecontract.CaptureCheck{}, false
	}
	check, ok := ctx.Value(bundleCaptureContextKey{}).(bundlecontract.CaptureCheck)
	check.Executables = append([]bundlecontract.ExecutableRef(nil), check.Executables...)
	check.Consents = append([]bundlecontract.ConsentRef(nil), check.Consents...)
	return check, ok
}

var (
	ErrCatalogUpdatePending        = errors.New("runtime: catalog update pending")
	ErrBundleDrainUnavailable      = errors.New("runtime: toolkit drain storage is unavailable")
	ErrToolkitAdmissionUnavailable = errors.New("runtime: toolkit admission requires an atomic snapshot store")
	ErrToolkitSnapshotUnavailable  = errors.New("runtime: toolkit run snapshot is unavailable")
)

type bundleFenceContextKey struct{}

func withBundleFence(ctx context.Context, check func(context.Context) error) context.Context {
	return context.WithValue(ctx, bundleFenceContextKey{}, check)
}

func checkBundleFence(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	check, _ := ctx.Value(bundleFenceContextKey{}).(func(context.Context) error)
	if check == nil {
		return nil
	}
	return check(ctx)
}

func (s *Service) ensureToolkitLive(ctx context.Context, runID domain.RunID) error {
	if s == nil || s.deps.Toolkits == nil {
		return nil
	}
	s.mu.Lock()
	check, ok := s.bundleChecks[runID]
	s.mu.Unlock()
	if !ok {
		var err error
		check, ok, err = s.loadToolkitCapture(ctx, runID)
		if err != nil {
			return err
		}
		if !ok {
			// Runs admitted before Toolkit snapshots existed remain readable.
			return nil
		}
	}
	if err := s.deps.Toolkits.CheckLive(ctx, check); err != nil {
		return fmt.Errorf("runtime: check toolkit run fence: %w", err)
	}
	return nil
}

// loadToolkitCapture distinguishes a legacy run.started event from a broken
// Toolkit admission. A marker without its durable snapshot always fails
// closed; only runs with no marker at all use the legacy path.
func (s *Service) loadToolkitCapture(ctx context.Context, runID domain.RunID) (bundlecontract.CaptureCheck, bool, error) {
	marked, marker, err := s.runBundleMarker(ctx, runID)
	if err != nil {
		return bundlecontract.CaptureCheck{}, false, err
	}
	if !marked {
		return bundlecontract.CaptureCheck{}, false, nil
	}
	if s.deps.Bundles == nil {
		return bundlecontract.CaptureCheck{}, false, ErrBundleDrainUnavailable
	}
	snapshot, err := s.deps.Bundles.LoadRunBundleSnapshot(ctx, runID)
	if err != nil {
		return bundlecontract.CaptureCheck{}, false, fmt.Errorf("%w: %v", ErrToolkitSnapshotUnavailable, err)
	}
	payload, err := storage.ValidateRunBundleSnapshot(snapshot)
	if err != nil || snapshot.RunID != runID || marker.SchemaVersion != snapshot.SchemaVersion ||
		marker.Digest != snapshot.PayloadSHA256 || marker.Epoch != payload.Catalog.Epoch ||
		marker.CatalogDigest != payload.Catalog.EffectiveDigest {
		return bundlecontract.CaptureCheck{}, false, fmt.Errorf("%w: durable marker and snapshot differ", ErrToolkitSnapshotUnavailable)
	}
	check := bundlecontract.CaptureCheckFor(payload)
	s.mu.Lock()
	if s.bundleChecks == nil {
		s.bundleChecks = make(map[domain.RunID]bundlecontract.CaptureCheck)
	}
	s.bundleChecks[runID] = check
	s.mu.Unlock()
	return check, true, nil
}

type runBundleMarker struct {
	SchemaVersion int
	Digest        string
	Epoch         int64
	CatalogDigest string
}

func (s *Service) runBundleMarker(ctx context.Context, runID domain.RunID) (bool, runBundleMarker, error) {
	it, err := s.deps.Journal.Replay(ctx, runID, 0)
	if err != nil {
		return false, runBundleMarker{}, err
	}
	defer func() { _ = it.Close() }()
	for it.Next() {
		event := it.Value().Event
		if event.Type != domain.EventRunStarted {
			continue
		}
		var payload payloadRunStarted
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return false, runBundleMarker{}, fmt.Errorf("runtime: decode run.started toolkit marker: %w", err)
		}
		if payload.BundleSchema == 0 && payload.BundleDigest == "" && payload.BundleEpoch == 0 && payload.BundleCatalogDigest == "" {
			return false, runBundleMarker{}, nil
		}
		if payload.BundleSchema <= 0 || payload.BundleDigest == "" || payload.BundleEpoch < 0 ||
			(payload.BundleEpoch == 0 && payload.BundleCatalogDigest != "") ||
			(payload.BundleEpoch > 0 && payload.BundleCatalogDigest == "") {
			return false, runBundleMarker{}, fmt.Errorf("%w: run.started toolkit marker is incomplete", ErrToolkitSnapshotUnavailable)
		}
		return true, runBundleMarker{SchemaVersion: payload.BundleSchema, Digest: payload.BundleDigest,
			Epoch: payload.BundleEpoch, CatalogDigest: payload.BundleCatalogDigest}, nil
	}
	if err := it.Err(); err != nil {
		return false, runBundleMarker{}, fmt.Errorf("runtime: replay run.started toolkit marker: %w", err)
	}
	// Pre-Toolkit and synthetic Run records can legitimately have no
	// RunStarted marker (older child workers may only persist child events).
	// They remain on the legacy surface only when no immutable Toolkit
	// snapshot exists. A snapshot without its matching marker is corruption.
	if s.deps.Bundles != nil {
		if _, snapshotErr := s.deps.Bundles.LoadRunBundleSnapshot(ctx, runID); snapshotErr == nil {
			return false, runBundleMarker{}, fmt.Errorf("%w: Toolkit snapshot exists without a run.started marker", ErrToolkitSnapshotUnavailable)
		} else if !errors.Is(snapshotErr, storage.ErrNotFound) {
			return false, runBundleMarker{}, fmt.Errorf("runtime: read legacy Toolkit snapshot: %w", snapshotErr)
		}
	}
	return false, runBundleMarker{}, nil
}

func (s *Service) restoreToolkitRun(ctx context.Context, runID domain.RunID) (ToolkitViewHandle, bundlecontract.CaptureCheck, bool, error) {
	expectedCheck, marked, err := s.loadToolkitCapture(ctx, runID)
	if err != nil {
		return nil, bundlecontract.CaptureCheck{}, false, err
	}
	if !marked {
		return nil, bundlecontract.CaptureCheck{}, false, nil
	}
	if s.deps.Bundles == nil || s.deps.Toolkits == nil {
		return nil, bundlecontract.CaptureCheck{}, false, ErrToolkitSnapshotUnavailable
	}
	snapshot, err := s.deps.Bundles.LoadRunBundleSnapshot(ctx, runID)
	if err != nil {
		return nil, bundlecontract.CaptureCheck{}, false, fmt.Errorf("%w: %v", ErrToolkitSnapshotUnavailable, err)
	}
	handle, check, err := s.deps.Toolkits.RestoreRun(ctx, snapshot)
	if err != nil {
		return nil, bundlecontract.CaptureCheck{}, false, err
	}
	if handle == nil {
		return nil, bundlecontract.CaptureCheck{}, false, ErrToolkitSnapshotUnavailable
	}
	if expectedJSON, expectedErr := bundlecontract.CanonicalJSON(check); expectedErr != nil {
		handle.Release()
		return nil, bundlecontract.CaptureCheck{}, false, expectedErr
	} else if actualJSON, actualErr := bundlecontract.CanonicalJSON(expectedCheck); actualErr != nil || string(expectedJSON) != string(actualJSON) {
		handle.Release()
		return nil, bundlecontract.CaptureCheck{}, false, fmt.Errorf("%w: restored capture differs from stored snapshot", ErrToolkitSnapshotUnavailable)
	}
	s.mu.Lock()
	if s.bundleViews == nil {
		s.bundleViews = make(map[domain.RunID]ToolkitViewHandle)
	}
	if s.bundleChecks == nil {
		s.bundleChecks = make(map[domain.RunID]bundlecontract.CaptureCheck)
	}
	s.bundleViews[runID] = handle
	s.bundleChecks[runID] = check
	s.mu.Unlock()
	return handle, check, true, nil
}

func (s *Service) inheritToolkitRun(ctx context.Context, parentID, childID domain.RunID) (ToolkitViewHandle, *storage.RunBundleSnapshot, *bundlecontract.CaptureCheck, error) {
	if s.deps.Toolkits == nil {
		return nil, nil, nil, nil
	}
	if s.deps.Bundles == nil {
		return nil, nil, nil, ErrBundleDrainUnavailable
	}
	marked, _, markerErr := s.runBundleMarker(ctx, parentID)
	if markerErr != nil {
		return nil, nil, nil, markerErr
	}
	if !marked {
		// A pre-Toolkit parent has no captured content to inherit. Its child
		// remains on the legacy execution surface and gains no Toolkit view.
		return nil, nil, nil, nil
	}
	s.mu.Lock()
	parentHandle := s.bundleViews[parentID]
	parentCheck, hasCheck := s.bundleChecks[parentID]
	s.mu.Unlock()
	snapshot, err := s.deps.Bundles.LoadRunBundleSnapshot(ctx, parentID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: parent run snapshot: %v", ErrToolkitSnapshotUnavailable, err)
	}
	payload, err := storage.ValidateRunBundleSnapshot(snapshot)
	if err != nil || snapshot.RunID != parentID {
		return nil, nil, nil, fmt.Errorf("%w: parent run snapshot is corrupt", ErrToolkitSnapshotUnavailable)
	}
	derived := bundlecontract.CaptureCheckFor(payload)
	if hasCheck && !sameToolkitCaptureCheck(parentCheck, derived) {
		return nil, nil, nil, fmt.Errorf("%w: parent run capture differs from its snapshot", ErrToolkitSnapshotUnavailable)
	}
	check := derived
	if parentHandle == nil {
		return nil, nil, nil, ErrToolkitSnapshotUnavailable
	}
	handle := parentHandle.Retain()
	if handle == nil {
		return nil, nil, nil, ErrToolkitSnapshotUnavailable
	}
	snapshot.RunID = childID
	snapCopy := snapshot
	checkCopy := check
	return handle, &snapCopy, &checkCopy, nil
}

func sameToolkitCaptureCheck(a, b bundlecontract.CaptureCheck) bool {
	if a.Catalog != b.Catalog || a.RevocationRevision != b.RevocationRevision || len(a.Executables) != len(b.Executables) {
		return false
	}
	for i := range a.Executables {
		if a.Executables[i] != b.Executables[i] {
			return false
		}
	}
	return true
}

func (s *Service) adoptToolkitRun(runID domain.RunID, handle ToolkitViewHandle, check *bundlecontract.CaptureCheck) {
	if handle == nil {
		return
	}
	s.mu.Lock()
	if s.bundleViews == nil {
		s.bundleViews = make(map[domain.RunID]ToolkitViewHandle)
	}
	if s.bundleChecks == nil {
		s.bundleChecks = make(map[domain.RunID]bundlecontract.CaptureCheck)
	}
	s.bundleViews[runID] = handle
	if check != nil {
		copy := *check
		copy.Executables = append([]bundlecontract.ExecutableRef{}, check.Executables...)
		s.bundleChecks[runID] = copy
	}
	s.mu.Unlock()
}

func (s *Service) toolkitContext(ctx context.Context, runID domain.RunID) context.Context {
	s.mu.Lock()
	handle := s.bundleViews[runID]
	check, hasCheck := s.bundleChecks[runID]
	s.mu.Unlock()
	if handle == nil {
		return ctx
	}
	ctx = withBundleView(ctx, handle.View())
	if hasCheck {
		ctx = withBundleCapture(ctx, check)
	}
	ctx = withBundleFence(ctx, func(checkCtx context.Context) error { return s.ensureToolkitLive(checkCtx, runID) })
	if resolver, ok := s.deps.Workspaces.(interface {
		Existing(context.Context, domain.RunID) (Workspace, bool, error)
	}); ok {
		workspace, found, err := resolver.Existing(ctx, runID)
		ctx = withToolkitWorkspaceRoot(ctx, workspace.Path, found, err)
	}
	return ctx
}

func (s *Service) hasToolkitRun(runID domain.RunID) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	_, ok := s.bundleViews[runID]
	s.mu.Unlock()
	return ok
}

func toolkitRunMarkers(snapshot *storage.RunBundleSnapshot, check *bundlecontract.CaptureCheck) (int, string, int64, string) {
	if snapshot == nil || check == nil {
		return 0, "", 0, ""
	}
	return snapshot.SchemaVersion, snapshot.PayloadSHA256, check.Catalog.Epoch, check.Catalog.EffectiveDigest
}

// CatalogUpdatePendingError identifies the publication that temporarily
// excludes new root runs. Callers may safely retry after publication ends.
type CatalogUpdatePendingError struct{ OperationID string }

func (e CatalogUpdatePendingError) Error() string {
	if e.OperationID == "" {
		return ErrCatalogUpdatePending.Error()
	}
	return fmt.Sprintf("%s: %s", ErrCatalogUpdatePending, e.OperationID)
}
func (e CatalogUpdatePendingError) Unwrap() error { return ErrCatalogUpdatePending }

// reserveBundleAdmission shares one lock with BeginDrain. Once BeginDrain
// returns, every previously admitted root is represented by a reservation
// until its durable run row and worker registration are visible.
func (s *Service) reserveBundleAdmission() (func(), error) {
	s.bundleGateMu.Lock()
	if s.bundleDrainID != "" {
		id := s.bundleDrainID
		s.bundleGateMu.Unlock()
		return nil, CatalogUpdatePendingError{OperationID: id}
	}
	s.bundleAdmissions++
	s.bundleGateMu.Unlock()
	return func() {
		s.bundleGateMu.Lock()
		if s.bundleAdmissions > 0 {
			s.bundleAdmissions--
		}
		s.bundleGateMu.Unlock()
	}, nil
}

// BeginDrain closes the root-run admission boundary for one publication.
// Existing workers, resumes and children of admitted roots remain governed
// by their own persisted lineage and can finish while the publication waits.
func (s *Service) BeginDrain(ctx context.Context, operationID string) error {
	if s == nil || operationID == "" {
		return errors.New("runtime: toolkit drain operation id is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.bundleGateMu.Lock()
	defer s.bundleGateMu.Unlock()
	if s.bundleDrainID == operationID {
		return nil
	}
	if s.bundleDrainID != "" {
		return CatalogUpdatePendingError{OperationID: s.bundleDrainID}
	}
	s.bundleDrainID = operationID
	return nil
}

// EndDrain releases only the publication that owns the gate. A stale cleanup
// callback cannot reopen a newer publication's admission window.
func (s *Service) EndDrain(operationID string) {
	if s == nil {
		return
	}
	s.bundleGateMu.Lock()
	released := false
	if s.bundleDrainID == operationID {
		s.bundleDrainID = ""
		released = true
	}
	s.bundleGateMu.Unlock()
	if !released {
		return
	}
	s.mu.Lock()
	stopping := s.stopping
	s.mu.Unlock()
	if !stopping {
		go s.resumeQueuedAfterToolkitDrain()
	}
}

func (s *Service) resumeQueuedAfterToolkitDrain() {
	s.mu.Lock()
	sessions := make([]domain.SessionID, 0, len(s.queues))
	for sessionID := range s.queues {
		sessions = append(sessions, sessionID)
	}
	s.mu.Unlock()
	for _, sessionID := range sessions {
		q, err := s.queueFor(context.Background(), sessionID)
		if err != nil {
			slog.Warn("toolkit publication could not reload queued turns", "session", string(sessionID), "err", err)
			continue
		}
		q.mu.Lock()
		pending := len(q.steer) != 0 || len(q.followUp) != 0
		q.mu.Unlock()
		if pending {
			s.drainFollowUps(sessionID, "")
		}
	}
}

func (s *Service) toolkitDrainState(ctx context.Context, operationID string) (bool, error) {
	s.bundleGateMu.Lock()
	drainID := s.bundleDrainID
	reservations := s.bundleAdmissions
	s.bundleGateMu.Unlock()
	if drainID != operationID || operationID == "" {
		return false, errors.New("runtime: toolkit drain operation does not own the admission gate")
	}
	if reservations > 0 {
		return false, nil
	}

	s.mu.Lock()
	localActive := len(s.active) != 0 || len(s.sessionControls) != 0
	s.mu.Unlock()
	if localActive {
		return false, nil
	}
	s.operationMu.Lock()
	localOperations := len(s.operationFlights) != 0
	s.operationMu.Unlock()
	if localOperations {
		return false, nil
	}
	if s.deps.Bundles == nil {
		return false, ErrBundleDrainUnavailable
	}
	page, err := s.deps.Bundles.ListBundleBlockers(ctx, storage.BlockerQuery{Limit: 100})
	if err != nil {
		return false, fmt.Errorf("runtime: read durable toolkit blockers: %w", err)
	}
	if len(page.Items) != 0 || page.NextCursor != "" {
		return false, nil
	}
	return true, nil
}

// WaitCatalogIdle waits for durable nonterminal runs, active workers, pending
// root admissions, and in-flight tool operations. Storage reads happen with
// no Service lock held, and suspended runs remain visible as durable blockers.
func (s *Service) WaitCatalogIdle(ctx context.Context, operationID string) error {
	if s == nil {
		return errors.New("runtime: nil service cannot drain toolkit publication")
	}
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	for {
		idle, err := s.toolkitDrainState(ctx, operationID)
		if err != nil {
			return err
		}
		if idle {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
