# Acceptance — CMP-3

How to manually confirm this slice works (development environment `just run` + `cd ui; pnpm dev` →
http://127.0.0.1:3015):

1. Open or create a session and run it until compaction is triggered (or go directly to
   Settings → General → the Context Compaction card and click "Compact now", provided the
   session has enough messages; when no compaction is needed, it reports "Compaction not
   currently needed").
2. After compaction succeeds, entries appear in the "Compaction history" block below the
   card: run badge + time + "Folded N messages" + summary text (folded to at most three
   lines). No manual page refresh is needed.
3. Empty-session case: the "Compaction history" block shows empty-state copy (no records);
   with no session open, it shows guidance copy. After switching between Chinese and
   English, all copy follows the selected language; no raw `settings.compaction.*` keys
   appear.
4. Interface: `POST /rpc` `session/compactions` with `{"session_id": "..."}`
   returns `{"compactions": [...]}` newest-first; an unknown session returns
   CodeNotFound; omitting `session_id` returns InvalidParams.
5. Regression: existing compaction-card configuration saving, "Compact now", and
   "Refresh usage" behavior is unchanged; both the existing and new specs in
   `ui/e2e/compaction-setting.spec.ts` are green.
