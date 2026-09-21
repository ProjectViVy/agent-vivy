# Acceptance — 物种工作台

Human view, after `.\launch-vivy-studio.ps1` (Studio at
`http://127.0.0.1:3090`) with the managed or daily backend running:

1. **Tab appears**: the conversation view ring shows 「物种工作台」 beside
   「Vivy 控制台」.
2. **当前形态**: with a backend up (`just run` or console 一键启动), the
   first pane shows real Generation / Binary / artifact sha256 / 策略 /
   Grants / Tools rows from `species/inspect`. Stop the backend and press
   「⟳ 探测」: the card says the backend is unreachable — the rest of the
   workbench keeps working.
3. **Candidates**: with an empty ledger the pane says so and points at the
   Recipe editor. After one pack, the candidate chip reads
   `gen_… · 缺评测 eval`; each completed step advances the chip
   (`缺发布 → 缺安装 → 已安装`).
4. **NG-25 gate**: press 发布/安装/回滚 without ticking 「我已确认」 —
   the job never starts and the error names 确认（NG-25）. With the tick,
   the job streams `vivy-studio.exe` output and, for release, the argv
   contains `--actor human --yes`. A switch is honest only about the next
   launch: the running Studio process keeps its old form until restarted.
5. **Recipe editor**: 「repo minimal.vivy.yml（复制为草稿）」 loads the
   repo text; 保存 writes only under
   `data/studio-home/species-workbench/recipes/` (verify nothing appeared
   under repo `recipes/`). 结构校验 catches a misspelled top-level key or
   duplicate module; 打包新物种 runs the real `go build` pack (minutes)
   and the job output ends with the Artifact JSON; the new `gen_…` then
   shows in 候选与生命周期.
6. **Air gap**: `data/vivy.db`, `data/demo/`, `data/workspaces/` untouched
   by every action above (file mtimes unchanged).
