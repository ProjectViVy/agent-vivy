import { useEffect, useRef } from 'react';
import { useRouter } from '@tanstack/react-router';

/** Use the host router's blocker; isolated Module previews still guard reloads. */
export function useNavigationGuard(
  enabled: boolean,
  shouldBlock: () => boolean,
  decide: () => Promise<boolean>,
) {
  const router = useRouter({ warn: false });
  const latest = useRef({ shouldBlock, decide });
  latest.current = { shouldBlock, decide };
  useEffect(() => {
    if (!enabled) return;
    if (router)
      return router.history.block({
        enableBeforeUnload: () => latest.current.shouldBlock(),
        blockerFn: () =>
          latest.current.shouldBlock() ? latest.current.decide() : false,
      });
    const beforeUnload = (event: BeforeUnloadEvent) => {
      if (latest.current.shouldBlock()) {
        event.preventDefault();
        event.returnValue = '';
      }
    };
    window.addEventListener('beforeunload', beforeUnload);
    return () => window.removeEventListener('beforeunload', beforeUnload);
  }, [enabled, router]);
}
