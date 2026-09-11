package observerhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/port/observer"
)

const (
	defaultDeliveryTimeout  = 2 * time.Second
	defaultDiagnosticBuffer = 64
)

var (
	ErrInvalidConfig     = errors.New("observerhost: invalid configuration")
	ErrDuplicateProvider = errors.New("observerhost: duplicate provider")
	ErrCursorCorrupt     = errors.New("observerhost: corrupt cursor")
)

type Config struct {
	Journal             storage.Journal
	Cursors             storage.SnapshotStore
	RunProviders        []observer.RunProvider
	DiagnosticProviders []observer.DiagnosticProvider
	DeliveryTimeout     time.Duration
	DiagnosticBuffer    int
}

type Host struct {
	journal         storage.Journal
	cursors         storage.SnapshotStore
	runProviders    []observer.RunProvider
	deliveryTimeout time.Duration

	wake    chan struct{}
	mu      sync.Mutex
	pending map[domain.RunID]struct{}

	diagnostics map[string]chan observer.Diagnostic
	diagByID    map[string]observer.DiagnosticProvider
	drops       map[string]*atomic.Uint64

	startOnce sync.Once
	closeOnce sync.Once
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func New(cfg Config) (*Host, error) {
	if len(cfg.RunProviders) > 0 && (cfg.Journal == nil || cfg.Cursors == nil) {
		return nil, fmt.Errorf("%w: run observers require Journal and cursor store", ErrInvalidConfig)
	}
	deliveryTimeout := cfg.DeliveryTimeout
	if deliveryTimeout <= 0 {
		deliveryTimeout = defaultDeliveryTimeout
	}
	buffer := cfg.DiagnosticBuffer
	if buffer <= 0 {
		buffer = defaultDiagnosticBuffer
	}
	h := &Host{
		journal:         cfg.Journal,
		cursors:         cfg.Cursors,
		deliveryTimeout: deliveryTimeout,
		wake:            make(chan struct{}, 1),
		pending:         make(map[domain.RunID]struct{}),
		diagnostics:     make(map[string]chan observer.Diagnostic),
		diagByID:        make(map[string]observer.DiagnosticProvider),
		drops:           make(map[string]*atomic.Uint64),
	}
	seen := map[string]struct{}{}
	for _, provider := range cfg.RunProviders {
		if provider == nil || strings.TrimSpace(provider.ID()) == "" {
			return nil, ErrInvalidConfig
		}
		id := strings.TrimSpace(provider.ID())
		if _, duplicate := seen["run\x00"+id]; duplicate {
			return nil, fmt.Errorf("%w: run %s", ErrDuplicateProvider, id)
		}
		seen["run\x00"+id] = struct{}{}
		h.runProviders = append(h.runProviders, provider)
	}
	for _, provider := range cfg.DiagnosticProviders {
		if provider == nil || strings.TrimSpace(provider.ID()) == "" {
			return nil, ErrInvalidConfig
		}
		id := strings.TrimSpace(provider.ID())
		if _, duplicate := seen["diagnostic\x00"+id]; duplicate {
			return nil, fmt.Errorf("%w: diagnostic %s", ErrDuplicateProvider, id)
		}
		seen["diagnostic\x00"+id] = struct{}{}
		h.diagByID[id] = provider
		h.diagnostics[id] = make(chan observer.Diagnostic, buffer)
		h.drops[id] = &atomic.Uint64{}
	}
	return h, nil
}

// Start begins isolated delivery workers. Calling Start more than once is safe.
func (h *Host) Start(parent context.Context) {
	h.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(parent)
		h.cancel = cancel
		if len(h.runProviders) > 0 {
			h.wg.Add(1)
			go h.runLoop(ctx)
		}
		for id, provider := range h.diagByID {
			queue := h.diagnostics[id]
			h.wg.Add(1)
			go h.diagnosticLoop(ctx, provider, queue)
		}
	})
}

func (h *Host) Close() {
	h.closeOnce.Do(func() {
		if h.cancel != nil {
			h.cancel()
		}
		h.wg.Wait()
	})
}

