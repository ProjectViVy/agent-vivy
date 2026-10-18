# Checkpoint

## Current state

The development-venue split is implemented in the working tree and has not
been committed. All local gates have been executed on the Linux filesystem,
the Docker packaging gate now passes, and Windows owner-test triage is the
only open dev-environment item. The Linux race lane remains red on the pinned
upstream Eino race and is not counted as passed.

Acceptance identity (copy under `~/vivy-acceptance`, no `.git` in the main
tree):

- candidate HEAD `5715950ddfc87e05cf60e3d2a0ca54983bef05eb`, 42 dirty entries
- tracked diff `4efb986d4f19f8447c86a1a25ce4dc6d52f8137f06f52b0e26ead43aff93ceb9`
- copied tree `96f30a306c3261000fc05b106dbdbfa4253358c33ecbf4ea8b0447d6cdb68b73`
- Laputa `4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24`, clean
- Docker `26.1.5+dfsg1`
- image `sha256:ff2a1f2496be2c7138c3791ac37b0ce7d1c8cde31bc2826c849dc92a9ce17780`,
  180,313,796 bytes
- smoke: healthy on a tmpfs `/data`; `smoke passed; container healthy on an
  empty /data`

## Gates

| Gate | Result |
|---|---|
| Windows `just check-fast`, ensure-laputa tests, plugin-ci, diva-recall, headless-compile | passed |
| Windows handshake owner test | skipped under elevation with reason; non-elevated path covered by CI |
| Linux `just ci` | passed |
| Linux real PostgreSQL 16.15 | passed (`internal/storage/postgres` 132.466s) |
| Linux `just race-ci` | executed, not passed: first-party `sdk/tui/face/rpc.go` race fixed; upstream pinned Eino `tool_node.go:1253` race remains |
| Docker empty-directory build + smoke | passed offline (`--network=none`) and through `docker-build.mjs --smoke` |
| Windows full `just ci` | not rerun locally; Windows CI owns it |

## Next commands

```bash
# Linux main development line
export PATH="$HOME/.nvm/versions/node/v24.21.0/bin:$HOME/.local/go/bin:$HOME/.local/bin:$PATH"
cd ~/work/agent-vivy
just doctor
just ci
VIVY_POSTGRES_TEST_DSN=postgres://vivy:vivy@127.0.0.1:55432/vivy?sslmode=disable \
  just test-postgres
just race-ci            # stays red on EINO-TOOLSNODE-ERR-RACE

# Docker acceptance from an empty parent with a cold cache
node scripts/docker-build.mjs --tag vivy:acceptance --smoke
```

Open items:

1. `EINO-TOOLSNODE-ERR-RACE` — decide on an Eino pin upgrade (the fix is in
   v0.9.21) in a separate authorized change. Do not skip the affected tests.
2. `DEV-ENV-WIN-TRIAGE` — keep generation-contract, artifact, and owner-test
   failures separate. The elevated sharing-lock observation is the only
   diagnosed owner-test case; non-elevated CI keeps the original coverage.

## Failure log (do not repeat blindly)

1. Dev smoke with no `VIVY_CONFIG` loaded the repo `config.yaml`:

   ```text
   "dir":"data\\logs"
   composition failed: storage module: open storage: storage migrations:
   migration 26 name drift: database="session_work_events" embedded="channel_deliveries"
   ```

   Cause: `cmd/vivy` falls back to `./config.yaml` when `VIVY_CONFIG` is
   unset. Fix: dev entries set `VIVY_CONFIG=scripts/dev.config.yaml` (empty
   overlay) and `VIVY_USER_HOME=.workspace/dev/home`. Do not point a dev run
   back at `data/` to "reproduce" this.

2. Docker build, Node-only UI stage:

   ```text
   stage-ui: could not run the Vivy SDK (spawnSync go ENOENT).
   ```

   Fix: one build stage with Node plus Go; `sdk/ui` is copied before
   `pnpm install` for `file:../sdk/ui`.

3. Docker build, missing UI file dependency: `ENOENT` on `/src/sdk/ui`. Fix:
   `COPY sdk/ui ./sdk/ui`; `.dockerignore` excludes its `node_modules`;
   `scripts/docker-build.mjs` fails closed if a UI `file:` dependency is not
   copied.

4. Docker frontend resolution timed out on `docker/dockerfile:1.7`. Fix:
   remove the `# syntax=` directive; named build contexts use the built-in
   BuildKit frontend.

5. Docker build had no system CA bundle in `node:*-slim`, so Go module
   downloads failed with `x509: certificate signed by unknown authority`.
   Fix: copy the bundle from the Go image. Do not add apt here; the build
   host cannot reach `deb.debian.org`.

6. Runtime stage apt could not find `wget` and could not reach Debian
   mirrors. Fix: copy the CA bundle, keep the base zoneinfo, and probe
   `/healthz` over bash `/dev/tcp`. Do not reintroduce apt or wget.

7. Corepack prepare succeeded but the next layer's `pnpm` shim tried the
   registry (`getaddrinfo EAI_AGAIN registry.npmmirror.com`) because the
   corepack cache mount existed only on the prepare layer. Fix: mount
   `/root/.cache/node/corepack` on every layer that invokes `pnpm`.

8. WSL DNS returns Clash fake-IP addresses in `198.18.0.0/16`; the Windows
   proxy listens only on `127.0.0.1:7890`; the WSL gateway cannot reach it,
   and containers inherit the unusable resolver. Do not keep switching Go
   proxies. Seed the BuildKit cache mounts from host caches and build with
   `--network=none`; record that as input completeness, not registry
   reachability.

9. WSL `bash -lc` can resolve Debian's Node 20 before nvm. Fix: prepend
   `$HOME/.nvm/versions/node/v24.21.0/bin`, `$HOME/.local/go/bin`, and
   `$HOME/.local/bin` in every Linux gate shell.

10. `memory-loop.mjs` app pass must run with cwd `internal/app`; from the
    repository root Go treated bare source filenames as import paths.

11. `TestMemoryLoopEffectHandshakeWaitsForWindowsSharingLock` read through a
    share-mode-0 lock in an elevated shell because Go's read open uses
    `FILE_FLAG_BACKUP_SEMANTICS`. Fix: skip on that observation, keep the
    non-elevated assertion, and classify it as an owner-test/environment
    interaction in `DEV-ENV-WIN-TRIAGE`.

12. A first-party race in `sdk/tui/face/rpc.go` (`admitSettledFollowUp` read
    `m.sessionID` outside the mutex) was fixed with `snapshot()`. The
    remaining race is upstream Eino v0.9.13 `compose/tool_node.go:1253`;
    v0.9.21 carries the fix. The pin was not changed.

## Transfer

The working changes are captured outside the tree as:

- `.workspace/dev-environment.patch` — tracked-file diff (including deletions)
- `.workspace/dev-environment-untracked.tar.gz` — new scripts, config, and
  documentation (excluding `.omc/`, `.omo/`, and `.workspace/`)

Apply with `git apply .workspace/dev-environment.patch` followed by extracting
the tarball at the repository root, then run `just doctor`. These artifacts are
scratch and are not part of the delivery commit.
