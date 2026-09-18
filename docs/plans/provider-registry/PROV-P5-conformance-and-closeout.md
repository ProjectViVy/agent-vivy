# PROV-P5 — Conformance, Evidence, and Closeout

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task.

**Goal:** Close the redesign with durable evidence: the conformance digest, the
full product gate, the iteration log, and the backlog rows.

**Architecture:** No new behaviour. This phase proves what `PROV-P1`..`PROV-P4`
built and records what was deliberately left undone.

**Tech Stack:** `just ci`, `sdk` conformance suite, browser smoke at `:3015`.

**Spec:** `docs/plans/provider-registry/MIGRATION.md` §5, §6.

**Depends on:** `PROV-P1`..`PROV-P4`.

---

### Task 1: Refresh the source digest

**Files:**

- Modify: `sdk/internal/assembly/conformance_results.json`

- [ ] Run the reproduction suite and read the wanted value from the failure
  message:

```powershell
go test ./sdk/internal/conformance -run Reproduction -count=1
```

- [ ] Update `sourceSha256` for the `internal` tree to that value.
- [ ] Re-run until green.
- [ ] Confirm the change is the **only** content edit in that artifact.

Context: `docs/TODO.md` `PROVIDER-PROFILE-DIGEST-PIN` records that this value is
maintained by hand and that the reproduction test now derives it from the live
tree. Do not "fix" the mechanism in this phase.

- [ ] Confirm the digest is computed from a **clean** `internal` tree: untracked
  files under `internal/` are inside the hash (`internal/sourcehash/tree.go:27-52`),
  so run this on the lane worktree, not on the root tree that carries WF-1 WIP.

### Task 2: Failure-path conformance

**Files:**

- Modify: `internal/modelhost/conformance_test.go`
- Modify: `internal/provider/provider_test.go`
- Modify: `internal/provider/secret_audit_test.go`
- Modify: `internal/app/model_test.go`

- [ ] Cover, at minimum: a data file naming an unsealed adapter (startup fails);
  a sealed supported adapter with no endpoint (startup fails); a deferred adapter
  selected directly (refused, no placeholder); a missing key
  (`KeyMissingError` naming the vendor's env var); a model whose metadata is
  unknown (conservative defaults, no panic); a `deepseek-thinking` endpoint with
  thinking off vs on (request body asserted); an OpenAI reasoning model receiving
  `reasoning_effort`; secret redaction across the new RPC payload; and a legacy
  `settings.yaml` read.
- [ ] No live network in any unit test.
- [ ] Assert the outbound `model` field is the raw model id with no inserted
  `provider/` prefix (root `AGENTS.md` rule).

### Task 3: Full product gate

- [ ] `just ci` from the repository root. Record the outcome verbatim.
- [ ] If any lane fails for a reason unrelated to this work (for example a
  pre-existing lane exclusion in the `justfile`), record the exact failure and the
  reason rather than working around it.
- [ ] Browser smoke at `http://127.0.0.1:3015`, not the embedded UI on `:8787`:
  select a DeepSeek model, switch the vendor to a multi-protocol vendor, select
  the second protocol variant, and confirm `settings.yaml` reflects the change.

### Task 4: Durable records

**Files:**

- Create: `docs/logs/<YYYY-MM-DD>-provider-registry/summary.md`
- Create: `docs/logs/<YYYY-MM-DD>-provider-registry/verification.md`
- Create: `docs/logs/<YYYY-MM-DD>-provider-registry/acceptance.md`
- Modify: `docs/TODO.md` §0.1
- Modify: `docs/plans/provider-registry/README.md` (status board)

- [ ] `summary.md`: what changed, scope, what was explicitly not done.
- [ ] `verification.md`: every command with its outcome, including skipped checks
  and why.
- [ ] `acceptance.md`: how a human confirms it worked (product view).
- [ ] `docs/TODO.md` §0.1: add `PROVIDER-AGENTIC-MIGRATION` (the deferral's lift
  conditions, per `EINO-CAPABILITY.md` §5) and `PROVIDER-DATA-CONFIG-EDIT` (the
  `env_key` trust boundary, per `DESIGN.md` §3, if not already open).
- [ ] Confirm `DEEPSEEK-REASONING-CONTENT` and `PROVIDER-PROFILE-DIGEST-PIN`
  remain accurate; update their text only if the facts changed.
- [ ] Flip the status board rows in `docs/plans/provider-registry/README.md`.

## Phase exit

Exit requires: the conformance digest matching a clean tree; `just ci` green or a
recorded unrelated failure; the browser path exercised; and a complete iteration
log.

## Commit

```
test(provider): prove the single-source provider registry
```

## Push policy

Do **not** push. No push authorization was given for this work; the lane lands
through its branch when the owner says so.
