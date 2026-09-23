package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

// DeliverableOperationPresent names the durable operation a model
// present_files call commits; the receipt is keyed on the stable tool_call
// id so a checkpoint resume replays the committed set.
const DeliverableOperationPresent = "deliverable.present"

// Delivery read outcome statuses beyond the shared history vocabulary; they
// answer "can the presented bytes still be served" per SC-D4 §8.
const (
	DeliveryReadChanged              = "changed"
	DeliveryReadMissing              = "missing"
	DeliveryReadWorkspaceUnavailable = "workspace_unavailable"
)

// DeliverableError is the public failure vocabulary for presentation and
// verified reads. Status uses the HistoryStatus vocabulary plus the
// delivery-specific read outcomes above.
type DeliverableError struct {
	Status string
	Reason string
}

func (e DeliverableError) Error() string {
	return fmt.Sprintf("deliverable %s: %s", e.Status, e.Reason)
}

// SessionTransferCloser is the session-deletion hook closing every live
// verified-download transfer of a removed session.
type SessionTransferCloser interface {
	CloseSessionTransfers(sessionID domain.SessionID)
}

// DeliverySetPage is the paged set envelope returned by List.
type DeliverySetPage struct {
	Items      []domain.DeliverySet `json:"items"`
	NextCursor string               `json:"next_cursor,omitempty"`
}

// DeliverableService presents workspace files as immutable delivery sets and
// serves their verified bytes through short-lived transfer snapshots. It
// never accepts caller-supplied workspace roots: every file resolves inside
// the run workspace the WorkspaceManager derives from trusted run/session
// identity.
type DeliverableService struct {
	manager    *WorkspaceManager
	runs       storage.RunStore
	journal    storage.Journal
	continuity storage.ContinuityStore
	transfers  *transferManager
	limits     domain.ContinuityLimits
	now        func() time.Time

	// postReadHook is a test seam executed after hashing a file and before
	// the identity recheck; it simulates a path swap mid-read.
	postReadHook func()
}

// NewDeliverableService wires presentation against the workspace manager,
// the Journal and the T4 receipt transaction. scratch is the dedicated
// transfer snapshot directory; its contents are purged at startup, which is
// the only cleanup allowed there.
func NewDeliverableService(manager *WorkspaceManager, runs storage.RunStore, journal storage.Journal, continuity storage.ContinuityStore, scratch string) (*DeliverableService, error) {
	s := &DeliverableService{manager: manager, runs: runs, journal: journal, continuity: continuity, limits: domain.DefaultContinuityLimits(), now: time.Now}
	// The clock is read through s.now() so tests can replace it after
	// construction and expiry stays deterministic.
	transfers, err := newTransferManager(scratch, func() time.Time { return s.now() })
	if err != nil {
		return nil, err
	}
	s.transfers = transfers
	return s, nil
}

// SetLimits narrows the effective continuity limits; a zero value restores
// the defaults.
func (s *DeliverableService) SetLimits(limits domain.ContinuityLimits) {
	s.limits = limits.Effective(0)
}

