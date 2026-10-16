# S0 — Dependency Truth

**Goal:** replace every dependency claim in this program with a measurement,
before any code is written.

**Status:** `UNSCHEDULED`. This document specifies the measurement; **it does not
contain the result.** The numbers below marked *known* are the raw material the
stage starts from (read from this repository and from the module cache); the four
answers in §4 are what the stage must produce.

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

```powershell
# 1. Throwaway module, outside the repository tree.
$spike = Join-Path $env:TEMP "vivy-agentic-spike"
New-Item -ItemType Directory -Force -Path $spike | Out-Null
Set-Location $spike
go mod init spike

# 2. Bring in the three targets.
go get github.com/cloudwego/eino@v0.9.21
go get github.com/cloudwego/eino-ext/components/model/agenticopenai@v0.2.4
go get github.com/cloudwego/eino-ext/components/model/agenticclaude@v0.1.7
```

3. **Module-graph delta.** Record `go list -m all` in the throwaway module and in
   the repository root, then classify every differing line into exactly one of:
   *added*, *removed*, *version-bumped*. A line that appears in both at different
   versions is a version bump, not two lines.

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

   Record bytes for each pair and the difference. Label every figure *measured*
   or *estimated* — an unlabelled number in this file is a defect.

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
| **Q3** | What is the **net binary-size delta** for the full three-adapter set, measured? | Binary size is the user-visible cost of the migration on a product that ships one executable |
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
- Do not construct or call a model. That is S2's job; this stage measures
  dependencies, not behaviour.
- Do not write files under `internal/`, and do not commit the throwaway module.
