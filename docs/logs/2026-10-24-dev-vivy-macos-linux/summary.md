# dev-vivy on macOS/Linux

## What changed

- `justfile`: `just dev` and `just dev-vivy` now route to `scripts/dev.sh`
  on non-Windows hosts and forward recipe arguments. Windows still uses
  `dev.ps1` (and `dev.ps1 -Split`).
- `scripts/dev.sh` (previously an untracked work-in-progress file):
  - stops a previous split backend identified as `vivy`, `vivy.exe`,
    `vivy-backend`, or `vivy-backend.exe`, matching the Windows launcher;
  - derives `GOTOOLCHAIN` from the `go.mod` Go version (overridable), so a
    newer local Go cannot break pinned dependencies;
  - uses `https://proxy.golang.org,direct` and routes Go module/toolchain
    downloads through `HTTPS_PROXY` (or the local `127.0.0.1:7897` mixed port
    when it is listening), without changing `go env`;
  - defaults `VIVY_USER_HOME` to `.workspace/dev/home`, keeping the loop off
    the daily `~/.vivy` journal;
  - runs log followers in their own process groups so shutdown does not hang
    on `tail`, and exits 0 on an intentional Ctrl+C/TERM.
- `scripts/ensure-laputa.mjs`: repository identity comparison accepts
  mirror/proxy-wrapped GitHub remotes (for example gh-proxy) as the same
  checkout. Fetched objects are still verified against the pinned commit.
- `scripts/ensure-laputa.test.mjs`: covers a proxy-wrapped origin.
- `README.md`: macOS/Linux dev entry, pinned toolchain, and isolated dev home.

## Explicitly not done

- `dev.ps1` behavior is unchanged; Windows still reads the daily home unless
  `VIVY_USER_HOME` is exported.
- Full `just ci` and Windows-native gates were not run for this change.

## Environment-only changes (not committed)

- Installed `just` 1.59.0 with Homebrew.
- Replaced the global git `insteadOf` rewrite from gh-proxy with
  `http.proxy=http://127.0.0.1:7897`.
- Advanced the sibling `../laputa` checkout to the pinned commit
  `9acc999301d5d575da2cea902a1c6d213763f381`.
- Set global `GOPROXY=https://proxy.golang.org,direct` and exported
  `HTTP_PROXY` / `HTTPS_PROXY` / `ALL_PROXY=http://127.0.0.1:7897` plus a
  loopback/LAN `NO_PROXY` in `~/.zshrc` (Go does not read the macOS system
  proxy).
