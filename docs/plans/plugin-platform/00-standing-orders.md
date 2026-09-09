# Plugin Platform Standing Orders

## 1. Scheduling and ownership

- All functional phases begin as `UNSCHEDULED`.
- A human explicitly selects the next phase and its scope before implementation.
- One scheduled phase is one deliverable lane unless its document declares
  independently mergeable tasks.
- A dirty root or a concurrent lane requires a dedicated branch and worktree.
- When subagents are explicitly authorized, use `gpt-5.6-luna` with reasoning
  effort `max`; the supervising agent owns integration and verification.
- Do not combine opportunistic cleanup with a phase.

## 2. Required implementation loop

For every code task:

1. read this file, the phase file, and all linked normative specs;
2. inspect the exact existing files named by the task;
3. perform the phase's Eino capability check when applicable;
4. write the smallest failing test for the stated behavior;
5. run it and record the expected failure;
6. write the minimal implementation;
7. rerun the focused test;
8. run the next broader package test;
9. update Inspect/conformance evidence in the same task;
10. commit that independently reviewable concern;
11. run the phase gate, then `just ci` before delivery.

Generated code is produced only by its generator. Never patch a generated file
to make a test pass.

## 3. Architecture invariants

- Module graph existence is compile-time; runtime instances are activation-time.
- No Module may create a second Runtime, Journal, Policy engine, registry, RPC
  server, or tool execution path.
- Public SDK types contain no Eino, storage, RPC-server, or internal package
  types.
- Public Modules provide only cataloged `std/*` Ports.
- Trust comes from Source Catalog and Recipe; Descriptor self-promotion fails.
- Grants are requested, intersected, frozen, scoped, and inspectable.
- T2 is process-level trust, not sandboxing. T3 remains out of process.
- UI code is fully open after Recipe selection; no UI permission system is
  introduced.
- Protected Tool IDs remain internal even in a minimal Recipe that omits them.
- Failure never selects an undeclared fallback.

## 4. Eino evidence

Every affected task records:

```text
pinned module and version
packages/APIs inspected
capability match or concrete gap
adapter boundary
result: ADAPT | DEFERRED-INDEFINITE
```

For this program, a missing Provider/model/OAuth/orchestration/RAG/MCP adapter
capability does not authorize custom implementation. Do not create a stub,
placeholder Port, or speculative abstraction for a deferred capability.

Known pinned anchors at plan time:

- `github.com/cloudwego/eino v0.9.13`;
- `github.com/cloudwego/eino-ext/components/model/openai v0.1.13`;
- `github.com/cloudwego/eino-ext/components/model/claude v0.1.25`;
- `github.com/cloudwego/eino-ext/components/tool/mcp v0.0.9`;
- Eino ADK, Tool, Skill, agentsmd, reduction, summarization, compose, checkpoint,
  and callback APIs already imported under `internal/runtime`;
- EinoExt model adapters already imported under `internal/provider`.

Re-inspect the repository-pinned source during execution; plan-time evidence is
not permission to assume a newer API exists.

## 5. Test evidence

Every task states the exact test name and expected RED failure. Tests cover a
real contract, not only generated text. At phase closure record:

- focused test commands and results;
- package-level test commands and results;
- `just ci` result;
- required `vivy-sdk verify`, `pack`, and `inspect-artifact` results once the v1
  commands are implemented;
- browser or executable smoke when behavior is user-visible;
- representative failure-path output, with Secrets redacted.

## 6. Review checklist

- [ ] No v0 type, parser, compatibility branch, or alias was introduced.
- [ ] No external Module is auto-discovered.
- [ ] Provider and Consumer are both named and typed.
- [ ] Cardinality, ownership, lifecycle, and failure semantics are tested.
- [ ] Effective Grants and Trust assignment are inspectable.
- [ ] Default Generation behavior remains equivalent.
- [ ] Minimal Recipe removes imports and artifacts, not only visibility.
- [ ] Eino imports remain quarantined.
- [ ] Generated files were not hand edited.
- [ ] Error causes are preserved and Secrets are absent.
- [ ] Unfixed findings are recorded in `docs/TODO.md` section 0.1.
- [ ] Iteration log and focused commit exist.

## 7. Stop conditions

Stop the active phase and return to architecture review when:

- a required Port is missing from the normative catalog;
- two documents assign final authority to different owners;
- implementing the request needs runtime code loading;
- a public contract would expose Eino or an internal type;
- a missing upstream Eino capability is being replaced with custom machinery;
- behavior parity would require changing a Kernel invariant;
- a second concurrent lane would edit the same files;
- the requested work exceeds the manually scheduled phase.

Difficulty, test volume, or an unsolved implementation detail is not itself a
reason to expand scope.
