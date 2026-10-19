# Masks in Vivy Bundles

**Date:** 2026-10-11\
**Status:** Product direction recorded from discussion; implementation details remain open.\
**Related:** [Suite proposal #20](https://github.com/ProjectViVy/agent-vivy/issues/20) and [Codex and ZCode preliminary research](suite-system-upstream-research-2026-10-11.md).

**Follow-up:** [Detailed content-bundle architecture](../architecture/VIVY-CONTENT-BUNDLES.md). The user subsequently confirmed recommended Skill association: selecting a Mask does not alter Skill availability or permissions. The architecture proposal carries the current design; the discussion below retains its original context.

**Correction r2:** The user confirmed the product name **Toolkit (工具包)**; bundle remains the content packaging term.  The user subsequently specified native-only toolkit authoring and a dedicated toolkit management UI. The normalization examples below explain how authors distinguish roles from procedures; they do not require a format converter, importer, or compatibility layer. The architecture's sections 16–17 carry this corrected scope.

## Confirmed direction

A Vivy bundle can contain multiple Masks alongside Skills, Rules, Hooks, and MCP declarations. Masks are a first-class content category in the proposed bundle model. Here, “bundle” refers to the content package discussed as a “suite” in #20; final naming and manifest syntax remain open.

When adapting an upstream plugin into a Vivy bundle, role-oriented or personality-guiding skills should be normalized into Masks according to their meaning. The upstream file type alone does not determine the Vivy component type. The user identified BMAD's PM and Dev expert roles as examples.

Original user direction:

> 我希望面具也加入bundle里面去。也就是说这个bundle可以带一些面具。然后的话如果说原始的插件里面，会有一些偏向于人格性引导的skill，在vivy这边制作bundle的时候会被规范为面具。例如bmad的pm、dev等专家团体。

*English translation:* "I want Masks included in the bundle as well — that is, a
bundle can carry some Masks. Also, if the original plugin contains skills
oriented toward persona guidance, they should be normalized into Masks when the
bundle is authored on the Vivy side. For example, BMAD's PM, Dev, and other
expert groups."

This normalization belongs to bundle authoring and adaptation. Automatic runtime classification is not established by this decision.

## Existing Mask semantics

Vivy's [Mask framing](../../internal/modules/masks/prompts/mask-frame.md) defines a Mask as optional working style and perspective for the current task. The active persona and runtime rules retain authority; explicit user requirements take precedence over Mask preferences. A Mask cannot change identity, permissions, tools, or model access.

Accordingly, an imported expert role can express professional perspective, behavioral preferences, communication style, and areas of attention within that boundary. Upstream instructions that claim a replacement identity or additional authority require adaptation.

The existing [Mask contract](../../internal/maskcontract/masks.go) provides a per-session selection and an immutable snapshot for each admitted run. Its definition carries identity, content, revision, and digest. Bundle-supplied definitions still require lifecycle and provenance integration; the current contract does not establish a bundle loader or manifest format.

## Proposed normalization rules

Classify content by its purpose in Vivy. Mixed upstream documents may need to be split into several components rather than copied unchanged into one Mask.

| Upstream content meaning | Proposed Vivy representation |
| --- | --- |
| PM responsibilities, product perspective, questioning habits, and tradeoff principles | PM Mask |
| Dev engineering perspective, implementation preferences, and communication habits | Dev Mask |
| Steps, inputs, templates, and deliverables for producing a PRD | Skill and supporting resources |
| Concrete implementation or code-review procedures | Skill |
| Expert task allocation, handoffs, or concurrent execution | Separate workflow and runtime assessment |

The BMAD examples illustrate the intended mapping. A file-by-file audit of the relevant BMAD version is still needed before claiming compatibility.

For example, an upstream PM skill may contain both a product-manager perspective and a procedure for creating a PRD. The perspective can become a PM Mask, while the procedure becomes a reusable Skill. This allows other Masks to use the same procedure and lets role guidance and task methods evolve independently.

Preserve upstream source identity, version or commit, original paths, and the transformation record so maintainers can trace each resulting component back to its source. The metadata schema remains to be designed.

## Proposed usage and maintenance

A bundle with several Masks can provide a selectable expert-role library. Installation should make those roles available through the existing Mask owner; selection and run capture should follow the existing session and snapshot semantics. Installing several expert roles does not itself specify concurrent agents, scheduling, or task handoffs.

The user confirmed allowing a Mask to reference recommended Skills while keeping those Skills independently usable. Selecting a Mask does not enable a fixed set of Skills or change permissions. Any association must preserve the existing rule that a Mask cannot grant tool or model access.

Bundle ownership should remain distinguishable from user-authored Masks. A proposed editing model is to let users save an adapted copy as their own Mask, avoiding silent replacement of personal edits during bundle updates. This ownership and editing policy is a recommendation, pending a lifecycle decision.

## Open decisions before implementation

- How recommended Skill references are resolved and how unavailable recommendations are presented. The recommendation-only relationship is confirmed.
- Bundle Mask identity, namespace, provenance fields, and integration with the existing Mask catalog and storage contracts.
- Enable, disable, update, and uninstall behavior when a session selects a bundle Mask, including preservation of admitted run snapshots.
- Whether users can edit bundle-owned definitions directly or must create independent copies.
- The concrete BMAD source version, adaptation rules, and any expert coordination features requiring separate runtime work.

This record extends the content scope of the preliminary suite research. It does not settle a manifest schema or authorize implementation. Any later runtime design must follow Vivy's architecture and Eino capability-check requirements.
