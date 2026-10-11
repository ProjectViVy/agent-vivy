package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/storage"
)

var ErrBundleHookUnavailable = errors.New("runtime: captured Toolkit Hook is unavailable")

type toolkitWorkspaceRootContextKey struct{}

type toolkitWorkspaceRoot struct {
	path  string
	found bool
	err   error
}

func withToolkitWorkspaceRoot(ctx context.Context, path string, found bool, err error) context.Context {
	return context.WithValue(ctx, toolkitWorkspaceRootContextKey{}, toolkitWorkspaceRoot{path: path, found: found, err: err})
}

func toolkitWorkspacePath(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", errors.New("runtime: Toolkit Hook workspace context is missing")
	}
	root, ok := ctx.Value(toolkitWorkspaceRootContextKey{}).(toolkitWorkspaceRoot)
	if !ok {
		return "", errors.New("runtime: Toolkit Hook workspace context is missing")
	}
	if root.err != nil {
		return "", fmt.Errorf("runtime: resolve Toolkit Hook workspace: %w", root.err)
	}
	if !root.found || strings.TrimSpace(root.path) == "" {
		return "", errors.New("runtime: Toolkit Hook requires an available workspace")
	}
	return root.path, nil
}

// BundleHookExecutionResolver resolves only the explicit execution context
// declared by one immutable Hook. Implementations must not return resources
// outside the captured release.
type BundleHookExecutionResolver interface {
	ResolveHookExecution(context.Context, bundlecontract.View, bundlecontract.ComponentRecord, string) (cwd, resourceDigest string, err error)
}

type BundleHookExecutionResolverFunc func(context.Context, bundlecontract.View, bundlecontract.ComponentRecord, string) (string, string, error)

func (f BundleHookExecutionResolverFunc) ResolveHookExecution(ctx context.Context, view bundlecontract.View, record bundlecontract.ComponentRecord, workspaceRoot string) (string, string, error) {
	return f(ctx, view, record, workspaceRoot)
}

// BundleHookDispatcher executes only PreToolUse hooks explicitly armed at
// admission. Its position after operator hooks preserves the established
// governance order; a captured but now-revoked Hook fails closed.
type BundleHookDispatcher struct {
	store    storage.BundleStore
	resolver BundleHookExecutionResolver
	timeout  time.Duration
}

func NewBundleHookDispatcher(store storage.BundleStore, resolver BundleHookExecutionResolver, timeout time.Duration) *BundleHookDispatcher {
	if timeout <= 0 {
		timeout = time.Second
	}
	return &BundleHookDispatcher{store: store, resolver: resolver, timeout: timeout}
}

func (d *BundleHookDispatcher) Name() string { return "bundle-hooks" }

func (*BundleHookDispatcher) GovernanceIdentity() string { return "bundle-hooks:v1" }

// GovernanceIdentityFor binds review hashes to the exact captured Hook
// declarations, consent revisions, cwd and resource release. Secret values
// are intentionally absent.
func (d *BundleHookDispatcher) GovernanceIdentityFor(ctx context.Context) string {
	view, hasView := bundleView(ctx)
	check, hasCapture := bundleCapture(ctx)
	if !hasView || !hasCapture {
		return "bundle-hooks:v1:none"
	}
	records := capturedBundleHooks(view, check)
	identities := make([]any, 0, len(records))
	workspaceRoot, workspaceErr := toolkitWorkspacePath(ctx)
	for _, record := range records {
		cwd, resourceDigest, err := "", "", workspaceErr
		if err == nil && d != nil && d.resolver != nil {
			cwd, resourceDigest, err = d.resolver.ResolveHookExecution(ctx, view, record, workspaceRoot)
		}
		if err != nil {
			identities = append(identities, struct {
				Ref         bundlecontract.ComponentRef `json:"ref"`
				Unavailable bool                        `json:"unavailable"`
			}{record.Ref, true})
			continue
		}
		consentDigest, _ := bundlecontract.ExecutableConsentDigest(record.Ref.InstallID, record.Release.ReleaseDigest, record.ExecutableDigest, view.Identity().ConfigRevision)
		identities = append(identities, struct {
			Ref               bundlecontract.ComponentRef `json:"ref"`
			ReleaseDigest     string                      `json:"release_digest"`
			DeclarationDigest string                      `json:"declaration_digest"`
			ConsentDigest     string                      `json:"consent_digest"`
			ConsentRevision   int64                       `json:"consent_revision"`
			Cwd               string                      `json:"cwd"`
			ResourceDigest    string                      `json:"resource_digest"`
		}{record.Ref, record.Release.ReleaseDigest, record.ExecutableDigest, consentDigest, capturedConsentRevision(check, record.Ref), cwd, resourceDigest})
	}
	canonical, err := bundlecontract.CanonicalJSON(identities)
	if err != nil {
		return "bundle-hooks:v1:invalid"
	}
	return "bundle-hooks:v1:" + digestText(string(canonical))
}

