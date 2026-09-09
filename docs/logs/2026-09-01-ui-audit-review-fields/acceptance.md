# Acceptance

## How to verify

1. Open `http://127.0.0.1:3015` (split Vite + `just run` control plane).
2. Open the Review sheet from the chat-page shield entry, or open `/approvals`.
3. Select any record. After Impact/Reversibility/Scope/Trust in the detail
   `<dl>`:
   - Created at / Expires at are always present, formatted for the interface
     language (switch language in Settings → Language and refresh; the time
     string follows the zh/en locale).
   - Records with `decided_at` (approved/rejected/expired) add a Decision time
     row.
   - A rejected approval shows the Decision reason (the reason entered for
     deny).
   - An expired/stale record shows the Stale reason; a failed submission shows a
     red Error row.
   - An approval with a precondition shows the Precondition hash (monospace,
     with long values allowed to wrap).
4. Check both languages: zh shows labels such as Created at/Expires at/Actor;
   after switching to English, the same rows use Created/Expires/Actor, with no
   raw i18n keys exposed.

## Quick ways to create observable states

- Pending approval: have the model execute a write operation that requires
  approval; a pending record appears (no Decision time/Decision reason yet,
  which is expected because fields render only when present).
- Expired state: let a record expire (or use old data) → the Stale reason row
  appears.
- Rejection: enter a reason in the deny field and submit → the record details
  show the Decision reason.
