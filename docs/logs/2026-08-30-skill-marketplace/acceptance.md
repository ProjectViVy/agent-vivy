# Acceptance: how a user can confirm this change works

## Prerequisite

```text
just run            # or just dev (backend + Vite)
# Open http://127.0.0.1:3015/skills in a browser
```

The top of the page **no longer has** a yellow "Demo / Local simulation" banner.

## 1. Installed skills tab (real data)

1. The list comes from `skills/list` (one SKILL.md per `skills_root` directory). When
   the directory is empty, the empty state says "Install a skill from the marketplace,
   or put a directory containing SKILL.md in skills_root and refresh".
2. Manually place a skill directory (`data/skills/demo-skill/SKILL.md`, with frontmatter
   containing `name: demo-skill` and `description`), then click "Refresh"; the skill
   appears in the list.
3. Open the skill: see the body (untrusted data), content hash, and an attached-file
   button (click to load that file's contents). If the body contains a phrase such as
   `curl `, an amber warning bar appears (server-injected scan result).
4. Enable/disable toggle: switch to "Disabled" and the badge becomes "Disabled"; run
   the agent once and the skill no longer appears in the `skill` tool catalog; switch
   back to "Enabled" and it returns. If another process changes the file in the
   meantime, the toggle reports 409 and reloads the catalog automatically.

## 2. Marketplace tab (skills.sh integration)

1. The "Marketplace" tab appears only when the backend broadcasts the
   `skills.marketplace` capability.
2. With no search term, show the built-in featured ranking (including snapshot date),
   with install counts (such as 846.6k) and source repositories.
3. Enter at least 2 characters in the search box (300ms debounce) to get skills.sh
   search results; show the empty state when there are too few results.
4. Click "Install": the button enters an installing state; after success, refresh the
   installed list automatically, show the new skill in the "Installed skills" tab, and
   create a same-named directory under `skills_root`; when installing the same skill
   again, disable the button (already installed).
5. If the network is offline or the upstream fails, the Marketplace tab shows an error
   box and a "Retry" button (RPC -32010 → 502 semantics), without affecting the
   Installed tab.
6. An installed skill can be used by the agent on the next turn (the Eino skill
   middleware re-lists from disk each round; no restart required): have the agent call
   the `skill` tool to load the newly installed skill.

## 3. Change requests tab (real staged revisions)

1. Have the agent call `skill_manage` in a conversation (for example, "Add a section
   explaining demo-skill"); the approval flow shows a diff preview (leave it staged and
   unapproved; approval is not required).
2. The `/skills` page's "Change requests (N)" tab shows the pending revision: skill
   name, action, target path, run binding, preview diff, and warning; the status badge
   is "Pending review".
3. The tab is read-only: approval/rejection still happens in the conversation's review
   flow.

## 4. Configuration

- `config.yaml` can override the marketplace base URL with
  `runtime.skills_marketplace_url` (default `https://skills.sh`; the
  `VIVY_SKILLS_MARKETPLACE_URL` environment variable takes precedence). Startup is
  rejected by config validation for an invalid URL.
- `python scripts/fetch_marketplace_featured.py` refreshes the built-in featured
  snapshot and rewrites `internal/runtime/marketplace_featured.yaml` (a rebuild is
  required for it to take effect).
