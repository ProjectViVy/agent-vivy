# Acceptance

How a human can tell the Vivy backend starts again.

1. **总控台 starts the backend.** Open `http://127.0.0.1:3090` (hard-refresh
   after the Studio restart) → 「Vivy 控制台」 → **总控台** → **▶ 一键启动**.
   The backend card must show 运行中 with a PID and 监听 `127.0.0.1:8787`, and no
   「进程在监听 8787 前已退出」 message.

2. **The control plane answers.** With the backend running:

   ```text
   curl.exe -i http://127.0.0.1:8787/rpc/bootstrap
   ```

   Any HTTP status from the server (not a connection failure) means the listener
   opened. Then open the dev UI at `http://127.0.0.1:3015` — the Vite `/rpc`
   proxy targets the managed backend, so a working app proves the whole split
   pair came up from the console.

3. **The gateway log has no abort.** `data/studio-home/vivy-console/gateway.out.log`
   ends with `msg":"vivy starting","addr":"127.0.0.1:8787"` and contains no
   `startup aborted` line for the new run.

4. **The config the console writes is `active`-only.** After a start:

   ```text
   type data\studio-home\vivy-console\config.yaml
   ```

   must show `providers:` followed by `  active: openai` and nothing else — no
   `bundle_dir`, no per-vendor `env_key`/`default_model` block.

5. **The producer cannot drift again.** From the repository root:

   ```text
   node --test studio/dsh-vivy-console/config.test.mjs
   ```

   must pass. That test fails if a future edit reintroduces a key the kernel's
   config struct does not declare.

## What would still be a bug

- `ok:false` with `startup aborted` and `field ... not found in type
  config.Providers` — the stale producer came back (check that the profile copy
  under `data/studio-home/profiles/<profile>/node_modules/dsh-vivy-console/` was
  re-synced and that Studio was restarted).
- Backend starts but `:8787` is unreachable — a different defect; this change
  only removes the config-shape abort.