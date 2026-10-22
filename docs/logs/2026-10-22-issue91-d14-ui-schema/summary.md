# D14 frontend and schema residue corrections

Workflow publication now displays its existing owner revision. PublishView and RevisionView reflect the actual RPC program_id instead of definition/catalog digest fields; the obsolete display helper and translation placeholder are removed. Persona document/review DTOs and fixtures match pinned Laputa persona.Document and persona.ChangeRequest: existing revision CAS remains, and expected_document carries exact existence/content or null historical unavailability.

Removed the obsolete approval digest failure enum, session-search mock/DTO hash, retired cross-face hash classifications, and stale approvals.precondition / vivy.tui.gate.base classifications. Classified the actual replacement content-conflict/program labels and existing narrow-search recovery copy so the contract agrees with current English/Chinese sources.

The deliverables.presented closed schema now accepts the real domain metadata without sha256 and rejects the retired field. A regression serializes the current domain DeliverySet and Deliverable values against that schema.

SDK stage-ui regenerated the independent Linux UI and passed full TypeScript checking. Its tracked generated delta is one UI_ASSEMBLY_MANIFEST line: plugin.vivy/workflow-ui.editor.published has only the revision placeholder and revision-only English/Chinese text. The integration lane owns the shared generated assembly and will regenerate it; no assembly output was hand-edited or copied into this source commit.

Scope is the assigned frontend/plugin/schema consumers and their focused tests. No backend runtime behavior, database migration, orchestration, shared RPC, production Journal or release artifact was changed. This is an implementation correction, not the independent final audit or whole-product acceptance.
