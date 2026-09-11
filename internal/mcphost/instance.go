package mcphost

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type instance struct {
	config InstanceConfig

	openMu              sync.Mutex
	mu                  sync.Mutex
	session             Session
	state               InstanceState
	tools               []ToolDefinition
	discovered          bool
	discoveryGeneration uint64
	failures            int
	circuitOpen         bool
	terminal            bool
	terminalErr         error
	closed              bool
}

func (instance *instance) beginDiscovery() (uint64, error) {
	instance.mu.Lock()
	defer instance.mu.Unlock()
	if instance.closed || instance.terminal || instance.state == StateDeferred || !instanceEnabled(instance.config) {
		return 0, ErrInstanceUnavailable
	}
	if instance.state == StateUnconfigured {
		return 0, ErrInstanceUnconfigured
	}
	instance.discoveryGeneration++
	return instance.discoveryGeneration, nil
}

func (instance *instance) currentDiscovery(generation uint64) bool {
	instance.mu.Lock()
	defer instance.mu.Unlock()
	return !instance.closed && !instance.terminal && instance.discoveryGeneration == generation
}

func (instance *instance) generation() uint64 {
	instance.mu.Lock()
	defer instance.mu.Unlock()
	return instance.discoveryGeneration
}

func newInstance(config InstanceConfig, state InstanceState) *instance {
	return &instance{config: config.clone(), state: state}
}

func (instance *instance) ensureSession(ctx context.Context, factory SessionFactory, timeout time.Duration, threshold int) (Session, error) {
	instance.mu.Lock()
	if instance.closed {
		instance.mu.Unlock()
		return nil, ErrInstanceUnavailable
	}
	if instance.terminal {
		instance.mu.Unlock()
		err := instance.terminalErr
		if err == nil {
			err = ErrInstanceUnavailable
		}
		return nil, err
	}
	if instance.config.Enabled != nil && !*instance.config.Enabled {
		instance.mu.Unlock()
		return nil, ErrInstanceUnavailable
	}
	if instance.state == StateDeferred {
		instance.mu.Unlock()
		return nil, ErrInstanceUnavailable
	}
	if instance.state == StateUnconfigured {
		instance.mu.Unlock()
		return nil, ErrInstanceUnconfigured
	}
	if instance.circuitOpen {
		instance.mu.Unlock()
		return nil, ErrCircuitOpen
	}
	if session := instance.session; session != nil {
		instance.mu.Unlock()
		return session, nil
	}
	instance.mu.Unlock()

	instance.openMu.Lock()
	defer instance.openMu.Unlock()

	instance.mu.Lock()
	if instance.closed {
		instance.mu.Unlock()
		return nil, ErrInstanceUnavailable
	}
	if instance.terminal {
		instance.mu.Unlock()
		err := instance.terminalErr
		if err == nil {
			err = ErrInstanceUnavailable
		}
		return nil, err
	}
	if instance.circuitOpen {
		instance.mu.Unlock()
		return nil, ErrCircuitOpen
	}
	if session := instance.session; session != nil {
		instance.mu.Unlock()
		return session, nil
	}
	instance.mu.Unlock()
	if factory == nil {
		return nil, fmt.Errorf("%w: session factory unavailable", ErrInstanceUnavailable)
	}

	openCtx, cancel := context.WithTimeout(ctx, timeout)
	session, err := factory.Open(openCtx, instance.config.clone())
	cancel()
	if err != nil {
		instance.mu.Lock()
		instance.failures++
		instance.state = StateUnavailable
		if threshold <= 0 {
			threshold = defaultCircuitFailureThreshold
		}
		if instance.failures >= threshold {
			instance.circuitOpen = true
		}
		instance.mu.Unlock()
		return nil, err
	}
	if session == nil {
		instance.mu.Lock()
		instance.failures++
		instance.state = StateUnavailable
		if threshold > 0 && instance.failures >= threshold {
			instance.circuitOpen = true
		}
		instance.mu.Unlock()
		return nil, errors.New("mcphost: session factory returned nil session")
	}

	instance.mu.Lock()
	if instance.closed {
		instance.mu.Unlock()
		_ = session.Close()
		return nil, ErrInstanceUnavailable
	}
	instance.session = session
	if instance.terminal {
		instance.mu.Unlock()
		_ = session.Close()
		return nil, ErrInstanceUnavailable
	}
	if instance.state == StateUnavailable {
		instance.state = StateInactive
	}
	instance.mu.Unlock()
	return session, nil
}

func (instance *instance) noteFailure(threshold int) {
	instance.mu.Lock()
	defer instance.mu.Unlock()
	instance.failures++
	instance.state = StateUnavailable
	if threshold <= 0 {
		threshold = defaultCircuitFailureThreshold
	}
	if instance.failures >= threshold {
		instance.circuitOpen = true
	}
}

func (instance *instance) markReady(definitions []ToolDefinition) {
	instance.mu.Lock()
	instance.tools = cloneDefinitions(definitions)
	instance.discovered = true
	instance.state = StateReady
	instance.failures = 0
	instance.circuitOpen = false
	instance.mu.Unlock()
}

func (instance *instance) markSchemaUnavailable() {
	instance.mu.Lock()
	instance.tools = nil
	instance.state = StateUnavailable
	instance.mu.Unlock()
}

func (instance *instance) markTerminal() {
	instance.mu.Lock()
	instance.terminal = true
	instance.tools = nil
	instance.state = StateUnavailable
	instance.mu.Unlock()
}

func (instance *instance) markSuccess() {
	instance.mu.Lock()
	instance.failures = 0
	instance.circuitOpen = false
	if len(instance.tools) > 0 {
		instance.state = StateReady
	} else if instance.state != StateDeferred && instance.state != StateUnconfigured {
		instance.state = StateInactive
	}
	instance.mu.Unlock()
}

func (instance *instance) closeSession() error {
	instance.openMu.Lock()
	defer instance.openMu.Unlock()
	instance.mu.Lock()
	session := instance.session
	instance.session = nil
	if instance.closed {
		instance.mu.Unlock()
		if session != nil {
			return session.Close()
		}
		return nil
	}
	if instance.state != StateDeferred && instance.state != StateUnconfigured {
		instance.state = StateUnavailable
	}
	instance.mu.Unlock()
	if session == nil {
		return nil
	}
	return session.Close()
}

func (instance *instance) closeForever() error {
	instance.openMu.Lock()
	defer instance.openMu.Unlock()
	instance.mu.Lock()
	if instance.closed {
		instance.mu.Unlock()
		return nil
	}
	instance.closed = true
	session := instance.session
	instance.session = nil
	instance.mu.Unlock()
	if session == nil {
		return nil
	}
	return session.Close()
}
