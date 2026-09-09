# UI-AUDIT-RUN-DETAIL — Keyboard-accessible details for Run event payloads

## Problem (audit item)

The `RunInspector` event list put structured `RunLogEvent.payload` only in the
HTML `title` attribute: it was visible only on hover (completely invisible on
touch) and unreachable by keyboard or screen reader—violating the "logs are
first-class" requirement (the backend payload is structured factual data, but
the UI had no readable presentation).

## Fix

Event rows (already `<button>` elements) become **expandable toggles**:

- Clicking (or pressing Enter/Space—the native button semantics) toggles the row
  details.
- When expanded, a `<pre>` renders the pretty-printed payload JSON below the row
  (`JSON.stringify(event.payload ?? null, null, 2)`); `whitespace-pre-wrap
  break-words` prevents overflow and `max-h-48` enables internal scrolling.
- `aria-expanded` marks the expanded state, and the expanded row is highlighted
  with `bg-muted`.
- `title` is removed (the tooltip duplicated the details and was the defect
  itself).
- No new i18n keys (the content is a JSON literal).

Only one row is expanded at a time (`openSeq: number | null` single-value state)
—expanding multiple rows in a long event stream would make the list lose its
anchor.

## Change list

- `ui/src/components/chat/RunInspector.tsx`: `openSeq` state + event-row
  expand/collapse + payload `<pre>` details; the other run/background/children
  panels are unchanged.
