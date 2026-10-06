# Human acceptance

1. Run `just dev` (or `./dev.ps1`) and open `http://127.0.0.1:3015`.
   The script builds the default assembly with `vivy_headless` and Vite proxies
   its control plane on port 8787.
2. Open Persona. On a new data directory initialize the five required documents
   (IDENTITY, RELATIONSHIP, REDLINE, USER, WORLD). Use an unmistakable short
   identity sentence as the test marker. The page must report a backend failure
   if initialization fails, never a successful local-only save.
3. Reload the page and verify the saved identity and authority revision.
4. Start a new chat session with a configured model. The model's system input
   and durable run prompt must include the saved identity. WORLD is intentionally
   not part of FrozenCore v2.
5. Edit the identity and save. A new chat session must use the new version; an
   already-started session retains its original frozen identity. Restarting the
   backend must preserve both the authority and prior session snapshots.
6. Two stale editors must not silently overwrite each other: the second save
   with an old revision fails, requiring reload. Errors and review decisions
   must come from the backend, not browser demo storage.

An existing deployment with an explicit governance profile that replaces the
default rules must explicitly permit the three owner persona write actions.
The patch does not overwrite deployment-specific policy.
