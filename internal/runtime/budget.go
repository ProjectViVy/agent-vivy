package runtime

import (
	"errors"
	"fmt"
	"sync"

	"agent-vivy/internal/domain"
)

// BudgetPolicy bounds one run and all of its descendants. A zero limit means
// unlimited for the standalone ledger; application configuration supplies
// positive defaults. Child ledgers can tighten a limit, never widen the
// shared parent account.
type BudgetPolicy struct {
	MaxEvents     int
	MaxModelCalls int
	MaxToolCalls  int
	MaxRetries    int
}

const (
	defaultMaxRunEvents    = 512
	defaultMaxModelCalls   = 32
	defaultMaxRunToolCalls = 64
	defaultMaxRunRetries   = 3
)

// DefaultBudgetPolicy is the application-level fail-safe used by services
// whose composition root does not provide explicit limits (notably tests).
func DefaultBudgetPolicy() BudgetPolicy {
	return BudgetPolicy{
		MaxEvents:     defaultMaxRunEvents,
		MaxModelCalls: defaultMaxModelCalls,
		MaxToolCalls:  defaultMaxRunToolCalls,
		MaxRetries:    defaultMaxRunRetries,
	}
}

func (p BudgetPolicy) validate() error {
	if p.MaxEvents < 0 || p.MaxModelCalls < 0 || p.MaxToolCalls < 0 || p.MaxRetries < 0 {
		return errors.New("runtime: budget limits must not be negative")
	}
	return nil
}

// BudgetKind identifies one bounded resource.
type BudgetKind string

const (
	BudgetEvents     BudgetKind = "events"
	BudgetModelCalls BudgetKind = "model_calls"
	BudgetToolCalls  BudgetKind = "tool_calls"
	BudgetRetries    BudgetKind = "retries"
)

// ErrBudgetExceeded is stable for callers that need to classify a circuit
// breaker without parsing a user-visible message.
var ErrBudgetExceeded = errors.New("runtime: budget exceeded")

// BudgetExceededError carries only bounded accounting data; it never carries
// prompt, tool arguments, provider output, or other user content.
type BudgetExceededError struct {
	Kind  BudgetKind
	Limit int
	Used  int
}

func (e *BudgetExceededError) Error() string {
	return fmt.Sprintf("%s: %s limit %d reached at %d", ErrBudgetExceeded, e.Kind, e.Limit, e.Used)
}

func (e *BudgetExceededError) Unwrap() error { return ErrBudgetExceeded }

// BudgetUsage is a point-in-time, process-local accounting snapshot. It is
// observability data only and contains no transcript or memory content.
type BudgetUsage struct {
	Events     int `json:"events"`
	ModelCalls int `json:"model_calls"`
	ToolCalls  int `json:"tool_calls"`
	Retries    int `json:"retries"`
}

// BudgetSnapshot combines limits and current shared usage.
type BudgetSnapshot struct {
	Policy BudgetPolicy
	Usage  BudgetUsage
}

type budgetAccount struct {
	mu     sync.Mutex
	policy BudgetPolicy
	usage  BudgetUsage
}

type budgetScope struct {
	policy BudgetPolicy
	usage  BudgetUsage
	parent *budgetScope
	mu     sync.Mutex
}

// BudgetLedger is a concurrency-safe local view over a shared budget
// account. Child views have their own local cap while reservations still
// consume the root account, so a child cannot bypass its parent.
type BudgetLedger struct {
	account *budgetAccount
	policy  BudgetPolicy
	scope   *budgetScope
}

// NewBudgetLedger creates a root accounting scope. Use DefaultBudgetPolicy
// when a bounded application scope is required.
func NewBudgetLedger(policy BudgetPolicy) (*BudgetLedger, error) {
	if err := policy.validate(); err != nil {
		return nil, err
	}
	return &BudgetLedger{
		account: &budgetAccount{policy: policy},
		policy:  policy,
		scope:   &budgetScope{policy: policy},
	}, nil
}

// Child creates a nested scope. The shared account retains the root limits,
// while policy supplies an optional tighter local cap for the child.
func (l *BudgetLedger) Child(policy BudgetPolicy) (*BudgetLedger, error) {
	if l == nil || l.account == nil {
		return nil, errors.New("runtime: nil parent budget ledger")
	}
	if err := policy.validate(); err != nil {
		return nil, err
	}
	effective := tighterPolicy(l.policy, policy)
	return &BudgetLedger{
		account: l.account,
		policy:  effective,
		scope:   &budgetScope{policy: effective, parent: l.scope},
	}, nil
}

