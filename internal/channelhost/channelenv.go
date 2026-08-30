package channelhost

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"agent-vivy/sdk/plugin"
)

// hostEnv is the world a channel adapter may touch: the per-plugin
// implementation of plugin.ChannelEnv handed to Channel.Start. Secrets
// fail closed on a missing grant or a missing value and are never logged;
// HTTP is outbound-only; there is no Listen capability on this surface.
type hostEnv struct {
	host   *Host
	seam   plugin.Channel // owning adapter, for grant checks
	client *http.Client
}

// envFor builds the ChannelEnv handed to one adapter's Start.
func (h *Host) envFor(ch plugin.Channel) plugin.ChannelEnv {
	return &hostEnv{host: h, seam: ch, client: h.client}
}

// Secret resolves a configured env_key name to its value. Fail-closed:
// without the secret.read grant, or when the variable is unset or empty,
// it returns an error. Values are never logged (D-010).
func (e *hostEnv) Secret(envKey string) (string, error) {
	if !e.hasGrant(plugin.GrantSecretRead) {
		return "", plugin.ErrDenied
	}
	if strings.TrimSpace(envKey) == "" {
		return "", errors.New("channelhost: empty env_key name")
	}
	value := os.Getenv(envKey)
	if value == "" {
		return "", errors.New("channelhost: environment variable for env_key is empty or unset")
	}
	return value, nil
}

// HTTP returns the shared outbound-only client. There is no Listen
// capability here; webhook/listen sockets belong to a later slice and to
// the host, never to the adapter.
func (e *hostEnv) HTTP() *http.Client {
	return e.client
}

// PublishInbound is the only path an inbound message may travel into the
// kernel. It forwards to the dispatch pipeline; see dispatch.go.
func (e *hostEnv) PublishInbound(ctx context.Context, msg plugin.InboundMessage) error {
	return e.host.publishInbound(ctx, msg)
}

// Media returns the first-cut media handle: a no-op store so the
// capability surface exists before any real media pipeline (C4).
func (e *hostEnv) Media() plugin.MediaStore {
	return e.host.media
}

func (e *hostEnv) hasGrant(need plugin.Grant) bool {
	for _, grant := range e.seam.Grants() {
		if grant == need {
			return true
		}
	}
	return false
}

// noopMediaStore is the v1 Media() handle: it validates the name and
// discards the content, returning a stable opaque ref. The first real
// adapter (C4) replaces it with a durable store.
type noopMediaStore struct{}

func (noopMediaStore) Put(_ context.Context, name string, _ io.Reader) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", errors.New("channelhost: media name must not be empty")
	}
	return "noop:" + name, nil
}
