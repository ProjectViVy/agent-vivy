# S0 — Dependency Truth

**Goal:** replace every dependency claim in this program with a measurement,
before any code is written.

**Status:** `DONE 2026-10-17`. §1–§6 are the method as reviewed; the result is
§7. Raw output is in `docs/logs/2026-10-17-provider-agentic-spike/notes.md`.

**Depends on:** nothing. **Blocks:** S1 (a component contract measured against an
unmeasurable dependency set is not decision-complete).

---

## 1. Why this stage exists

The 2026-09-18 capability check recorded the migration's dependency cost as *"a
second OpenAI client in one binary"* and stopped there, leaving the trade-off to
be *"weighed when the migration is scheduled"*
(`docs/plans/provider-registry/EINO-CAPABILITY.md` §5.3). That is an argument,
not a measurement, and it is now load-bearing: it is one of the two reasons the
migration was deferred.

This stage converts it into numbers. It also settles a question the earlier check
never asked, because it never considered the Anthropic side: whether the target
set is an **addition** to the current dependency graph or a **replacement** of
part of it.

## 2. Method

All steps run inside a throwaway module. **This repository's `go.mod` and
`go.sum` are not touched** (standing order; `README.md` decision D8), and nothing
is written under `internal/`.

1. **Pin the comparison baseline.** Record the exact repository commit, Go
   toolchain, target OS/architecture, build tags and flags. The reviewed source
   baseline is `a5f1f90d83137f4d166fc12c9c4b54dc917243b7`, parent of the
   documentation commit `08e31d86`; re-inventory if the migration base changes.

2. **Make like-for-like throwaway module copies outside the repository.** Copy
   the baseline module constraints (`go.mod`/`go.sum` and relevant local-module
   replacements) into isolated baseline and candidate probes. Keep unrelated
   application constraints identical. In the candidate only, apply the proposed
   Eino/component replacements; record any additional version selected by Go.
   Do not compare a fresh three-component module with the entire application.

3. **Module-graph delta.** Capture `go list -m all` for both isolated probes.
   Classify changes as added, removed or version-bumped, with each attributable
   to the proposed replacements. Do not count unrelated application modules as
   removals. Distinguish direct requirements, selected transitive modules and
   packages actually linked. A removal is conditional until all application
   consumers (including the titler) are accounted for in S3; any unresolved
   application constraint or pin interaction is explicitly unknown/blocking.

4. **Component requires, verbatim.** Copy the `require` blocks of
   `agenticopenai@v0.2.4/go.mod` and `agenticclaude@v0.1.7/go.mod` and of
   `eino@v0.9.21/go.mod` into this document, so a later reader does not have to
   re-open the cache.

5. **Binary-size probe.** For each target, a throwaway `main` that imports the
   component and constructs a model, plus a baseline importing only `eino`, plus
   a probe of the **replacement set** (`agenticopenai` + `agenticclaude` in one
   binary) and of the **current set** (`components/model/openai` +
   `components/model/claude` in one binary). Build each with:

   ```powershell
   go build -ldflags="-s -w" -o spike.exe .
   ```

   Use the same toolchain, platform, flags and exercised calls for both probes;
   include all three proposed bindings, including Responses. Record bytes and
   differences as **measured component-probe values**, not the product binary
   delta. Link reachability and shared application dependencies can change the
   result. The real migration must build comparable full product binaries to
   establish its actual net cost (S3 §2.7); do not claim that result here.

6. **Pin-legality check.** Confirm that the pseudo-version
   `libs/acl/openai v0.1.18-0.20260924031408-10df7cac3f6d` that
   `agenticopenai@v0.2.4/go.mod:9` requires is admissible under this
   repository's pin policy, which already accepts `v0.0.0-<timestamp>-<hash>`
   pins for first-party modules (`go.mod:7,18,19,23`, plus `go.mod:23` for a
   third-party module). Cite the precedent; do not restate the policy.

## 3. Raw material (known before the stage runs)

**Pins today** — repository `go.mod`:

| Module | Version | Line |
|---|---|---|
| `github.com/cloudwego/eino` | `v0.9.20` | `:14` |
| `eino-ext/components/model/openai` | `v0.1.13` | `:16` |
| `eino-ext/components/model/claude` | `v0.1.25` | `:15` |
| `eino-ext/libs/acl/openai` | `v0.1.17` (indirect) | `:174` |
| `github.com/meguminnnnnnnnn/go-openai` | `v0.1.2` (indirect) | `:191` |