// Present fingerprints each requested file independently and commits the
// resulting set as one deliverables.presented event. Successes and failures
// are recorded honestly in the same event; an all-failure request yields a
// failed set rather than a fake success.
func (s *DeliverableService) Present(ctx context.Context, request domain.PresentRequest) (domain.DeliverySet, error) {
	sessionID := tools.SessionIDFromContext(ctx)
	runID := tools.RunIDFromContext(ctx)
	callID := tools.ToolCallIDFromContext(ctx)
	if sessionID == "" || runID == "" || callID == "" {
		return domain.DeliverySet{}, DeliverableError{Status: string(domain.HistoryStatusForbidden), Reason: "present requires an active run tool invocation"}
	}
	if s.continuity == nil || s.journal == nil {
		return domain.DeliverySet{}, DeliverableError{Status: string(domain.HistoryStatusUnavailable), Reason: "deliverable commits unavailable"}
	}
	if err := request.Validate(s.limits); err != nil {
		return domain.DeliverySet{}, DeliverableError{Status: string(domain.HistoryStatusInvalidArgument), Reason: err.Error()}
	}
	seen := make(map[string]struct{}, len(request.Files))
	for _, file := range request.Files {
		if _, dup := seen[file.Path]; dup {
			return domain.DeliverySet{}, DeliverableError{Status: string(domain.HistoryStatusInvalidArgument), Reason: "present request contains duplicate paths"}
		}
		seen[file.Path] = struct{}{}
	}
	workspace, found, err := s.manager.Existing(ctx, runID)
	if err != nil {
		return domain.DeliverySet{}, err
	}
	if !found {
		return domain.DeliverySet{}, DeliverableError{Status: DeliveryReadWorkspaceUnavailable, Reason: "run workspace is unavailable"}
	}
	set := domain.DeliverySet{
		ID:         newPrefixedID("dvs_"),
		SessionID:  sessionID,
		RunID:      runID,
		ToolCallID: callID,
		CreatedAt:  s.now().UnixMilli(),
		Title:      request.Title,
	}
	var total int64
	for _, file := range request.Files {
		item, failure := s.fingerprint(ctx, workspace, file, sessionID, runID, callID, total)
		if failure != nil {
			set.Failures = append(set.Failures, *failure)
			continue
		}
		total += item.Size
		set.Items = append(set.Items, item)
	}
	switch {
	case len(set.Items) == 0:
		set.Status = domain.DeliveryStatusFailed
	case len(set.Failures) > 0:
		set.Status = domain.DeliveryStatusPartial
	default:
		set.Status = domain.DeliveryStatusOK
	}
	if err := set.Validate(s.limits); err != nil {
		return domain.DeliverySet{}, DeliverableError{Status: string(domain.HistoryStatusInvalidArgument), Reason: err.Error()}
	}
	event, err := deliverableSetEvent(set)
	if err != nil {
		return domain.DeliverySet{}, err
	}
	inputHash := deliverableInputHash(request)
	result, err := s.continuity.CommitContinuityOperation(ctx, storage.ContinuityMutation{
		SessionID: sessionID,
		RunID:     runID,
		Event:     event,
		InputHash: inputHash,
		Receipt: domain.ContinuityReceipt{
			SessionID: sessionID,
			Operation: DeliverableOperationPresent,
			RequestID: callID,
			InputHash: inputHash,
		},
	})
	if err != nil {
		return domain.DeliverySet{}, err
	}
	if !result.NewlyCommitted {
		return decodeDeliverySetEvents(result.Events)
	}
	return set, nil
}

