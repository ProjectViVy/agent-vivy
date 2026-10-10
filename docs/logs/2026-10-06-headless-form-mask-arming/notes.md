# Notes — headless form identity decision

## Why the identity lives in the generated artifact

The MASK-3 gate ("unsealed embedders stay dormant") was written when the only
identity source was the linker-embedded sealed manifest. That made the dev
binary a second-class form: same compiled mask module, same action sets in
`BuildDefault()`, but every `module.action.invoke` answered
"module action capability is not configured" because the composition could
not prove an identity. The user's design principle is that masks are
species-inherent — every form carries them — and the dev binary is one form
of the headless species, not a lesser embedder.

Options considered:

1. **Runtime fallback constant** (a declared ID injected in `runtimeGenerationID`
   when no manifest exists): rejected. It cannot distinguish "the headless
   default composition" from "a custom test embedder Assembly" without extra
   machinery, would arm admission for minimal embedders whose backends lack
   `RunAdmissionStore` (hard startup failure), and puts the identity outside
   any artifact.
2. **Content-derived dev identity** (hash of the assembly): rejected. Resume
   is same-Generation-only and fail-closed (`internal/runtime/service.go:2226`);
   every rebuild touching the composition would strand pending
   approvals/questions of existing dev sessions.
3. **Declared identity in the generated artifact** (chosen): the identity is
   part of what the compiler emitted, stable across rebuilds, and consumed by
   the existing precedence chain (`runtimeGenerationID`: assembly first,
   manifest second). Pack replaces `zz_default.go` wholesale via build overlay
   and never passes `WithFormIdentity`, so sealed builds keep their
   manifest-derived identity — there is no leak path.

## Fixed constant, not content hash

Within any binary's lifetime the embedded catalog is frozen, so
identity↔content is 1:1 exactly as in a sealed build. Across dev rebuilds the
snapshot's full rendered instruction is reused verbatim (immutability by
design), so a changed catalog does not corrupt old sessions; it only means an
old session keeps the mask body it was admitted with — the same semantics as
opening a session across a release boundary.

## `sealed` split

The combined bool `sealed := presentation.SealedGeneration || generationID != ""`
feeds only the two capability seams (mask manager, primary admission); locale
and provenance read the `presentation.SealedGeneration` constant directly.
Giving the artifact an identity therefore arms capabilities without changing
provenance behavior — the split was already in place and is now explicit in
the comments.

## Provenance vs capability identity

`species/inspect` continues to report `builtin` for the dev binary. That label
describes the sealed-artifact provenance (manifest + UI artifact hash), which
the headless form does not claim. The declared `HeadlessGenerationID` is a
capability identity for admission/resume/action-host consistency. The two
answers are about different facts; conflating them would make dev inspect
lie about provenance.

## Trust model

A declared constant is an attestation, not a proof: nothing cryptographically
binds it to the compiled code the way the sealed manifest's SHA-256 identity
does. That is the intended tradeoff for a development form; any surface that
requires proof (embedded host SDK, `--inspect-generation`, pack verification)
still hard-fails without the manifest.
