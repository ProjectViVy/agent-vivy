# UI-AUDIT-REVIEW-INSPECTOR: Run Inspector Review tab via shared ReviewCard

## Scope

`docs/architecture/hitl-review-center.md` §UI requires "The run inspector uses the
same `renderReviewCard` renderer for inline decisions", but Run Inspector had only
run/background/children tabs, so approvals could be handled only in Review Center.

- Add `ui/src/components/approvals/ReviewCard.tsx`: a shared approval card extracted from
  the ApprovalsView detail (title + status badge + audit dl + prompt/preview [diff first]/
  risk/redacted parameters + inline decision controls while pending). ApprovalsView details
  and RunInspector reuse the same component, satisfying the "same renderer" constraint.
- `RunInspector.tsx`: make Tabs controlled and add a fourth "Reviews" tab (with count),
  filter store reviews by the current run's `run_id`, and refresh `loadReviews()` when the
  tab is selected (event-driven approval/question refresh remains unchanged). Card actions
  call `respondReview` directly and use the same queue busy lock (`reviewBusyIds`).
- Make ApprovalsView a thin shell: retain list + layout, replace the detail body with
  `ReviewCard` (`key` remounts by review ID, preserving the old "changing the selection clears
  the draft" behavior). Export `statusLabel` from ReviewCard for reuse.
- Add `runInspector.review` (Reviews {{count}}) and `runInspector.noReviews` to i18n en/zh.

## Explicitly not done

- Did not write e2e for "a running process produces a real approval → inline decision in the
  inspector": it requires a real provider to trigger tool approval; the spec covers only
  entry reachability + empty state (provider-independent).
- Did not change the reviews polling strategy (no new polling; it still refreshes when a run
  opens, on events, and when switching tabs).
- Did not change Review Center queue interactions or the Review sheet.
