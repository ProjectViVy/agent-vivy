# Acceptance: what a human should see

## Toasts for transient failures

1. Start the split pair (`just run`, then `cd ui && pnpm dev`) and open
   `http://127.0.0.1:3016`.
2. Trigger any action failure, for example rename a session to something the
   backend rejects, or stop the backend and switch the model. A card appears at
   the **top right**, under the header, with a coloured left rail, a short
   human title, at most two lines of detail, and a retry button when retrying is
   meaningful.
3. Repeat the same failure: the existing card moves to the top and shows `×2`
   instead of stacking duplicates. A fourth distinct failure drops the oldest.
4. Hover the stack: the auto-dismissing `ok`/`info` card stops counting down and
   resumes where it left off when the pointer leaves. Focus a card's close button
   and press Escape: the top card disappears.
5. Open the header model menu while a toast is visible: the toast stays above it.
   Open `/settings` on a narrow window: the connection pill becomes icon-only and
   the toasts span the viewport minus the 1.5 rem gutter.

## Machine noise stays folded

A failure whose text is an RPC/JSON/stack dump shows a plain sentence and a
`详情` disclosure. Expanding it reveals the raw text in a scrollable `<pre>` with
`复制诊断`; copying flips the label to `已复制` for two seconds. Failures the
backend already wrote for humans (for example `模型服务触发限流（HTTP 429）` or
`Unable to connect! Check your provider configuration!`) stay as the main
sentence — they are not hidden behind the disclosure.

## Persistent leftovers remain visible

Loading a page with a failing region (sessions list, chat history, run failure,
work controls, MCP/cron/skills/Marketplace lists, settings, approvals,
lifecycle, files, every first-party plugin panel) leaves a quiet raised card with
a danger rail in place until the region recovers. Retry buttons call the same
loader that originally failed. Dialog and field errors stay next to their input.

Screenshots of the rendered states are in `shots/`.
