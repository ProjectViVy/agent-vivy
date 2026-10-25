import { useEffect, useMemo, useRef, useState } from 'react';
import { ShieldCheck } from 'lucide-react';
import { ApprovalQuickCard, pickPendingReview } from '@/components/approvals/ApprovalQuickCard';
import { useVivyStore } from '@/lib/store';
import { useTranslation } from '@/i18n';

/**
 * The approval trigger and its floating card. Open state and drag offset are
 * local to the composer: widening the SDK Face contract with presentation state
 * would make every surface carry it. Approval data itself stays in the store.
 */
export function ComposerApproval() {
  const { t } = useTranslation();
  const reviews = useVivyStore((state) => state.reviews);
  const reviewsError = useVivyStore((state) => state.reviewsError);
  const reviewBusyIds = useVivyStore((state) => state.reviewBusyIds);
  const respondReview = useVivyStore((state) => state.respondReview);
  const openReviewCenter = useVivyStore((state) => state.setReviewCenterOpen);
  const activeSessionId = useVivyStore((state) => state.activeSessionId);
  const [open, setOpen] = useState(false);
  const [offset, setOffset] = useState({ x: 0, y: 0 });

  const pendingCount = useMemo(() => reviews.filter((review) => review.status === 'pending').length, [reviews]);
  // One decision at a time: this session's earliest pending item, then the
  // earliest globally.
  const quickReview = useMemo(() => pickPendingReview(reviews, activeSessionId), [reviews, activeSessionId]);

  // New approvals pop the card; after a manual close the same batch stays closed.
  const previousPendingCount = useRef(0);
  useEffect(() => {
    if (pendingCount === 0) setOpen(false);
    else if (pendingCount > previousPendingCount.current) setOpen(true);
    previousPendingCount.current = pendingCount;
  }, [pendingCount]);

  return (
    <>
      <button
        type="button"
        aria-expanded={open}
        aria-controls="approval-quick-card"
        onClick={() => setOpen(!open)}
        className="relative shrink-0 rounded-lg p-1.5 transition-colors hover:bg-hover"
        title={t('chatInput.reviewCenter')}
        aria-label={t('chatInput.reviewCenter')}
      >
        <ShieldCheck className="h-4 w-4" />
        {pendingCount ? <span className="absolute right-0 -top-0.5 grid h-[18px] min-w-[18px] place-items-center rounded-full bg-danger px-1 text-2xs font-bold leading-none text-danger-fg" aria-hidden="true">{pendingCount}</span> : null}
      </button>
      {open ? (
        <ApprovalQuickCard
          review={quickReview}
          pendingCount={pendingCount}
          busy={quickReview !== null && reviewBusyIds.includes(quickReview.id)}
          error={reviewsError}
          offset={offset}
          onOffsetChange={setOffset}
          onRespond={async (action) => {
            if (!quickReview) return;
            await respondReview(quickReview.id, { action });
            // A successful decision closes the card; a failed one keeps the
            // store's reviewsError visible inside the open card.
            setOpen(false);
          }}
          onOpenCenter={() => openReviewCenter(true)}
          onClose={() => setOpen(false)}
        />
      ) : null}
    </>
  );
}
