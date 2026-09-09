# SKILL-MKT-1 — marketplace skill version comparison and in-place upgrade

## Deliverables

Marketplace skills were previously create-only: reinstalling an existing skill name returned
409 ("already exists"), so the skill had to be deleted and reinstalled; there was no concept
of "which version is installed / whether a newer version exists." This makes version
relationships a local content truth (byte-level comparison of the hosted file set) and adds
in-place upgrades:

- **Provenance manifest**: on install/upgrade, write `.vivy-skill.json` in the skill root
  (`marketplace_id` / `snapshot_hash` / `installed_at` / `upgraded_at`). The skill loader
  reads only SKILL.md and the four hosted directories; the manifest is completely invisible
  to content, warnings, and listing.
- **`skills/marketplace/install` adds a `mode` parameter**: `create` (default, unchanged
  behavior; existing skill returns 409) and `upgrade` (in-place upgrade: requires the skill
  to have a manifest and matching `marketplace_id`, otherwise 409 with a delete-and-reinstall
  hint). The response adds `outcome`: `created` / `upgraded` / `up_to_date` (no disk write
  when the hosted set is byte-for-byte identical).
- **Upgrade semantics**: mirror the snapshot's hosted set (SKILL.md + references/templates/
  scripts/assets) — add new files and delete hosted files no longer present in the snapshot;
  leave root-level loose files outside the hosted scope untouched. Run all snapshot
  validation (path traversal/size/binary/frontmatter name) before any disk write; upgrades
  roll back from an in-memory backup (no temporary directory outside the skill root is
  created, `ListSkills` never sees an invalid-name directory, and all writes use atomicWrite).
- **`skills/marketplace/check` (new RPC)**: look up the marketplace snapshot named by an
  installed skill's manifest and return `not_installed` / `unmanaged` / `up_to_date` /
  `upgrade_available` (with `marketplace_id` and `snapshot_hash`).
- **UI (`MarketplaceTab`)**: change the installed row from the dead "Installed" button to
  "Check for updates" → show `upgrade_available` as an "Upgrade" button (mode=upgrade),
  `up_to_date` as an "Up to date" badge, and `unmanaged` as a "Locally placed skill" hint;
  show an outcome notification bar after an upgrade. Remove the unused `installedLabel` key.

## Explicitly not done

- No automatic batch checks (N skills = N upstream downloads; the user clicks one skill at a
  time).
- Do not migrate existing manually placed skills without a manifest (`unmanaged` is shown
  truthfully and the delete-and-reinstall path remains).
- The upstream `hash` field is display-only and does not participate in the decision (the
  algorithm is unknown; byte-level content comparison is the only truth).

## Related files

`internal/tools/skills.go` (interfaces/types), `internal/runtime/marketplace.go`
(manifest/upgrade/check), `internal/rpc/control.go` (mode validation/check route/error code),
`ui/src/lib/api.ts`, `ui/src/components/skills/MarketplaceTab.tsx`,
`ui/src/i18n/{en,zh}.ts`.
