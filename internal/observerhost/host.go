package observerhost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	defaultRetryDelay       = time.Second
	defaultMaxRetryDelay    = time.Minute
)

var (
	ErrInvalidConfig       = errors.New("observerhost: invalid configuration")
	ErrDuplicateProvider   = errors.New("observerhost: duplicate provider")
	ErrCursorCorrupt       = errors.New("observerhost: corrupt cursor")
	ErrInvalidSubscription = errors.New("observerhost: invalid run subscription")
	ErrDeliveryPending     = errors.New("observerhost: delivery pending")
	ErrDeliveryFailed      = errors.New("observerhost: delivery failed")
	ErrInvalidReceipt      = errors.New("observerhost: invalid delivery receipt")
)

// RunSubscription is build-owned policy. Providers cannot subscribe
// themselves or receive fields outside this projection.
type RunSubscription struct {
	Provider             observer.RunProvider
	EventTypes           []string
	AllowedPayloadFields []string
}

type Config struct {
	Journal          storage.Journal
	Cursors          storage.SnapshotStore
	RunSubscriptions []RunSubscription
	// RecoverRunIDs are terminal runs discovered from durable Run state at
	// composition startup. Replaying from each provider cursor makes recovery
	// idempotent and closes the commit-to-hook crash window.
	RecoverRunIDs       []domain.RunID
	DiagnosticProviders []observer.DiagnosticProvider
	DeliveryTimeout     time.Duration
	RetryDelay          time.Duration
	// MaxRetryDelay caps the exponential retry backoff. Delivery is never
	// abandoned, but a persistently failing receiver must not spin at the
	// base retry interval forever.
	MaxRetryDelay    time.Duration
	DiagnosticBuffer int
}

type Host struct {
	journal          storage.Journal
	cursors          storage.SnapshotStore
	runSubscriptions []RunSubscription
	deliveryTimeout  time.Duration
	retryDelay       time.Duration
	maxRetryDelay    time.Duration

	wake    chan struct{}
	mu      sync.Mutex
	pending map[domain.RunID]struct{}
	// attempts counts consecutive failed deliveries per run; the next retry
	// delay doubles per attempt up to maxRetryDelay and resets on success.
	attempts map[domain.RunID]int

	diagnostics map[string]chan observer.Diagnostic
	diagByID    map[string]observer.DiagnosticProvider
	drops       map[string]*atomic.Uint64

	startOnce sync.Once
	closeOnce sync.Once
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func New(cfg Config) (*Host, error) {
	if len(cfg.RunSubscriptions) > 0 && (cfg.Journal == nil || cfg.Cursors == nil) {
		return nil, fmt.Errorf("%w: run observers require Journal and cursor store", ErrInvalidConfig)
	}
	deliveryTimeout := cfg.DeliveryTimeout
	if deliveryTimeout <= 0 {
		deliveryTimeout = defaultDeliveryTimeout
	}
	retryDelay := cfg.RetryDelay
	if retryDelay <= 0 {
		retryDelay = defaultRetryDelay
	}
	maxRetryDelay := cfg.MaxRetryDelay
	if maxRetryDelay <= 0 {
		maxRetryDelay = defaultMaxRetryDelay
	}
	if maxRetryDelay < retryDelay {
		maxRetryDelay = retryDelay
	}
	buffer := cfg.DiagnosticBuffer
	if buffer <= 0 {
		buffer = defaultDiagnosticBuffer
	}
	h := &Host{
		journal:         cfg.Journal,
		cursors:         cfg.Cursors,
		deliveryTimeout: deliveryTimeout,
		retryDelay:      retryDelay,
		maxRetryDelay:   maxRetryDelay,
		wake:            make(chan struct{}, 1),
		pending:         make(map[domain.RunID]struct{}),
		attempts:        make(map[domain.RunID]int),
		diagnostics:     make(map[string]chan observer.Diagnostic),
		diagByID:        make(map[string]observer.DiagnosticProvider),
		drops:           make(map[string]*atomic.Uint64),
	}
	seen := map[string]struct{}{}
	for _, subscription := range cfg.RunSubscriptions {
		provider := subscription.Provider
		if provider == nil || strings.TrimSpace(provider.ID()) == "" {
			return nil, ErrInvalidConfig
		}
		id := strings.TrimSpace(provider.ID())
		if _, duplicate := seen["run\x00"+id]; duplicate {
			return nil, fmt.Errorf("%w: run %s", ErrDuplicateProvider, id)
		}
		seen["run\x00"+id] = struct{}{}
		normalized, err := normalizeSubscription(subscription)
		if err != nil {
			return nil, err
		}
		h.runSubscriptions = append(h.runSubscriptions, normalized)
	}
	for _, runID := range cfg.RecoverRunIDs {
		if runID != "" {
			h.pending[runID] = struct{}{}
		}
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
		if len(h.runSubscriptions) > 0 {
			h.wg.Add(1)
			go h.runLoop(ctx)
			if len(h.pending) > 0 {
				select {
				case h.wake <- struct{}{}:
				default:
				}
			}
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
	if event.RunID == "" || len(h.runSubscriptions) == 0 {
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
				if err := h.DeliverRun(ctx, runID); err != nil {
					h.scheduleRetry(ctx, runID, retryDelayFor(h.retryDelay, h.maxRetryDelay, h.noteFailure(runID)))
					continue
				}
				h.clearAttempts(runID)
			}
		}
	}
}

// retryDelayFor doubles the base delay once per prior failed attempt and caps
// the result at max. attempts <= 1 yields the base delay itself.
func retryDelayFor(base, max time.Duration, attempts int) time.Duration {
	delay := base
	for attempt := 1; attempt < attempts && delay < max; attempt++ {
		delay *= 2
	}
	if delay > max {
		return max
	}
	return delay
}

func (h *Host) noteFailure(runID domain.RunID) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.attempts[runID]++
	return h.attempts[runID]
}

