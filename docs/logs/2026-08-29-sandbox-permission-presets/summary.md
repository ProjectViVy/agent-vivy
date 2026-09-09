# 2026-08-29 — Sandbox real integration and three-tier permission presets

## Goal

Upgrade the settings page “Sandbox” from a migration preview to real configuration, and connect the chat area's
“Cautious” / “Smart” / “Trusted” to the session-level sandbox mode + approval policy. EINO
filesystem / command / HTTP backends execute according to the current turn's session policy.

## Changes

- Domain: `PermissionPreset` (cautious / smart / trusted / custom) maps to `read_only+ask` / `workspace_write+ask` /
  `danger_full_access+auto`.
- Storage: `SessionStore.UpdateSandboxPolicy`; new sessions write the default preset.
- Runtime: `SandboxManager` passes the mode with each call; `toolAdapter` hooks into `EvaluateApprovalPolicy`; `Service` pins
  the current turn's knobs at `turn/start`.
- RPC: `session/set_permission`; `sessionResult` returns sandbox fields; `settings/get|update` adds a sandbox section.
- UI: `SandboxSettingsCard` replaces the preview; the chat-area permission selector writes to the current session; switching to
  “Trusted” requires confirmation.

## Explicitly not done

- OS-level process sandbox (bwrap / Seatbelt / Windows ACL)
- deny glob, editable `auto_approve_tools`, and tool timeouts inside the sandbox
- hot-switching presets during an in-progress run
- `danger_full_access` still cannot escape the per-run workspace

The not-done items are recorded in `docs/TODO.md` §0.1: `SBX-OS` / `SBX-GLOB` / `SBX-LIVE`.