func (d *BundleHookDispatcher) MatchesToolContext(ctx context.Context, toolName string) bool {
	view, hasView := bundleView(ctx)
	check, hasCapture := bundleCapture(ctx)
	if !hasView || !hasCapture || d == nil {
		return false
	}
	for _, record := range capturedBundleHooks(view, check) {
		if record.Hook != nil && matchesHook(record.Hook.Matcher, toolName) {
			return true
		}
	}
	return false
}

func (d *BundleHookDispatcher) PreToolUse(ctx context.Context, call ToolHookCall) (PreToolUseResult, error) {
	view, hasView := bundleView(ctx)
	check, hasCapture := bundleCapture(ctx)
	if !hasView || !hasCapture {
		return PreToolUseResult{Decision: hookAllow}, nil
	}
	records := capturedBundleHooks(view, check)
	matched := make([]bundlecontract.ComponentRecord, 0, len(records))
	for _, record := range records {
		if record.Hook != nil && matchesHook(record.Hook.Matcher, call.ToolName) {
			matched = append(matched, record)
		}
	}
	if len(matched) == 0 {
		return PreToolUseResult{Decision: hookAllow}, nil
	}
	if d == nil || d.store == nil || d.resolver == nil {
		return PreToolUseResult{}, fmt.Errorf("%w: Hook execution owner is unavailable", ErrHookBlocked)
	}
	if err := checkBundleFence(ctx); err != nil {
		return PreToolUseResult{}, fmt.Errorf("%w: captured Hook guard was revoked: %v", ErrHookBlocked, err)
	}
	workspaceRoot, err := toolkitWorkspacePath(ctx)
	if err != nil {
		return PreToolUseResult{}, fmt.Errorf("%w: %v", ErrHookBlocked, err)
	}
	scripts := make([]ToolHook, 0, len(matched))
	for _, record := range matched {
		captured, ok := capturedExecutable(check, record)
		if !ok {
			continue
		}
		currentConsent, found, consentErr := d.store.ReadBundleConsent(ctx, record.Ref.InstallID, bundlecontract.Hook, record.Ref.LocalID)
		if consentErr != nil {
			return PreToolUseResult{}, fmt.Errorf("%w: read captured Hook consent: %v", ErrHookBlocked, consentErr)
		}
		expectedDigest, digestErr := bundlecontract.ExecutableConsentDigest(record.Ref.InstallID, record.Release.ReleaseDigest, record.ExecutableDigest, view.Identity().ConfigRevision)
		if digestErr != nil || !found || !currentConsent.Allowed || currentConsent.DeclarationDigest != captured.ConsentDigest ||
			currentConsent.DeclarationDigest != expectedDigest || currentConsent.Revision != captured.ConsentRevision {
			return PreToolUseResult{}, fmt.Errorf("%w: captured Hook consent changed or was revoked", ErrHookBlocked)
		}
		cwd, resourceDigest, resolveErr := d.resolver.ResolveHookExecution(ctx, view, record, workspaceRoot)
		if resolveErr != nil {
			return PreToolUseResult{}, fmt.Errorf("%w: resolve captured Hook execution context: %v", ErrHookBlocked, resolveErr)
		}
		env, envErr := resolveBundleHookEnvironment(record.Hook.EnvFrom)
		if envErr != nil {
			return PreToolUseResult{}, fmt.Errorf("%w: resolve captured Hook bindings: %v", ErrHookBlocked, envErr)
		}
		contextBytes, _ := bundlecontract.CanonicalJSON(struct {
			ConsentDigest  string `json:"consent_digest"`
			Cwd            string `json:"cwd"`
			ResourceDigest string `json:"resource_digest"`
		}{captured.ConsentDigest, cwd, resourceDigest})
		contextDigest := digestText(string(contextBytes))
		hook := NewScriptHook(record.Hook.Command, record.Hook.Matcher, time.Duration(record.Hook.TimeoutMs)*time.Millisecond)
		hook.Cwd, hook.Env = cwd, env
		hook.ContextDigest, hook.ResourceDigest = contextDigest, resourceDigest
		hook.runScoped = runHookProcessScoped
		scripts = append(scripts, hook)
	}
	if len(scripts) == 0 {
		return PreToolUseResult{Decision: hookAllow}, nil
	}
	chain := NewToolHookChain(d.timeout, scripts...)
	updated, err := chain.PreToolUse(ctx, call)
	if err != nil {
		return PreToolUseResult{}, err
	}
	if bytes.Equal(updated, call.Arguments) || (len(updated) == 0 && len(call.Arguments) == 0) {
		return PreToolUseResult{Decision: hookAllow}, nil
	}
	return PreToolUseResult{Decision: hookRewrite, UpdatedArgs: updated}, nil
}

