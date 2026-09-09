# Real Skills UI + marketplace (skills.sh integration)

- Date: 2026-08-30
- Branch: `feat/skill-marketplace` (worktree `../agent-vivy-skill-market`, following
  parallel-worktree-isolation: the root tree had uncommitted compaction changes that
  overlapped this iteration's files)
- Reference: agent-diva's skills.sh marketplace integration
  (`agent-diva-manager/src/marketplace.rs`, `data/marketplace_featured.yaml`,
  `scripts/fetch_marketplace_featured.py`, `MarketplaceTab.vue`)

## Background

The Skill implementation in Vivy's kernel was already real (`EinoSkillBackend` + Eino
skill middleware + `skills_list/skill_view/skill_manage` tools + RPC `skills/list`,
`skills/get`); only the UI was fake: `/skills` used localStorage demo data from
`demo-api.ts`, the real client in `api.ts` was never called, and the UI's expected
`SkillDto` shape did not match the real payload. DIVA's "plugin marketplace" is actually
a skills marketplace backed by skills.sh; this change ports that logic to Vivy.

## Changes

### Core (Go)

- `internal/tools/skills.go`: add `enabled` to `SkillSummary`; add `SetSkillEnabled`
  to `SkillOperations` (control-plane CAS toggle, not HITL staging); add the
  `SkillsMarketplace` interface and `MarketplaceSkill` / `MarketplaceFeatured` /
  `MarketplaceInstallResult` DTOs.
- `internal/runtime/skills_backend.go`: parse `enabled` in frontmatter (default true);
  Eino `List/Get` expose only enabled skills (disabled `Get` errors, while control-plane
  `ListSkills/ViewSkill` still see all); `SetSkillEnabled` uses content-hash CAS and
  atomically rewrites normalized frontmatter while preserving content.
- New `internal/runtime/marketplace.go`: skills.sh adapter — `Search` (q≥2 characters,
  limit clamped to 1..50, default 20), `Featured` (offline `go:embed` snapshot), and
  `Install` (`owner/repo/slug` three-part id, segment allowlist, create-only conflict
  error, SKILL.md frontmatter name must equal slug, attached files limited to
  references/templates/scripts/assets, 512KiB per file, 128 files, 5MiB total,
  binaries rejected, paths normalized, whole install rolled back on failure, rechecked
  with the real loader and returned as SkillView, out-of-bounds paths recorded in
  skipped_files). Upstream failure returns `MarketplaceUpstreamError`;
  `VIVY_SKILLS_MARKETPLACE_URL` overrides the base URL.
- New `internal/runtime/marketplace_featured.yaml`: built-in featured ranking snapshot
  (ported from the DIVA snapshot, generated_at 2026-08-20).
- New `scripts/fetch_marketplace_featured.py`: snapshot refresh script (uses the
  leaderboard API with a token, fetches the home-page ranking without one;
  `VIVY_SKILLS_MARKETPLACE_TOKEN`).
- `internal/config`: add `runtime.skills_marketplace_url` (default
  `https://skills.sh`, validated as an absolute http(s) URL).
- `internal/rpc/control.go`: add `skills/set-enabled` (409 = stale hash),
  `skills/revisions/list` (read-only list of staged `skill_manage` revisions), and
  `skills/marketplace/search|featured|install` (upstream failure → new error code
  `-32010`, mapped to 502 by the UI); add `skills.marketplace` /
  `skills.revisions` to `initialize` capabilities (broadcast according to injected
  dependencies).
- `internal/app/app.go`: assemble `MarketplaceService` when `skills_root` is configured
  and inject it into ControlDeps; point `SkillRevisions` at the Journal.

### UI (React)

- `ui/src/lib/api.ts`: add five RPC methods and types (`MarketplaceSkill`,
  `MarketplaceFeatured`, `MarketplaceInstallResult`, `SkillRevision`,
  `SkillSummary.enabled`); `mapCode` adds `-32010 → 502 bad_gateway`.
- `ui/src/components/skills/SkillsView.tsx`: switch the whole page to real RPC (matching
  the MCP page's `import * as api` pattern), with three tabs: Installed (list + detail +
  enable/disable toggle + warnings + click-to-view attached files), Marketplace
  (capability-gated), and Change requests (read-only cards for real pending revisions,
  with preview / warning / run binding). Reload the catalog automatically on a 409
  toggle response.
- New `ui/src/components/skills/MarketplaceTab.tsx`: 300ms debounced search (≥2
  characters), featured ranking with snapshot date when no search is entered, k/m
  install-count formatting, disable installed slugs, busy state, retry on failure, and
  refresh the installed list after a successful install.
- `ui/src/routes/_layout.skills.tsx`: remove `DemoBanner`.
- `ui/src/i18n/{zh,en}.ts`: add marketplace / enable-disable / revision keys to the
  skills domain (zh is the baseline, en mirrors it); retain the
  `builtin/user/alwaysLoaded/...` keys still used by the Evolution page.

### Explicitly not done

- The Evolution page remains a demo (`UI-EVO`, pending the kernel Evolution/AutoDream
  capability proposal).
- Skill content changes (create/edit/patch/delete) still go only through `skill_manage`
  + HITL review; the UI marketplace page does not edit content.
- Update/upgrade paths for marketplace skills are not implemented (DIVA also deletes and
  reinstalls); persistent `always` injection was not ported. TODOs `SKILL-MKT-1` and
  `SKILL-MKT-2` are recorded.
- Enable/disable toggles do not create SkillRevision audit rows (control-plane metadata
  flip, not a content change).
