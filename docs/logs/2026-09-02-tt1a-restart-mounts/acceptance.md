# Acceptance — TT-1a

How a human can tell it worked:

1. Start a chat, use a skill that mounts hidden tools (e.g. a skill
   declaring `echo_info`), then trigger a run that suspends on an approval
   or an ask_user question *after* the mount.
2. Restart `vivy.exe` while the run is still suspended.
3. Answer the question / approve the request. The resumed run can still
   call the skill-mounted tools — before this fix the restart silently
   dropped every mount and the tool call failed after resume.
4. The Run Inspector shows the same `tool.mounted` audit events as before
   the restart; recovery replayed them, it did not add new ones.

Regression guarantees: runs that never mounted anything rebuild exactly as
before (nil registry); a corrupt `tool.mounted` payload degrades to "no
mounts" with a warn log instead of failing recovery; TT-2's same-process
resume behavior is unchanged (its regression test still passes).
