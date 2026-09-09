# Acceptance — VC-1f

## Manual acceptance (environment with a provider key, 127.0.0.1:3015 split pair)

1. In chat, ask Vivy to edit a workspace file (for example, "Change two to TWO in notes.txt").
2. In Review Center (or approval inside the bubble), the patch/write approval preview is no longer plain text:
   it shows `+N −M` statistics + a "Unified / Split view" toggle + line-number-colored diff;
   after switching to split view, the two columns align, deletion blocks are red, addition blocks are green, and the padded side has a gray background.
3. After approval, return to the chat page: the tool-result bubble shows "tool result + file path + the same diff
   view," and "Raw result" below expands to the raw JSON; other tool-result bubbles such as bash/grep
   retain their original plain-text style.
4. Scenarios such as missing AGENTS.md are unaffected: non-diff content (bash output, error text) is not misclassified
   as a diff (only a `--- `/`diff `/`Index: ` file header plus an `@@` hunk triggers diff rendering).

## Equivalent verification without a key

- The DiffView SSR rendering cases in `pnpm test` assert statistics, toggle buttons, and
  hunk content on the real component; diff.test.ts covers parsing, pairing, truncation markers, and rejection of non-diffs.
- Playwright e2e regression confirms that chat-page and settings-page interactions were not broken by this change.

## Rollback

`git revert` of this delivery's single commit removes the go-udiff dependency as well; no independent migration is required.