// fingerprint opens one workspace file through the shared safe-open helper
// and hashes its bytes from the validated descriptor. Every failure returns
// a safe reason; nothing about the internal path layout is exposed.
func (s *DeliverableService) fingerprint(ctx context.Context, workspace Workspace, file domain.PresentFile, sessionID domain.SessionID, runID domain.RunID, callID string, used int64) (domain.Deliverable, *domain.DeliveryFailure) {
	fail := func(err error) (domain.Deliverable, *domain.DeliveryFailure) {
		public := "file cannot be presented"
		var openErr *secureOpenError
		if errors.As(err, &openErr) {
			public = openErr.public
		}
		return domain.Deliverable{}, &domain.DeliveryFailure{Path: file.Path, Reason: public}
	}
	sf, err := openWorkspaceFileSecure(workspace.Path, file.Path)
	if err != nil {
		return fail(err)
	}
	defer sf.close()
	max := s.limits.PresentFileBytes
	if sf.info.Size() > max {
		return domain.Deliverable{}, &domain.DeliveryFailure{Path: file.Path, Reason: fmt.Sprintf("file exceeds the %d byte limit", max)}
	}
	if used+sf.info.Size() > s.limits.PresentSetBytes {
		return domain.Deliverable{}, &domain.DeliveryFailure{Path: file.Path, Reason: fmt.Sprintf("delivery set exceeds the %d byte limit", s.limits.PresentSetBytes)}
	}
	prefix := make([]byte, 512)
	n, readErr := sf.file.Read(prefix)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return fail(readErr)
	}
	prefix = prefix[:n]
	if _, err := sf.file.Seek(0, io.SeekStart); err != nil {
		return fail(err)
	}
	hash := sha256.New()
	written, err := io.CopyN(hash, ctxReader{ctx: ctx, r: sf.file}, max+1)
	if err != nil && !errors.Is(err, io.EOF) {
		return fail(err)
	}
	if written > max {
		return domain.Deliverable{}, &domain.DeliveryFailure{Path: file.Path, Reason: fmt.Sprintf("file exceeds the %d byte limit", max)}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if s.postReadHook != nil {
		s.postReadHook()
	}
	rootReal, err := filepath.EvalSymlinks(workspace.Path)
	if err != nil {
		return fail(err)
	}
	if err := sf.recheckIdentity(rootReal); err != nil {
		return fail(err)
	}
	return domain.Deliverable{
		ID:               newPrefixedID("dvl_"),
		SessionID:        sessionID,
		RunID:            runID,
		WorkspaceID:      workspace.ID,
		Path:             sf.clean,
		Name:             safeDeliveryName(path.Base(sf.clean)),
		Description:      file.Description,
		Size:             written,
		SHA256:           hex.EncodeToString(hash.Sum(nil)),
		MediaType:        http.DetectContentType(prefix),
		CapturedAt:       s.now().UnixMilli(),
		OriginToolCallID: callID,
	}, nil
}

// List folds the session's committed deliverables.presented events into one
// keyset page ordered by (created_at, id). It reads domain events only; no
// mutable delivery table exists.
func (s *DeliverableService) List(ctx context.Context, sessionID domain.SessionID, cursor string, limit int) (DeliverySetPage, error) {
	if owner := tools.SessionIDFromContext(ctx); owner == "" || owner != sessionID {
		return DeliverySetPage{}, DeliverableError{Status: string(domain.HistoryStatusForbidden), Reason: "list requires the owning session"}
	}
	if limit <= 0 {
		limit = 20
	}
	sets, err := s.sessionDeliverySets(ctx, sessionID)
	if err != nil {
		return DeliverySetPage{}, err
	}
	start := 0
	if cursor != "" {
		key, err := decodeDeliveryCursor(cursor)
		if err != nil {
			return DeliverySetPage{}, DeliverableError{Status: string(domain.HistoryStatusInvalidArgument), Reason: "cursor is invalid"}
		}
		start = sort.Search(len(sets), func(i int) bool {
			return sets[i].CreatedAt > key.at || (sets[i].CreatedAt == key.at && sets[i].ID > key.id)
		})
	}
	page := DeliverySetPage{Items: []domain.DeliverySet{}}
	end := start + limit
	if end > len(sets) {
		end = len(sets)
	}
	page.Items = append(page.Items, sets[start:end]...)
	if end < len(sets) && len(page.Items) > 0 {
		last := page.Items[len(page.Items)-1]
		page.NextCursor = encodeDeliveryCursor(last.CreatedAt, last.ID)
	}
	return page, nil
}

// Get resolves one committed set by id inside the authorized session.
func (s *DeliverableService) Get(ctx context.Context, sessionID domain.SessionID, setID string) (domain.DeliverySet, error) {
	if setID == "" {
		return domain.DeliverySet{}, DeliverableError{Status: string(domain.HistoryStatusInvalidArgument), Reason: "session and set id are required"}
	}
	if owner := tools.SessionIDFromContext(ctx); owner == "" || owner != sessionID {
		return domain.DeliverySet{}, DeliverableError{Status: string(domain.HistoryStatusForbidden), Reason: "get requires the owning session"}
	}
	set, err := s.findSet(ctx, sessionID, setID)
	if err != nil {
		return domain.DeliverySet{}, err
	}
	if set == nil {
		return domain.DeliverySet{}, DeliverableError{Status: string(domain.HistoryStatusNotFound), Reason: "delivery set not found"}
	}
	return *set, nil
}

