# acceptance

1. Open Vivy Studio → 「Vivy 控制台」.
2. Stop any running managed backend, then ▶ 启动后端（或一键启动）.
3. Status shows running with a PID; message mentions **真实 provider · 数据隔离**
   (not「mock 模式」).
4. Backend log no longer contains
   `field mock not found in type config.Runtime`.
5. `data/studio-home/vivy-console/config.yaml` has no `runtime.mock` line
   after start (console rewrites it on each start).
6. UI at `http://127.0.0.1:3015` can reach `/rpc` once the frontend is up;
   chat uses the real provider / settings path (wizard or settings), not
   `mock reply`.

If step 3–4 still fail with the old mock error, Studio is still running the
pre-sync host plugin — fully restart Studio after the profile sync.
