# Plugin I18N Contract Freeze Design

> Status: Approved for implementation planning on 2026-09-09
>
> Scope: Normative plugin localization contract for PLG-P1, PLG-P6, and
> PLG-P9
>
> Delivery posture: Contract work only; this design does not schedule any
> functional plugin-platform phase

## 1. Purpose

PLG-P1 cannot implement strict `vivy.module/v1` descriptor parsing or sealed
Generation provenance while plugin localization remains a non-normative
proposal. This design freezes the descriptor, catalog, ownership, fallback,
hashing, diagnostics, and phase-responsibility rules needed to remove that
contract blocker without implementing the plugin runtime ahead of its
scheduled phase.

The design preserves the existing product decisions:

- English is the technical fallback locale.
- Plugin development guidance requires English and Chinese as the baseline.
- A valid third-party Module may still compile when Chinese is incomplete;
  that gap remains visible in build evidence and Inspect.
- The current runtime accepts only `en` and `zh` as active locales.
- Catalogs may package additional valid locales so later product expansion
  does not require a descriptor schema change.
- Web and TUI use one Host-owned localization surface and one backend-owned
  locale preference.

## 2. Chosen approach

Vivy will adopt a normative catalog contract now and implement it in the
existing plugin-platform phase order:

1. This contract-freeze delivery makes the descriptor, catalog, validation,
   provenance, and fallback semantics normative.
2. PLG-P1 implements parsing, validation, canonical hashing, compiler
   evidence, and Manifest projection.
3. PLG-P6 implements the shared Web/TUI Host resolver.
4. PLG-P9 proves whole-Generation conformance, removal, and rollback.

The rejected alternatives are a metadata-only descriptor extension that
would leave P1 guessing about catalog semantics, and an immediate full
runtime implementation that would bypass the P1/P2/P6 dependency order.

## 3. Descriptor boundary

A backend-only Module with no human-readable labels MAY omit `i18n`. A Module
that provides UI content or refers to a localization key MUST declare exactly
one catalog:

```yaml
i18n:
  catalog: i18n/catalog.json
  default_locale: en
  locales: [en, zh, ja]
```

The fields have these meanings:

- `catalog` is a Module-source-relative path to one shared catalog file. It
  MUST remain inside the selected Module source and MUST NOT be a URL,
  absolute path, symlink escape, or runtime-discovered path.
- `default_locale` MUST be `en` in v1. Retaining the explicit field keeps the
  descriptor extensible without weakening the current English fallback.
- `locales` declares the packaged locale set. It MUST include `en`, MUST have
  no duplicates after locale normalization, and MAY include valid future
  locales even though only `en` and `zh` are currently selectable.

Catalog selection is an explicit Recipe/package input. Runtime directory
scanning, network catalog downloads, and plugin-owned persisted locale state
are forbidden.

## 4. Shared catalog schema

The catalog is strict JSON with one semantic translation-unit map:

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

Each unit MUST contain:

- one stable fully qualified key;
- a non-empty translator-facing `description`;
- an explicit sorted set of named `placeholders`;
- a `messages` map containing a non-empty English message.

`short` and `long` are optional presentation variants of the same semantic
unit. Messages use the existing `{{name}}` interpolation form. Every present
locale and presentation variant MUST use exactly the declared placeholder set;
positional interpolation and executable message expressions are forbidden.
Catalog strings are plain UTF-8 text, not HTML or executable templates.

Unknown semantic fields and duplicate JSON keys are errors. This rule keeps
canonicalization deterministic and prevents a producer and consumer from
interpreting the same bytes differently.

## 5. Ownership and namespaces

Core VIVY continues to own `vivy.*`. A public Module owns exactly:

```text
plugin.<module-id>.*
```

The approved lowercase namespace-qualified Module ID is inserted literally.
For example, `example/search-tools` owns
`plugin.example/search-tools.*`. No slash-to-dot or other lossy rewriting is
allowed.

The catalog does not repeat its owner ID; ownership is derived from the
descriptor that selects it. A Module MUST NOT declare core keys, another
Module's keys, or an ambiguous normalized equivalent. A namespace violation
is an Assembly compile error and emits no formal Generation artifact.

Descriptor-based UI sends `label_key` and `label_args`, never pre-rendered
locale-specific labels. Full-code UI Modules call the same Host localization
surface. Localization creates no Grant, permission, or new authority boundary.

## 6. Resolution and fallback

The Host owns resolution. Plugins MUST NOT implement a second fallback policy
or persist an independent active locale.

For the base message, resolution is:

1. the active locale;
2. English;
3. a bounded visible diagnostic containing the unresolved key.

For `short` or `long`, resolution is:

1. the requested form in the active locale;
2. the base message in the active locale;
3. the requested form in English;
4. the base message in English;
5. the same bounded visible diagnostic.

Missing interpolation arguments remain visibly unexpanded as `{{name}}` and
produce a bounded diagnostic. The resolver does not crash, silently delete a
placeholder, interpret HTML, or treat plugin text as trusted backend input.

