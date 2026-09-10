# Plugin I18N Normative Contract Freeze Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Promote the approved plugin localization design into Vivy's normative architecture and executable phase plans without implementing or scheduling plugin runtime code.

**Architecture:** The four canonical plugin contracts define one optional Module-owned shared catalog, compiler-owned validation and provenance, and one Host-owned Web/TUI resolver. The program README and PLG-P1/PLG-P6/PLG-P9 plans then assign implementation and conformance work to the existing dependency order while all functional phases remain `UNSCHEDULED`.

**Tech Stack:** Markdown, Git, ripgrep consistency checks, existing Node preflight checks, and `just ci`.

**Spec:** `docs/superpowers/specs/2026-09-09-plugin-i18n-contract-freeze-design.md`

## Global Constraints

- Documentation changes only: no Go, TypeScript, generated code, JSON Schema artifact, fixture payload, runtime behavior, or active-locale expansion.
- The current runtime continues to accept only `en` and `zh` as active locales.
- Plugin development guidance requires English and Chinese as the baseline; valid English-complete third-party catalogs may compile with visible `INCOMPLETE_LOCALE` evidence when Chinese is partial or absent.
- `default_locale` is `en` in v1; catalogs may package additional valid locales.
- A UI or `label_key` Module declares one explicit source-confined shared catalog; runtime discovery and download are forbidden.
- Core owns `vivy.*`; a Module with ID `<module-id>` owns literal namespace `plugin.<module-id>.*` with no lossy rewriting.
- Messages use named `{{name}}` placeholders. Every present locale and form has exact placeholder parity with the unit declaration.
- Initial limits are 1 MiB raw catalog bytes, 4,096 units, 8 KiB UTF-8 bytes per message/form, and 32 placeholders per unit.
- The canonical catalog SHA-256 digest is a sealed Generation input; runtime locale and workspace preference are not.
- P1 implements descriptor parsing, validation, hashing, evidence, and Manifest projection; P6 implements shared Host resolution; P9 proves conformance, removal, and rollback.
- PLG-P1 through PLG-P9 remain `UNSCHEDULED`; this plan only clears the accepted-I18N-contract dependency.
- Preserve exactly 14 public Ports. Localization is a Host surface available through selected UI/Face composition, not a fifteenth public Port or a Grant.
- All human-readable documentation remains English.

---

### Task 1: Promote the approved design into the four normative contracts

**Files:**

- Modify: `docs/architecture/VIVY-MODULE-STANDARD.md` sections 3 and 12
- Modify: `docs/architecture/VIVY-PORT-CATALOG.md` sections 2, 11, and 14
- Modify: `docs/architecture/VIVY-PLUGIN-SPEC.md` sections 3, 7.1, 9, 10, and 12
- Modify: `docs/architecture/VIVY-ASSEMBLY.md` sections 3, 6–8, 11–12, and 15

**Interfaces:**

- Consumes: the approved descriptor, catalog, namespace, fallback, limit,
  digest, and evidence rules from the design spec.
- Produces: one normative localization contract used by P1 parsing, P6 Host
  resolution, P9 conformance, and all later plugin documentation.

- [ ] **Step 1: Record the failing proposal-state baseline**

  Run:

  ```bash
  rg -n "Proposed plugin I18N|non-normative|proposed follow-up contract" \
    docs/architecture/VIVY-MODULE-STANDARD.md \
    docs/architecture/VIVY-PORT-CATALOG.md \
    docs/architecture/VIVY-PLUGIN-SPEC.md \
    docs/architecture/VIVY-ASSEMBLY.md
  ```

  Expected: at least one match in `VIVY-PLUGIN-SPEC.md` section 7.1, proving
  the approved contract is not yet normative.

- [ ] **Step 2: Extend the canonical Module Descriptor**

  In `VIVY-MODULE-STANDARD.md`, add the optional canonical shape:

  ```yaml
  i18n:
    catalog: i18n/catalog.json
    default_locale: en
    locales: [en, zh, ja]
  ```

  State that backend-only Modules without human-readable keys may omit the
  block, while UI or `label_key` Modules require it. Define source-confined
  path semantics, English completeness, packaged-locale normalization,
  development baseline, ownership, and the no-side-effect rule. Add a review
  question that rejects a UI key without a catalog or a catalog with an
  owner-derived namespace mismatch.