func (h *Host) clearAttempts(runID domain.RunID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.attempts, runID)
}

func (h *Host) scheduleRetry(ctx context.Context, runID domain.RunID, delay time.Duration) {
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		h.mu.Lock()
		h.pending[runID] = struct{}{}
		h.mu.Unlock()
		select {
		case h.wake <- struct{}{}:
		default:
		}
	}()
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
	for _, subscription := range h.runSubscriptions {
		if err := h.deliverProvider(ctx, subscription, runID); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", subscription.Provider.ID(), err))
		}
	}
	return errors.Join(failures...)
}

func (h *Host) deliverProvider(ctx context.Context, subscription RunSubscription, runID domain.RunID) error {
	provider := subscription.Provider
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
	// Snapshot the committed replay before writing cursor state. Some storage
	// engines deliberately expose a single database connection, so holding a
	// streaming read open while Snapshot.Put starts a transaction would
	// deadlock. Stable event IDs make replaying this bounded run slice safe.
	entries := make([]domain.RunEvent, 0)
	for iterator.Next() {
		entries = append(entries, iterator.Value().Event)
	}
	if err := iterator.Err(); err != nil {
		_ = iterator.Close()
		return err
	}
	if err := iterator.Close(); err != nil {
		return err
	}
	for _, entry := range entries {
		if subscriptionAllows(subscription, string(entry.Type)) {
			payload, projectErr := projectPayload(entry.Payload, subscription.AllowedPayloadFields)
			if projectErr != nil {
				return projectErr
			}
			projection := observer.NewRunEvent(
				observer.NewEventID(string(entry.RunID), int64(entry.Seq)),
				string(entry.Type),
				entry.CreatedAt,
				payload,
			)
			deliveryCtx, cancel := context.WithTimeout(ctx, h.deliveryTimeout)
			err = deliverEvent(deliveryCtx, provider, projection)
			cancel()
			if err != nil {
				return err
			}
		}
		if err := h.cursors.Put(ctx, key, []byte(strconv.FormatInt(int64(entry.Seq), 10)), version); err != nil {
			return err
		}
		version++
		cursor = entry.Seq
	}
	_ = cursor
	return nil
}

