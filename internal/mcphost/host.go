package mcphost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultOperationTimeout        = 8 * time.Second
	defaultMaxSafeRetries          = 1
	defaultCircuitFailureThreshold = 3
	defaultMaxSchemaBytes          = 256 << 10
	defaultMaxContentBytes         = 512 << 10
)

var (
	ErrInvalidConfig          = errors.New("mcphost: invalid configuration")
	ErrDuplicateInstance      = errors.New("mcphost: duplicate instance")
	ErrUnknownInstance        = errors.New("mcphost: unknown instance")
	ErrInstanceUnconfigured   = errors.New("mcphost: instance is unconfigured")
	ErrInstanceUnavailable    = errors.New("mcphost: instance is unavailable")
	ErrCircuitOpen            = errors.New("mcphost: instance circuit is open")
	ErrDuplicateRemoteTool    = errors.New("mcphost: duplicate remote tool")
	ErrProjectedToolCollision = errors.New("mcphost: projected tool collision")
	ErrProtectedToolID        = errors.New("mcphost: protected tool id")
	ErrUnknownTool            = errors.New("mcphost: unknown tool")
	ErrInvalidTool            = errors.New("mcphost: invalid remote tool")
	ErrSchemaChanged          = errors.New("mcphost: remote tool schema changed")
	ErrStaleDiscovery         = errors.New("mcphost: stale tool discovery")
	ErrContentTooLarge        = errors.New("mcphost: content exceeds bound")
)

// Namespace segments are carried into the model-visible projected Tool ID.
// They are validated as supplied; trimming, replacing, or hashing an unsafe
// value would create an ambiguous identity and is therefore forbidden.
func safeSegmentPattern(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	if !isASCIINamespaceLetter(value[0]) && !isASCIINamespaceDigit(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		if !isASCIINamespaceLetter(value[index]) && !isASCIINamespaceDigit(value[index]) && value[index] != '_' && value[index] != '-' {
			return false
		}
	}
	return true
}

func isASCIINamespaceLetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func isASCIINamespaceDigit(value byte) bool { return value >= '0' && value <= '9' }

type InstanceState string

const (
	StateUnconfigured InstanceState = "unconfigured"
	StateInactive     InstanceState = "inactive"
	StateReady        InstanceState = "ready"
	StateUnavailable  InstanceState = "unavailable"
	StateDeferred     InstanceState = "deferred"
)

type InstanceConfig struct {
	ID             string
	Endpoint       string
	Command        string
	Args           []string
	EnvFrom        map[string]string
	Cwd            string
	AuthEnv        string
	ResourceBridge bool
	DeferredReason string
	// Enabled defaults to true. A disabled saved instance remains in the
	// snapshot as inactive but cannot open or revive a remote session.
	Enabled *bool
}

