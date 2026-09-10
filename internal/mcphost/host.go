package mcphost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
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
	ErrUnknownTool            = errors.New("mcphost: unknown tool")
	ErrInvalidTool            = errors.New("mcphost: invalid remote tool")
	ErrContentTooLarge        = errors.New("mcphost: content exceeds bound")
)

var safeSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var unsafeSegmentRune = regexp.MustCompile(`[^A-Za-z0-9._-]`)

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
}

func (config InstanceConfig) clone() InstanceConfig {
	config.ID = strings.TrimSpace(config.ID)
	config.Endpoint = strings.TrimSpace(config.Endpoint)
	config.Command = strings.TrimSpace(config.Command)
	config.Cwd = strings.TrimSpace(config.Cwd)
	config.AuthEnv = strings.TrimSpace(config.AuthEnv)
	config.DeferredReason = strings.TrimSpace(config.DeferredReason)
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
	ID          string
	InstanceID  string
	RemoteName  string
	Description string
	Schema      json.RawMessage
	SchemaHash  string
}

type ToolResult struct {
	Text    string
	IsError bool
}

type RemoteResource struct {
	URI         string
	Name        string
	Description string
	MediaType   string
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

type SessionFactory interface {
	Open(context.Context, InstanceConfig) (Session, error)
}

type Config struct {
	Instances               []InstanceConfig
	Factory                 SessionFactory
	OperationTimeout        time.Duration
	MaxSafeRetries          int
	CircuitFailureThreshold int
	MaxSchemaBytes          int
	MaxContentBytes         int
}

type Status struct {
	ID                  string
	State               InstanceState
	ToolCount           int
	ConsecutiveFailures int
	CircuitOpen         bool
	ResourceBridge      bool
	DeferredReason      string
}

type toolBinding struct {
	instance   *instance
	definition ToolDefinition
}

type Host struct {
	factory          SessionFactory
	operationTimeout time.Duration
	maxSafeRetries   int
	circuitThreshold int
	maxSchemaBytes   int
	maxContentBytes  int

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
	if maxRetries == 0 {
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
		instances:        make(map[string]*instance),
		toolIndex:        make(map[string]toolBinding),
	}
	for _, raw := range config.Instances {
		instanceConfig := raw.clone()
		if !safeSegmentPattern.MatchString(instanceConfig.ID) {
			return nil, fmt.Errorf("%w: invalid instance id %q", ErrInvalidConfig, instanceConfig.ID)
		}
		if _, duplicate := host.instances[instanceConfig.ID]; duplicate {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateInstance, instanceConfig.ID)
		}
		state := StateInactive
		if !instanceConfig.configured() {
			state = StateUnconfigured
		}
		if instanceConfig.DeferredReason != "" {
			state = StateDeferred
		}
		host.instances[instanceConfig.ID] = newInstance(instanceConfig, state)
	}
	for _, instance := range host.instances {
		if instance.config.configured() && instance.state != StateDeferred && host.factory == nil {
			return nil, fmt.Errorf("%w: configured instances require a session factory", ErrInvalidConfig)
		}
	}
	return host, nil
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
	var last error
	for attempt := 0; attempt <= host.maxSafeRetries; attempt++ {
		session, err := host.ensureSession(ctx, instance)
		if err != nil {
			last = err
			if errors.Is(err, ErrCircuitOpen) || errors.Is(err, ErrInstanceUnconfigured) || instance.config.DeferredReason != "" {
				return nil, err
			}
			continue
		}
		callCtx, cancel := context.WithTimeout(ctx, host.operationTimeout)
		remote, callErr := session.DiscoverTools(callCtx)
		cancel()
		if callErr != nil {
			last = host.failSession(instance, callErr)
			continue
		}
		definitions, projectErr := host.projectTools(instance, remote)
		if projectErr != nil {
			host.failLogical(instance, projectErr)
			return nil, projectErr
		}
		host.replaceToolIndex(instance, definitions)
		instance.markReady(definitions)
		return cloneDefinitions(definitions), nil
	}
	if last == nil {
		last = ErrInstanceUnavailable
	}
	return nil, last
}

