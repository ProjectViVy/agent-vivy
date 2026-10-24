# Search diagnostics: name the provider and the env knob

## What changed

A tester delegated research to a sub-agent and `network_search` failed three
times. On this branch those failures already surface to the model as
recoverable tool results, so the diagnostic text is the only information the
model (and through it the operator) receives. The old text said neither
which provider was used nor which environment variable to configure, so the
failure was unactionable.

- `internal/runtime/network_search.go`
  - The single error funnel in `Search` now wraps the selected provider's
    error with its name: `network search: provider %s failed: %w`. Selection
    picks exactly one provider and only tries that one, so the wording never
    claims "tried all providers". The `%w` preserves the cause chain.
  - The shared HTTP helper (`searchHTTPProvider.get`) now knows the provider
    name, so endpoint-missing, endpoint-invalid, transport, rate-limit, and
    non-2xx messages all name the provider; the non-2xx message reads
    `search provider %s returned HTTP %d`. The empty-endpoint message derives
    the env knob from the authoritative `searchProviderEnvNames` list (for
    searxng this yields `SEARXNG_SEARCH_URL`) instead of duplicating names.
  - bing missing key: `bing search API key is not configured: set
    BING_SEARCH_API_KEY`. google missing key/cx: `google search API key or cx
    is not configured: set GOOGLE_SEARCH_API_KEY and GOOGLE_SEARCH_CX`.
    Both interpolate the env names from the provider fields.
- `internal/tools/network_search.go` — the tool `Description` no longer
  advertises a provider fallback the selection code does not perform
  (displayed truth must derive from capability truth). It now states that
  exactly one provider serves each call, there is no fallback once selected,
  and a failure returns a diagnostic naming that provider and its env knob.
- `internal/runtime/network_search_test.go` — existing rate-limit/size-limit
  assertions updated to the wrapped wording; new
  `TestNetworkSearchServiceNamesProviderAndEnvKnobInFailure` asserts a
  model-visible failure for an explicitly named but unconfigured provider
  (bing, google) contains both the provider name and the env knob names.
  Deterministic: no HTTP request is issued (missing-key check precedes it).

Mechanism, provider selection order, and the reason vocabulary are unchanged;
only message text and the tool description changed.

## Explicitly not done

- `network_search` is not hidden; it remains available exactly as before.
- No reachability probe or capability gate was added. Keyless providers
  (duckduckgo, wikipedia) remain legitimately `Configured: true`.
- No reason vocabulary changed, no new config fields, no new abstractions.
