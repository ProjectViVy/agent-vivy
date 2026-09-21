# Summary — Vivy Studio 物种工作台 (dsh-species-workbench)

New first-party Studio bundle `studio/dsh-species-workbench` (submodule
commit `3145241` on `feat/hub-delete-source-autocommit`): a conversation
view tab 「物种工作台」 (id `species-workbench`, order 40, beside the
console's order 30) that makes the species lifecycle operable from inside
Studio without ever violating the product contract.

## What changed

- **当前形态 (read-only)**: `species.js` performs the real control-plane
  handshake — GET `/rpc/bootstrap` → `ws://<addr>/rpc?token=…` →
  JSON-RPC `species/inspect` — and renders the Report (generation, binary,
  artifact/UI digests, policy, recipe, grants, tools). NG-23 honored: this
  surface never calls `promotions/promote` or `evals/start`; the species
  side stays read-only. Unreachable backend degrades to a card message,
  not a failure.
- **候选与生命周期**: the host reads the Studio ledger through
  `vivy-studio.exe list <kind>` / `workspace list` and the UI derives each
  candidate's next step (`缺评测 → 缺发布 → 缺安装 → 已安装`). All writes
  (pack/eval/release/reject/install/rollback) spawn `vivy-studio.exe` on
  the pinned worktree through one-concurrency jobs with a streamed 500-line
  output tail. NG-25 is enforced in `jobs.js` **before any spawn**:
  release/install/rollback require `confirm===true`, and only then does
  release forward `--actor human --yes`. Switching lands on the next
  launch of the daily location — there is no runtime hot-swap and none is
  pretended.
- **Recipe 编辑器**: draft recipes live only under Studio scratch
  (`data/studio-home/species-workbench/recipes/`); repo recipes are
  read-only via the new `GET /repo-recipes/<name>` route and enter the
  editor through 「复制为草稿」. `recipe.js` offers structural validation
  (key whitelist, apiVersion, duplicate modules; both block and inline
  list shapes) and is explicitly **not** a second schema authority — the
  sdk compiler is. `{strict:true}` validate runs a real pack into a
  throwaway dir. Pack speaks the v1 contract fixed in
  `2026-09-21-pack-recipe-argv`: `--recipe` required, optional
  `--source`/`--out`; the v0 `--with` list is never emitted (regression
  test pins it).
- **Path allowlist**: the browser sends recipe **names**; host
  `resolveRecipeArg` expands a bare name to the draft dir, `recipes/<name>`
  to the repo, and accepts draftsDir-absolute paths only as issued by
  strict validate. Traversal, wrong-extension and outside-allowlist inputs
  resolve to refusal.

## Explicitly not done

- No promotion RPC, no in-process swap, no second Journal (ST-2 air gap
  respected: only `data/studio-home/species-workbench/` is written).
- Draft promotion into repo `recipes/` stays a human copy; the editor does
  not write there.
- Strict-validate throwaway dirs under `validate/` are not auto-cleaned
  yet (owned by pack `--out` semantics; noted on the backlog as harmless
  scratch).
- No change to the plugin-hub install flow; first-party composition stays
  in `launch-vivy-studio.ps1`.