**Targets**, present in the module cache and the module download cache:
`eino@v0.9.21`, `agenticopenai@v0.2.4`, `agenticclaude@v0.1.7`. Their `go.mod`
files require `eino v0.9.21` in both cases
(`agenticopenai@v0.2.4/go.mod:8`, `agenticclaude@v0.1.7/go.mod:10`).

**Already present in this repository**, which is why the Anthropic side is
predicted to be near-zero and must still be confirmed:

- `github.com/anthropics/anthropic-sdk-go v1.56.0` — `go.mod:41`, exactly the
  version `agenticclaude@v0.1.7/go.mod:6` requires.
- The AWS SDK block — `go.mod:42-55`, whose members match
  `agenticclaude@v0.1.7/go.mod:7-8,20-31`.
- `cloud.google.com/go/auth*` and `google.golang.org/api v0.189.0` —
  `go.mod:34-36,135`, matching `agenticclaude@v0.1.7/go.mod:17-19,81`.

These are already in the graph because `eino-ext/components/model/claude v0.1.25`
brings them. If S0 confirms this, the Anthropic migration's dependency delta is
a **replacement with no net addition** — a materially different story from the
2026-09-18 record.

**Known additions on the OpenAI side**:
`github.com/openai/openai-go/v3 v3.44.0` (`agenticopenai@v0.2.4/go.mod:12`) and
the pseudo-versioned `libs/acl/openai` (`:9`). Whether the current stack's
components can be removed in the same change determines whether the second
OpenAI client is an addition or a substitution.

## 4. The four questions S0 must answer

Answer each with a number, a table, or the explicit word **unknown**. "Probably
fine" is not an answer; it is the state this stage exists to eliminate.

| # | Question | Why it is load-bearing |
|---|---|---|
| **Q1** | Is the module-graph delta a **replacement** or an **addition**, and of exactly which modules? | Decides whether the "second OpenAI client" objection survives |
| **Q2** | Does dropping `components/model/openai` and `components/model/claude` remove `github.com/meguminnnnnnnnn/go-openai`, or only bump it? | `agenticopenai@v0.2.4/go.mod:37` still carries it at `v0.1.6` indirect, against this repository's `v0.1.2` (`go.mod:191`). If it stays, it is a bump, not a removal, and the module graph grows either way |
| **Q3** | What is the measured component-probe delta for the full three-adapter set, and what remains unknown about the full product binary? | A probe informs the decision but does not establish the shipped executable's net cost |
| **Q4** | Does any transitive pin **conflict** with this repository's existing pins? | A conflict is a scheduling blocker, not a detail |

## 5. Exit evidence

- The four answers in §4, filled in, with the raw command output appended below
  them.
- The verbatim `require` blocks from step 4.
- The binary-size table from step 5 with every figure labelled *measured* or
  *estimated*.
- The pin-legality citation from step 6.
- A one-line statement of which answers, if any, came back **unknown**, with the
  reason.

## 6. Explicit non-goals

- Do not add any of the three targets to this repository's `go.mod` or `go.sum`.
- Probe programs may reference constructors for comparable linking, but do not
  run model requests. Behavioral execution belongs to S2.
- Do not write files under `internal/`, and do not commit the throwaway module.

## 7. Result (2026-10-17)

### 7.1 Baselines

Two baselines were measured, because the plan's baseline is not on `main`:

| Label | Commit | Eino core | Note |
|---|---|---|---|
| **M** (primary) | `e7dcf85` upstream `main` | `v0.9.13` | The base a migration would actually start from |
| **B** (plan) | `a5f1f90` | `v0.9.20` | Plan's cited baseline; sits on 17 unmerged issue-58 commits, which carry the v0.9.13 → v0.9.20 bump |

§3's "pins today" table (`eino v0.9.20`) is true only for **B**. On `main` the
core pin is `v0.9.13` (`go.mod:14`), held there by the owner decision
`EINO-CORE-UPGRADE` (`docs/DEFER.MD:30`). Every other row of §3 matches `main`.

Platform: linux/amd64, Go 1.26.4, `-ldflags="-s -w"`. The plan's command is
written for Windows (`spike.exe`); figures below are linux/amd64 only.

### 7.2 Method as run