// Read serves one bounded chunk of a presented file. The first read creates
// a verified temporary snapshot — the complete file is re-hashed and the
// copy is kept only when it still matches the presented digest; subsequent
// chunks must name the live transfer, stay sequential and remain inside the
// owner session.
func (s *DeliverableService) Read(ctx context.Context, request domain.DeliveryReadRequest) (domain.DeliveryChunk, error) {
	sessionID := tools.SessionIDFromContext(ctx)
	if sessionID == "" {
		return domain.DeliveryChunk{}, DeliverableError{Status: string(domain.HistoryStatusForbidden), Reason: "read requires a session"}
	}
	if s.transfers == nil {
		return domain.DeliveryChunk{}, DeliverableError{Status: string(domain.HistoryStatusUnavailable), Reason: "deliverable transfers unavailable"}
	}
	if err := request.Validate(s.limits); err != nil {
		return domain.DeliveryChunk{}, DeliverableError{Status: string(domain.HistoryStatusInvalidArgument), Reason: err.Error()}
	}
	item, set, err := s.findItem(ctx, sessionID, request.ItemID)
	if err != nil {
		return domain.DeliveryChunk{}, err
	}
	if item == nil {
		return domain.DeliveryChunk{}, DeliverableError{Status: string(domain.HistoryStatusNotFound), Reason: "deliverable item not found"}
	}
	if request.ExpectedDigest != item.SHA256 {
		return domain.DeliveryChunk{}, DeliverableError{Status: string(domain.HistoryStatusConflict), Reason: "expected digest does not match the presented item"}
	}
	var tr *deliveryTransfer
	if request.TransferID == "" {
		tr, err = s.openTransfer(ctx, sessionID, *set, *item)
	} else {
		tr, err = s.transfers.lookup(sessionID, request.TransferID, request.ItemID, item.SHA256)
	}
	if err != nil {
		return domain.DeliveryChunk{}, mapDeliverableReadError(err)
	}
	return s.serveChunk(request, tr)
}

// openTransfer verifies the presented file still exists in its original run
// workspace and snapshots its bytes into the transfer scratch. A file that
// moved, vanished or became a link is reported per the SC-D4 access table.
func (s *DeliverableService) openTransfer(ctx context.Context, sessionID domain.SessionID, set domain.DeliverySet, item domain.Deliverable) (*deliveryTransfer, error) {
	workspace, found, err := s.manager.Existing(ctx, set.RunID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, &secureOpenError{public: "workspace is unavailable", cause: fs.ErrNotExist}
	}
	sf, err := openWorkspaceFileSecure(workspace.Path, item.Path)
	if err != nil {
		return nil, err
	}
	defer sf.close()
	rootReal, err := filepath.EvalSymlinks(workspace.Path)
	if err != nil {
		return nil, &secureOpenError{public: "workspace is unavailable", cause: err}
	}
	tr, err := s.transfers.create(ctx, sf, rootReal, sessionID, item.ID, item.SHA256, s.limits.PresentFileBytes)
	if err != nil {
		return nil, err
	}
	return tr, nil
}

