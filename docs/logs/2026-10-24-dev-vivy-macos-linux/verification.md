# Verification

Host: macOS 27.0.1 arm64, Go 1.27.1 local, `just` 1.59.0.
Repository: `stable` at `2f587e3e` plus this change.

## Checks run

```text
bash -n scripts/dev.sh                         # syntax ok
just --dry-run dev-vivy                        # bash ./scripts/dev.sh
just --dry-run dev                             # bash ./scripts/dev.sh
node --test scripts/ensure-laputa.test.mjs     # 10 passed, 0 failed
git diff --check                               # clean
```

## Real split loop

```text
just dev-vivy --no-browser
```

Result:

- previous `go run` backend (PID 39415) and Vite (PID 39360) were stopped;
- backend built with `GOTOOLCHAIN=go1.26.4` derived from `go.mod`;
- `split loop ready: http://127.0.0.1:3015` reached;
- `GET http://127.0.0.1:8787/healthz` -> 200;
- `GET http://127.0.0.1:3015/` -> 200 (Vite HTML);
- `POST http://127.0.0.1:3015/rpc` -> 401 from the backend, proving the
  Vite `/rpc` proxy reaches the control plane;
- `TERM` to the supervisor -> `dev.sh: stopping split loop`, exit 0,
  ports 8787/3015 free, no surviving `tail` or `vite` processes;
- dev database created at `.workspace/dev/home/vivy.db`; the daily
  `~/.vivy/vivy.db` was not touched (mtime unchanged).

## Failures reproduced and fixed

1. `ensure-laputa` rejected the proxy-cloned sibling:
   `Laputa origin is https://gh-proxy.com/...; expected https://github.com/...`.
   Fixed by owner/repo identity normalization.
2. Go 1.27.1 build failure:
   `grpc@v1.64.1 ... undefined: http2.TrailerPrefix` caused by x/net's
   Go 1.27 HTTP/2 switch. Fixed in the dev loop by pinning the `go.mod`
   toolchain (`go1.26.4`).
3. Backend startup failure:
   `storage migrations: unsupported_source_format: migration 36 name drift`
   because the daily `~/.vivy` journal came from a different schema
   revision. Fixed by defaulting the dev home to `.workspace/dev/home`.
4. Cleanup hang: the `tail | awk` log followers left `tail` alive and
   `wait` blocked. Fixed by running each follower in its own process group.

## Go module proxy

```text
curl -x http://127.0.0.1:7897 https://proxy.golang.org/...  # 200
curl -x http://127.0.0.1:7897 https://sum.golang.org/...    # 200
HTTPS_PROXY=http://127.0.0.1:7897 GOPROXY=https://proxy.golang.org,direct \
  GOTOOLCHAIN=go1.26.4 go list -m github.com/google/uuid@v1.6.0
# github.com/google/uuid v1.6.0
```

`dev.sh` now selects the same route for `go build`: an existing
`HTTPS_PROXY`/`VIVY_HTTPS_PROXY` wins, otherwise `127.0.0.1:7897` is used
when that port is listening. `go env` is left untouched by the script.

## Global Go configuration

```text
go env -w GOPROXY=https://proxy.golang.org,direct
# go env -w HTTPS_PROXY is unsupported; ~/.zshrc now exports:
#   HTTP_PROXY / HTTPS_PROXY / ALL_PROXY=http://127.0.0.1:7897
#   NO_PROXY=localhost,127.0.0.1,::1,192.168.0.0/16,10.0.0.0/8,172.16.0.0/12,.local

zsh -ic 'echo $HTTPS_PROXY'          # http://127.0.0.1:7897
zsh -ic 'curl https://proxy.golang.org/...'  # 200
GOMODCACHE=/tmp/vivy-gomod-test GOTOOLCHAIN=go1.26.4 \
  go list -m github.com/google/uuid@v1.6.0    # downloaded via the proxy
```

## Not run

- `just ci` (full product gate; not required for this scoped dev-entry change).
- Windows `dev.ps1` / `dev-vivy` native acceptance.
