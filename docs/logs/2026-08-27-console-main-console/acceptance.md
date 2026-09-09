# Acceptance — 2026-08-27 console Main Console

How a human can tell the change worked.

1. Open Vivy Studio at `http://127.0.0.1:3090` and refresh the browser
   (no Studio restart needed), then open the **「Vivy Console」** tab.
2. The tab has two sections: **Main Console / Logs** — there is no separate
   Backend/Frontend tab division any more.
3. Main Console shows:
   - an overview line (Not running → Backend running · Frontend not running → … → All running);
   - **▶ Start all** — starts backend (auto-compiles the pure-API
     `vivy_headless` build when not overridden) then frontend; the message
     line reports the combined result;
   - **■ Stop all** and **⟳ Restart all** for both together;
   - **Backend status** card (Status/PID/Listening/EXE/Form/Build + individual buttons +
     EXE override) and **Frontend status** card (Status/PID/Entry/Command/Proxy + individual
     buttons + Open button) side by side.
4. Logs section still shows one unified timeline of Backend + Frontend lines with
   source chips, pause, and clear.
5. One-click flow on a clean state: click ▶ Start all → both cards flip to
   Running; open `http://127.0.0.1:3015` (the dev server is the app);
   click ■ Stop all → both cards flip to Stopped and ports free up.
