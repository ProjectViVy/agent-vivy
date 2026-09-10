# Acceptance — Plugin platform P1/P2

1. `go run ./sdk verify <module-directory>` accepts all nine converted
   first-party Module sources and rejects a legacy Descriptor before graph
   construction.
2. `go run ./sdk pack --recipe recipes/default.vivy.yml --output <directory>`
   produces an executable `vivy`, generated binder, and content-addressed
   Generation Manifest; `inspect-artifact` verifies the manifest embedded in
   that exact executable rather than trusting a mutable sidecar.
3. Packing `recipes/minimal.vivy.yml` omits Channel imports, constructors,
   grants, and edges from the binder and produces a smaller linked binary than
   the default generation.
4. Starting the default app with no Channel envelopes starts zero Channels;
   all compiled Channels remain inspectable as unconfigured.
5. Functional source contains no v0 SDK import, Seam/God interface, registry,
   or legacy Descriptor file.
6. Generated code executes Module lifecycle in compiler order with rollback
   and reverse shutdown, and the application consumes the generated Tool,
   ToolWorld, Channel, and Face providers.
7. The build-owned Source Catalog enforces source ref and deterministic tree
   hash; verify and pack share the AST capability/import firewall and Go
   linkability checks.
8. Core Port ownership, provider identity/namespace uniqueness, protected Tool
   reservation, exact requirement identity, grant constraints/evidence, and
   selectable-Port evidence fail closed in compiler tests.
9. Packing is atomic, supports a real standalone `--source` Go Module, and
   seals dependency locks, UI hashes, compiled I18N catalogs, and capability
   states into the Generation identity.
10. ToolWorld filesystem access is workspace-contained; successful writes
    preserve the existing file-version history and stale-read seam, while LSP
    background processes shut down through Provider lifecycle.
11. Runtime Provider identities are checked against the sealed compiler plan;
    protected Tool Hosts cannot invoke a sibling implementation, and effective
    filesystem, process, secret, and network constraints remain attached to
    generated Host bindings and fail closed per operation.
12. The LSP Provider is instantiated once by generated code and that same
    instance supplies ToolWorld tools, write-diagnostic observation, and
    language-server status. `inspect-artifact` parses its linker-bound manifest
    without executing the target binary.
13. Every selected T2 Module is pinned by the Recipe, compiled from a snapshot,
    and rejected if its source tree or resolved dependency locks change during
    the build.
14. Channel HTTP and websocket connections cross the same granted Host network
    boundary; a Module cannot replace that transport with an ungoverned proxy
    or dialer.
15. The executable conformance suite exercises each promoted public Port
    through its production Host consumer and covers graph, lifecycle, failure,
    redaction, provenance, and default-behavior evidence.
