package loopexp

// WarmingFactory constructs policy only; a Host retains model/accounting authority.
type WarmingFactory func() WarmingPolicy

type WarmingInput struct {
	Mode                                                  string
	SupportsWarming                                       bool
	CacheLifetimeSeconds, PrefixTokens                    int
	InputPerMTokens, CachedInputPerMTokens, MinSavingsUSD float64
}

type WarmingRequest struct {
	Prompt          string
	MaxOutputTokens int
	TimeoutMillis   int64
}

type WarmingDecision struct {
	Reason  string
	Request *WarmingRequest
}
type WarmingPolicy interface {
	Decide(WarmingInput) WarmingDecision
}
