# Editing prompts

These UTF-8 Markdown files are the shipped defaults. They are compiled into the
executable so a copied binary still works. To edit a running installation without
rebuilding, copy only the files you want to customize into the agent's `RIG_HOME`
directory, or into `.rig/` in its trusted working directory:

| File | Purpose |
| --- | --- |
| `ASSISTANT.md` | Persistent assistant's base instructions and conversational style |
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
they take precedence over `ASSISTANT.md` / `CODING.md`. They and `APPEND_SYSTEM.md`
keep their existing startup-only loading behavior. Remove an old `SYSTEM.md`
override if you want the new role file to control that layer. Managed persona
still precedes the base role. Dream keeps its separate role.

## Editing persona

Write the desired personality in a separate `PERSONA.md` file. In the assistant:

```text
/persona show
/persona update 0 PERSONA.md
```

Replace `0` with the revision reported by `show`. The command imports the Markdown
into Laputa's versioned store; it does not watch the file. Repeat `show` and `update`
after later edits. A concurrent evolution update causes a revision conflict instead
of silently overwriting it. `/persona rollback <current> <target>` restores a prior
published version. Do not edit `.vivy/laputa/persona.json` by hand. Persona is
distinct from base instructions, Recap content and the internal Dream role.
