# CH-P0-2 acceptance

## Observable result

The default product still exposes the same five Channel Providers and the same
management RPC behavior, but backend ownership is now replaceable at the
generated Assembly seam. App and RPC core compose only contracts; the canonical
Module owns all Channel-specific backend work.

## Acceptance map

| Requirement | Evidence | Result |
|---|---|---|
| CH-03 one composition seam | generated `ChannelFactory`; app stores only `channelcontract.Owned`; Assembly presence validation | PASS |
| CH-04 one existing Run path | focused dependency callback enters `Service.RunWithOptions`; owner is the existing Run hook and deliverer | PASS |
| CH-05 typed management contribution | Module supplies three bindings; generic dispatcher validates once and dispatches exact methods | PASS |
| CH-06 honest absence | core has no Channel switch/capabilities; no contribution produces standard `MethodNotFound` | PASS |
| CH-07 distinct truth | owner inspection and `ProcessAvailable`; `WithoutEars` constructs but does not start | PASS |
| CH-08 shared settings authority | atomic shared-document update preserves unrelated fields and maps validation/frozen/read-only policy | PASS |
| CH-11 lifecycle safety | inert construct, start/ready after recovery, failed-start Stop/Close, idempotent owner and app shutdown | PASS |
| CH-12 stable default | five Providers and existing management wire tests; full backend, UI, i18n, headless and plugin gates | PASS |
| ownership direction | forbidden imports absent in app/RPC; SDK evidence anchors point to Module binding | PASS |
| slice fence | no reduced Recipe or Channel UI/generated-UI diff | PASS |

## Human review checklist

- Confirm `internal/modules/channel` is the only package that imports the
  private Host while owning Provider binding and management handlers.
- Confirm generated Assembly supplies one canonical factory and does not also
  create the legacy placeholder lifecycle owner.
- Confirm app shutdown and rollback invoke only `Owned.Stop`/`Owned.Close`
  before storage teardown.
- Confirm RPC core contains no `channel/*` switch case or hard-coded
  `channel.*` capability.
- Confirm no document claims executable or asset omission.

CH-P0-2 acceptance does not imply CH-P0-3, CH-P0-4, or CH-P0-5 completion.