For each baseline, `git archive` of the commit plus the matching sibling
`../laputa` checkout (`go.mod:265-269` replaces it locally), all inside the
session scratchpad. Three copies per baseline:

- **ctl** — `go mod tidy -e` only. The repository `go.mod` is not tidy (local
  plugin replaces are unused by the root module), so the tidied copy, not the raw
  file, is the like-for-like reference.
- **add** — ctl + a blank-import file in `internal/provider` for `agenticopenai`
  and `agenticclaude` + `go get eino@v0.9.21 agenticopenai@v0.2.4
  agenticclaude@v0.1.7` + tidy. Both generations present.
- **rep** — add + the five `components/model/{openai,claude}` imports removed
  (`internal/provider/{openai,claude,resolving}.go`,
  `internal/runtime/{cachewarmer,model_stream_observer}.go`) + tidy. Approximates
  the replacement graph; the copy does not compile and is used for `go mod`
  only.

### 7.3 Q1 — replacement or addition?

**Both baselines give the identical delta** (ctl → rep):

| Class | Modules |
|---|---|
| **Added (10)** | `agenticopenai v0.2.4`, `agenticclaude v0.1.7`, `openai/openai-go/v3 v3.44.0`, `Azure/azure-sdk-for-go/sdk/azcore v1.17.0`, `.../azidentity v1.7.0`, `.../sdk/internal v1.10.0`, `AzureAD/microsoft-authentication-library-for-go v1.2.2`, `golang-jwt/jwt/v5 v5.2.1`, `kylelemons/godebug v1.1.0`, `pkg/browser v0.0.0-20240102092130-5ac0b6a4141c` |
| **Removed (2)** | `eino-ext/components/model/openai v0.1.13`, `eino-ext/components/model/claude v0.1.25` |
| **Bumped (5)** | `eino v0.9.13→v0.9.21` (B: `v0.9.20→v0.9.21`), `eino-ext/libs/acl/openai v0.1.17→v0.1.18-0.20260924031408-10df7cac3f6d`, `meguminnnnnnnnn/go-openai v0.1.2→v0.1.6`, `go-viper/mapstructure/v2 v2.4.0→v2.5.0`, `bytedance/mockey v1.3.0→v1.4.6` |

The "add" variant differs from "rep" only by keeping the two removed modules.

Attribution (`go mod why -m` in rep): everything added comes in through
`agenticopenai` → `openai-go/v3` → `openai-go/v3/azure`. `azidentity` and its
three dependents are reached only from `openai-go/v3/azure.test` (not linked).
`azcore` and `sdk/internal` **are** linked (`go list -deps ./internal/provider`).
`mapstructure` is bumped by `agenticclaude`; `mockey` only by
`agenticopenai.test`.

**Answer:** an **addition**, not a replacement. The "second OpenAI client"
objection survives: `openai-go/v3` (plus `azcore`) joins the binary, and the
existing `go-openai` fork stays. The Anthropic side is a clean substitution
(nothing added for `agenticclaude`, as §3 predicted).

### 7.4 Q2 — does `go-openai` go away?

**No — it is bumped `v0.1.2 → v0.1.6`.** `agenticopenai`'s chat-completions
model is built on `libs/acl/openai` (`chat_model.go:25,178`), which still uses
`go-openai`. Dropping the current components does not remove it.

### 7.5 Q3 — binary cost

All values **measured**, linux/amd64, stripped.

| Probe | Bytes | Δ |
|---|---|---|
| `eino` only (typed runner reference) | 11,641,097 | — |
| **current set** (`components/model/openai` + `claude`, two constructors) | 34,009,353 | — |
| **replacement set** (`agenticopenai` chat + responses + `agenticclaude`, three constructors) | 39,194,889 | **+5,185,536 (+15.2 %) vs current** |
| product `cmd/vivy`, M ctl | 103,182,601 | — |
| product `cmd/vivy`, M add (both generations, agentic packages only blank-imported) | 105,885,961 | +2,703,360 (+2.6 %) |
| product `cmd/vivy`, B ctl / B add | 103,616,777 / 106,258,697 | +2,641,920 (+2.5 %) |

The probe set was built in one module at eino v0.9.21 for both sides, so the
current-set figure uses the newer helper libraries; it is a component
comparison, not a product number.

