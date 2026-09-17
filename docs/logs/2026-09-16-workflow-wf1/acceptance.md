# Acceptance — Workflow WF-1 slice

How a human can tell the slice works. Start the split pair
(`just run` + `cd ui; pnpm dev`) against a scratch data directory and open
`http://127.0.0.1:3015`.

1. **Capability advertised.** `capabilities` (initialize response) contains
   `workflow.definitions` and `workflow.run` for the default Generation.
2. **Definitions round-trip canonically.** Call `workflow/define` with a
   definition whose JSON keys are deliberately scrambled; the response
   returns `{id, rev: 1, hash}`. `workflow/get` returns canonical bytes
   (sorted keys, no whitespace) with the same hash; re-defining the same
   content under a different key order yields the identical hash and a new
   revision.
3. **Structured diagnostics.** `workflow/validate` with a cyclic graph, a
   duplicate node id, an uncovered `nodes.*.output` reference, an unknown
   model profile, or an over-budget node count returns
   `valid: false` with `{check, path, message}` diagnostics naming the exact
   problem; `workflow/define` with the same payload fails `-32602` and
   stores nothing.
4. **Unavailable execution is explicit.** `workflow/run` on a valid
   definition returns code `-32011` with message
   `workflow execution is not configured in this generation`; `workflow/runs`
   afterwards shows **no** run row for the workflow (no orphan), and the
   session list shows no new workflow session.
5. **History is append-only.** Redefining the same workflow id appends
   `rev 2`; `workflow/get` with `rev: 1` still returns the original bytes
   and hash.
6. **Agent tools present and governed.** The default Generation's tool
   catalog exposes `workflow_list/get/validate/define/run/runs`. Under the
   `plan` or `read_only` policy profile the two effectful tools are denied
   while `workflow_list` stays usable; under `default`, asking the agent to
   save a workflow produces an approval proposal carrying a bounded summary
   (id, next revision, hash, node count) — not the full definition and not a
   silent save. Approving persists exactly one new revision.
7. **Minimal Generation has nothing.** Building the minimal Recipe
   (`recipes/minimal.vivy.yml`) yields an Assembly whose manifest contains
   neither the `vivy/workflow` module, nor the six tool identities, nor any
   workflow source import; with a minimal Generation the `workflow/*` RPC
   family answers `-32601 MethodNotFound` and `capabilities` does not
   advertise the tokens, while chat works normally.
8. **Config cannot resurrect a removed module.** Adding a `workflow:` config
   section to a minimal-Generation install aborts startup with
   `configured workflow section requires compiled WorkflowHost`.
