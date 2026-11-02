# Editing prompts

These UTF-8 Markdown files are the shipped defaults. They are compiled into the
executable so a copied binary still works. To edit a running installation without
rebuilding, copy only the files you want to customize into the agent's `RIG_HOME`
directory, or into `.rig/` in its trusted working directory:

| File | Purpose |
| --- | --- |
| `CODING.md` | Coding agent's base instructions; a separate recipe |
| `RECAP.md` | Instructions for summarizing prior events, with no tools |
| `AUTODREAM.md` | Internal evolution role, without conversational persona |

For each file, the trusted project's `.rig/` override wins over `RIG_HOME`, which
wins over the compiled default. An untrusted project's files are ignored, including
files created after startup. Existing project approval (`/trust yes` or `--approve`)
applies. Files are plain Markdown, without frontmatter or template substitutions.
UTF-8 with or without a BOM is supported. Missing files use the next source; empty,
unreadable or invalid UTF-8 overrides report their path and prevent that new
operation from dispatching. Remove an override to return to its fallback.

Edits are read for each new model operation, including the next operation after a
tool call. They do not alter an in-flight request. Persistent model and Recap
retries use their frozen requests. A failed Recap can be retried after fixing its
file with `/recap refresh`. Prompt files only configure capabilities compiled by
the recipe; they cannot install tools or change lifecycle/permission checks.

`SYSTEM.md` and `--system-prompt` remain explicit replacements of the base role;
they take precedence over `CODING.md`. Managed persona still wraps and precedes
the base role, so a system-prompt override cannot erase it. `SYSTEM.md` and
`APPEND_SYSTEM.md` keep their existing startup-only loading behavior. Dream keeps
its separate, persona-free role.

## Editing persona

Personal instructions live in Laputa's six core files under the workspace:

```text
.vivy/laputa/personas/main/
  MISSION.MD        # required; never changed automatically
  IDENTITY.MD       # required; AutoDream may evolve it
  RELATIONSHIP.MD   # required; AutoDream may evolve it
  REDLINE.MD        # required; never changed automatically
  DREAM.MD          # optional; AutoDream may evolve it
  DARK.MD           # optional; AutoDream may evolve it
```

The files are assembled in that fixed order, separated by blank lines, and wrapped
in one `<persona>` block at the start of the system prompt. Empty optional files
are skipped. `MISSION.MD` must not be blank; every file is limited to 32 KiB and
the assembled persona to 64 KiB. Files appear only once the first save publishes
a revision; before that, compiled defaults are used without writing anything.

Diva's desktop Settings shows these files under **Core persona** and edits the same
store. In the terminal agent, edit the files directly or import one with:

```text
/persona show [NAME]
/persona update <revision> <NAME> <path>
/persona rollback <revision> <target>
```

Use the revision reported by `show`. An import replaces exactly one file and
publishes a new revision. Direct edits are used by the next model operation and
leave the published revision untouched until the next save; `show` reports that
drift. A concurrent evolution or external save causes a revision conflict instead
of silently overwriting it. A rollback restores all six files from the target
revision and publishes a new revision; target `0` restores the compiled defaults.

AutoDream may only propose changes to `IDENTITY.MD`, `RELATIONSHIP.MD`,
`DREAM.MD` and `DARK.MD`; `MISSION.MD` and `REDLINE.MD` are reported for human
review instead. Do not edit `.vivy/laputa/persona.json` or `revisions/` by hand;
they are the integrity-checked published snapshots.