**Why the cost (owner question, 2026-10-18).** Almost all of it is
`openai-go/v3 v3.44.0`, the official generated OpenAI SDK that
`agenticopenai` uses for the Responses API: 134,370 non-test lines, of which
`responses/` is 27,785. `responses/*.go` imports the SDK's root `openai`
package (83,698 lines: the whole client with 22 services), so using Responses
alone links most of the SDK. Its types use reflection-based JSON and many
interface methods, which the Go linker cannot drop once a type is reachable.
`openai-go/v3/azure` (imported by `agenticopenai`) also links `azcore`. For
comparison, the current `components/model/openai v0.1.13` is 433 lines on top
of the `go-openai` fork (~6,400 lines). No Vivy-side reduction exists short of
forking the SDK (D9). The product "add" row links the agentic
packages' init and reachable code only, not a migrated runtime. **The real
product delta after migration is unknown** and belongs to the migration gate
(S3 §2.7). A reasonable bracket from these numbers is +2.6 MB to +5.2 MB on a
~103 MB binary; that is an estimate, not a measurement.

### 7.6 Q4 — pin conflicts?

**None found.** On both baselines the **add** copy (`eino v0.9.21` plus every
bump in §7.3, existing code unchanged) passes `go build ./...`, and
`go test ./internal/provider/... ./internal/runtime/...` gives the same result
as the ctl copy. Both fail only `TestDeliverableReadSnapshotWriteFailure`, which
expects an unwritable directory and fails because the sandbox runs as root
(`deliverable_service_test.go:562`); it is environmental and fails identically
without the bump.

This shows the bump compiles and keeps provider/runtime unit behavior. It
does **not** lift `EINO-CORE-UPGRADE`: that deferral is about checkpoint
compatibility, which unit tests do not cover (S3 R13, S4 blocker B1).

### 7.7 Pin legality (step 6)

`libs/acl/openai v0.1.18-0.20260924031408-10df7cac3f6d` is a standard Go
pseudo-version. Precedent for pseudo-version pins already exists on `main`:
`go.mod:7` (`ProjectViVy/inofy v0.0.0-20260930141905-71e2c9bbe47d`) and
`go.mod:18-19` (`laputa/*`). The plan's `go.mod:23` citation is off on `main`;
`:7,18,19` are the current lines. Admissible.

### 7.8 Verbatim component requires (step 4, direct block only)

```text
agenticopenai@v0.2.4/go.mod
	github.com/bytedance/mockey v1.4.6
	github.com/bytedance/sonic v1.15.0
	github.com/cloudwego/eino v0.9.21
	github.com/cloudwego/eino-ext/libs/acl/openai v0.1.18-0.20260924031408-10df7cac3f6d
	github.com/eino-contrib/jsonschema v1.0.3
	github.com/go-viper/mapstructure/v2 v2.5.0
	github.com/openai/openai-go/v3 v3.44.0
	github.com/smartystreets/goconvey v1.8.1
	github.com/stretchr/testify v1.11.1
	github.com/wk8/go-ordered-map/v2 v2.1.8
	golang.org/x/sync v0.10.0

agenticclaude@v0.1.7/go.mod
	github.com/anthropics/anthropic-sdk-go v1.56.0
	github.com/aws/aws-sdk-go-v2/config v1.27.27
	github.com/aws/aws-sdk-go-v2/credentials v1.17.27
	github.com/bytedance/sonic v1.15.0
	github.com/cloudwego/eino v0.9.21
	github.com/eino-contrib/jsonschema v1.0.3
	github.com/go-viper/mapstructure/v2 v2.5.0
	github.com/wk8/go-ordered-map/v2 v2.1.8

eino@v0.9.21/go.mod
	github.com/bmatcuk/doublestar/v4 v4.10.0
	github.com/bytedance/sonic v1.15.0
	github.com/eino-contrib/jsonschema v1.0.3
	github.com/google/uuid v1.6.0
	github.com/nikolalohinski/gonja v1.5.3
	github.com/slongfield/pyfmt v0.0.0-20220222012616-ea85ff4c361f
	github.com/smartystreets/goconvey v1.8.1
	github.com/stretchr/testify v1.10.0
	github.com/wk8/go-ordered-map/v2 v2.1.8
	go.uber.org/mock v0.4.0
	gopkg.in/yaml.v3 v3.0.1
```

### 7.9 Unknowns

One: the **full product binary delta after migration** (Q3). It needs the
migrated runtime to exist; it is assigned to S3 §2.7 and listed as an S4
conditional blocker. Windows figures were not measured (linux-only sandbox).