func (host *Host) CallTool(ctx context.Context, projectedID string, arguments json.RawMessage) (ToolResult, error) {
	binding, ok := host.lookupTool(projectedID)
	if !ok {
		return ToolResult{}, fmt.Errorf("%w: %s", ErrUnknownTool, projectedID)
	}
	session, err := host.ensureSession(ctx, binding.instance)
	if err != nil {
		return ToolResult{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, host.operationTimeout)
	result, err := session.CallTool(callCtx, binding.definition.RemoteName, append(json.RawMessage(nil), arguments...))
	cancel()
	if err != nil {
		return ToolResult{}, host.failSession(binding.instance, err)
	}
	if len(result.Text) > host.maxContentBytes {
		return ToolResult{}, ErrContentTooLarge
	}
	binding.instance.markSuccess()
	return result, nil
}

func (host *Host) ListResources(ctx context.Context, instanceID string) ([]RemoteResource, error) {
	instance, err := host.lookupInstance(instanceID)
	if err != nil {
		return nil, err
	}
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
			last = host.failSession(instance, callErr)
			continue
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
			last = host.failSession(instance, callErr)
			continue
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
	id = strings.TrimSpace(id)
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
	if instance.config.DeferredReason != "" {
		return nil, fmt.Errorf("%w: %s", ErrInstanceUnavailable, id)
	}
	if !instance.config.configured() {
		return nil, fmt.Errorf("%w: %s", ErrInstanceUnconfigured, id)
	}
	return instance, nil
}

func (host *Host) lookupTool(id string) (toolBinding, bool) {
	host.mu.RLock()
	binding, ok := host.toolIndex[strings.TrimSpace(id)]
	host.mu.RUnlock()
	return binding, ok
}

func (host *Host) ensureSession(ctx context.Context, instance *instance) (Session, error) {
	return instance.ensureSession(ctx, host.factory, host.operationTimeout)
}

func (host *Host) failSession(instance *instance, cause error) error {
	instance.noteFailure(host.circuitThreshold)
	_ = instance.closeSession()
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
		name := strings.TrimSpace(tool.Name)
		if name == "" {
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
		if _, duplicate := seenProjected[id]; duplicate {
			return nil, fmt.Errorf("%w: %s", ErrProjectedToolCollision, id)
		}
		seenProjected[id] = struct{}{}
		out = append(out, ToolDefinition{
			ID:          id,
			InstanceID:  instance.config.ID,
			RemoteName:  name,
			Description: strings.TrimSpace(tool.Description),
			Schema:      append(json.RawMessage(nil), tool.Schema...),
			SchemaHash:  schemaHash(tool.Schema),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (host *Host) replaceToolIndex(instance *instance, definitions []ToolDefinition) {
	host.mu.Lock()
	for id, binding := range host.toolIndex {
		if binding.instance == instance {
			delete(host.toolIndex, id)
		}
	}
	for _, definition := range definitions {
		host.toolIndex[definition.ID] = toolBinding{instance: instance, definition: definition}
	}
	host.mu.Unlock()
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
	return "mcp." + namespaceSegment(instanceID) + "." + namespaceSegment(remoteName)
}

func namespaceSegment(value string) string {
	value = strings.TrimSpace(value)
	if safeSegmentPattern.MatchString(value) {
		return value
	}
	cleaned := unsafeSegmentRune.ReplaceAllString(value, "_")
	cleaned = strings.Trim(cleaned, "._-")
	if cleaned == "" {
		cleaned = "capability"
	}
	if len(cleaned) > 96 {
		cleaned = cleaned[:96]
	}
	digest := sha256.Sum256([]byte(value))
	return cleaned + "_" + hex.EncodeToString(digest[:4])
}

func schemaHash(raw json.RawMessage) string {
	canonical := bytes.TrimSpace(raw)
	if len(canonical) > 0 {
		var value any
		if json.Unmarshal(canonical, &value) == nil {
			if encoded, err := json.Marshal(value); err == nil {
				canonical = encoded
			}
		}
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:])
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