func (*BundleHookDispatcher) PostToolUse(context.Context, ToolHookCall, string, error) error {
	return nil
}

func capturedBundleHooks(view bundlecontract.View, check bundlecontract.CaptureCheck) []bundlecontract.ComponentRecord {
	executableRefs := make(map[bundlecontract.ComponentRef]bundlecontract.ExecutableRef, len(check.Executables))
	for _, ref := range check.Executables {
		if ref.Kind == bundlecontract.Hook {
			executableRefs[ref.ComponentRef] = ref
		}
	}
	records := make([]bundlecontract.ComponentRecord, 0, len(executableRefs))
	for _, record := range view.Components() {
		ref, ok := executableRefs[record.Ref]
		if !ok || record.Ref.Kind != bundlecontract.Hook || record.Hook == nil || record.Hook.Event != "PreToolUse" || record.ExecutableDigest != ref.DeclarationDigest {
			continue
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Release.BundleID != records[j].Release.BundleID {
			return records[i].Release.BundleID < records[j].Release.BundleID
		}
		return records[i].Ref.LocalID < records[j].Ref.LocalID
	})
	return records
}

func capturedExecutable(check bundlecontract.CaptureCheck, record bundlecontract.ComponentRecord) (bundlecontract.ExecutableRef, bool) {
	for _, ref := range check.Executables {
		if ref.ComponentRef == record.Ref && ref.DeclarationDigest == record.ExecutableDigest && ref.Kind == bundlecontract.Hook {
			return ref, true
		}
	}
	return bundlecontract.ExecutableRef{}, false
}

func capturedConsentRevision(check bundlecontract.CaptureCheck, ref bundlecontract.ComponentRef) int64 {
	for _, consent := range check.Consents {
		if consent.ComponentRef == ref {
			return consent.Revision
		}
	}
	return 0
}

func matchesHook(matcher, toolName string) bool {
	if matcher == "" || matcher == "*" {
		return true
	}
	matched, err := path.Match(matcher, toolName)
	return err == nil && matched
}

func resolveBundleHookEnvironment(envFrom map[string]string) ([]string, error) {
	values := map[string]string{}
	for _, name := range []string{"PATH", "PATHEXT", "SYSTEMROOT", "WINDIR", "TEMP", "TMP", "LANG", "LC_ALL", "NO_COLOR", "TERM"} {
		if value, ok := os.LookupEnv(name); ok && value != "" {
			values[name] = value
		}
	}
	children := make([]string, 0, len(envFrom))
	for child := range envFrom {
		children = append(children, child)
	}
	sort.Strings(children)
	for _, child := range children {
		host := strings.TrimSpace(envFrom[child])
		if !validHookEnvironmentName(child) || !validHookEnvironmentName(host) {
			return nil, errors.New("invalid environment binding name")
		}
		if _, reserved := values[child]; reserved {
			return nil, errors.New("environment binding cannot replace a host execution variable")
		}
		value, ok := os.LookupEnv(host)
		if !ok || value == "" {
			return nil, fmt.Errorf("required host binding %q is not set", host)
		}
		if len(value) > 4096 || strings.IndexByte(value, 0) >= 0 || !utf8.ValidString(value) {
			return nil, fmt.Errorf("host binding %q is invalid or too large", host)
		}
		values[child] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+values[key])
	}
	return env, nil
}

func validHookEnvironmentName(value string) bool {
	if value == "" || len(value) > 128 || !((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z') || value[0] == '_') {
		return false
	}
	for _, ch := range value[1:] {
		if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_') {
			return false
		}
	}
	return true
}
