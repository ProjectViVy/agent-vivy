# Acceptance guide — How a person can confirm "tools are connected"

## 1. Tools are available in chat (direct acceptance for this complaint)

1. Open http://127.0.0.1:3015 with `just dev` and create a session.
2. Send: `What tools do you have now? Can you see what is in the workspace?`
3. Expected: the model no longer answers "I was not assigned any tools"; it should report the tool list and, when asked about
   the workspace, **actually call** `list_dir` / `read_file` and provide the real directory contents
   (tool-call records should be visible in the trajectory panel or the response).
4. Verify with a message that contains no "keywords" (under the old bug, Chinese messages always had 0 tools).

## 2. Settings → Tools is real configuration

1. Settings → Tools should show the complete built-in catalog (about 24 tools), with read-only / approval-required
   badges and toggles for each; it should no longer be fake "demo sandbox mode" cards.
2. Turn off a tool (such as `search_files`) → save → ask the model in a new session which tools it has:
   it should no longer appear in the list; turn it back on and save to restore it.
3. Turn everything off and save → the new session is pure chat mode (the model says no tools are available); the Restore defaults button
   returns to the config-default set with one click.
4. **No restart is required** after saving: the next run takes effect (the engine is rebuilt while idle).

## 3. SKILL brings up hidden tools

1. In Settings → Tools, hide `write_file` (or any tool you do not want permanently in the context) and save.
2. Prepare a skill with this SKILL.md frontmatter:
   ```yaml
   ---
   name: note-writer
   description: Write key points to a workspace file
   tools:
     - write_file
   ---
   ```
   (Create and place the directory under `skills_root`; use `skills_manage` or place the file directly.)
3. In chat, ask the model to "view the note-writer skill and follow it": after calling `skill_view`,
   it can call `write_file` **in the same turn** and actually write the file; in other sessions that have not viewed the skill,
   the tool remains invisible and unavailable.
4. If a skill declares a nonexistent tool name, it is ignored when mounted (the run does not fail); editing the skill with `skill_manage`
   does not lose the `tools:` declaration.

## 4. Approval semantics are unchanged

When a hidden effectful tool is called after a skill mounts it, the normal approval prompt still appears;
"mounting" does not bypass the human gate (D-012).
