# Verification — console start guard (2026-09-17)

## Static checks

| Check | Command | Result |
|---|---|---|
| Host half syntax | `node --check studio/dsh-vivy-console/index.js` | exit 0 |
| Client half syntax | `node --check studio/dsh-vivy-console/client.js` | exit 0 |
| Smoke script syntax | `node --check data/studio-home/vivy-console/smoke-startguard.mjs` | exit 0 |
| Profile copy synced | `Get-FileHash` source vs installed copy | identical (SHA256 `8626842A8B8D5B60…`; `vivy-studio` and `vivy-studio-next` are hardlinks to one inode, so both are current) |

## In-isolation smoke of the real host half

`node data/studio-home/vivy-console/smoke-startguard.mjs` (cwd = repo root;
mock `ctx`, real `/vivy-console/api` routes, real ports 3015/8787). The script
hides `ui/node_modules/.bin` to reproduce the reported breakage.

Result — **STARTGUARD SMOKE ALL PASS** (27 checks), key lines:

```
PASS backend start with a dead child reports ok:false
BACKEND failure message: "进程在监听 8787 前已退出（无法启动进程（系统错误 -4058））；日志为空"
PASS healthy backend start reports ok:true
PASS backend answers /healthz
PASS backend restart after a hard stop reports ok:true
PASS restart waited out the lease
PASS restart wait matched the 30s lease TTL
BACKEND restart ok in 34559ms: "已启动 (PID 13732)，监听 127.0.0.1:8787（…）（已等待上一次运行的 workspace 租约过期）"
PASS frontend start with a dead child reports ok:false
PASS frontend failure quotes the vite error
FRONTEND failure message:
进程在监听 3015 前已退出（exit 1）；日志尾部：
> vivy-ui@1.0.0 dev C:\…\agent-vivy\ui
> vite --port 3015 --host 0.0.0.0 --strictPort
ELIFECYCLE  Command failed with exit code 1.
'vite' is not recognized as an internal or external command,
operable program or batch file.
PASS healthy frontend start reports ok:true
PASS dev server answers at :3015
PASS both stopped after smoke
```

The first run of this smoke caught a defect introduced during the refactor
(`startFrontend` still passed the `[path, offset]` array where
`startFailureDetail` now expects a tail string, so the message stringified the
array). The call site was fixed and the full smoke re-run from a clean state.

## Baseline evidence for the "silent success" defect (old code, live console)

Before the fix, on the running Studio (old host-half code), stopping and
immediately starting the backend:

```
POST /restart -> {"ok":true,"pid":19216,"message":"已启动 (PID 19216)，监听 127.0.0.1:8787（…）"}
5s later      -> running=False listening=False pid=
gateway.out.log: {"level":"ERROR","msg":"composition failed",
  "err":"storage module: occupy shared workspace: storage: organism lease held:
         the shared Vivy workspace is already in use by another process"}
```

The same class of failure, reproduced from the one-click restart path.

## Real-path browser/API smoke

The dev pair was exercised over HTTP after each start: `GET /` on
`http://127.0.0.1:3015` returned 200, `GET :3015/src/main.tsx` returned the
Vite-transformed module, and `/rpc/bootstrap` through the Vite proxy returned
`{"protocol_version":"vivy.rpc.v1",…}` from the managed backend.

## Post-restart production proof

`data/studio-home/post-restart-bringup.ps1` restarts the Studio server
(detached, via the bundled `restart-studio.ps1`) and then starts the pair
through the **new** host half, logging every API answer to
`data/studio-home/vivy-console/post-restart-bringup.log`. Recorded outcome:

- see that log — `backend start attempt 1` / `frontend start` results and the
  final status projection.

## Not verified

- No `just ci` run: the change is confined to the Studio overlay submodule
  (JS, outside the Go build), and the host repository's only edits are this log
  and one `docs/TODO.md` row.
- No graceful-shutdown path for the backend: stopping still relies on a hard
  kill, which is why the lease wait exists.
