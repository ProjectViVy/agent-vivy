# One-shot sub-agent transcript leak — verification

## Go gates (macOS host, pinned toolchain)

```text
GOTOOLCHAIN=go1.26.4 GOPROXY=https://goproxy.cn,direct go vet ./internal/runtime ./internal/storage/...
# clean, no findings

GOTOOLCHAIN=go1.26.4 GOPROXY=https://goproxy.cn,direct go test -p 1 -run 'TestMessageProjector' ./internal/runtime
# ok  internal/runtime  1.365s

GOTOOLCHAIN=go1.26.4 GOPROXY=https://goproxy.cn,direct go test -p 1 ./internal/storage/migrations
# ok  internal/storage/migrations  3.169s

GOTOOLCHAIN=go1.26.4 GOPROXY=https://goproxy.cn,direct go test -p 1 ./internal/runtime ./internal/storage/sqlite
# ok  internal/runtime          67.426s
# ok  internal/storage/sqlite    9.922s
```

`-p 1` is required on this host (memory pressure, see `project-vivy-build-env`).

## Dev-data repair (read-only queries after restart)

The dev pair was stopped, `.workspace/dev/home/vivy.db` backed up to
`vivy.db.bak-20261024-pre048`, and both processes restarted with the new binary so migration 048
applied at startup.

Before restart:

```text
schema_migrations head                                       47
messages rows for child runs (all sessions)                 325
```

After restart:

```text
sqlite3 "file:.workspace/dev/home/vivy.db?mode=ro" \
  "select version,name from schema_migrations order by version desc limit 3;"
48|oneshot_child_projection_cleanup
47|channel_session_identity
46|child_exact_inputs

sqlite3 ... "select count(*) from messages m join runs r on r.id=m.run_id
             where r.kind='child' and r.child_mode<>'continuable';"
0
```

The reported session `sess_fed3d415787a9794`:

```text
messages rows                                               20   (positions 1-20)
rows whose run_id starts with child_                        0
last row     position 20, run_d5abbb64ecc2bba4, assistant,
             "子 agent 三项测试全部完成，结论如下。"
child runs still present (child/list)                       3   (all completed, one-shot)
```

The three children's Journal summaries are intact — `child.completed` payload sizes 2387, 4335 and
4513 bytes for `child_1837a503d3acf231`, `child_e42d07ecc2538f69` and `child_8cfbec6d18f7a034`.

## Browser smoke (`http://127.0.0.1:3015`, split pair)

- Reopened the session: the transcript ends at the parent's 19:05 summary. No binary-tree message
  follows it; the third child's answer appears only inside the collapsed `agent` tool-call row, which
  is the parent's own tool result.
- Settings → Vivy 功能 → Run Inspector: the run picker shows `run_d5abbb64ecc2bba4 · completed`, the
  **子 Run 3** tab lists `child_1837a503d3acf231`, `child_e42d07ecc2538f69`,
  `child_8cfbec6d18f7a034` (all `completed`), and selecting one renders its full result text.
- The browser surface reported `visibilityState: hidden` for pointer clicks, so the tab switch and
  child selection were driven with dispatched pointer/mouse events in the page; the rendered DOM was
  read back from `#radix-...-content-children`.

## Not run on this host

- Linux `just wiring-ci` + `just kernel-ci`: not available on macOS. They remain the owner's
  promotion gate for this change.
- PostgreSQL-backed migration execution: the paired SQL is identical to SQLite and is covered by the
  migration package's parity/manifest checks only; no live DSN run here.
- Full `just ci` (Windows) and the UI Vitest lane were not run: no UI code changed in this delivery.