// OnRunEvent implements runtime.RunHook. The event body is intentionally not
// trusted here; the worker replays the durable Journal record by RunID.
func (h *Host) OnRunEvent(_ context.Context, event domain.RunEvent) {
	if event.RunID == "" || len(h.runProviders) == 0 {
		return
	}
	h.mu.Lock()
	h.pending[event.RunID] = struct{}{}
	h.mu.Unlock()
	select {
	case h.wake <- struct{}{}:
	default:
	}
}

func (h *Host) runLoop(ctx context.Context) {
	defer h.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.wake:
			for {
				runID, ok := h.takePending()
				if !ok {
					break
				}
				_ = h.DeliverRun(ctx, runID)
			}
		}
	}
}

func (h *Host) takePending() (domain.RunID, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for runID := range h.pending {
		delete(h.pending, runID)
		return runID, true
	}
	return "", false
}

// DeliverRun replays committed events after each provider's durable cursor.
// A provider success followed by cursor-write failure intentionally causes the
// same stable event ID to be redelivered on the next attempt.
func (h *Host) DeliverRun(ctx context.Context, runID domain.RunID) error {
	if h.journal == nil || h.cursors == nil {
		return nil
	}
	var failures []error
	for _, provider := range h.runProviders {
		if err := h.deliverProvider(ctx, provider, runID); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", provider.ID(), err))
		}
	}
	return errors.Join(failures...)
}

func (h *Host) deliverProvider(ctx context.Context, provider observer.RunProvider, runID domain.RunID) error {
	key := cursorKey(provider.ID(), runID)
	cursorBytes, version, err := h.cursors.Get(ctx, key)
	if err != nil {
		return err
	}
	cursor := domain.EventSeq(0)
	if len(cursorBytes) > 0 {
		parsed, parseErr := strconv.ParseInt(string(cursorBytes), 10, 64)
		if parseErr != nil || parsed < 0 {
			return fmt.Errorf("%w: %s", ErrCursorCorrupt, key)
		}
		cursor = domain.EventSeq(parsed)
	}
	iterator, err := h.journal.Replay(ctx, runID, cursor)
	if err != nil {
		return err
	}
	defer iterator.Close()
	for iterator.Next() {
		entry := iterator.Value().Event
		projection := observer.NewRunEvent(
			observer.NewEventID(string(entry.RunID), int64(entry.Seq)),
			string(entry.Type),
			entry.CreatedAt,
			[]byte(tools.RedactSensitive(string(entry.Payload))),
		)
		deliveryCtx, cancel := context.WithTimeout(ctx, h.deliveryTimeout)
		err = provider.ObserveRun(deliveryCtx, projection)
		cancel()
		if err != nil {
			return err
		}
		if err := h.cursors.Put(ctx, key, []byte(strconv.FormatInt(int64(entry.Seq), 10)), version); err != nil {
			return err
		}
		version++
		cursor = entry.Seq
	}
	if err := iterator.Err(); err != nil {
		return err
	}
	_ = cursor
	return nil
}

func cursorKey(providerID string, runID domain.RunID) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(providerID) + "\x00" + string(runID)))
	return "observer/run/" + hex.EncodeToString(sum[:])
}

// EmitDiagnostic never blocks the caller. A full provider queue increments a
// visible drop counter and discards only that provider's diagnostic copy.
func (h *Host) EmitDiagnostic(diagnostic observer.Diagnostic) {
	for id, queue := range h.diagnostics {
		select {
		case queue <- observer.NewDiagnostic(diagnostic.Namespace, diagnostic.Level, diagnostic.Message, diagnostic.Fields):
		default:
			h.drops[id].Add(1)
		}
	}
}

func (h *Host) DiagnosticDrops(providerID string) uint64 {
	counter := h.drops[strings.TrimSpace(providerID)]
	if counter == nil {
		return 0
	}
	return counter.Load()
}

func (h *Host) diagnosticLoop(ctx context.Context, provider observer.DiagnosticProvider, queue <-chan observer.Diagnostic) {
	defer h.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case diagnostic := <-queue:
			func() {
				defer func() { _ = recover() }()
				provider.ObserveDiagnostic(ctx, diagnostic)
			}()
		}
	}
}