// Reserve charges one unit to the local scope and the shared root account.
// The check and increment are atomic across concurrent child reservations.
func (l *BudgetLedger) Reserve(kind BudgetKind) error {
	if l == nil || l.account == nil {
		return errors.New("runtime: nil budget ledger")
	}
	if l.scope == nil {
		return errors.New("runtime: budget ledger has no scope")
	}
	scopes := scopeChain(l.scope)
	for _, scope := range scopes {
		scope.mu.Lock()
	}
	defer func() {
		for i := len(scopes) - 1; i >= 0; i-- {
			scopes[i].mu.Unlock()
		}
	}()

	for _, scope := range scopes {
		localUsed := usageFor(scope.usage, kind)
		localLimit := limitFor(scope.policy, kind)
		if localLimit > 0 && localUsed >= localLimit {
			return &BudgetExceededError{Kind: kind, Limit: localLimit, Used: localUsed}
		}
	}

	l.account.mu.Lock()
	defer l.account.mu.Unlock()
	sharedUsed := usageFor(l.account.usage, kind)
	sharedLimit := limitFor(l.account.policy, kind)
	if sharedLimit > 0 && sharedUsed >= sharedLimit {
		return &BudgetExceededError{Kind: kind, Limit: sharedLimit, Used: sharedUsed}
	}
	for _, scope := range scopes {
		addUsage(&scope.usage, kind)
	}
	addUsage(&l.account.usage, kind)
	return nil
}

func (l *BudgetLedger) ReserveEvent() error     { return l.Reserve(BudgetEvents) }
func (l *BudgetLedger) ReserveModelCall() error { return l.Reserve(BudgetModelCalls) }
func (l *BudgetLedger) ReserveToolCall() error  { return l.Reserve(BudgetToolCalls) }
func (l *BudgetLedger) ReserveRetry() error     { return l.Reserve(BudgetRetries) }

// Snapshot returns the shared root usage and this ledger's effective policy.
func (l *BudgetLedger) Snapshot() BudgetSnapshot {
	if l == nil || l.account == nil {
		return BudgetSnapshot{}
	}
	l.account.mu.Lock()
	defer l.account.mu.Unlock()
	return BudgetSnapshot{Policy: l.policy, Usage: l.account.usage}
}

// ReplayEvent reconstructs accounting from durable run events during restart
// recovery. Terminal events are mandatory closure records and do not consume
// the non-terminal event quota.
func (l *BudgetLedger) ReplayEvent(ev domain.RunEvent) error {
	if !ev.Type.Terminal() && ev.Type != domain.EventModelDelta && ev.Type != domain.EventModelReasoningDelta {
		if err := l.ReserveEvent(); err != nil {
			return err
		}
	}
	switch ev.Type {
	case domain.EventProviderRetry:
		return l.ReserveRetry()
	case domain.EventToolRequested:
		if err := l.ReserveModelCall(); err != nil {
			return err
		}
		return l.ReserveToolCall()
	case domain.EventModelCompleted:
		return l.ReserveModelCall()
	default:
		return nil
	}
}

func usageFor(usage BudgetUsage, kind BudgetKind) int {
	switch kind {
	case BudgetEvents:
		return usage.Events
	case BudgetModelCalls:
		return usage.ModelCalls
	case BudgetToolCalls:
		return usage.ToolCalls
	case BudgetRetries:
		return usage.Retries
	default:
		return 0
	}
}

func limitFor(policy BudgetPolicy, kind BudgetKind) int {
	switch kind {
	case BudgetEvents:
		return policy.MaxEvents
	case BudgetModelCalls:
		return policy.MaxModelCalls
	case BudgetToolCalls:
		return policy.MaxToolCalls
	case BudgetRetries:
		return policy.MaxRetries
	default:
		return 0
	}
}

func addUsage(usage *BudgetUsage, kind BudgetKind) {
	switch kind {
	case BudgetEvents:
		usage.Events++
	case BudgetModelCalls:
		usage.ModelCalls++
	case BudgetToolCalls:
		usage.ToolCalls++
	case BudgetRetries:
		usage.Retries++
	}
}

func scopeChain(leaf *budgetScope) []*budgetScope {
	var reversed []*budgetScope
	for scope := leaf; scope != nil; scope = scope.parent {
		reversed = append(reversed, scope)
	}
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return reversed
}

func tighterPolicy(parent, child BudgetPolicy) BudgetPolicy {
	return BudgetPolicy{
		MaxEvents:     tighterLimit(parent.MaxEvents, child.MaxEvents),
		MaxModelCalls: tighterLimit(parent.MaxModelCalls, child.MaxModelCalls),
		MaxToolCalls:  tighterLimit(parent.MaxToolCalls, child.MaxToolCalls),
		MaxRetries:    tighterLimit(parent.MaxRetries, child.MaxRetries),
	}
}

func tighterLimit(parent, child int) int {
	if parent == 0 {
		return child
	}
	if child == 0 || parent < child {
		return parent
	}
	return child
}
