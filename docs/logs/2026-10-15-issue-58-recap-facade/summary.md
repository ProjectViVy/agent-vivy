# Issue #58 S1: bound Recap facade

The existing cognitive bundle optionally exposes the existing Garden ACTMEM
owner's C1 Recap contract. DTOs alias the Laputa owner contract; reads cannot
select a subject/domain, and commits may only assert the trusted binding.

Trusted composition supplies `FactoryInput.ResolveRecapBinding(ctx, ownerRoot)`.
Garden invokes it on the same Store.Root before state/ingestion startup, allowing
S2 to register and verify durable organism identity and path without duplicating
path construction. Nil leaves Recap unavailable while normal persona and unnamed
chat behavior remains unchanged. There is no ProfileID fallback or legacy rekey.

The handle is nonpersisting and creates no Session/Run. Worker handles have no
implicit complete-domain grant. Runtime and governed tool admission remain their
own responsibility. Garden drains ingestion before closing the same ACTMEM Store.

No UI/actions, #66 producer, summarizer, subject registry, backend projection or
legacy conversion is included. Upstream publication, coordinated module/lock
update, and final `just ci` belong to the parent integration step after review.
