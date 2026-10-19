# T0 verification

Host: macOS arm64. Pinned Go `go1.26.4` (`GOTOOLCHAIN=go1.26.4`), Node 26.11.0, pnpm 12.10.1, Playwright 1.62.1 with Chrome channel.

- `node --check ui/e2e/e2e-ports.mjs scripts/toolkits-e2e.mjs` — pass.
- Resolver checks: defaults 18787/3015; `18887/3115` accepted; `0`, `abc`, `1.5` and equal endpoints rejected before launch.
- `VIVY_TOOLKITS_E2E_BACKEND_PORT=abc node scripts/toolkits-e2e.mjs` — exit 1, names the invalid variable.
- `VIVY_TOOLKITS_E2E_BACKEND_PORT=19000 VIVY_TOOLKITS_E2E_UI_PORT=19000 node scripts/toolkits-e2e.mjs` — exit 1, distinct-port message.
- Occupied UI port (temporary listener on 19001) — exit 1, `EADDRINUSE`, no Playwright launch.
- `VIVY_TOOLKITS_E2E_EXTERNAL_BACKEND=1 node scripts/toolkits-e2e.mjs` — exit 1, manual-mode refusal.
- `just toolkits-e2e` with isolated ports 18889/3117 — **all suites passed**: lifecycle 1 passed; accessibility 2 passed. Artifact: generation `ec984775dbd5c7be9dbc4be1f478b56a596c6fa99a8f449c5a2fa31fdefddee0`, recipe `32366ca2c0f13ae9a5bc86da2e08734bfdfdf38767987934b7eb9bd35b9e7a76`, UI dist `066a0cb6e013410e51004c091dc32d4c3d269c83d6d67792b56196a01b673493`, binary SHA-256 `928e55bd7e6fafac7f4e73acba248b852efad2a30b86381807b829f452a1a864`. The two suites used distinct temporary data roots.
- External binary check with a space in the path (`.../.workspace/path with spaces/vivy`) and its inspected artifact dir, ports 18888/3116 — lifecycle 1 passed, accessibility 2 passed; generation `18d3c6fd…`, binary SHA-256 `be56ffbe…`.
- `--list` through the Toolkit config with the same spaced binary path — 1 test discovered, config ports 18887/3115 honored.
- Default-port attempt at 18787/3015 failed after the backend started because **3015 was already occupied by a concurrent local server**; the backend was stopped and 18787 was free afterward. The supported isolated-port run above is the recorded gate evidence; this is not a product failure.
- Ports 18888/3116, 18889/3117 and 18787 were free after the runs (no leaked gate processes).

## Full CI

`just ci` (pinned `go1.26.4`, workspace `GOCACHE`, `NODE_OPTIONS=--localstorage-file=...`, and `GIT_CONFIG_GLOBAL=/dev/null` to bypass the host's gh-proxy `insteadOf` rewrite) passed bootstrap/fmt/ui-ci/vet and then failed only in `sdk/codeclient`:

```text
--- FAIL: TestClientAgainstRealVivyCode
    client_test.go:175: codeclient: handshake get_state: codeclient: protocol stream ended
```

The same focused test fails identically in the unmodified root checkout at this base commit, so it is a pre-existing local `vivy-code` handshake/toolchain issue, not a change in this lane. The remaining CI recipes were run separately and passed: `diva-recall-test` (six real DIVA memory-loop cases), `headless-compile`, and `plugin-ci` (22 modules). No full-CI green claim is made.

Not verified here: Linux and native Windows acceptance venues. The default-port attempt at 3015 was blocked by a concurrent server; the isolated-port gate above is the recorded evidence. `docs/TODO.md` keeps `TOOLKITS-E2E-VENUES` open.
