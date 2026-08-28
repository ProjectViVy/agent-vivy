# Acceptance

A later agent or human working in this repo should:

1. Start the split pair: `just dev` (or `just run` + `cd ui && pnpm dev`).
2. Open `http://127.0.0.1:3015/dashboard`.
3. Confirm the **DemoBanner is gone** from the dashboard page.
4. Click the **Token** tab. With a fresh database, see the empty state message
   ("所选周期内暂无模型调用记录" / "No model calls recorded in this period").
5. Send at least one chat message that triggers a model call with usage data.
6. Refresh the Token tab. Verify:
   - KPI cards show non-zero Total Tokens / Input / Output / Request Count.
   - Model distribution table lists the model used.
   - Provider distribution shows the provider bundle name.
   - Timeline chart has bars matching the current period.
   - Session table lists the session with correct token counts.
7. Switch periods (1d → 3d → 1w). Old numbers stay visible with a loading
   indicator until the new response arrives; no flash to zero.
8. Click Export. The downloaded JSON contains real `period`, `total`, `models`,
   `providers`, `timeline`, `sessions` — not demo data.
9. Overview and Trajectory tabs still show their respective demo content
   (those are separate TODOs: overview KPIs, `UI-TRAJ`).

## Not acceptance criteria

- Cost figures (no pricing table exists).
- Cache token figures (schema has no cache fields).
- Real-time streaming updates (polling on period change is sufficient for v1).
