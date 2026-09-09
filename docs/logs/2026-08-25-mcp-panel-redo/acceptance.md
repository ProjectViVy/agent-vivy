# Acceptance

Open “Tools Management → MCP” at `http://127.0.0.1:3015`:

- The page uses the same visual system as the rest of the site: white-background
  cards, standard buttons/badges/switches, and no pink theme, hearts, or gradient decoration.
- Each list row makes the object immediately recognizable: name + transport badge +
  connection target (command or URL) + tool count; the switch is the enabled
  state, with no duplicate status text.
- Every available action produces a real result:
  - switching a toggle persists immediately and updates the header count;
  - “Add Service” requires a connection target matching the transport, shows errors
    in the dialog, and preserves the input;
  - Edit can change the name/transport/target; Delete requires confirmation and
    clearly warns that it cannot be undone;
  - JSON import/export round-trips commands and URLs (including the nested
    `tools.mcpServers` structure).
- All states are covered: loading skeleton, empty list (with import/add entry
  points), filtered empty state (Clear Filters), and row-level busy locking.
- Compared with the old version, the pink gradient page, floating hearts, four
  statistic cards, raw `connected` status code, and incomplete add-only form are gone.
