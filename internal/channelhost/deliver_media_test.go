package channelhost

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/channelhost/fake"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	plugin "agent-vivy/sdk/port/channel"
)

// mediaChannel wraps a fake with a recording outbound-media face. failNext
// makes the next SendMedia calls fail so the retry path can be exercised.
type mediaChannel struct {
	*fake.Channel

	mu       sync.Mutex
	calls    [][]plugin.Part
	chatIDs  []string
	total    int
	failNext int
}

// capabilitySourceChannel models the production assembly wrapper: the base
// Channel lives on the wrapper while optional capabilities live on its target.
type capabilitySourceChannel struct {
	plugin.Channel
	target any
}

func (c *capabilitySourceChannel) CapabilityTarget() any { return c.target }

func (m *mediaChannel) SendMedia(_ context.Context, chatID string, parts []plugin.Part) ([]string, error) {
	m.mu.Lock()
	m.total++
	if m.failNext > 0 {
		m.failNext--
		m.mu.Unlock()
		return nil, errors.New("media upload refused")
	}
	m.calls = append(m.calls, append([]plugin.Part(nil), parts...))
	m.chatIDs = append(m.chatIDs, chatID)
	m.mu.Unlock()
	return []string{"m-1"}, nil
}

func (m *mediaChannel) snapshot() ([][]plugin.Part, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls, m.failNext
}

func (m *mediaChannel) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.total
}