## 7. Validation states and limits

The compiler reports these evidence-derived states:

| Condition | Compiler result | Evidence / Inspect state |
|---|---|---|
| Backend-only Module omits `i18n` | Success | `NOT_APPLICABLE` |
| UI or `label_key` Module omits `i18n` | Failure | No formal artifact |
| Catalog has complete English and Chinese | Success | `COMPLETE` |
| English is complete and Chinese is missing or partial | Success | `INCOMPLETE_LOCALE` |
| A future packaged locale is partial | Success | That locale is `INCOMPLETE` |
| English is absent or partial | Failure | No formal artifact |
| Path, schema, namespace, duplicate-key, or placeholder violation | Failure | No formal artifact |

The engineering contract permits English fallback, while the developer
standard continues to require English and Chinese as the baseline for public
plugins. Documentation or descriptor claims cannot promote incomplete
evidence to `COMPLETE`.

Initial compiler limits are:

- 1 MiB raw bytes per catalog;
- 4,096 translation units per catalog;
- 8 KiB UTF-8 bytes per message or presentation variant;
- 32 named placeholders per translation unit.

Exceeding a limit is a compile error. Limits are checked before expensive
projection and are not configurable by an untrusted Module.

## 8. Canonical hashing and Generation identity

After strict validation, P1 serializes canonical catalog JSON by sorting
object keys, normalizing locale tags, sorting sets whose order has no semantic
meaning, and preserving translation text exactly after UTF-8 validation. It
then computes:

```text
catalog_digest = SHA-256(canonical_catalog_json)
```

The digest is an explicit Generation input and Manifest field. The Manifest
records catalog path, schema version, digest, default locale, packaged
locales, per-locale completeness, and evidence identifiers. It does not need
to embed the full translation body.

The existing Module source hash remains authoritative for exact source
provenance. Consequently, semantically equivalent catalog formatting produces
the same catalog digest, while exact source changes remain observable through
the Module source hash. A translation, placeholder, schema, or packaged-locale
change changes the catalog digest and therefore the Generation ID.

Runtime locale selection, workspace preference, Secret values, activation,
health, and user data remain outside Generation identity.

## 9. Phase responsibilities

### Contract-freeze delivery

- Promote the plugin I18N proposal to normative language in the four plugin
  architecture contracts.
- Align the plugin program plan, P1/P6/P9 tasks, core I18N design, and TODO
  status with this decision.
- Add no parser, resolver, SDK, generated code, or runtime behavior.

### PLG-P1

- Add the descriptor field and strict catalog schema.
- Implement path confinement, duplicate-key rejection, namespace validation,
  placeholder parity, resource limits, and canonical hashing.
- Emit Manifest completeness and evidence facts.
- Keep PLG-P1 and PLG-P2 as the existing clean-break landing unit.

### PLG-P6

- Provide one Host-owned `t(key, args, form)` behavior to Web and TUI.
- Consume the existing backend-authoritative active locale.
- Implement the exact fallback and bounded diagnostic rules above.
- Preserve the existing full-code UI trust model and server-side authority.

### PLG-P9

- Derive support claims from compiler and conformance evidence.
- Prove deterministic builds and invalid-input no-artifact behavior.
- Prove that removing a Module removes its catalog, imports, assets, Manifest
  entry, and runtime availability.
- Exercise rollback with catalog and Generation identities included.

PLG-P1 remains `UNSCHEDULED` after this contract freeze. Only the human owner
may schedule functional implementation.

## 10. Acceptance matrix

P1 fixtures and later conformance MUST cover at least:

- a complete English/Chinese catalog;
- missing and partially missing Chinese with English fallback evidence;
- a packaged future locale;
- stable canonical digest across non-semantic JSON ordering/formatting;
- digest change for translation, placeholder, or locale-set changes;
- core and cross-Module namespace violations;
- duplicate JSON keys and unknown semantic fields;
- placeholder declaration and message drift;
- absolute path, traversal, and symlink escape;
- every resource limit;
- missing English;
- missing Catalog for a UI or `label_key` Module;
- deterministic Manifest projection;
- failed compilation emitting no formal artifact;
- shared Web/TUI resolution and no independent locale persistence;
- Module removal and whole-Generation rollback.

The contract-freeze delivery itself runs repository document checks and
`just ci` under the normal repository gate. Any skipped executable slice must
be explained in its iteration verification log.

## 11. Out of scope

This design does not:

- schedule PLG-P1 through PLG-P9;
- implement a runtime plugin loader or runtime catalog discovery;
- add more active core locales beyond `en` and `zh`;
- introduce ICU MessageFormat, positional interpolation, HTML templates, or
  executable translation expressions;
- centralize plugin vocabulary in VIVY's core catalogs;
- grant plugins backend authority through localization;
- add v0 compatibility, migration, aliasing, or identity fallback.