// serveChunk reads one page from the verified snapshot. Offset zero or a
// replay of the last served window are allowed; any other offset must
// continue exactly where the last chunk ended.
func (s *DeliverableService) serveChunk(request domain.DeliveryReadRequest, tr *deliveryTransfer) (domain.DeliveryChunk, error) {
	// The verified snapshot is immutable, so re-requesting the last served
	// window replays identical bytes; only offsets past it must advance.
	replay := tr.hasServed && request.Offset == tr.servedStart
	if !replay {
		expected := int64(0)
		if tr.hasServed {
			expected = tr.servedStart + tr.servedLen
		}
		if request.Offset != expected {
			return domain.DeliveryChunk{}, DeliverableError{Status: string(domain.HistoryStatusInvalidArgument), Reason: fmt.Sprintf("offset %d does not continue at %d", request.Offset, expected)}
		}
	}
	file, err := os.Open(tr.snapshot)
	if err != nil {
		return domain.DeliveryChunk{}, fmt.Errorf("runtime: open transfer snapshot: %w", err)
	}
	defer file.Close()
	if _, err := file.Seek(request.Offset, io.SeekStart); err != nil {
		return domain.DeliveryChunk{}, fmt.Errorf("runtime: seek transfer snapshot: %w", err)
	}
	length := request.Length
	if length > s.limits.BinaryPageBytes {
		length = s.limits.BinaryPageBytes
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(length)))
	if err != nil {
		return domain.DeliveryChunk{}, fmt.Errorf("runtime: read transfer snapshot: %w", err)
	}
	eof := request.Offset+int64(len(data)) >= tr.size
	if !replay {
		tr.servedStart = request.Offset
		tr.servedLen = int64(len(data))
		tr.hasServed = true
	}
	return domain.DeliveryChunk{
		TransferID: tr.id,
		ItemID:     request.ItemID,
		Digest:     tr.digest,
		Offset:     request.Offset,
		DataBase64: base64.StdEncoding.EncodeToString(data),
		EOF:        eof,
		ExpiresAt:  tr.expiresAt.UnixMilli(),
	}, nil
}

// CloseTransfer removes one live handle owned by the caller's session.
func (s *DeliverableService) CloseTransfer(ctx context.Context, transferID string) error {
	sessionID := tools.SessionIDFromContext(ctx)
	if sessionID == "" {
		return DeliverableError{Status: string(domain.HistoryStatusForbidden), Reason: "close requires a session"}
	}
	if s.transfers == nil {
		return nil
	}
	return mapDeliverableReadError(s.transfers.close(sessionID, transferID))
}

// CloseSessionTransfers drops every live transfer of a session; the session
// deletion path invokes it so removed sessions cannot keep serving bytes.
func (s *DeliverableService) CloseSessionTransfers(sessionID domain.SessionID) {
	if s == nil || s.transfers == nil {
		return
	}
	s.transfers.closeSession(sessionID)
}

// sessionDeliverySets folds every committed deliverables.presented event of
// the session, oldest first.
func (s *DeliverableService) sessionDeliverySets(ctx context.Context, sessionID domain.SessionID) ([]domain.DeliverySet, error) {
	if s.runs == nil || s.journal == nil {
		return nil, DeliverableError{Status: string(domain.HistoryStatusUnavailable), Reason: "deliverable lookup unavailable"}
	}
	runs, err := s.runs.ListRunsBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	sets := make([]domain.DeliverySet, 0, 4)
	for _, run := range runs {
		iter, err := s.journal.Replay(ctx, run.ID, 0)
		if err != nil {
			return nil, err
		}
		for iter.Next() {
			entry := iter.Value()
			if entry.Event.Type != domain.EventDeliverablesPresented {
				continue
			}
			var payload payloadDeliverablesPresented
			if err := json.Unmarshal(entry.Event.Payload, &payload); err != nil {
				_ = iter.Close()
				return nil, fmt.Errorf("deliverable: decode committed set: %w", err)
			}
			sets = append(sets, payload.DeliverySet)
		}
		if err := iter.Err(); err != nil {
			_ = iter.Close()
			return nil, err
		}
		_ = iter.Close()
	}
	sort.Slice(sets, func(i, j int) bool {
		if sets[i].CreatedAt != sets[j].CreatedAt {
			return sets[i].CreatedAt < sets[j].CreatedAt
		}
		return sets[i].ID < sets[j].ID
	})
	return sets, nil
}

