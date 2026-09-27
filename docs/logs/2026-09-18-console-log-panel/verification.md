# Verification — 2026-09-18 console log panel

Product files changed: `studio/dsh-vivy-console/{logs.js, logs.test.mjs,
index.js, client.js, package.json, README.md}` (submodule
`ProjectViVy/vivy-studio`, commit `4483d84`). No Go, UI or product-contract
file was touched.

## Static checks

| Command | Result |
| --- | --- |
| `node --check studio/dsh-vivy-console/index.js` | exit 0 |
| `node --check studio/dsh-vivy-console/client.js` | exit 0 |
| `node --check studio/dsh-vivy-console/logs.js` | exit 0 |
| `git -C studio diff --check` | clean (no whitespace errors) |

## Unit tests

```text
node --test studio/dsh-vivy-console/logs.test.mjs
ℹ tests 14   ℹ pass 14   ℹ fail 0
```

Cases: level normalisation and inference (wording, ANSI red/amber, stream
default); ANSI OSC/CSI stripping with CJK preserved; slog nanosecond timestamps
and epoch numbers; caller/file shortening on real kernel values; byte-accurate
line splitting for multi-byte (CJK) and CRLF lines and no phantom trailing
line; structured record parsing (fields, caller, file, line, timestamp, the
duplicate `level` key in the milestone line); continuation chaining and a
non-continuation line ending the chain; a partial line and its completed version
sharing one id; timeline ordering and read-order fallback; cursor parse/format
round-trip with malformed cursors refused.

## In-isolation API smoke (`data/studio-home/vivy-console/smoke-logs.mjs`)

Imports the real host half with a mock `webServer` ctx and drives the real
`GET /vivy-console/api/logs` route against synthetic log files in a throwaway
workspace (log paths derive from `process.cwd()`), so no live dev-loop log and
no process was touched. Run with `cwd` = that workspace:

```text
SMOKE LOGS ALL PASS      (36 checks, exit 0)
```

Covered: tail mode with `omitted`, a four-offset cursor; byte-offset ids and
structured fields (caller `channelhost.Host.StartAll`, file
`internal/channelhost/host.go`, line 88, `channel=telegram`, timestamp);
ERROR/WARN/DEBUG levels; the unterminated line marked `partial` with the cursor
held at its start; the older `err` timestamp sorting before `out`; ANSI escapes
stripped while the glyphs stay; per-stream tail clipping (300 of 350 lines);
legacy `src`/`text` keys still present and unique ids; an idle delta returning
exactly the still-incomplete line with the same id; a delta returning the
completed line (same id, whole message) plus the next record; a second idle
delta returning nothing; a malformed cursor falling back to a tail; rotation and
a longer rewrite both reported as `reset` with no fragment records.

## Installed-copy sync

Seven files were copied to both profiles
(`data/studio-home/profiles/vivy-studio/node_modules/dsh-vivy-console/` and
`…/vivy-studio-next/node_modules/dsh-vivy-console/`), then SHA256-compared
against the source: **OK for all 7 files in both profiles** (including the new
`logs.js`, which the host half imports). Both installed copies were also
imported directly —

```text
node -e "import('file:///…/profiles/<profile>/node_modules/dsh-vivy-console/index.js')…"
vivy-studio      apply=function inject=["webServer"]
vivy-studio-next apply=function inject=["webServer"]
```

— which proves the installed tree resolves `./logs.js` (a missing file there
would fail the import).

## Skipped checks and why

- **`just ci`** — the default gate covers the Go kernel, the UI and the host
  tree; it does not run anything under `studio/` (the submodule has no JS
  toolchain). This iteration changed only submodule JS plus this record, so `ci`
  would have exercised nothing new. The same reasoning was recorded by the
  2026-09-17 console lane. The submodule's own checks are the `node --test` run
  and the API smoke above; `git diff --check` covers the whitespace class of
  problem `fmt-check` would catch.
- **Browser rendering** — no browser automation was run from this lane. The
  rendered appearance is verified by the human acceptance walkthrough
  (`acceptance.md`); `verification.md` only claims what the commands above show.

## Live path (Studio restart + bring-up)

The plugin has no hot reload: the host half and the client bundle are both read
when the Studio server boots/requests. Applying the change therefore needs a
Studio restart, run **detached** (an in-tree kill would end the agent's own
turn) via `data/studio-home/post-restart-bringup.ps1`, which restarts the
server, waits for `/vivy-console/api/status`, then starts the backend and the
frontend through the new server and logs to
`data/studio-home/vivy-console/post-restart-bringup.log`.

Live assertions on `GET /vivy-console/api/logs` (tail → cursor → delta), on the
real `gateway.*`/`frontend.*` logs written by the restarted pair, and the human
acceptance walkthrough are recorded in the same iteration after the restart;
`docs/logs/2026-09-18-console-log-panel/acceptance.md` describes what to look
for.