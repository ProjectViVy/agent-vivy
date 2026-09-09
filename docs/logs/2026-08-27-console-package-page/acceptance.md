# Acceptance — 2026-08-27 console Packaging & Version page

How a human can tell the change worked.

1. Open Vivy Studio at `http://127.0.0.1:3090` and refresh the browser.
   The 「Vivy Console」 tab now shows **three sections: Main Console / Packaging & Version /
   Logs**.
2. Main Console is unchanged: the dev loop (pure-API backend + Vite dev server,
   one-click start/stop/restart, backend/frontend status cards).
3. Packaging & Version page:
   - header states it is the distribution lifecycle via `vivy-studio.exe`,
     **separate from Main Console's dev mode — no dev process is started or
     stopped here**;
   - ledger chips (generations / evals / releases / installs / events /
     worktrees) load real rows from the Studio ledger (a JSON table with
     the literal "Refresh" button);
   - the action form (pack / eval / release / reject / install / rollback /
     inspect inputs) and buttons run as **one concurrent job** with a
     streamed output viewer;
   - Release (literal UI action "Release") is refused unless the literal checkbox "I confirm release: manual operation" is checked
     (NG-25); install/rollback targets the tool refuses source tree/`data/`.
4. Separation is structural: switching between Main Console and Packaging & Version never
   changes the other page's state; starting a backend in Main Console is
   unaffected by the packaging page and vice versa.
