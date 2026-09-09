# UI adaptive layout

Date: 2026-08-25
Status: complete

## What changed

Vivy UI now stays usable from ~360px phone width through desktop window resize.

- App shell uses `dvh` and viewport safe-area instead of `h-screen`.
- Narrow nav is a left Sheet, not a homemade overlay. Leaving mobile restores the desktop rail.
- Header keeps mask/model switching on small widths (icon-only) and drops secondary status copy.
- Shared Dialog / AlertDialog / Sheet cap to the dynamic viewport; Body scrolls, Header/Footer stay put.
- Notebook, Memory, Skills, Approvals (full page), and Cron use one `MasterDetail`: two panes from `md` up, list-or-detail with Back to List below `md`.
- Chat preflight actions wrap; composer keeps a bottom safe-area; bubbles and code blocks cannot stretch the page.

## Unchanged

RPC, store, demo data protocol, unused `components/ui/sidebar.tsx`, Studio overlay, and the still-disconnected chat attachment / draw / voice controls.

## Not done

Live walk of the split Vite UI on `http://127.0.0.1:3015` (no browser automation in this session). Playwright e2e covered 390 and 1280 against the embedded UI after `pnpm build`. Skills / Cron / full-page Approvals exclusive flows were implemented but not added as extra e2e cases beyond notebook and memory.

## Scope

UI only (`ui/`).