func waitDeliverySettled(t *testing.T, h *Host, runID domain.RunID) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		open, err := h.deps.Deliveries.ListOpenChannelDeliveries(context.Background())
		if err == nil {
			openForRun := false
			for _, d := range open {
				if d.RunID == runID {
					openForRun = true
				}
			}
			if !openForRun {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("delivery for %s never settled; open rows: %+v (err=%v)", runID, open, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestOutboundMediaRidesTheDeliveryLedger: a completed run whose assistant
// row carries media delivers the text through Send and the images through
// SendMedia in one batch; success deletes the intent.
func TestOutboundMediaRidesTheDeliveryLedger(t *testing.T) {
	mc := &mediaChannel{Channel: fake.New()}
	bound := &capabilitySourceChannel{Channel: mc.Channel, target: mc}
	media := []domain.Attachment{
		{Name: "photo-1.jpg", MimeType: "image/jpeg", Data: mediaTestJPEG()},
	}
	// The recorder persists the user row; wrap it to also append the
	// assistant reply with its media.
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	run := func(ctx context.Context, sessionID domain.SessionID, text string, atts []domain.Attachment, prov *domain.Provenance) (domain.RunID, error) {
		runID, err := runs.run(ctx, sessionID, text, atts, prov)
		if err != nil {
			return runID, err
		}
		if err := backend.AppendMessage(ctx, domain.Message{
			ID: "msg-assistant-media", SessionID: sessionID, RunID: runID,
			Role: domain.RoleAssistant, CreatedAt: time.Now().UnixMilli(),
			Content: "here you go", Attachments: media,
		}); err != nil {
			return runID, err
		}
		return runID, nil
	}
	host := New(Deps{
		Journal:    backend,
		Messages:   backend,
		Sessions:   backend,
		Deliveries: backend,
		Run:        run,
		Channels:   []plugin.Channel{bound},
		Config:     config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:     testLogger(),
	})
	ctx := context.Background()
	if err := host.envFor(mc.Channel).PublishInbound(ctx, plugin.InboundMessage{
		Channel: "fake", ChatID: "chat-9", Sender: "alice", MessageID: "m-in",
		Parts: []plugin.Part{{Kind: plugin.PartText, Text: "draw me"}},
	}); err != nil {
		t.Fatalf("publish inbound: %v", err)
	}
	calls := runs.snapshot()
	if len(calls) != 1 {
		t.Fatalf("run calls = %d, want 1", len(calls))
	}
	host.OnRunEvent(ctx, domain.RunEvent{
		RunID: calls[0].runID, Type: domain.EventRunCompleted,
		CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1,
	})

	waitDeliverySettled(t, host, calls[0].runID)
	sent := mc.Snapshot()
	if len(sent) != 1 || sent[0].Parts[0].Text != "here you go" {
		t.Fatalf("text delivery = %+v", sent)
	}
	mediaCalls, _ := mc.snapshot()
	if len(mediaCalls) != 1 || len(mediaCalls[0]) != 1 {
		t.Fatalf("media calls = %+v, want one batch of one part", mediaCalls)
	}
	part := mediaCalls[0][0]
	if part.Kind != plugin.PartMedia || part.Media.Name != "photo-1.jpg" ||
		part.Media.MimeType != "image/jpeg" || len(part.Media.Data) == 0 {
		t.Fatalf("media part = %+v", part)
	}
	if mc.chatIDs[0] != "chat-9" {
		t.Fatalf("media chat id = %q, want chat-9", mc.chatIDs[0])
	}
}

// TestOutboundMediaRetryReuploads: a media failure fails the attempt and
// the retry re-reads the bytes and uploads again — at-least-once with
// fresh bytes, then the success clears the intent.
func TestOutboundMediaRetryReuploads(t *testing.T) {
	mc := &mediaChannel{Channel: fake.New(), failNext: 1}
	media := []domain.Attachment{
		{Name: "photo-2.png", MimeType: "image/png", Data: mediaTestPNG()},
	}
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	run := func(ctx context.Context, sessionID domain.SessionID, text string, atts []domain.Attachment, prov *domain.Provenance) (domain.RunID, error) {
		runID, err := runs.run(ctx, sessionID, text, atts, prov)
		if err != nil {
			return runID, err
		}
		if err := backend.AppendMessage(ctx, domain.Message{
			ID: "msg-assistant-media", SessionID: sessionID, RunID: runID,
			Role: domain.RoleAssistant, CreatedAt: time.Now().UnixMilli(),
			Content: "picture", Attachments: media,
		}); err != nil {
			return runID, err
		}
		return runID, nil
	}
	host := New(Deps{
		Journal:    backend,
		Messages:   backend,
		Sessions:   backend,
		Deliveries: backend,
		Run:        run,
		Channels:   []plugin.Channel{mc},
		Config:     config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:     testLogger(),
	})
	ctx := context.Background()
	if err := host.envFor(mc.Channel).PublishInbound(ctx, plugin.InboundMessage{
		Channel: "fake", ChatID: "chat-9", Sender: "alice", MessageID: "m-in",
		Parts: []plugin.Part{{Kind: plugin.PartText, Text: "draw me"}},
	}); err != nil {
		t.Fatalf("publish inbound: %v", err)
	}
	calls := runs.snapshot()
	host.OnRunEvent(ctx, domain.RunEvent{
		RunID: calls[0].runID, Type: domain.EventRunCompleted,
		CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1,
	})

	waitDeliverySettled(t, host, calls[0].runID)
	if got := mc.callCount(); got != 2 {
		t.Fatalf("SendMedia calls = %d, want 2 (one failed attempt, one retry)", got)
	}
	mediaCalls, _ := mc.snapshot()
	for _, batch := range mediaCalls {
		if len(batch) != 1 || batch[0].Media.Name != "photo-2.png" || len(batch[0].Media.Data) == 0 {
			t.Fatalf("media batch = %+v", batch)
		}
	}
}

// TestOutboundMediaWithoutSenderStillDelivers: an ear without a MediaSender
// gets the text and a warning; the delivery succeeds.
func TestOutboundMediaWithoutSenderStillDelivers(t *testing.T) {
	fc := fake.New()
	media := []domain.Attachment{
		{Name: "photo-3.jpg", MimeType: "image/jpeg", Data: mediaTestJPEG()},
	}
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	run := func(ctx context.Context, sessionID domain.SessionID, text string, atts []domain.Attachment, prov *domain.Provenance) (domain.RunID, error) {
		runID, err := runs.run(ctx, sessionID, text, atts, prov)
		if err != nil {
			return runID, err
		}
		if err := backend.AppendMessage(ctx, domain.Message{
			ID: "msg-assistant-media", SessionID: sessionID, RunID: runID,
			Role: domain.RoleAssistant, CreatedAt: time.Now().UnixMilli(),
			Content: "plain reply", Attachments: media,
		}); err != nil {
			return runID, err
		}
		return runID, nil
	}
	host := New(Deps{
		Journal:    backend,
		Messages:   backend,
		Sessions:   backend,
		Deliveries: backend,
		Run:        run,
		Channels:   []plugin.Channel{fc},
		Config:     config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:     testLogger(),
	})
	ctx := context.Background()
	if err := host.envFor(fc).PublishInbound(ctx, plugin.InboundMessage{
		Channel: "fake", ChatID: "chat-9", Sender: "alice", MessageID: "m-in",
		Parts: []plugin.Part{{Kind: plugin.PartText, Text: "hi"}},
	}); err != nil {
		t.Fatalf("publish inbound: %v", err)
	}
	calls := runs.snapshot()
	host.OnRunEvent(ctx, domain.RunEvent{
		RunID: calls[0].runID, Type: domain.EventRunCompleted,
		CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1,
	})

	waitDeliverySettled(t, host, calls[0].runID)
	sent := fc.Snapshot()
	if len(sent) != 1 || sent[0].Parts[0].Text != "plain reply" {
		t.Fatalf("text delivery = %+v", sent)
	}
}

// TestOutboundMediaInvalidPartsDropped: the host re-validates outbound
// parts against the shared limits — an oversize or lying-MIME attachment
// is dropped with a log, the valid sibling still ships.
func TestOutboundMediaInvalidPartsDropped(t *testing.T) {
	mc := &mediaChannel{Channel: fake.New()}
	oversize := append(mediaTestPNG(), make([]byte, 5<<20)...)
	media := []domain.Attachment{
		{Name: "big.png", MimeType: "image/png", Data: oversize},
		{Name: "liar.jpg", MimeType: "image/jpeg", Data: mediaTestPNG()}, // png bytes
		{Name: "ok.jpg", MimeType: "image/jpeg", Data: mediaTestJPEG()},
	}
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	run := func(ctx context.Context, sessionID domain.SessionID, text string, atts []domain.Attachment, prov *domain.Provenance) (domain.RunID, error) {
		runID, err := runs.run(ctx, sessionID, text, atts, prov)
		if err != nil {
			return runID, err
		}
		if err := backend.AppendMessage(ctx, domain.Message{
			ID: "msg-assistant-media", SessionID: sessionID, RunID: runID,
			Role: domain.RoleAssistant, CreatedAt: time.Now().UnixMilli(),
			Content: "mixed", Attachments: media,
		}); err != nil {
			return runID, err
		}
		return runID, nil
	}
	host := New(Deps{
		Journal:    backend,
		Messages:   backend,
		Sessions:   backend,
		Deliveries: backend,
		Run:        run,
		Channels:   []plugin.Channel{mc},
		Config:     config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:     testLogger(),
	})
	ctx := context.Background()
	if err := host.envFor(mc.Channel).PublishInbound(ctx, plugin.InboundMessage{
		Channel: "fake", ChatID: "chat-9", Sender: "alice", MessageID: "m-in",
		Parts: []plugin.Part{{Kind: plugin.PartText, Text: "hi"}},
	}); err != nil {
		t.Fatalf("publish inbound: %v", err)
	}
	calls := runs.snapshot()
	host.OnRunEvent(ctx, domain.RunEvent{
		RunID: calls[0].runID, Type: domain.EventRunCompleted,
		CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1,
	})

	waitDeliverySettled(t, host, calls[0].runID)
	mediaCalls, _ := mc.snapshot()
	if len(mediaCalls) != 1 || len(mediaCalls[0]) != 1 || mediaCalls[0][0].Media.Name != "ok.jpg" {
		t.Fatalf("media calls = %+v, want one batch carrying only ok.jpg", mediaCalls)
	}
}

// TestDiscoverReportsMediaSenderBit: an ear implementing the batch
// MediaSender advertises the Media capability through Discover — the
// channel/inspect surface reflects it without extra wiring.
func TestDiscoverReportsMediaSenderBit(t *testing.T) {
	caps := Discover(&mediaChannel{Channel: fake.New()})
	if !caps.Media {
		t.Fatalf("capabilities = %+v, want Media", caps)
	}
	plain := Discover(fake.New())
	if plain.Media {
		t.Fatalf("plain fake capabilities = %+v, want no Media", plain)
	}
}
