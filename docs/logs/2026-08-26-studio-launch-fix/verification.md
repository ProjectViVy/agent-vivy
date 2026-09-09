# Verification — 2026-08-26 studio-launch-fix

## Reproduction before the fix

`powershell -NoProfile -ExecutionPolicy Bypass -File ./launch-vivy-studio.ps1`
→ dsh threw `ERR_MODULE_NOT_FOUND` (`dsh-plugin` could not be resolved); `:3090` was unresponsive.

## Verification after the fix (real launch, not unit tests)

1. Restarted `launch-vivy-studio.ps1`; log:

   ```
   [hub] routes mounted (profile=vivy-studio, loader=provided)
   dsh web: http://127.0.0.1:3090
   ```

2. Route health checks (all HTTP 200):

   | Endpoint | Status |
   |---|---|
   | `http://127.0.0.1:3090/` (main page) | 200 |
   | `/vivy-debugger/api/status` (debugger) | 200 |
   | `/dsh-plugin-hub/settings` (plugin-hub) | 200 |
   | `/dsh-plugin-hub/debug/loader-entries` (plugin-hub) | 200 |

3. `node_modules/dsh-plugin/package.json` is named `dsh-plugin`, and missing
   modules such as `lib/services/install/` are present.

## just ci

`just ci` is the kernel/UI gate; this change consists of the Studio PowerShell
launch script, the third-party plugin-hub `lib` rebuild, and installed-profile
runtime state, all outside `just ci`’s coverage (`justfile` ci: fmt-check vet test
headless-compile ui-ci). They were verified separately through a real launch and
route health checks. `just ci` also completed in the background: **exit 0** (only
the usual UI build chunk-size warning), with no collateral breakage.
