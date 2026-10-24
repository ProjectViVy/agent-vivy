# Sub-agent fault isolation — acceptance

## A failing sub-agent leaves the chat usable

1. Start the split pair (`just run`, then `cd ui && pnpm dev`) and open
   `http://127.0.0.1:3015`.
2. Ask Vivy to delegate work that will fail — e.g. point the provider at a mock that fails only the
   child request, or give the child a task that violates its budget.
3. Expected: the turn continues. The transcript shows an ordinary tool row whose result starts with
   `sub-agent failed: …`, and Vivy can answer again in the same conversation. The provider banner
   “模型服务出错 / 无法连接” must not appear for a child failure.

## The failure is attributable afterwards

- The kernel log contains `WARN child run failed run=child_… err=…` naming the child run and its
  cause. Before this cut, child terminals were silent, so a failed delegation could not be explained
  from logs.
- The Journal's `tool.finished` for the delegated call carries
  `outcome=recoverable`, `reason=child_failed`, `effects=unknown`, and a bounded (≤ 2 KiB),
  redacted diagnostic. The Run Inspector shows the same values.

## Genuine provider outages are still reported as such

- If the parent's own next provider call fails (bad key, transport error, 5xx), the run still ends
  with the provider banner and `cause_category=provider_error`. A tool diagnostic that happens to
  quote provider wording can no longer take that path.

## Model mistakes use the existing recovery

- An empty, oversized, or non-object `agent` task returns `invalid_arguments` and the model can
  retry, instead of terminating the run.

## A child is told only what the engine declares

- Inspecting a child run's admitted prompt payload shows `Persona.Source = runtime/task-child` and
  `Persona.Body` equal to the engine's restricted child instruction. No web persona, no mask frame,
  and no concatenation of two instructions.

## Not covered by this acceptance

- `workflow` task failures still end the parent run (`WORKFLOW-CHILD-FAULT`).
- Hallucinated tool names, hook blocks and the unselected-tool guard remain run-fatal
  (`TOOL-REFUSAL-RESIDUAL`).
