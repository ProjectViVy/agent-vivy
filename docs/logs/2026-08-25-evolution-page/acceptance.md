# Acceptance — evolution page

Date: 2026-08-25

From a user’s perspective, how to confirm that “the Evolution page cannot be
opened” is fixed and the new page works:

1. Open `http://127.0.0.1:3015` (the development split pair; or any entry point
   with the new version installed).
2. In the Vivy group of the left navigation, click “Evolution”: the “Evolution
   is not yet connected” notice no longer appears; instead, the “Evolution” page
   opens with a yellow “Demo / Local Mock” banner and the subtitle
   “Skill authority and auditable evolution governance (local demo data)” at the top.

In a fresh browser (or after clearing `vivy.demo.skills` /
`vivy.demo.skill-requests` / `vivy.demo.skill-docs` / `vivy.demo.autodream` and
refreshing), the complete demo loop is visible:

3. The default tab is “Pending Review (1)”: the list has three requests
   (Pending Review / Accepted / Stale). Click “Sync Documentation: Add Rollback
   Steps” → proposal details appear on the right (Markdown, base hash, Evidence JSON).
4. Click “Accept”: the request becomes “Accepted,” and the tab counts become
   Pending Review (0) / Skill (1).
5. Switch to “Skill (1)”: `vivy-doc-sync` appears. Open it to see that the
   proposal content is now the authoritative document, its content hash has been
   updated, and Edit / Disable / Hard Delete / History Snapshots actions are available below.
6. Switch to “AutoDream (3)”: three run records appear. Click `run-demo-1` to
   see the run facts card and 5 progress events; click “View Pending Review
   Request” to return automatically to the Pending Review tab with the proposal selected.
7. The “New Request” action in the upper-right of the “Pending Review” tab lets
   you fill out and submit a new pending request (slug/title/reason required);
   it appears in the list after submission.
8. Details for a stale request show a red-box notice “Request is stale… cannot
   accept”; the Accept/Reject buttons are disabled. This is intentional governance
   semantics, not a bug.
9. Refresh the page; all of the above state is retained (data is stored in the
   browser’s localStorage).

Boundary notes (expected behavior, not defects):

- This is a demo-data-layer page (like Persona / Memory / Notebook), not Vivy
  server state; the kernel’s real AutoDream/Evolution capabilities are still
  planned (TODO board MEM-1).
- If the browser previously visited the old Skills page, the Skill tab may be
  empty (the old cache has no evolution-managed item): clear the `vivy.demo.*`
  keys above or accept a request directly to see the evolution-managed skill.
