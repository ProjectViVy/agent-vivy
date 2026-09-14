package channelhost

import (
	"context"
	"errors"
	"time"

	plugin "agent-vivy/sdk/port/channel"
)

// healthProbeTimeout bounds one Health call. The HealthChecker contract is
// read-only internal state (no network I/O), so a well-behaved adapter
// returns instantly; the timeout only keeps a misbehaving one from stalling
// the inspect surface.
const healthProbeTimeout = 2 * time.Second

// ChannelHealth is the live transport health of one started adapter that
// implements plugin.HealthChecker. OK reports a nil Health error; otherwise
// Class carries the CH-R-1 classification (rate-limit / temporary / dead)
// and Detail the adapter's bounded reason. Class is empty when OK.
type ChannelHealth struct {
	OK     bool
	Class  string
	Detail string
}

// probeHealth asks one adapter for its live state and classifies the
// answer. Non-HealthChecker adapters have nothing to probe; a Health error
// that is not a *HealthError defaults to temporary — the right assumption
// for a supervised, redialing ear.
func (h *Host) probeHealth(ch plugin.Channel) *ChannelHealth {
	hc, ok := ch.(plugin.HealthChecker)
	if !ok {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), healthProbeTimeout)
	defer cancel()
	if err := hc.Health(ctx); err != nil {
		out := &ChannelHealth{Class: string(plugin.ClassTemporary), Detail: err.Error()}
		var classified *plugin.HealthError
		if errors.As(err, &classified) {
			out.Class = string(classified.Class)
		}
		return out
	}
	return &ChannelHealth{OK: true}
}
