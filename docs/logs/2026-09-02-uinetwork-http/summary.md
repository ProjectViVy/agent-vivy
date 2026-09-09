# UI-NETWORK-HTTP: http_request tool surface in settings docs + RPC section + settings card

## Deliverables

The domain allowlist and request timeout for `http_request` (read-only web fetch) move from
plain `config.yaml` fields to a three-layer structure of "config defaults + user-workspace
settings.yaml overlay", with an editing entry in the Network tools settings card.

- **config**: `runtime.http_timeout_seconds` (default 10; runtime backend clamps 1–120),
  alongside the existing `runtime.http_allowed_hosts`.
- **settings.yaml**: `http` overlay (`HTTPSettings{allowed_hosts *[]string;
  timeout_seconds int}`). A nil pointer = use config; nil hosts inside the block = use the
  config allowlist; timeout 0 = use config; an explicit empty list = deny all on the
  read-only surface. An empty overlay normalizes to nil on load (stable document rewrite).
- **runtime**: `EinoHTTPBackend` adds `SetConfig(allowedHosts, timeoutSeconds)` for live
  application (RWMutex protects allowlist/client reads); the constructor adds a timeoutSeconds
  argument and `httpTimeout` clamping (≤0 → 10s, >120 → 120s).
- **RPC**: `settings/get` adds an `http` section (effective value + `config_*` fallback +
  `overlay_set`); `settings/update` accepts `http{allowed_hosts, timeout_seconds}` with the
  same replacement semantics as sandbox (omitted hosts inside the block = retain current
  value; explicit empty array = deny all).
- **live-apply**: `applyLiveHTTPSettings` merges the settings overlay into the running
  backend at startup and on OnSettingsChanged (following the `applyLiveSandboxSettings`
  pattern).
- **UI**: `NetworkToolsCard` adds an http_request tool-surface block — domain-allowlist
  textarea (newline/comma-separated, with `*.domain` wildcard semantics), numeric timeout
  input (0 = use config default, >120 disables Save), override badge, and independent save
  confirmation; `settingsUpdateFrom` carries the http section with the full document to
  prevent saves from other sections from accidentally clearing it.

Enable/disable (`tools_enabled`) is outside this slice — the existing tool-toggle overlay
covers it, and the card copy only indicates read-only semantics.

## Explicitly not done

- An http_request enable/disable switch (`tools_enabled` is existing capability; do not
  duplicate it).
- Finer-grained fetch policies such as request headers and User-Agent (no requirements
  input).

## Related

- TODO: UI-NETWORK-HTTP → DONE (moved in §0.1 + §10 record).
- Pattern precedents: the sandbox/compaction overlay (pointer fields distinguish "unset" from
  "set empty"), `applyLiveSandboxSettings`, and `NetworkToolsCard`.