- [ ] **Step 3: Define localization as a Host surface without a new Port**

  In `VIVY-PORT-CATALOG.md`, keep the public Port table at exactly 14 rows.
  Under full-code UI, specify that PresentationHost exposes a read-only
  localization function with this semantic signature:

  ```text
  t(key, args, form) -> localized plain UTF-8 text
  ```

  `form` is empty, `short`, or `long`; Web and TUI consume the same selected
  catalog units and fallback contract. The surface creates no Grant, route,
  registry, permission prompt, runtime discovery, or backend authority.
  Record localization discovery/registration as closed to public extension
  outside the selected Module catalog.

- [ ] **Step 4: Replace the proposed Plugin Spec section with the normative contract**

  Rename section 7.1 to `Plugin localization contract` and remove proposal
  qualifiers. Include the strict single-file JSON shape:

  ```json
  {
    "apiVersion": "vivy.i18n/v1",
    "units": {
      "plugin.example/search-tools.results.count": {
        "description": "Number of search results",
        "placeholders": ["count"],
        "messages": {
          "en": "{{count}} results",
          "zh": "{{count}} 个结果",
          "ja": "{{count}} 件の結果"
        },
        "short": {
          "en": "{{count}} results",
          "zh": "{{count}} 项"
        }
      }
    }
  }
  ```

  Make `description`, `placeholders`, and non-empty English `messages` values
  required. Define optional `short`/`long`, strict unknown-field and duplicate
  JSON-key rejection, literal owner namespace, resolution order, visible
  diagnostics, resource limits, compilation states, and no-artifact failure.
  Update verify/pack/inspect, failure, and completion sections so this contract
  is part of v1 acceptance rather than an optional follow-up.

- [ ] **Step 5: Add catalog processing to Assembly gates and identity**

  In `VIVY-ASSEMBLY.md`, assign:

  - G0: strict JSON parsing, duplicate-key rejection, locale normalization,
    resource bounds, and canonical serialization;
  - G1: source-confined path resolution and literal Module namespace ownership;
  - G4: English completeness, locale/form placeholder parity, Web/TUI selected
    input projection, and deterministic evidence;
  - G5: `SHA-256(canonical_catalog_json)`, Manifest facts, and sealed identity.

  Add catalog schema version, path, digest, default locale, packaged locales,
  per-locale completeness, and evidence IDs to Manifest/Inspect. State that a
  changed translation, placeholder declaration, schema, or packaged-locale set
  changes the digest and Generation ID. Extend removal and rollback so omitted
  Modules leave no catalog, generated projection, asset, or Manifest record.

