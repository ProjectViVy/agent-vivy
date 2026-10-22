package channelhost

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

// ChannelSessionID allocates the new-session namespace from the entire platform
// tuple. Components are independently encoded, so separators and NUL bytes
// cannot merge distinct tuples. Existing sessions resolve their persisted
// binding first and retain their historical IDs.
func ChannelSessionID(channel, chatID, topicID string) domain.SessionID {
	encode := func(value string) string { return base64.RawURLEncoding.EncodeToString([]byte(value)) }
	return domain.SessionID("sess_ch_v2_" + encode(channel) + "." + encode(chatID) + "." + encode(topicID))
}

// EnsureSession returns the mapped session for the chat, creating it on
// first contact. Created sessions carry no explicit sandbox knobs, so
// Session.EffectiveSandbox applies the product defaults. A concurrent
// create for the same chat loses the unique insert and re-reads the
// winner's row.
func (h *Host) EnsureSession(ctx context.Context, channel, chatID, topicID string) (domain.SessionID, error) {
	owner, ok := h.deps.Sessions.(storage.ChannelSessionStore)
	if !ok {
		return "", fmt.Errorf("channelhost: channel session binding unavailable")
	}
	binding := domain.ChannelSessionBinding{Channel: channel, ChatID: chatID, TopicID: topicID}
	if id, err := owner.FindChannelSession(ctx, binding); err == nil {
		return id, nil
	} else if !errors.Is(err, storage.ErrNotFound) {
		return "", fmt.Errorf("channelhost: read channel session binding: %w", err)
	}
	id := ChannelSessionID(channel, chatID, topicID)
	title := "channel/" + channel + "/" + chatID
	if topicID != "" {
		title += "#" + topicID
	}
	err := h.deps.Sessions.CreateSession(ctx, domain.Session{
		ID:             id,
		Title:          title,
		CreatedAt:      time.Now().UnixMilli(),
		ChannelBinding: &binding,
	})
	if err != nil {
		// Another dispatch for the same chat may have created it first.
		if winner, getErr := owner.FindChannelSession(ctx, binding); getErr == nil {
			return winner, nil
		}
		return "", fmt.Errorf("channelhost: create channel session: %w", err)
	}
	return id, nil
}