func (config InstanceConfig) clone() InstanceConfig {
	config.Endpoint = strings.TrimSpace(config.Endpoint)
	config.Command = strings.TrimSpace(config.Command)
	config.Cwd = strings.TrimSpace(config.Cwd)
	config.AuthEnv = strings.TrimSpace(config.AuthEnv)
	config.DeferredReason = strings.TrimSpace(config.DeferredReason)
	if config.Enabled != nil {
		enabled := *config.Enabled
		config.Enabled = &enabled
	}
	config.Args = append([]string(nil), config.Args...)
	if config.EnvFrom != nil {
		values := make(map[string]string, len(config.EnvFrom))
		for key, value := range config.EnvFrom {
			values[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
		config.EnvFrom = values
	}
	return config
}

func (config InstanceConfig) configured() bool {
	return config.Endpoint != "" || config.Command != ""
}

type RemoteTool struct {
	Name        string
	Description string
	Schema      json.RawMessage
}

type ToolDefinition struct {
	ID           string
	InstanceID   string
	RemoteName   string
	Description  string
	Schema       json.RawMessage
	SchemaHash   string
	InstanceHash string
	RemoteHash   string
}

// TerminalFailurePolicy lets an adapter tell MCPHost that a failed operation
// permanently invalidated the current session. MCPHost then owns the
// fail-closed transition and does not reopen that session during its retry
// loop. This is primarily used for a dead private stdio process.
type TerminalFailurePolicy interface {
	TerminalFailure(error) bool
}

// RetryFailurePolicy lets a session narrow the failures for which this Host
// may safely reopen and retry an operation. The default remains retry-on-
// failure for generic Session implementations; MCP's runtime adapter uses
// this hook to distinguish a terminated HTTP session from a failed handshake.
type RetryFailurePolicy interface {
	RetryFailure(error) bool
}

type ToolResult struct {
	Text    string
	IsError bool
}

type RemoteResource struct {
	URI         string
	Name        string
	Title       string
	Description string
	MediaType   string
	Size        int64
	Annotations json.RawMessage
	Meta        json.RawMessage
}

type ResourceContent struct {
	URI       string
	MediaType string
	Text      string
	Blob      []byte
}

type Session interface {
	DiscoverTools(context.Context) ([]RemoteTool, error)
	CallTool(context.Context, string, json.RawMessage) (ToolResult, error)
	ListResources(context.Context) ([]RemoteResource, error)
	ReadResource(context.Context, string) (ResourceContent, error)
	Close() error
}

// PromptSession is the optional control-plane capability for an MCP session.
// Prompts remain untrusted control-plane data; MCPHost never projects them
// into Skills or model-visible Tools automatically.
type PromptSession interface {
	ListPrompts(context.Context) ([]RemotePrompt, error)
	GetPrompt(context.Context, string, map[string]string) (PromptResult, error)
}

type RemotePromptArgument struct {
	Name        string
	Title       string
	Description string
	Required    bool
}

type RemotePrompt struct {
	Name        string
	Title       string
	Description string
	Arguments   []RemotePromptArgument
}

type PromptResult struct {
	Name        string
	Description string
	Text        string
}

type SessionFactory interface {
	Open(context.Context, InstanceConfig) (Session, error)
}

type Config struct {
	Instances []InstanceConfig
	Factory   SessionFactory
	// ProtectedIDs are reserved Tool identities supplied by the composition
	// root. A remote capability may not claim either its raw name or projected
	// identity when it matches one of these IDs.
	ProtectedIDs []string
	// ProtectedToolIDs is a source-compatible alias for callers that name the
	// values explicitly as Tool IDs.
	ProtectedToolIDs []string
	OperationTimeout time.Duration
	MaxSafeRetries   int
	// MaxSafeRetriesSet lets a composition root explicitly select zero safe
	// retries while preserving the historical default for zero-valued callers.
	MaxSafeRetriesSet       bool
	CircuitFailureThreshold int
	MaxSchemaBytes          int
	MaxContentBytes         int
}

type Status struct {
	ID                  string
	State               InstanceState
	Enabled             bool
	ToolCount           int
	ConsecutiveFailures int
	CircuitOpen         bool
	ResourceBridge      bool
	DeferredReason      string
}

type toolBinding struct {
	instance   *instance
	definition ToolDefinition
	generation uint64
}

type Host struct {
	factory          SessionFactory
	operationTimeout time.Duration
	maxSafeRetries   int
	circuitThreshold int
	maxSchemaBytes   int
	maxContentBytes  int
	protected        map[string]struct{}

	mu        sync.RWMutex
	instances map[string]*instance
	toolIndex map[string]toolBinding
	closed    bool
}

func New(config Config) (*Host, error) {
	timeout := config.OperationTimeout
	if timeout <= 0 {
		timeout = defaultOperationTimeout
	}
	maxRetries := config.MaxSafeRetries
	if maxRetries < 0 {
		return nil, fmt.Errorf("%w: max safe retries cannot be negative", ErrInvalidConfig)
	}
	if maxRetries == 0 && !config.MaxSafeRetriesSet {
		maxRetries = defaultMaxSafeRetries
	}
	threshold := config.CircuitFailureThreshold
	if threshold <= 0 {
		threshold = defaultCircuitFailureThreshold
	}
	maxSchema := config.MaxSchemaBytes
	if maxSchema <= 0 {
		maxSchema = defaultMaxSchemaBytes
	}
	maxContent := config.MaxContentBytes
	if maxContent <= 0 {
		maxContent = defaultMaxContentBytes
	}

	host := &Host{
		factory:          config.Factory,
		operationTimeout: timeout,
		maxSafeRetries:   maxRetries,
		circuitThreshold: threshold,
		maxSchemaBytes:   maxSchema,
		maxContentBytes:  maxContent,
		protected:        make(map[string]struct{}, len(config.ProtectedIDs)+len(config.ProtectedToolIDs)),
		instances:        make(map[string]*instance),
		toolIndex:        make(map[string]toolBinding),
	}
	for _, id := range append(append([]string(nil), config.ProtectedIDs...), config.ProtectedToolIDs...) {
		if id = strings.TrimSpace(id); id != "" {
			host.protected[id] = struct{}{}
		}
	}
	for _, raw := range config.Instances {
		instanceConfig := raw.clone()
		if !safeSegmentPattern(instanceConfig.ID) {
			return nil, fmt.Errorf("%w: invalid instance id %q", ErrInvalidConfig, instanceConfig.ID)
		}
		if _, duplicate := host.instances[instanceConfig.ID]; duplicate {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateInstance, instanceConfig.ID)
		}
		state := StateInactive
		if instanceEnabled(instanceConfig) && instanceConfig.DeferredReason != "" {
			state = StateDeferred
		} else if instanceEnabled(instanceConfig) && !instanceConfig.configured() {
			state = StateUnconfigured
		}
		host.instances[instanceConfig.ID] = newInstance(instanceConfig, state)
	}
	for _, instance := range host.instances {
		if instance.config.configured() && instance.state != StateDeferred && instanceEnabled(instance.config) && host.factory == nil {
			return nil, fmt.Errorf("%w: configured instances require a session factory", ErrInvalidConfig)
		}
	}
	return host, nil
}

// ReplaceInstances atomically replaces the live instance catalog while
// retaining unchanged instances. Changed and removed instances are detached
// from the tool index before their sessions are closed, so an old binding can
// neither be discovered nor invoked after the replacement is published.
func (host *Host) ReplaceInstances(configs []InstanceConfig) error {
	if host == nil {
		return ErrInstanceUnavailable
	}
	normalized := make(map[string]InstanceConfig, len(configs))
	for _, raw := range configs {
		config := raw.clone()
		if !safeSegmentPattern(config.ID) {
			return fmt.Errorf("%w: invalid instance id %q", ErrInvalidConfig, config.ID)
		}
		if _, duplicate := normalized[config.ID]; duplicate {
			return fmt.Errorf("%w: %s", ErrDuplicateInstance, config.ID)
		}
		normalized[config.ID] = config
	}
	host.mu.RLock()
	factory := host.factory
	closed := host.closed
	host.mu.RUnlock()
	if closed {
		return ErrInstanceUnavailable
	}
	for _, config := range normalized {
		if config.configured() && config.DeferredReason == "" && instanceEnabled(config) && factory == nil {
			return fmt.Errorf("%w: configured instances require a session factory", ErrInvalidConfig)
		}
	}

	host.mu.Lock()
	if host.closed {
		host.mu.Unlock()
		return ErrInstanceUnavailable
	}
	next := make(map[string]*instance, len(normalized))
	retired := make(map[*instance]struct{})
	for id, previous := range host.instances {
		config, keep := normalized[id]
		if keep && reflect.DeepEqual(previous.config, config) {
			next[id] = previous
			continue
		}
		retired[previous] = struct{}{}
	}
	for id, config := range normalized {
		if _, keep := next[id]; !keep {
			state := StateInactive
			if instanceEnabled(config) && config.DeferredReason != "" {
				state = StateDeferred
			} else if instanceEnabled(config) && !config.configured() {
				state = StateUnconfigured
			}
			next[id] = newInstance(config, state)
		}
	}
	host.instances = next
	for id, binding := range host.toolIndex {
		if _, remove := retired[binding.instance]; remove {
			delete(host.toolIndex, id)
		}
	}
	host.mu.Unlock()

	var failures []error
	for instance := range retired {
		if err := instance.closeForever(); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (host *Host) Status() []Status {
	host.mu.RLock()
	instances := make([]*instance, 0, len(host.instances))
	for _, instance := range host.instances {
		instances = append(instances, instance)
	}
	host.mu.RUnlock()
	sort.Slice(instances, func(i, j int) bool { return instances[i].config.ID < instances[j].config.ID })
	out := make([]Status, 0, len(instances))
	for _, instance := range instances {
		instance.mu.Lock()
		out = append(out, Status{
			ID:                  instance.config.ID,
			State:               instance.state,
			Enabled:             instance.config.Enabled == nil || *instance.config.Enabled,
			ToolCount:           len(instance.tools),
			ConsecutiveFailures: instance.failures,
			CircuitOpen:         instance.circuitOpen,
			ResourceBridge:      instance.config.ResourceBridge,
			DeferredReason:      instance.config.DeferredReason,
		})
		instance.mu.Unlock()
	}
	return out
}

func (host *Host) DiscoverTools(ctx context.Context, instanceID string) ([]ToolDefinition, error) {
	instance, err := host.lookupInstance(instanceID)
	if err != nil {
		return nil, err
	}
	generation, err := instance.beginDiscovery()
	if err != nil {
		return nil, err
	}
	var last error
	for attempt := 0; attempt <= host.maxSafeRetries; attempt++ {
		if err := host.discoveryStateError(instance, generation); err != nil {
			return nil, err
		}
		session, err := host.ensureSession(ctx, instance)
		if err != nil {
			last = err
			if errors.Is(err, ErrCircuitOpen) || errors.Is(err, ErrInstanceUnconfigured) || instance.config.DeferredReason != "" {
				if !host.invalidateDiscovery(instance, generation, false) {
					if stateErr := host.discoveryStateError(instance, generation); stateErr != nil {
						return nil, stateErr
					}
					return nil, ErrInstanceUnavailable
				}
				return nil, err
			}
			continue
		}
		callCtx, cancel := context.WithTimeout(ctx, host.operationTimeout)
		remote, callErr := session.DiscoverTools(callCtx)
		cancel()
		if callErr != nil {
			last = host.failSession(instance, session, callErr, generation)
			if errors.Is(last, ErrStaleDiscovery) {
				return nil, last
			}
			if policy, ok := session.(RetryFailurePolicy); ok && !policy.RetryFailure(callErr) {
				return nil, last
			}
			continue
		}
		if err := host.discoveryStateError(instance, generation); err != nil {
			return nil, err
		}
		definitions, projectErr := host.projectTools(instance, remote)
		if projectErr != nil {
			if !host.invalidateDiscovery(instance, generation, false) {
				if err := host.discoveryStateError(instance, generation); err != nil {
					return nil, err
				}
				return nil, ErrInstanceUnavailable
			}
			host.failLogical(instance, projectErr)
			return nil, projectErr
		}
		if reconcileErr := host.reconcileToolIndex(instance, generation, definitions); reconcileErr != nil {
			return nil, reconcileErr
		}
		return cloneDefinitions(definitions), nil
	}
	if last == nil {
		last = ErrInstanceUnavailable
	}
	if !host.invalidateDiscovery(instance, generation, false) {
		if err := host.discoveryStateError(instance, generation); err != nil {
			return nil, err
		}
		return nil, ErrInstanceUnavailable
	}
	return nil, last
}

func (host *Host) CallTool(ctx context.Context, projectedID string, arguments json.RawMessage) (ToolResult, error) {
	binding, ok := host.lookupCurrentTool(projectedID)
	if !ok {
		return ToolResult{}, fmt.Errorf("%w: %s", ErrUnknownTool, projectedID)
	}
	session, err := host.ensureSession(ctx, binding.instance)
	if err != nil {
		return ToolResult{}, err
	}
	if !host.bindingReady(binding) {
		return ToolResult{}, fmt.Errorf("%w: %s", ErrUnknownTool, projectedID)
	}
	callCtx, cancel := context.WithTimeout(ctx, host.operationTimeout)
	result, err := session.CallTool(callCtx, binding.definition.RemoteName, append(json.RawMessage(nil), arguments...))
	cancel()
	if err != nil {
		return ToolResult{}, host.failSession(binding.instance, session, err, binding.generation)
	}
	if !host.bindingReady(binding) {
		return ToolResult{}, fmt.Errorf("%w: %s", ErrUnknownTool, projectedID)
	}
	if len(result.Text) > host.maxContentBytes {
		return ToolResult{}, ErrContentTooLarge
	}
	binding.instance.markSuccess()
	return result, nil
}

// CallRemoteTool is the control-plane compatibility seam for an explicitly
// named remote capability. Normal model execution uses CallTool after a
// discovered ToolWorld binding; this method keeps the existing MCP control
// surface useful for diagnostics/tests without making the backend own a
// transport or retry loop.
func (host *Host) CallRemoteTool(ctx context.Context, instanceID, remoteName string, arguments json.RawMessage) (ToolResult, error) {
	instance, err := host.lookupInstance(instanceID)
	if err != nil {
		return ToolResult{}, err
	}
	session, err := host.ensureSession(ctx, instance)
	if err != nil {
		return ToolResult{}, err
	}
	generation := instance.generation()
	callCtx, cancel := context.WithTimeout(ctx, host.operationTimeout)
	result, callErr := session.CallTool(callCtx, strings.TrimSpace(remoteName), append(json.RawMessage(nil), arguments...))
	cancel()
	if callErr != nil {
		return ToolResult{}, host.failSession(instance, session, callErr, generation)
	}
	if len(result.Text) > host.maxContentBytes {
		return ToolResult{}, ErrContentTooLarge
	}
	instance.markSuccess()
	return result, nil
}

func (host *Host) ListResources(ctx context.Context, instanceID string) ([]RemoteResource, error) {
	instance, err := host.lookupInstance(instanceID)
	if err != nil {
		return nil, err
	}
	generation := instance.generation()
	var last error
	for attempt := 0; attempt <= host.maxSafeRetries; attempt++ {
		session, err := host.ensureSession(ctx, instance)
		if err != nil {
			last = err
			continue
		}
		callCtx, cancel := context.WithTimeout(ctx, host.operationTimeout)
		resources, callErr := session.ListResources(callCtx)
		cancel()
		if callErr != nil {
			last = host.failSession(instance, session, callErr, generation)
			if errors.Is(last, ErrStaleDiscovery) || errors.Is(last, ErrInstanceUnavailable) {
				return nil, last
			}
			if policy, ok := session.(RetryFailurePolicy); ok && !policy.RetryFailure(callErr) {
				return nil, last
			}
			continue
		}
		if stateErr := host.discoveryStateError(instance, generation); stateErr != nil {
			return nil, stateErr
		}
		if err := host.validateResources(resources); err != nil {
			return nil, err
		}
		instance.markSuccess()
		return cloneResources(resources), nil
	}
	return nil, last
}

func (host *Host) ReadResource(ctx context.Context, instanceID, uri string) (ResourceContent, error) {
	instance, err := host.lookupInstance(instanceID)
	if err != nil {
		return ResourceContent{}, err
	}
	generation := instance.generation()
	uri = strings.TrimSpace(uri)
	if uri == "" || len(uri) > host.maxContentBytes {
		return ResourceContent{}, ErrInvalidConfig
	}
	var last error
	for attempt := 0; attempt <= host.maxSafeRetries; attempt++ {
		session, err := host.ensureSession(ctx, instance)
		if err != nil {
			last = err
			continue
		}
		callCtx, cancel := context.WithTimeout(ctx, host.operationTimeout)
		content, callErr := session.ReadResource(callCtx, uri)
		cancel()
		if callErr != nil {
			last = host.failSession(instance, session, callErr, generation)
			if errors.Is(last, ErrStaleDiscovery) || errors.Is(last, ErrInstanceUnavailable) {
				return ResourceContent{}, last
			}
			if policy, ok := session.(RetryFailurePolicy); ok && !policy.RetryFailure(callErr) {
				return ResourceContent{}, last
			}
			continue
		}
		if stateErr := host.discoveryStateError(instance, generation); stateErr != nil {
			return ResourceContent{}, stateErr
		}
		if len(content.Text)+len(content.Blob) > host.maxContentBytes {
			return ResourceContent{}, ErrContentTooLarge
		}
		content.Blob = append([]byte(nil), content.Blob...)
		instance.markSuccess()
		return content, nil
	}
	return ResourceContent{}, last
}

// ListPrompts performs the explicit MCP prompt control-plane operation. A
// session that does not advertise the optional prompt capability fails closed
// as an empty catalog; prompts are never converted into Skills.
func (host *Host) ListPrompts(ctx context.Context, instanceID string) ([]RemotePrompt, error) {
	instance, err := host.lookupInstance(instanceID)
	if err != nil {
		return nil, err
	}
	generation := instance.generation()
	var last error
	for attempt := 0; attempt <= host.maxSafeRetries; attempt++ {
		session, err := host.ensureSession(ctx, instance)
		if err != nil {
			last = err
			continue
		}
		promptSession, ok := session.(PromptSession)
		if !ok {
			instance.markSuccess()
			return []RemotePrompt{}, nil
		}
		callCtx, cancel := context.WithTimeout(ctx, host.operationTimeout)
		prompts, callErr := promptSession.ListPrompts(callCtx)
		cancel()
		if callErr != nil {
			last = host.failSession(instance, session, callErr, generation)
			if errors.Is(last, ErrStaleDiscovery) || errors.Is(last, ErrInstanceUnavailable) {
				return nil, last
			}
			if policy, ok := session.(RetryFailurePolicy); ok && !policy.RetryFailure(callErr) {
				return nil, last
			}
			continue
		}
		if stateErr := host.discoveryStateError(instance, generation); stateErr != nil {
			return nil, stateErr
		}
		instance.markSuccess()
		return append([]RemotePrompt(nil), prompts...), nil
	}
	return nil, last
}

// GetPrompt performs the explicit MCP prompt control-plane operation. The
// result is bounded by the same Host content policy used by resources.
func (host *Host) GetPrompt(ctx context.Context, instanceID, name string, arguments map[string]string) (PromptResult, error) {
	instance, err := host.lookupInstance(instanceID)
	if err != nil {
		return PromptResult{}, err
	}
	generation := instance.generation()
	var last error
	for attempt := 0; attempt <= host.maxSafeRetries; attempt++ {
		session, err := host.ensureSession(ctx, instance)
		if err != nil {
			last = err
			continue
		}
		promptSession, ok := session.(PromptSession)
		if !ok {
			return PromptResult{}, ErrInstanceUnavailable
		}
		callCtx, cancel := context.WithTimeout(ctx, host.operationTimeout)
		result, callErr := promptSession.GetPrompt(callCtx, name, clonePromptArguments(arguments))
		cancel()
		if callErr != nil {
			last = host.failSession(instance, session, callErr, generation)
			if errors.Is(last, ErrStaleDiscovery) || errors.Is(last, ErrInstanceUnavailable) {
				return PromptResult{}, last
			}
			if policy, ok := session.(RetryFailurePolicy); ok && !policy.RetryFailure(callErr) {
				return PromptResult{}, last
			}
			continue
		}
		if stateErr := host.discoveryStateError(instance, generation); stateErr != nil {
			return PromptResult{}, stateErr
		}
		if len(result.Text) > host.maxContentBytes {
			return PromptResult{}, ErrContentTooLarge
		}
		instance.markSuccess()
		return result, nil
	}
	return PromptResult{}, last
}

func clonePromptArguments(arguments map[string]string) map[string]string {
	if arguments == nil {
		return nil
	}
	out := make(map[string]string, len(arguments))
	for key, value := range arguments {
		out[key] = value
	}
	return out
}

func (host *Host) ResourceBridgeEnabled(instanceID string) bool {
	instance, err := host.lookupInstance(instanceID)
	return err == nil && instance.config.ResourceBridge
}

func (host *Host) Close() error {
	host.mu.Lock()
	if host.closed {
		host.mu.Unlock()
		return nil
	}
	host.closed = true
	instances := make([]*instance, 0, len(host.instances))
	for _, instance := range host.instances {
		instances = append(instances, instance)
	}
	host.toolIndex = make(map[string]toolBinding)
	host.mu.Unlock()
	var failures []error
	for _, instance := range instances {
		if err := instance.closeForever(); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (host *Host) lookupInstance(id string) (*instance, error) {
	if !safeSegmentPattern(id) {
		return nil, fmt.Errorf("%w: %s", ErrUnknownInstance, id)
	}
	host.mu.RLock()
	instance := host.instances[id]
	closed := host.closed
	host.mu.RUnlock()
	if closed {
		return nil, ErrInstanceUnavailable
	}
	if instance == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownInstance, id)
	}
	instance.mu.Lock()
	terminalErr := instance.terminalErr
	instance.mu.Unlock()
	if terminalErr != nil {
		return nil, terminalErr
	}
	if !instanceEnabled(instance.config) {
		return nil, fmt.Errorf("%w: %s", ErrInstanceUnavailable, id)
	}
	if instance.config.DeferredReason != "" {
		return nil, fmt.Errorf("%w: %s", ErrInstanceUnavailable, id)
	}
	if !instance.config.configured() {
		return nil, fmt.Errorf("%w: %s", ErrInstanceUnconfigured, id)
	}
	return instance, nil
}

func (host *Host) lookupCurrentTool(id string) (toolBinding, bool) {
	host.mu.RLock()
	binding, ok := host.toolIndex[id]
	if !ok || host.closed || host.instances[binding.instance.config.ID] != binding.instance {
		host.mu.RUnlock()
		return toolBinding{}, false
	}
	binding.instance.mu.Lock()
	ready := binding.instance.state == StateReady && !binding.instance.terminal && binding.instance.discoveryGeneration == binding.generation
	binding.instance.mu.Unlock()
	host.mu.RUnlock()
	return binding, ready
}

func (host *Host) bindingReady(binding toolBinding) bool {
	host.mu.RLock()
	if host.closed || host.instances[binding.instance.config.ID] != binding.instance || host.toolIndex[binding.definition.ID].instance != binding.instance || host.toolIndex[binding.definition.ID].generation != binding.generation {
		host.mu.RUnlock()
		return false
	}
	binding.instance.mu.Lock()
	ready := binding.instance.state == StateReady && !binding.instance.terminal && binding.instance.discoveryGeneration == binding.generation
	binding.instance.mu.Unlock()
	host.mu.RUnlock()
	return ready
}

// LookupTool returns a discovered binding only while its owning instance is
// ready. A schema reconciliation failure removes the binding immediately,
// so callers cannot execute a stale contract.
func (host *Host) LookupTool(id string) (ToolDefinition, bool) {
	binding, ready := host.lookupCurrentTool(id)
	if !ready {
		return ToolDefinition{}, false
	}
	definition := binding.definition
	definition.Schema = append(json.RawMessage(nil), definition.Schema...)
	return definition, true
}

func (host *Host) ensureSession(ctx context.Context, instance *instance) (Session, error) {
	return instance.ensureSession(ctx, host.factory, host.operationTimeout, host.circuitThreshold)
}

func instanceEnabled(config InstanceConfig) bool {
	return config.Enabled == nil || *config.Enabled
}

func (host *Host) failSession(instance *instance, session Session, cause error, generation uint64) error {
	if err := host.discoveryStateError(instance, generation); err != nil {
		return err
	}
	if policy, ok := session.(TerminalFailurePolicy); ok && policy.TerminalFailure(cause) {
		if !host.invalidateDiscovery(instance, generation, true, cause) {
			if err := host.discoveryStateError(instance, generation); err != nil {
				return err
			}
			return ErrInstanceUnavailable
		}
		return cause
	}
	if !host.invalidateDiscovery(instance, generation, false) {
		if err := host.discoveryStateError(instance, generation); err != nil {
			return err
		}
		return ErrInstanceUnavailable
	}
	instance.noteFailure(host.circuitThreshold)
	return cause
}

func (host *Host) failLogical(instance *instance, cause error) {
	instance.noteFailure(host.circuitThreshold)
}

func (host *Host) projectTools(instance *instance, remote []RemoteTool) ([]ToolDefinition, error) {
	seenRemote := make(map[string]struct{}, len(remote))
	seenProjected := make(map[string]struct{}, len(remote))
	out := make([]ToolDefinition, 0, len(remote))
	for _, tool := range remote {
		name := tool.Name
		if !safeSegmentPattern(name) {
			return nil, ErrInvalidTool
		}
		if _, duplicate := seenRemote[name]; duplicate {
			return nil, fmt.Errorf("%w: %s/%s", ErrDuplicateRemoteTool, instance.config.ID, name)
		}
		seenRemote[name] = struct{}{}
		if len(tool.Schema) > host.maxSchemaBytes || (len(bytes.TrimSpace(tool.Schema)) > 0 && !json.Valid(tool.Schema)) {
			return nil, fmt.Errorf("%w: invalid schema for %s/%s", ErrInvalidTool, instance.config.ID, name)
		}
		id := projectedToolID(instance.config.ID, name)
		if host.isProtected(name) || host.isProtected(id) {
			return nil, fmt.Errorf("%w: %s", ErrProtectedToolID, id)
		}
		if _, duplicate := seenProjected[id]; duplicate {
			return nil, fmt.Errorf("%w: %s", ErrProjectedToolCollision, id)
		}
		seenProjected[id] = struct{}{}
		out = append(out, ToolDefinition{
			ID:           id,
			InstanceID:   instance.config.ID,
			RemoteName:   name,
			Description:  strings.TrimSpace(tool.Description),
			Schema:       append(json.RawMessage(nil), tool.Schema...),
			SchemaHash:   schemaHash(tool.Schema),
			InstanceHash: identityHash("mcp-instance", instance.config.ID),
			RemoteHash:   identityHash("mcp-remote", instance.config.ID, name),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (host *Host) isProtected(id string) bool {
	_, ok := host.protected[strings.TrimSpace(id)]
	return ok
}

func (host *Host) reconcileToolIndex(instance *instance, generation uint64, definitions []ToolDefinition) error {
	if err := host.discoveryStateError(instance, generation); err != nil {
		return err
	}
	instance.mu.Lock()
	discovered := instance.discovered
	previous := cloneDefinitions(instance.tools)
	instance.mu.Unlock()
	if discovered && definitionsChanged(previous, definitions) {
		if !host.invalidateDiscovery(instance, generation, false) {
			return ErrStaleDiscovery
		}
		return fmt.Errorf("%w: instance %s capability set changed", ErrSchemaChanged, instance.config.ID)
	}
	if !host.publishDiscovery(instance, generation, definitions) {
		return ErrStaleDiscovery
	}
	return nil
}

func definitionsChanged(previous, current []ToolDefinition) bool {
	if len(previous) != len(current) {
		return true
	}
	byID := make(map[string]ToolDefinition, len(previous))
	for _, definition := range previous {
		byID[definition.ID] = definition
	}
	for _, definition := range current {
		old, ok := byID[definition.ID]
		// Some MCP servers omit inputSchema on a later tools/list response.
		// Preserve that live compatibility: the new projection is allowed to
		// carry an empty schema, but an actual non-empty schema change remains a
		// fail-closed reconciliation event.
		schemaChanged := old.SchemaHash != definition.SchemaHash
		if len(definition.Schema) == 0 && len(old.Schema) > 0 {
			schemaChanged = false
		}
		if !ok || schemaChanged || old.InstanceHash != definition.InstanceHash || old.RemoteHash != definition.RemoteHash || old.RemoteName != definition.RemoteName || old.InstanceID != definition.InstanceID {
			return true
		}
	}
	return false
}

func (host *Host) isCurrentInstance(instance *instance) bool {
	host.mu.RLock()
	current := !host.closed && host.instances[instance.config.ID] == instance
	host.mu.RUnlock()
	return current
}

func (host *Host) isCurrentDiscovery(instance *instance, generation uint64) bool {
	if !host.isCurrentInstance(instance) {
		return false
	}
	return instance.currentDiscovery(generation)
}

func (host *Host) discoveryStateError(instance *instance, generation uint64) error {
	if !host.isCurrentInstance(instance) {
		return ErrInstanceUnavailable
	}
	if !instance.currentDiscovery(generation) {
		return ErrStaleDiscovery
	}
	return nil
}

// invalidateDiscovery atomically retires the current tool bindings and
// session for one discovery generation. The open mutex prevents a concurrent
// ensureSession from installing a replacement between detaching the stale
// session and publishing the cleared index.
func (host *Host) invalidateDiscovery(instance *instance, generation uint64, terminal bool, terminalCause ...error) bool {
	instance.openMu.Lock()
	defer instance.openMu.Unlock()
	host.mu.Lock()
	if host.closed || host.instances[instance.config.ID] != instance {
		host.mu.Unlock()
		return false
	}
	instance.mu.Lock()
	if instance.discoveryGeneration != generation || instance.closed {
		instance.mu.Unlock()
		host.mu.Unlock()
		return false
	}
	for id, binding := range host.toolIndex {
		if binding.instance == instance {
			delete(host.toolIndex, id)
		}
	}
	session := instance.session
	instance.session = nil
	instance.tools = nil
	instance.discovered = false
	instance.state = StateUnavailable
	instance.terminal = terminal
	instance.terminalErr = nil
	if terminal && len(terminalCause) > 0 {
		instance.terminalErr = terminalCause[0]
	}
	instance.mu.Unlock()
	host.mu.Unlock()
	if session != nil {
		_ = session.Close()
	}
	return true
}

func (host *Host) publishDiscovery(instance *instance, generation uint64, definitions []ToolDefinition) bool {
	host.mu.Lock()
	defer host.mu.Unlock()
	if host.closed || host.instances[instance.config.ID] != instance {
		return false
	}
	instance.mu.Lock()
	defer instance.mu.Unlock()
	if instance.closed || instance.terminal || instance.discoveryGeneration != generation {
		return false
	}
	for id, binding := range host.toolIndex {
		if binding.instance == instance {
			delete(host.toolIndex, id)
		}
	}
	for _, definition := range definitions {
		host.toolIndex[definition.ID] = toolBinding{instance: instance, definition: definition, generation: generation}
	}
	instance.tools = cloneDefinitions(definitions)
	instance.discovered = true
	instance.state = StateReady
	instance.failures = 0
	instance.circuitOpen = false
	return true
}

func (host *Host) validateResources(resources []RemoteResource) error {
	used := 0
	seen := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		uri := strings.TrimSpace(resource.URI)
		if uri == "" {
			return ErrInvalidConfig
		}
		if _, duplicate := seen[uri]; duplicate {
			return fmt.Errorf("%w: duplicate resource URI", ErrInvalidConfig)
		}
		seen[uri] = struct{}{}
		used += len(uri) + len(resource.Name) + len(resource.Description) + len(resource.MediaType)
		if used > host.maxContentBytes {
			return ErrContentTooLarge
		}
	}
	return nil
}

func projectedToolID(instanceID, remoteName string) string {
	return "mcp." + instanceID + "." + remoteName
}

func schemaHash(raw json.RawMessage) string {
	canonical := bytes.TrimSpace(raw)
	if len(canonical) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(canonical))
		decoder.UseNumber()
		var value any
		if decoder.Decode(&value) == nil {
			if encoded, err := json.Marshal(value); err == nil {
				canonical = encoded
			}
		}
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:])
}

func identityHash(kind string, parts ...string) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(kind))
	for _, part := range parts {
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(part))
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func cloneDefinitions(definitions []ToolDefinition) []ToolDefinition {
	out := append([]ToolDefinition(nil), definitions...)
	for index := range out {
		out[index].Schema = append(json.RawMessage(nil), out[index].Schema...)
	}
	return out
}

func cloneResources(resources []RemoteResource) []RemoteResource {
	return append([]RemoteResource(nil), resources...)
}
