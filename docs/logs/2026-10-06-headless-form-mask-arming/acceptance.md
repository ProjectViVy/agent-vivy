# Acceptance — how a human can tell it worked

1. Start the split pair (`just dev`) and open `http://127.0.0.1:3015`.
2. Open the Masks sidebar, pick any mask, and select it for the current
   session. Before this change the selection failed with
   "module action capability is not configured"; now it succeeds, the
   selection persists across a page reload, and it survives a backend
   restart.
3. Creating, editing, and deleting masks in the sidebar works in dev the same
   way (they ride the same module action channel).
4. The backend startup log no longer contains
   "control actions disabled: sealed Generation identity unavailable".
5. A run started in the dev pair is admitted under the headless identity:
   a prompt snapshot row keyed `vivy-headless/1` appears in
   `run_prompt_snapshots` for that run, and the selected mask's body is part
   of the admitted instruction.
6. A packed build still behaves exactly as before: its identity comes from
   the embedded Generation Manifest, not from `vivy-headless/1`.
