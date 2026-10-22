# D14 custom-provider live readiness

The independent Windows acceptance checkout was fast-forwarded from `5b0b572d`
to `397b6f17`, which includes the custom-provider runtime repair. The stable
root was not merged or modified. An unmerged stable root was not the cause of
the manual failure: the acceptance server intentionally runs from this separate
checkout, and its previous executable lacked the repair.

The actual minimal-chat Recipe was packed and inspected. Its Generation is
`1c6fd4a5bf0a9ebfb3f4a0be06465186`, with eleven Modules. The isolated backend
now runs that package on `127.0.0.1:8787`, reusing only the disposable D14
configuration and home. The split Tiny UI at `3015` was regenerated with the
same Generation. The original package remains available for rollback.

A real Tiny UI `Hello` attempt now fails with the selected custom provider's
explicit `API key missing` error, rather than `no embedded vendor data`.
This proves the previous construction failure is removed, but is not a
successful gateway call or human D14 verdict. No provider key was read back,
entered, or saved during this follow-up.

The migration-rule question raised in chat was unnecessary. The governing
complete-deletion plan explicitly retires migration checksums and calls for
revising the corresponding AGENTS clauses. This follow-up changes no migration
rule or storage implementation.

There is no push, stable-root merge, publication, production database access,
or D14/issue-91 closure. Release notes are omitted because this is local manual
acceptance readiness, not a release.
