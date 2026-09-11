// Package controlaction defines the typed std/control-action@v1 Provider
// contract. Providers describe one namespaced backend operation and receive a
// Host facade for the very small set of kernel-owned paths an action may use.
// The package intentionally contains no RPC, storage, Policy, Secret store, or
// Eino types.
package controlaction

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"agent-vivy/sdk/module"
)

const Port = "std/control-action@v1"

// Grant aliases keep the focused Port contract convenient for Provider
// authors while preserving the canonical Module Grant vocabulary.
type Grant = module.Grant

const (
	GrantFSRead         = module.GrantFSRead
	GrantFSWrite        = module.GrantFSWrite
	GrantChannelPoll    = module.GrantChannelPoll
	GrantChannelWebhook = module.GrantChannelWebhook
	GrantChannelListen  = module.GrantChannelListen
	GrantChannelA2A     = module.GrantChannelA2A
	GrantSecretRead     = module.GrantSecretRead
	GrantProcSpawn      = module.GrantProcSpawn
	GrantTTY            = module.GrantTTY
	GrantArgv           = module.GrantArgv
	GrantRPCClient      = module.GrantRPCClient
	GrantNetClient      = module.GrantNetClient
)

// Effect describes the authority and audit class of an action. The values are
// deliberately closed: a new effect needs a Port contract update rather than
// silently being treated as a read.
type Effect string

const (
	EffectRead           Effect = "read"
	EffectWrite          Effect = "write"
	EffectExternalEffect Effect = "external-effect"
	// EffectExternal is a short source-level alias for callers that prefer the
	// noun form. It has the same wire value as EffectExternalEffect.
	EffectExternal Effect = EffectExternalEffect
)

var (
	ErrInvalidDefinition         = errors.New("controlaction: invalid action definition")
	ErrInvalidInput              = errors.New("controlaction: invalid action input")
	ErrInvalidOutput             = errors.New("controlaction: invalid action output")
	ErrOwnerMismatch             = errors.New("controlaction: action owner mismatch")
	ErrActionNotFound            = errors.New("controlaction: action not found")
	ErrUnauthenticated           = errors.New("controlaction: caller is not authenticated")
	ErrGenerationUnavailable     = errors.New("controlaction: generation unavailable")
	ErrInstanceUnavailable       = errors.New("controlaction: action instance unavailable")
	ErrGrantDenied               = errors.New("controlaction: grant denied")
	ErrApprovalRequired          = errors.New("controlaction: action approval required")
	ErrSecretLeak                = errors.New("controlaction: action result contains a Secret")
	ErrActionTimeout             = errors.New("controlaction: action timed out")
	ErrProviderPanic             = errors.New("controlaction: action provider panicked")
	ErrProviderFailed            = errors.New("controlaction: action provider failed")
	ErrHostClosed                = errors.New("controlaction: host is closed")
	ErrDuplicateAction           = errors.New("controlaction: duplicate action")
	ErrAuthenticationUnavailable = errors.New("controlaction: authentication is unavailable")
	ErrAuthorizationUnavailable  = errors.New("controlaction: authorization is unavailable")
	ErrAuditUnavailable          = errors.New("controlaction: audit is unavailable")
	ErrSecretUnavailable         = errors.New("controlaction: secret is unavailable")
	ErrRunDenied                 = errors.New("controlaction: run bridge denied")
	ErrToolDenied                = errors.New("controlaction: tool bridge denied")
	ErrConcurrencyLimit          = errors.New("controlaction: concurrency limit reached")
)