- [ ] **Step 6: Verify the normative corpus has one contract**

  Run:

  ```bash
  test "$(rg -n '^\| `std/' docs/architecture/VIVY-PORT-CATALOG.md | wc -l)" -eq 14
  ! rg -n "Proposed plugin I18N|non-normative|proposed follow-up contract" \
    docs/architecture/VIVY-MODULE-STANDARD.md \
    docs/architecture/VIVY-PORT-CATALOG.md \
    docs/architecture/VIVY-PLUGIN-SPEC.md \
    docs/architecture/VIVY-ASSEMBLY.md
  rg -n "vivy\.i18n/v1|INCOMPLETE_LOCALE|canonical_catalog_json|plugin\.<module-id>" \
    docs/architecture/VIVY-MODULE-STANDARD.md \
    docs/architecture/VIVY-PORT-CATALOG.md \
    docs/architecture/VIVY-PLUGIN-SPEC.md \
    docs/architecture/VIVY-ASSEMBLY.md
  ```

  Expected: the Port count assertion exits 0, the proposal scan finds no
  matches, and every contract term is owned by at least one canonical document
  without contradicting another.

---

### Task 2: Align the executable phase plans and project status

**Files:**

- Modify: `docs/plans/plugin-platform/README.md` I18N section and P1 status
- Modify: `docs/plans/plugin-platform/PLG-P1-v1-sdk-and-compiler.md` constraints and Tasks 1, 2, and 5
- Modify: `docs/plans/plugin-platform/PLG-P6-full-ui-modules.md` constraints and UI Host tasks
- Modify: `docs/plans/plugin-platform/PLG-P9-release-conformance.md` Tasks 2–5 and 7
- Modify: `docs/superpowers/specs/2026-09-09-i18n-design.md` status, plugin contract, testing, delivery order, and final status
- Modify: `docs/TODO.md` section 0.1 `PLG-1` ruling, `TUI-CMD-I18N`, and `PLG-1` row

**Interfaces:**

- Consumes: the normative contract produced by Task 1.
- Produces: exact future implementation ownership and an accurate unscheduled
  project board with the I18N design blocker marked accepted.

- [ ] **Step 1: Record the failing cross-document status baseline**

  Run:

  ```bash
  rg -n "proposal|not yet scheduled|Do not schedule P1 until|plugin I18N remains a non-normative proposal|Task 8.*deferred" \
    docs/plans/plugin-platform/README.md \
    docs/plans/plugin-platform/PLG-P1-v1-sdk-and-compiler.md \
    docs/plans/plugin-platform/PLG-P6-full-ui-modules.md \
    docs/superpowers/specs/2026-09-09-i18n-design.md \
    docs/TODO.md
  ```

  Expected: matches show the pre-freeze proposal and dependency language that
  must be replaced. Historical research statements outside these files are
  not rewritten.

- [ ] **Step 2: Mark the I18N contract accepted without scheduling P1**

  In the program README, replace the cross-cutting proposal with an accepted
  normative-contract summary and link the approved design plus four canonical
  contracts. Keep the P1 state `UNSCHEDULED`, change its dependency evidence
  from `P0 + accepted I18N descriptor contract` to
  `P0 + I18N contract accepted 2026-09-09`, and state explicitly that only the
  dependency is satisfied; the human scheduling gate remains closed.

- [ ] **Step 3: Give P1 exact catalog compiler responsibilities**

  Remove the obsolete “do not schedule until accepted” constraint and replace
  it with the frozen rules. Extend Task 1's descriptor types with optional
  `I18N` metadata. Extend Task 2's strict parser work with these future files:

  ```text
  sdk/module/i18n.go
  sdk/module/i18n_test.go
  sdk/internal/i18n/catalog.go
  sdk/internal/i18n/catalog_test.go
  sdk/internal/testdata/plugin-v1/i18n/
  ```

  Name tests for missing English, missing Chinese evidence, future locales,
  duplicate keys, unknown fields, path traversal/symlink escape, namespace
  ownership, placeholder parity, all four limits, and stable canonical digest.
  Extend Task 5's Assembly Manifest work with catalog digest and completeness
  evidence. Do not mark any checkbox complete.

- [ ] **Step 4: Give P6 one Web/TUI Host resolver contract**

  Replace the proposal section with accepted constraints. Extend the FullUIHost
  contract with `t(key, args, form)` and assign future implementation/tests to:

  ```text
  sdk/ui/src/i18n.ts
  sdk/ui/src/i18n.test.ts
  ui/src/plugins/localization-host.ts
  ui/src/plugins/localization-host.test.ts
  sdk/tui/i18n/plugin_catalog.go
  sdk/tui/i18n/plugin_catalog_test.go
  internal/generated/assembly/i18n_catalogs.go
  ```

  Require active-locale then English then visible diagnostic fallback; for a
  requested form, prefer the same-locale base message before English. Missing
  args remain visible and generate bounded diagnostics. Assert that Web and
  TUI share selected units and no plugin persists locale state. Do not create
  a new public Port or permission surface.

- [ ] **Step 5: Extend P9 release evidence**

  Add catalog completeness to evidence-derived support state. Extend the
  failure matrix with invalid schema, owner namespace, duplicate key,
  placeholder mismatch, missing English, path escape, and resource limits.
  Extend default/minimal removal and rollback tests to compare catalog digest,
  generated projections, assets, Manifest facts, and runtime availability.
  Add a final Web/TUI conformance requirement for identical resolution and
  bounded diagnostics.

- [ ] **Step 6: Reconcile the core I18N design and living board**

  In the core I18N design, change plugin localization from non-normative
  proposal to accepted contract while retaining the truth that implementation
  is deferred to unscheduled P1/P6/P9. Do not claim current SDK or runtime
  support. In `docs/TODO.md`, record that the contract dependency is closed,
  catalog implementation remains future PLG work, core Web/TUI verification
  remains separately open, and PLG-P1–P9 are still unscheduled.

- [ ] **Step 7: Verify phase and board consistency**

  Run:

  ```bash
  ! rg -n "not yet scheduled|Do not schedule P1 until|plugin I18N remains a non-normative proposal" \
    docs/plans/plugin-platform/README.md \
    docs/plans/plugin-platform/PLG-P1-v1-sdk-and-compiler.md \
    docs/plans/plugin-platform/PLG-P6-full-ui-modules.md \
    docs/superpowers/specs/2026-09-09-i18n-design.md
  rg -n "UNSCHEDULED" \
    docs/plans/plugin-platform/README.md \
    docs/plans/plugin-platform/PLG-P1-v1-sdk-and-compiler.md \
    docs/plans/plugin-platform/PLG-P6-full-ui-modules.md \
    docs/plans/plugin-platform/PLG-P9-release-conformance.md \
    docs/TODO.md
  rg -n "PLG-P1|PLG-P6|PLG-P9|INCOMPLETE_LOCALE|vivy\.i18n/v1" \
    docs/plans/plugin-platform/README.md \
    docs/plans/plugin-platform/PLG-P1-v1-sdk-and-compiler.md \
    docs/plans/plugin-platform/PLG-P6-full-ui-modules.md \
    docs/plans/plugin-platform/PLG-P9-release-conformance.md \
    docs/superpowers/specs/2026-09-09-i18n-design.md \
    docs/TODO.md
  ```

  Expected: obsolete blocker language is absent, unscheduled states remain
  explicit, and responsibility terms appear in the correct phase documents.

---

### Task 3: Verify and deliver the contract-freeze update

**Files:**

- Create: `docs/logs/2026-09-09-plugin-i18n-contract-freeze/summary.md`
- Create: `docs/logs/2026-09-09-plugin-i18n-contract-freeze/verification.md`
- Create: `docs/logs/2026-09-09-plugin-i18n-contract-freeze/acceptance.md`
- Modify only if an unfixed finding remains: `docs/TODO.md` section 0.1

**Interfaces:**

- Consumes: Tasks 1–2 and the current green `main` baseline.
- Produces: one reviewable documentation-only commit with exact verification
  evidence and no false implementation or scheduling claim.

- [ ] **Step 1: Audit the diff scope**

  Run:

  ```bash
  git status --short
  git diff --name-only
  git diff --check
  ```

  Expected: only the files named by this plan plus the iteration log are
  changed; there are no Go, TypeScript, workflow, `justfile`, generated, or
  fixture changes and no whitespace errors.

- [ ] **Step 2: Run the plugin v1 preflight checks**

  Run:

  ```bash
  node scripts/check-plugin-v1-fixtures.mjs
  node --test scripts/check-plugin-v1-fixtures.test.mjs
  ```

  Expected: exit 0. These checks ensure the pre-existing P1 acceptance fixture
  index remains valid even though this contract-only delivery adds no fixture.

- [ ] **Step 3: Run the repository product gate**

  Run:

  ```bash
  just ci
  ```

  Expected: exit 0 for fmt-check, UI/I18N checks, vet, tests,
  headless-compile, and plugin-ci. If `just` is unavailable, use the now-split
  GitHub Actions jobs as remote evidence and record the local environment
  limitation; do not claim local `just ci` passed.

- [ ] **Step 4: Write the iteration record**

  `summary.md` lists the normative contracts and plan-routing changes and
  states that no functional code or phase scheduling was delivered.
  `verification.md` records every exact command, exit status, remote run URL
  when used, and any skipped local slice. `acceptance.md` lets a human verify
  the descriptor, schema, namespace, fallback, limit, digest, evidence,
  phase-owner, and unscheduled-state requirements.

- [ ] **Step 5: Re-run static checks after the log exists**

  Run:

  ```bash
  git diff --check
  ! rg -n "TBD|FIXME|fill in details|implement later" \
    docs/architecture/VIVY-MODULE-STANDARD.md \
    docs/architecture/VIVY-PORT-CATALOG.md \
    docs/architecture/VIVY-PLUGIN-SPEC.md \
    docs/architecture/VIVY-ASSEMBLY.md \
    docs/plans/plugin-platform/README.md \
    docs/plans/plugin-platform/PLG-P1-v1-sdk-and-compiler.md \
    docs/plans/plugin-platform/PLG-P6-full-ui-modules.md \
    docs/plans/plugin-platform/PLG-P9-release-conformance.md \
    docs/superpowers/specs/2026-09-09-i18n-design.md \
    docs/TODO.md \
    docs/logs/2026-09-09-plugin-i18n-contract-freeze
  ```

  Expected: exit 0 and no unresolved placeholder wording.

- [ ] **Step 6: Commit the single contract-freeze concern**

  Stage only the paths listed by this plan, review `git diff --cached --stat`,
  and commit:

  ```bash
  git commit -m "docs(plugin): freeze i18n contract"
  ```

- [ ] **Step 7: Verify the committed state**

  Run:

  ```bash
  git status --short --branch
  git show --stat --oneline HEAD
  git show --check HEAD
  ```

  Expected: clean worktree, one focused documentation commit, and no whitespace
  errors. Pushing requires explicit human authorization.

## Acceptance

A reviewer can trace the same descriptor, shared catalog, literal namespace,
fallback, resource limits, canonical digest, evidence states, and phase owners
from the design spec through all four normative contracts, the program README,
P1/P6/P9 plans, the core I18N design, and the living board. The public Port
count remains 14, no runtime code changes, PLG-P1 remains unscheduled, and all
available verification evidence is recorded without overstating local CI.
