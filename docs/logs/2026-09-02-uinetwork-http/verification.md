# Verification: UI-NETWORK-HTTP http_request settings surface

## Commands and results (actual record)

Focused unit tests (quick pre-submission loop, all passed):

```text
go build ./...                                     → ok
go test ./internal/runtime/ -run 'TestEinoHTTP'    → ok (including new TestEinoHTTPBackendSetConfigLiveApply)
go test ./internal/app/settings/                   → ok (including TestSaveAndLoadHTTPOverlay / TestValidateHTTPOverlayBounds)
go test ./internal/rpc/                            → ok (including TestControlHandlerHTTPSettingsSegment)
go test ./internal/config/                         → ok
cd ui; pnpm exec tsc -b --noEmit                   → clean
```

Product gates (each round was a full background `just ci` + `just ui-e2e`, tail-checking its
own log):

- Round 1: `just ci` → **CI-EXIT:0** (`/tmp/ci-uinet-http.log`);
  `just ui-e2e` → E2E-EXIT:1, with the only failure in the new spec
  `network-tools-setting.spec.ts › edits the http_request allowlist and timeout`.
- Round 2 (after fixing the root cause): `just ci` → **CI-EXIT:0**
  (`/tmp/ci-uinet-http2.log`); `just ui-e2e` → **E2E-EXIT:0**
  (`/tmp/uie2e-uinet-http2.log`, 17 passed / 1 skipped, including the http block spec in
  this slice and the button-scope revision in the existing network_search spec).

## Round 1 failure root cause and fix (a lesson worth recording)

Failure symptom: the http spec's opening assertion that there was no "Settings override
active" badge failed — the badge was already present on initial load, and the allowlist/
timeout displayed the config defaults.

Root cause: `settings/get` always returned an `http` section (value type, including the
config fallback), and `settingsUpdateFrom` carried the full document by "return any section
that exists" → when another section (the network_search spec) saved, it wrote the config
defaults back to settings.yaml as an http overlay, incorrectly lighting `overlay_set`.

Fix: `settingsUpdateFrom` returns the `http` section only when
`settings.http.overlay_set === true` — without an overlay, saving another section must not
invent one. The e2e work directory (`ui/.e2e-workdir/state/settings.yaml`) is rebuilt by
globalSetup each round; the rerun was clean after the fix.

## Test coverage inventory

- `internal/runtime/http_request_test.go`: SetConfig live replacement of the allowlist
  (including `*.domain` matching only subdomains, not the bare domain, URL-shape
  normalization, and trim), timeout clamping (10 default / 120 maximum / explicit value),
  and nil hosts retaining the current table.
- `internal/app/settings/settings_test.go`: http overlay save/read round-trip, non-zero
  detection, empty overlay normalization to nil, rejection of timeout -1/121, and rejection
  of blank hosts.
- `internal/rpc/control_test.go`: `TestControlHandlerHTTPSettingsSegment` — config fallback
  + no overlay on get; write/echo/persist on update; explicit empty list = deny all; timeout
  0 retains the current value; restore config value; document validation rejects 999;
  OnSettingsChanged exactly 3 times.
- `ui/e2e/network-tools-setting.spec.ts`: real-path spec — existing network_search spec
  (Save button `.first()` scope revision) + new http spec (no badge on initial load, 999
  cannot save, 45 save confirmation, refresh echo + badge, restore config default).
