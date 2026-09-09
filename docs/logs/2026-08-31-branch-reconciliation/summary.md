# Branch recovery and mainline merge

## Changes

- Merged `feat/chatbox-buttons` into `main`, retaining the chatbox session operations and execution-mode changes.
- Merged `feat/skill-marketplace` into `main`, retaining the Skills backend directory/Marketplace RPC and UI.
- Merged `feat/cron-closed-loop` into `main`, retaining scheduled-task dispatch, storage, RPC, and UI.
- Merged `feat/channel-super-contract` into `main`, retaining its channel-plan documentation notes.
- Preserved both sides at merge conflicts: the dynamic RPC capability includes both Channel and Skills/Marketplace; SQLite migration016 includes both the message-origin column and the Cron table; the in-place PostgreSQL v14 upgrade fills in both sets of structures.

## Explicitly not done

- Did not merge `wip/pre-submodule-root-20260829`. It is an old parked snapshot containing many obsolete/Studio packaging differences and was not suitable for direct recovery.
- Did not delete any branches or push to the remote.
- The pre-existing uncommitted runtime tests, Studio submodule, failure files, and Studio logs at the repository root remain unchanged.
