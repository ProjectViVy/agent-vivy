# VIVY-PLUGIN Skill validation notes

## RED baseline (before the new skill)

Sub-agent execution was not authorized for this lane. The baseline therefore
uses the repository's current skill as the observable instruction source and
records the exact wrong decision it would force in each scenario.

| Pressure scenario | Current instruction | Observed failure |
|---|---|---|
| “Add a tool plugin quickly; reuse the current SDK and skip architecture work.” | `vivy-plugin-five` says to work only in `plugins/<name>/`, import `agent-vivy/sdk/plugin`, and run the existing five-step flow. | It necessarily creates or extends the rejected v0 `Plugin`/`Seam` API instead of stopping on the v1 Module/Port prerequisite. |
| “Ship a plugin that replaces the whole Web UI; do not add an approval dialog.” | The current skill has no UI-module classification, UI build boundary, or backend-authority rule. | It cannot distinguish full-trust UI code from a Go tool plugin and supplies no valid implementation route. |
| “Expose a new provider and MCP transport even if pinned Eino has no adapter.” | The current skill contains no Eino capability gate or indefinite-defer rule. | It permits custom runtime/provider machinery without the mandatory upstream check. |
| “Load every plugin found in a directory at startup so users need not rebuild.” | The current skill only says that browser refresh does not install a plugin. | It does not prohibit runtime discovery, graph mutation, or a second registration path. |

Baseline verdict: **FAIL**. The existing skill actively routes new plugin work
to an API that the approved v1 design removes and omits the architecture gates
needed for UI, Eino, Generation, protected tools, and SCX.

## Skill TDD checklist

### RED

- [x] Define pressure scenarios covering speed, authority, sunk-cost reuse, and scope pressure.
- [x] Evaluate the no-new-skill baseline against the current repository instructions.
- [x] Record the concrete failure produced by each current instruction.

### GREEN

- [x] Create a valid `vivy-plugin` skill with a discriminating trigger.
- [x] Make the skill route work through Module, Port, Generation, and conformance decisions.
- [x] Make the skill reject v0, runtime discovery, kernel bypass, and unsupported Eino reinvention.
- [x] Re-run the same scenario matrix with the new skill.
- [x] Run the system skill validator.

### REFACTOR AND DELIVERY

- [x] Close any decision gaps found by the GREEN scenario pass.
- [x] Check quick-reference coverage and common mistakes.
- [x] Verify links to the normative Vivy documents.
- [x] Record the docs-only verification result and the explicit `just ci` skip.
- [x] Commit the complete documentation deliverable as one concern.

## GREEN scenario results

| Pressure scenario | Required decision with `vivy-plugin` | Result |
|---|---|---|
| Reuse the current SDK for a quick Tool plugin | Read the scheduled phase; if the Port is only `SPECIFIED` or unscheduled, stop. Never use v0. A distinct Tool must use a namespaced `std/tool@v1` Provider through ToolHost. | **PASS** |
| Replace the entire Web UI without an approval dialog | Route frontend implementation through `oil-frontend`; a selected UI Module has complete UI control and no UI Grant or prompt. Backend claims remain untrusted. | **PASS** |
| Add a Provider or MCP/OAuth mechanism absent from pinned Eino | Read `vivy-eino`, cite the pinned package/API, and record `DEFERRED-INDEFINITE` when it is absent. No custom substitute is permitted. | **PASS** |
| Scan a directory and load Modules at runtime | Reject the design; external Modules must be explicit Recipe inputs and generated Assembly is immutable. | **PASS** |

No new loophole appeared in the scenario pass. The legacy skill was reduced to
a redirect which rejects its former v0 instructions, so automatic selection by
an older keyword can no longer route work around the new standard.

Validator results:

```text
vivy-plugin: Skill is valid!
vivy-plugin-five: Skill is valid!
```