func normalizeSubscription(input RunSubscription) (RunSubscription, error) {
	if input.Provider == nil || len(input.EventTypes) == 0 {
		return RunSubscription{}, ErrInvalidSubscription
	}
	out := RunSubscription{Provider: input.Provider}
	seenTypes := map[string]struct{}{}
	for _, value := range input.EventTypes {
		value = strings.TrimSpace(value)
		if !domain.EventType(value).Valid() {
			return RunSubscription{}, fmt.Errorf("%w: event type", ErrInvalidSubscription)
		}
		if _, duplicate := seenTypes[value]; duplicate {
			return RunSubscription{}, fmt.Errorf("%w: duplicate event type", ErrInvalidSubscription)
		}
		seenTypes[value] = struct{}{}
		out.EventTypes = append(out.EventTypes, value)
	}
	seenFields := map[string]struct{}{}
	for _, value := range input.AllowedPayloadFields {
		value = strings.TrimSpace(value)
		if !validProjectionField(value) {
			return RunSubscription{}, fmt.Errorf("%w: payload field", ErrInvalidSubscription)
		}
		if _, duplicate := seenFields[value]; duplicate {
			return RunSubscription{}, fmt.Errorf("%w: duplicate payload field", ErrInvalidSubscription)
		}
		seenFields[value] = struct{}{}
		out.AllowedPayloadFields = append(out.AllowedPayloadFields, value)
	}
	return out, nil
}

func validProjectionField(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index, r := range value {
		if !(r == '_' || r >= 'a' && r <= 'z' || index > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func subscriptionAllows(subscription RunSubscription, eventType string) bool {
	for _, allowed := range subscription.EventTypes {
		if allowed == eventType {
			return true
		}
	}
	return false
}

func projectPayload(raw []byte, allowed []string) ([]byte, error) {
	if len(allowed) == 0 {
		return json.RawMessage(`{}`), nil
	}
	var source map[string]json.RawMessage
	if err := json.Unmarshal(raw, &source); err != nil {
		return nil, fmt.Errorf("observerhost: project payload: %w", err)
	}
	projected := make(map[string]json.RawMessage, len(allowed))
	for _, field := range allowed {
		if value, ok := source[field]; ok {
			redacted, err := redactJSONValue(value)
			if err != nil {
				return nil, fmt.Errorf("observerhost: redact payload field %s: %w", field, err)
			}
			projected[field] = redacted
		}
	}
	encoded, err := json.Marshal(projected)
	if err != nil {
		return nil, fmt.Errorf("observerhost: encode projection: %w", err)
	}
	return encoded, nil
}

func redactJSONValue(raw json.RawMessage) (json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	value = redactJSONStrings(value)
	return json.Marshal(value)
}

func redactJSONStrings(value any) any {
	switch typed := value.(type) {
	case string:
		return tools.RedactSensitive(typed)
	case map[string]any:
		for key, child := range typed {
			if sensitiveJSONField(key) {
				typed[key] = "[REDACTED]"
				continue
			}
			typed[key] = redactJSONStrings(child)
		}
	case []any:
		for index, child := range typed {
			typed[index] = redactJSONStrings(child)
		}
	}
	return value
}

func sensitiveJSONField(key string) bool {
	key = strings.ToLower(key)
	for _, marker := range []string{"token", "secret", "password", "passwd", "api_key", "apikey", "authorization", "credential"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}

func deliverEvent(ctx context.Context, provider observer.RunProvider, event observer.RunEvent) error {
	if receiptProvider, ok := provider.(observer.ReceiptRunProvider); ok {
		receipt, err := receiptProvider.ObserveRunWithReceipt(ctx, event)
		if err != nil {
			return err
		}
		if receipt.EventID != event.ID || receipt.ReceiptID == "" {
			return ErrInvalidReceipt
		}
		switch receipt.State {
		case observer.DeliveryAccepted, observer.DeliveryCompleted:
			return nil
		case observer.DeliveryPending:
			return ErrDeliveryPending
		case observer.DeliveryFailed:
			return ErrDeliveryFailed
		default:
			return ErrInvalidReceipt
		}
	}
	return provider.ObserveRun(ctx, event)
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
