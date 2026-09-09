# Acceptance — VC-1e

How to prove to a person that this delivery works.

## 1. `vivy init` (human perspective)

Run it in a directory containing an existing project:

```text
$ vivy init
created C:\path\to\project\AGENTS.md
record only what is non-obvious: an agent can read the code, but it cannot guess intent.
```

- The generated `AGENTS.md` has four empty sections (overview / build / conventions / pitfalls), each with an HTML-comment prompt.
- If `.cursorrules` or `.github/copilot-instructions.md` already exists in the directory, the output names those files and the generated AGENTS.md ends with a "Keep in sync or reference" section.
- An empty directory (not even `main.go`) → exits with an error and generates nothing.
- An existing AGENTS.md → exits with an error and leaves the original file byte-for-byte unchanged.

## 2. AGENTS.md injection (run perspective)

1. Put AGENTS.md in a run's workspace (for example, add it through the UI/tool after `vivy init`, or let the run write it itself).
2. Send a message in that session.
3. **Observable effect**: the model's response follows the instructions in AGENTS.md (for example, a verifiable rule such as "Start the answer with VIVY"); when AGENTS.md is absent, behavior is exactly as before.
4. **Per-run isolation**: another run whose workspace lacks the file is unaffected.

## 3. Transience (D6's core promise)

- Injected content appears only at the model-call site: the AGENTS.md body can **never be found** in Journal event replay or message storage (tests #4/#5 assert this with a marker).
- Context compaction never needs to process it—the injection happens after the compaction layer and therefore never enters the summary.

## 4. Boundaries

- A run without AGENTS.md: zero injection, zero behavior difference (test #2).
- After approval suspension → resume: injected content still appears exactly once and is not duplicated by the checkpoint round trip (test #5).
