# Acceptance — P2-1 Diva capability inventory

How a human confirms it is effective:

1. Open `docs/research/diva-capability-inventory.md`: 10 domain tables + §11 statistics +
   §12 re-entry rules; every row should have Diva evidence paths and two columns comparing
   the current Vivy state.
2. Spot-check falsifiable rows:
   - Row 2.1 should record 47 provider presets (verify by counting `- name:` in
     `agent-diva-providers/src/providers.yaml`);
   - Row 9.1 should record 176+1 Tauri commands (verify with grep
     `#[tauri::command]`);
   - Row 1.10 (message edit/rewind/fork) should point to the in-progress UI-CHAT-ACT
     status.
3. In §0.1 of `docs/TODO.md`, the P2-1 line should be DONE 2026-09-02 with a corresponding
   §10 record; `docs/research/OPEN-ITEMS.md` should be synchronized for P2-1.
4. Usefulness criterion: someone who wants to implement a Defer capability next should be
   able to find its Diva comparison, current gap, and the reminder that a proposal must be
   written first, without re-archaeologizing agent-diva.
