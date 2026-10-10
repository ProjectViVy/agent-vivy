# Acceptance

- Edit ordinary internal code/tests and first-party Module sources; run ordinary
  tests and SDK verify/stage/build without refreshing any authored digest.
- The SDK catalog resolves current first-party bytes, including an edited Module
  omitted by the minimal Recipe. A selected changed Module receives no historical
  passing conformance records.
- The live conformance suite still executes Provider/Host behavior and all
  registration, version, graph, Grant, failure, cancellation, lifecycle and
  provenance assertions. An external source mismatch remains a negative case.
- Explicit external Recipe pins, source confinement, artifact/binary binding,
  catalog/asset integrity and dependency locks continue rejecting tampering.
- `just release-conformance` is a separate opt-in exact-source check. It should
  reject this changed internal tree against the unchanged historical bundle.
  Development output is not a verified species publication.
- Use an isolated draft PR, retain #48 fixes, coordinate #49/#50 integration,
  and do not merge or deploy.
