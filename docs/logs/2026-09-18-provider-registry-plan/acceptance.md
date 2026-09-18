# Acceptance — 2026-09-18 provider registry plan

A human confirms this delivery by reading, not by running. Success means the plan
can be handed to an engineer who has not seen the design conversation and who
needs to make no design decision of their own.

## 1. Five-minute review of the program

Open `docs/plans/provider-registry/README.md`.

- [ ] The goal is stated in one sentence and matches what was agreed.
- [ ] The 19-row decision ledger contains no open question. Every row is a
      decision, not a topic.
- [ ] The status board shows all five phases `UNSCHEDULED`, and the page says only
      a human schedules one.
- [ ] The "Relationship to PLG-P5" section explains that this is a new epic, that
      PLG-P5's output is revised rather than duplicated, and that no second source
      of truth is created.
- [ ] "Expected net effect" lists what is deleted as concretely as what is added.

## 2. Verify the Eino conclusion independently

Open `docs/plans/provider-registry/EINO-CAPABILITY.md`.

- [ ] For each of the three adapters, the document names the pinned component and
      the API used.
- [ ] The `openai-responses` deferral is supported by evidence that can be
      re-checked on this machine: the newest tag of
      `eino-ext/components/model/openai` is `v0.1.13` and has no Responses path;
      the component path `components/model/responses` does not exist; the
      Responses implementation lives in `components/model/agenticopenai v0.2.2`
      on `model.AgenticModel`.
- [ ] The document states that Eino provides no `Message`↔`AgenticMessage`
      bridge, which is why a boundary shim was rejected rather than designed.
- [ ] The committed migration path is recorded as the only lift condition, with
      its blast radius.

Re-check with:

```powershell
go list -m -versions github.com/cloudwego/eino-ext/components/model/openai
go list -m github.com/cloudwego/eino-ext/components/model/responses@latest
```

## 3. Verify the plan is implementation-ready

Open `docs/plans/provider-registry/MIGRATION.md` and one phase file.

- [ ] Every changed file has a precise path, a "today" description, and a
      "change" description.
- [ ] The document warns that its line numbers are perishable and gives the
      `git grep` commands to re-derive them.
- [ ] The deletion list states, for each entry, the evidence that deletion is safe.
- [ ] The `settings.yaml` migration is three concrete rows plus the read/write-back
      rule.
- [ ] The rollback section is per phase and names the one ordering hazard
      (`PROV-P3` changes stored value semantics).
- [ ] A phase file's steps read as work, not as discussion: each names its files,
      its interfaces, its failure-first tests, its exit condition, and its commit
      message.

## 4. Verify the delivery's own footprint

```powershell
git diff --stat
git status --short -- internal sdk ui fixtures schemas Dockerfile config.yaml config.example.yaml
git diff -- sdk/internal/assembly/conformance_results.json
```

- [ ] Only `docs/` paths appear.
- [ ] The second command prints nothing.
- [ ] The third prints nothing.

## 5. Product-level outcome (what this unblocks)

The delivery itself changes nothing a user can see. What a human can confirm is
that the *next* step is unambiguous:

- [ ] Scheduling `PROV-P1` is the only decision needed to start implementation.
- [ ] The four owner-chosen decisions (three protocol-named adapters; the deferred
      Responses adapter with a committed `AgenticModel` migration;
      `internal/provider/data/` as the single data directory; embedded rather than
      disk-read data) appear in the ledger and are reflected in the phase files.
- [ ] The two retained problems are visible rather than hidden:
      `PROVIDER-AGENTIC-MIGRATION` and `PROVIDER-DATA-CONFIG-EDIT` are on the
      living board, and `DEEPSEEK-REASONING-CONTENT` is explicitly out of scope.

## Not accepted as evidence

- A chat transcript. The durable record is `docs/plans/provider-registry/` plus
  `docs/logs/2026-09-18-provider-registry-plan/`.
- A passing `just ci`. This delivery has no executable behaviour to gate; its
  acceptance is that a reviewable, decision-complete plan exists.