var (
	ownerPattern  = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?/[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)
	actionPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._/-]*[a-z0-9])?$`)
)

// Definition is the immutable metadata for one Provider. Owner and ModuleID,
// OutputSchema and ResultSchema, and RequiredGrants and Grants are aliases
// retained in the source contract so generated and hand-written providers can
// use the vocabulary used by their surrounding Module. When both members of a
// pair are present they must be identical.
type Definition struct {
	ID          string
	Description string

	// Owner is the canonical Module identity. ModuleID is its explicit alias.
	Owner    string
	ModuleID string

	// InstanceID identifies the configured runtime instance. Empty means this
	// is a generation-scoped action and needs no instance lookup.
	InstanceID string

	InputSchema  json.RawMessage
	ResultSchema json.RawMessage
	// OutputSchema is an alias for ResultSchema.
	OutputSchema json.RawMessage

	Effect Effect

	RequiredGrants []module.Grant
	// Grants is an alias for RequiredGrants.
	Grants []module.Grant

	RequiresApproval bool
	// ApprovalRequired is an alias for RequiresApproval.
	ApprovalRequired bool

	// Timeout is an optional per-action execution limit. The Host applies its
	// own maximum even when a Provider asks for a longer duration.
	Timeout time.Duration
	// TimeoutMS is a JSON/config-friendly alias for Timeout.
	TimeoutMS int

	MaxInputBytes  int
	MaxOutputBytes int
}

// Provider is the sole public implementation shape for control actions.
// Host owns every validation, authority, and lifecycle decision around this
// call; a Provider only implements the operation itself.
type Provider interface {
	Definition() Definition
	Invoke(context.Context, Host, json.RawMessage) (json.RawMessage, error)
}

// ProviderSet is compiler-emitted Assembly metadata. The ModuleID and
// AllowedIDs come from the sealed Source Catalog; EffectiveGrants are the
// compiler intersection. Host composition must use this envelope instead of
// registering an arbitrary Provider slice.
type ProviderSet struct {
	ModuleID        string
	AllowedIDs      []string
	Providers       []Provider
	EffectiveGrants []module.GrantBinding
	InstanceID      string
}

// ProviderFunc adapts a function to Provider and is useful for small Modules
// and conformance fixtures.
type ProviderFunc struct {
	Def Definition
	Fn  func(context.Context, Host, json.RawMessage) (json.RawMessage, error)
}

func (provider ProviderFunc) Definition() Definition { return provider.Def }

func (provider ProviderFunc) Invoke(ctx context.Context, host Host, input json.RawMessage) (json.RawMessage, error) {
	if provider.Fn == nil {
		return nil, errors.New("controlaction: nil ProviderFunc")
	}
	return provider.Fn(ctx, host, input)
}

// Host is the scoped capability facade available to a Provider. StartRun and
// InvokeTool are intentionally named bridges: implementations must re-enter
// the existing Service.Run and ToolHost paths supplied by the kernel Host.
type Host interface {
	module.Host

	// Grant checks the effective, compiler-approved Grant for this Module.
	Grant(module.Grant) error
	HasGrant(module.Grant) bool

	// Secret resolves only a named, effective secret.read reference. A Host
	// implementation may return the value to the Provider, but must reject any
	// attempt to return it through action output or errors.
	Secret(string) (string, error)

	// Settings returns a redacted/opaque settings projection owned by the Host.
	Settings() json.RawMessage

	StartRun(context.Context, RunRequest) (RunResult, error)
	InvokeTool(context.Context, string, json.RawMessage) (json.RawMessage, error)
}

// RunRequest is the only action-to-runtime Run bridge. Policy, approval,
// identity, Journal, and workspace data are filled or checked by Service.Run;
// fields here are intentionally bounded and transport-neutral.
type RunRequest struct {
	SessionID     string `json:"session_id"`
	Text          string `json:"text"`
	Mode          string `json:"mode,omitempty"`
	PolicyProfile string `json:"policy_profile,omitempty"`
}

type RunResult struct {
	ID     string `json:"id"`
	Status string `json:"status,omitempty"`
}

// Normalize validates and canonicalizes source aliases. Hosts should use the
// normalized value once at registration and never mutate a Provider's return
// value. The returned slices and JSON values are owned copies.
func (definition Definition) Normalize() (Definition, error) {
	owner := strings.TrimSpace(definition.Owner)
	moduleID := strings.TrimSpace(definition.ModuleID)
	if owner != "" && moduleID != "" && owner != moduleID {
		return Definition{}, fmt.Errorf("%w: owner %q disagrees with moduleID %q", ErrInvalidDefinition, definition.Owner, definition.ModuleID)
	}
	if owner == "" {
		owner = moduleID
	}
	definition.Owner = owner
	definition.ModuleID = owner
	definition.InstanceID = strings.TrimSpace(definition.InstanceID)

	inputSchema, err := canonicalJSON("input schema", definition.InputSchema)
	if err != nil {
		return Definition{}, err
	}
	resultSchema, err := aliasJSON("result schema", definition.ResultSchema, definition.OutputSchema)
	if err != nil {
		return Definition{}, err
	}
	definition.ResultSchema = resultSchema
	definition.OutputSchema = append(json.RawMessage(nil), resultSchema...)

	grants, err := aliasGrants(definition.RequiredGrants, definition.Grants)
	if err != nil {
		return Definition{}, err
	}
	definition.RequiredGrants = grants
	definition.Grants = append([]module.Grant(nil), grants...)

	if definition.RequiresApproval && definition.ApprovalRequired == false {
		definition.ApprovalRequired = true
	}
	if definition.ApprovalRequired {
		definition.RequiresApproval = true
	}
	if err := normalizeTimeout(&definition); err != nil {
		return Definition{}, err
	}
	definition.InputSchema = inputSchema
	definition.ResultSchema = append(json.RawMessage(nil), definition.ResultSchema...)
	definition.OutputSchema = append(json.RawMessage(nil), definition.OutputSchema...)
	definition.RequiredGrants = append([]module.Grant(nil), definition.RequiredGrants...)
	definition.Grants = append([]module.Grant(nil), definition.Grants...)
	if err := definition.Validate(); err != nil {
		return Definition{}, err
	}
	return definition, nil
}

// Validate checks Provider-local metadata. It does not resolve Trust,
// effective Grants, configured instances, or caller identity; those are Host
// decisions.
func (definition Definition) Validate() error {
	if strings.TrimSpace(definition.ID) == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidDefinition)
	}
	if definition.ID != strings.TrimSpace(definition.ID) || len(definition.ID) > maxIdentifierBytes || containsControl(definition.ID) || !actionPattern.MatchString(definition.ID) {
		return fmt.Errorf("%w: invalid action id %q", ErrInvalidDefinition, definition.ID)
	}
	if len(definition.Description) > maxDescriptionBytes || containsControl(definition.Description) {
		return fmt.Errorf("%w: description is too large or contains control bytes", ErrInvalidDefinition)
	}
	owner := strings.TrimSpace(definition.Owner)
	moduleID := strings.TrimSpace(definition.ModuleID)
	if owner == "" && moduleID == "" {
		return fmt.Errorf("%w: owner is required", ErrInvalidDefinition)
	}
	if owner != "" && moduleID != "" && owner != moduleID {
		return fmt.Errorf("%w: owner %q disagrees with moduleID %q", ErrInvalidDefinition, owner, moduleID)
	}
	if owner == "" {
		owner = moduleID
	}
	if len(owner) > maxIdentifierBytes || containsControl(owner) || !ownerPattern.MatchString(owner) {
		return fmt.Errorf("%w: invalid owner Module ID %q", ErrInvalidDefinition, owner)
	}
	if definition.InstanceID != "" {
		if len(definition.InstanceID) > maxIdentifierBytes || containsControl(definition.InstanceID) || definition.InstanceID != strings.TrimSpace(definition.InstanceID) {
			return fmt.Errorf("%w: invalid instance id", ErrInvalidDefinition)
		}
	}
	namespace := strings.SplitN(owner, "/", 2)[0] + "."
	literalNamespace := "plugin." + owner + "."
	if !strings.HasPrefix(definition.ID, namespace) && !strings.HasPrefix(definition.ID, literalNamespace) {
		return fmt.Errorf("%w: action id %q is outside owner namespace %s*", ErrInvalidDefinition, definition.ID, namespace)
	}
	switch definition.Effect {
	case EffectRead, EffectWrite, EffectExternalEffect:
	default:
		return fmt.Errorf("%w: unsupported effect %q", ErrInvalidDefinition, definition.Effect)
	}
	if err := validateSchema("input", definition.InputSchema); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidDefinition, err)
	}
	resultSchema, err := aliasJSON("result schema", definition.ResultSchema, definition.OutputSchema)
	if err != nil {
		return err
	}
	if err := validateSchema("result", resultSchema); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidDefinition, err)
	}
	grants, err := aliasGrants(definition.RequiredGrants, definition.Grants)
	if err != nil {
		return err
	}
	seen := make(map[module.Grant]struct{}, len(grants))
	for _, grant := range grants {
		if !grant.Valid() {
			return fmt.Errorf("%w: unknown Grant %q", ErrInvalidDefinition, grant)
		}
		if _, exists := seen[grant]; exists {
			return fmt.Errorf("%w: duplicate Grant %q", ErrInvalidDefinition, grant)
		}
		seen[grant] = struct{}{}
	}
	if err := validateTimeout(definition); err != nil {
		return err
	}
	if definition.MaxInputBytes < 0 || definition.MaxOutputBytes < 0 {
		return fmt.Errorf("%w: payload bounds must not be negative", ErrInvalidDefinition)
	}
	if definition.MaxInputBytes > maxPayloadBytes || definition.MaxOutputBytes > maxPayloadBytes {
		return fmt.Errorf("%w: payload bounds exceed %d bytes", ErrInvalidDefinition, maxPayloadBytes)
	}
	return nil
}

const (
	maxIdentifierBytes  = 256
	maxDescriptionBytes = 4096
	maxPayloadBytes     = 16 << 20
	maxSchemaBytes      = 64 << 10
	maxSchemaDepth      = 32
	maxSchemaNodes      = 2048
	maxSchemaKeywords   = 8192
)

func containsControl(value string) bool {
	return strings.ContainsAny(value, "\r\n\x00")
}

func validateTimeout(definition Definition) error {
	if definition.Timeout < 0 || definition.TimeoutMS < 0 {
		return fmt.Errorf("%w: timeout must not be negative", ErrInvalidDefinition)
	}
	if definition.TimeoutMS == 0 || definition.Timeout == 0 {
		return nil
	}
	converted := time.Duration(definition.TimeoutMS) * time.Millisecond
	// Overflow turns a positive duration negative. Treat that as malformed
	// metadata instead of allowing a wrapped deadline through the Host.
	if converted <= 0 || definition.Timeout != converted {
		return fmt.Errorf("%w: timeout and timeout_ms disagree", ErrInvalidDefinition)
	}
	return nil
}

func normalizeTimeout(definition *Definition) error {
	if err := validateTimeout(*definition); err != nil {
		return err
	}
	if definition.Timeout == 0 && definition.TimeoutMS > 0 {
		converted := time.Duration(definition.TimeoutMS) * time.Millisecond
		if converted <= 0 {
			return fmt.Errorf("%w: timeout is out of range", ErrInvalidDefinition)
		}
		definition.Timeout = converted
	}
	// Preserve sub-millisecond Go durations without manufacturing a lossy
	// TimeoutMS alias. Whole-millisecond durations get the config projection.
	if definition.Timeout > 0 && definition.Timeout%time.Millisecond == 0 {
		definition.TimeoutMS = int(definition.Timeout / time.Millisecond)
	} else if definition.TimeoutMS == 0 {
		definition.TimeoutMS = 0
	}
	return nil
}

func aliasJSON(name string, first, second json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(first)) == 0 {
		return canonicalJSON(name, second)
	}
	canonicalFirst, err := canonicalJSON(name, first)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(second)) == 0 {
		return canonicalFirst, nil
	}
	canonicalSecond, err := canonicalJSON(name, second)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(canonicalFirst, canonicalSecond) {
		return nil, fmt.Errorf("%w: %s aliases disagree", ErrInvalidDefinition, name)
	}
	return canonicalFirst, nil
}

func aliasGrants(first, second []module.Grant) ([]module.Grant, error) {
	if len(first) > 0 && len(second) > 0 {
		if len(first) != len(second) {
			return nil, fmt.Errorf("%w: grants aliases disagree", ErrInvalidDefinition)
		}
		for i := range first {
			if first[i] != second[i] {
				return nil, fmt.Errorf("%w: grants aliases disagree", ErrInvalidDefinition)
			}
		}
	}
	if len(first) == 0 {
		first = second
	}
	return append([]module.Grant(nil), first...), nil
}

func validateSchema(name string, raw json.RawMessage) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return fmt.Errorf("%s schema is required", name)
	}
	if len(trimmed) > maxSchemaBytes {
		return fmt.Errorf("%s schema exceeds %d bytes", name, maxSchemaBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("%s schema is invalid JSON: %w", name, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return fmt.Errorf("%s schema must contain one JSON value: %w", name, err)
	}
	if value == nil {
		return fmt.Errorf("%s schema must be a JSON object", name)
	}
	if _, ok := value.(map[string]any); !ok {
		return fmt.Errorf("%s schema must be a JSON object", name)
	}
	if err := validateSchemaShape(value); err != nil {
		return fmt.Errorf("%s schema: %v", name, err)
	}
	return nil
}

func canonicalJSON(name string, raw json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("%w: %s schema is required", ErrInvalidDefinition, name)
	}
	if len(trimmed) > maxSchemaBytes {
		return nil, fmt.Errorf("%w: %s schema exceeds %d bytes", ErrInvalidDefinition, name, maxSchemaBytes)
	}
	value, err := decodeOneJSON(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%w: %s schema is invalid JSON: %v", ErrInvalidDefinition, name, err)
	}
	if _, ok := value.(map[string]any); !ok {
		return nil, fmt.Errorf("%w: %s schema must be a JSON object", ErrInvalidDefinition, name)
	}
	if err := validateSchemaShape(value); err != nil {
		return nil, fmt.Errorf("%w: %s schema: %v", ErrInvalidDefinition, name, err)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: canonicalize %s schema: %v", ErrInvalidDefinition, name, err)
	}
	return canonical, nil
}

func decodeOneJSON(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return nil, err
	}
	return value, nil
}

func validateSchemaShape(value any) error {
	nodes, keywords := 0, 0
	var walk func(any, int) error
	walk = func(node any, depth int) error {
		if depth > maxSchemaDepth {
			return errors.New("maximum schema depth exceeded")
		}
		nodes++
		if nodes > maxSchemaNodes {
			return errors.New("maximum schema node count exceeded")
		}
		switch typed := node.(type) {
		case map[string]any:
			keywords += len(typed)
			if keywords > maxSchemaKeywords {
				return errors.New("maximum schema keyword count exceeded")
			}
			if schemaURL, ok := typed["$schema"]; ok {
				value, ok := schemaURL.(string)
				if !ok || value != "https://json-schema.org/draft/2020-12/schema" {
					return errors.New("only JSON Schema draft 2020-12 is supported")
				}
			}
			for _, child := range typed {
				if err := walk(child, depth+1); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range typed {
				if err := walk(child, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(value, 0)
}
