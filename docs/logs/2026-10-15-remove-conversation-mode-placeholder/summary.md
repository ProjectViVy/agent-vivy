# Remove the Code / Life preview selector

The chat header no longer displays the leftover Code / Life preview button.
`ConversationModePlaceholder` was mounted by `ui/src/routes/_layout.tsx` and
only toggled component-local state; it did not send Run metadata, change
session identity, or select an internal policy.

Removed the component, its sole shell import/render, its four exclusive en/zh
translation keys, and their cross-face catalog classifications. Updated the
existing masks browser acceptance case to expect no preview selector while
retaining the selected mask.

The separate, currently unmounted `CodeModeControl` and internal code Face,
policy, tool access, queue restoration, Work controls, and Plan/Act controls
remain unchanged. Ordinary-session persona behavior is unchanged. No setup,
data migration, Issue #58 memory runtime, or Issue #66 sidebar implementation
is included. The unique pinned LIFE main-chat contract remains separate work.

Work was isolated in `feat/issue-58-remove-mode-toggle`, based on `67eebf6`.
A fresh fetch found `origin/main` unchanged at `8088f5bc`; there was no newer
main-branch selector removal to reuse.

Release notes are omitted because this is source cleanup, not a release.
