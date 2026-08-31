# Acceptance — VC-1a bash tool

How a human can tell it worked:

1. Start the split pair (`just dev`) and open http://127.0.0.1:3015.
2. Pick the Programmer mask (code face) and ask the model something that
   needs the shell, e.g. "run `ls` and tell me what you see".
3. The model now has a `bash` tool available; a read-only script such as
   `ls` under approval policy `auto` runs immediately — no approval card
   interrupts the run, and the run journal shows a
   `policy.evaluated` governance event with reason
   "safe read-only invocation auto-approved" before the tool executes.
4. Ask for something effectful (e.g. "create a file with the current
   date in it"): the pre-existing approval flow fires exactly as before
   VC-1a — the classifier never widens the approval surface.
5. Ask for something in the deny table (e.g. "run `rm -rf /`"): the call
   is refused outright regardless of profile or approval policy.
6. On a host without bash installed, the tool reports
   "bash is not available on this host" instead of failing obscurely.

Rollback: revert the single `feat/vc1a-bash-tool` commit; `bash` leaves
the default enabled surface with it (config default was the only
registration point).
