# VC-0: face skeleton (kernel + UI + schemas)

Date: 2026-08-31. Branch: `feat/vc0-face-skeleton` (worktree `agent-vivy-vc0`).

## What changed

Introduced the per-run `face` dimension (web | tui | code), at the same level as RunMode: face attributes
the run to its entry point (which entry point is serving the run), not to the process mode. VC-0 only adds the
assembly skeleton and code-face prompt framing; it does not change the tool surface.

- Kernel
  - `internal/domain/face.go`: `Face` enum and `Valid()`.
  - `internal/runtime/face.go`: `normalizeFace` (empty → web; unknown → `ErrInvalidFace`),
    `withFace`/`runFace` (falls back to web when the context is unbound).
  - `internal/runtime/service.go`: `RunOptions.Face`; `run.started`,
    `tool.approval_required`, and `user.question_required` event payloads gain the
    `face` field (not omitempty; explicitly writes web consistently); the suspend/resume chain
    approvalDetails/questionDetails → rebuildPending/rebuildPendingQuestion →
    resumeRun chain passes face through each link, and interruptions preserve it.
  - `internal/runtime/prompt.go`: `composeRunPreamble` injects a code-mode preamble by face
    (operate directly on files in this run's workspace; perform no version-control operations unless the user explicitly requests them;
    prefer `path:line` for code references). The static Instruction has no face (it is an engine-level singleton).
  - `internal/runtime/preflight.go`: validates and echoes face in preflight.
  - `internal/rpc/control.go`: `preflight/run` and `turn/start` accept the `face` parameter;
    responses echo it; `ErrInvalidFace` maps to RPC -32602.
- Schemas: `schemas/events/payloads/{run.started,tool.approval_required,user.question_required}.json`
  adds the `face` enum field (old Journal entries without it are treated as web).
- UI (VC-0c)
  - `ui/src/lib/api.ts`: `Face` type; `preflight`/`startTurn` pass through `face`.
  - `ui/src/components/masks/mask-catalog.ts`: `faceForMaskId` — the programmer
    mask maps to code face; other masks leave it unspecified (the server defaults to web).
  - `ui/src/lib/store.ts` / `ui/src/components/chat/ChatView.tsx`: startRun,
    preflight, pending resume, and regeneration all carry the current mask's face.
  - i18n: the programmer mask's capability list adds "Runs with the code face".
- Contract documentation: `docs/dev/NEW_UI_ARCHITECTURE.md` synchronizes the Face type and preflight fields.

## Design trade-offs

- **No face → tool-surface filter table.** D1 decided that bash/job/grep/glob/multiedit are shared mainline
  tools available to every face; introduce switching only when VC-3 plugin tools create genuinely differentiated faces
  (at that point, mount it at the same level as `withSelectedTools`).
- **Masks remain a local UI personality choice**, not a kernel concern; programmer → code is a pure UI mapping.
- `tui` is retained only as an enum value; no entry-point assembly exists yet.

## Explicitly not done

- Face-specific tool surfaces or permission differences (VC-3).
- TUI entry point.
- Showing a face badge to the user in the message stream (optional; not scheduled).
