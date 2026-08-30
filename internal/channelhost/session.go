package channelhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// ChannelSessionID maps a platform chat to the deterministic Vivy session
// id: "sess_ch_" + the first 16 hex chars of sha256 over
// channel + "\x00" + chatID + "\x00" + topicID. The prefix keeps channel
// sessions and UI sessions disjoint by construction (ids minted by the UI
// never collide with this namespace), and the digest makes the mapping
// stable across restarts without a new storage table.
func ChannelSessionID(channel, chatID, topicID string) domain.SessionID {
	sum := sha256.Sum256([]byte(channel + "\x00" + chatID + "\x00" + topicID))
	return domain.SessionID("sess_ch_" + hex.EncodeToString(sum[:8]))
}

// EnsureSession returns the mapped session for the chat, creating it on
// first contact. Created sessions carry no explicit sandbox knobs, so
// Session.EffectiveSandbox applies the product defaults. A concurrent
// create for the same chat loses the unique insert and re-reads the
// winner's row.
func (h *Host) EnsureSession(ctx context.Context, channel, chatID, topicID string) (domain.SessionID, error) {
	id := ChannelSessionID(channel, chatID, topicID)
	if _, err := h.deps.Sessions.GetSession(ctx, id); err == nil {
		return id, nil
	} else if !errors.Is(err, storage.ErrNotFound) {
		return "", fmt.Errorf("channelhost: read session %s: %w", id, err)
	}
	title := "channel/" + channel + "/" + chatID
	if topicID != "" {
		title += "#" + topicID
	}
	err := h.deps.Sessions.CreateSession(ctx, domain.Session{
		ID:        id,
		Title:     title,
		CreatedAt: time.Now().UnixMilli(),
	})
	if err != nil {
		// Another dispatch for the same chat may have created it first.
		if _, getErr := h.deps.Sessions.GetSession(ctx, id); getErr == nil {
			return id, nil
		}
		return "", fmt.Errorf("channelhost: create channel session: %w", err)
	}
	return id, nil
}