func (s *DeliverableService) findSet(ctx context.Context, sessionID domain.SessionID, setID string) (*domain.DeliverySet, error) {
	sets, err := s.sessionDeliverySets(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	for i := range sets {
		if sets[i].ID == setID {
			return &sets[i], nil
		}
	}
	return nil, nil
}

// findItem resolves an item id through its owning set inside the authorized
// session; arbitrary paths are never accepted by read.
func (s *DeliverableService) findItem(ctx context.Context, sessionID domain.SessionID, itemID string) (*domain.Deliverable, *domain.DeliverySet, error) {
	sets, err := s.sessionDeliverySets(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	for i := range sets {
		for j := range sets[i].Items {
			if sets[i].Items[j].ID == itemID {
				return &sets[i].Items[j], &sets[i], nil
			}
		}
	}
	return nil, nil, nil
}

func mapDeliverableReadError(err error) error {
	var openErr *secureOpenError
	if !errors.As(err, &openErr) {
		return err
	}
	switch {
	case errors.Is(openErr.cause, errSecureChanged):
		return DeliverableError{Status: DeliveryReadChanged, Reason: openErr.public}
	case errors.Is(openErr.cause, fs.ErrNotExist):
		if openErr.public == "workspace is unavailable" {
			return DeliverableError{Status: DeliveryReadWorkspaceUnavailable, Reason: openErr.public}
		}
		if openErr.public == "transfer is not active" {
			return DeliverableError{Status: string(domain.HistoryStatusNotFound), Reason: openErr.public}
		}
		return DeliverableError{Status: DeliveryReadMissing, Reason: openErr.public}
	case errors.Is(openErr.cause, errSecureSymlink), errors.Is(openErr.cause, errSecureOutside), errors.Is(openErr.cause, errSecureIrregular), errors.Is(openErr.cause, errSecureSensitive):
		// A file replaced by a link or special file is refused, never
		// followed and never reported as a successful serve.
		return DeliverableError{Status: string(domain.HistoryStatusForbidden), Reason: openErr.public}
	default:
		return DeliverableError{Status: string(domain.HistoryStatusUnavailable), Reason: openErr.public}
	}
}

type deliveryCursorKey struct {
	at int64
	id string
}

func encodeDeliveryCursor(at int64, id string) string {
	return strconv.FormatInt(at, 36) + ":" + id
}

func decodeDeliveryCursor(cursor string) (deliveryCursorKey, error) {
	at, rest, ok := strings.Cut(cursor, ":")
	if !ok || rest == "" {
		return deliveryCursorKey{}, errors.New("malformed cursor")
	}
	value, err := strconv.ParseInt(at, 36, 64)
	if err != nil {
		return deliveryCursorKey{}, err
	}
	return deliveryCursorKey{at: value, id: rest}, nil
}

func deliverableSetEvent(set domain.DeliverySet) (domain.RunEvent, error) {
	payload, err := json.Marshal(payloadDeliverablesPresented{DeliverySet: set})
	if err != nil {
		return domain.RunEvent{}, fmt.Errorf("deliverable: marshal event payload: %w", err)
	}
	return domain.RunEvent{
		RunID:          set.RunID,
		Type:           domain.EventDeliverablesPresented,
		CreatedAt:      set.CreatedAt,
		PayloadVersion: 1,
		Payload:        payload,
	}, nil
}

func decodeDeliverySetEvents(events []domain.RunEvent) (domain.DeliverySet, error) {
	for _, event := range events {
		if event.Type != domain.EventDeliverablesPresented {
			continue
		}
		var payload payloadDeliverablesPresented
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return domain.DeliverySet{}, fmt.Errorf("deliverable: decode committed event: %w", err)
		}
		return payload.DeliverySet, nil
	}
	return domain.DeliverySet{}, fmt.Errorf("deliverable: committed events carry no delivery set")
}

func deliverableInputHash(request domain.PresentRequest) string {
	data, err := json.Marshal(request)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(append([]byte("v1:"), data...))
	return hex.EncodeToString(sum[:])
}

// safeDeliveryName strips control and bidi characters from a presented file
// name so card rendering cannot be confused by the label itself.
func safeDeliveryName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if unicode.IsControl(r) || (r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069) {
			continue
		}
		b.WriteRune(r)
	}
	clean := strings.TrimSpace(b.String())
	if clean == "" || clean == "." || clean == ".." {
		return "file"
	}
	return clean
}
