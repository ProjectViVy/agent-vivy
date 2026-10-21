---
name: testing-vivy-ui
description: Exercise Vivy browser behavior through an isolated split backend/Vite and an explicit Recipe, using deterministic local models or authorized real-provider runs.
---

# Testing Vivy UI

Use root AGENTS.md and vivy-kernel-ci for check scope and completion policy.
Keep an explicit disposable VIVY_USER_HOME and configuration; do not inspect
or modify daily tenant Journals. Use backend :8787 and Vite :3015, which proxies
/rpc. Set VIVY_UI_RECIPE for the selected development Recipe.

For deterministic minimal chat, reuse ui/playwright.minimal.config.ts and its
local model fixtures. Pack recipes/minimal-chat.vivy.yml, stage its UI, and set
VIVY_MINIMAL_CHAT_ARTIFACT and VIVY_UI_ASSEMBLY_ENTRY before Playwright. The small
CI browser scope runs this production RPC/Service.Run path, not a second loop.
Exercise the changed interaction, approvals/cancel when affected, exact tool
result continuation and terminal/idle behavior. Empty or skipped required tests
are not acceptance.

For other selected Modules, use their actual Recipe and existing behavior tests.
A mock needing an active parent Run must create it through the real provider/RPC
path; never synthesize database state. Real-provider tests require authorized
credentials through existing private settings/environment, without logging them.

Inspect the current accessible UI state rather than remembered coordinates.
Do not change approval presets to hide a regression. Report its failing path.
Record the tested Recipe, candidate and observed outcome according to root rules.
