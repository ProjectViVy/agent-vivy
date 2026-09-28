# DIVA PEN research: source archive for VIVY

This directory preserves the complete eight-document DIVA research package on
Workbench, PEN, Mirror, Neuro-Link, and Companion Node, together with its four
iteration records and the two product-scope decisions cited by the package.
It is a source record for the VIVY design discussion, not a VIVY product
contract or an implementation plan.

Source: `ProjectViVy/agent-diva`, `dev` at
[`c565bb245cc920258d7f8c7fcd9544fbba545af7`](https://github.com/ProjectViVy/agent-diva/tree/c565bb245cc920258d7f8c7fcd9544fbba545af7).
The original research package is dated 2026-08-23. Its stated status is
**Research / Proposal**: the user accepted the overall direction, while
production implementation had not been authorized in that record.

## Read the source

1. [Research package and original reading order](source/docs/research/diva-workbench-pen-mirror-neurolink-2026-08/README.md)
2. [Concept and authority boundaries](source/docs/research/diva-workbench-pen-mirror-neurolink-2026-08/concept-model.md)
3. [Core architecture and capability projection](source/docs/research/diva-workbench-pen-mirror-neurolink-2026-08/architecture.md)
4. [Neuro-Link frontend contract](source/docs/research/diva-workbench-pen-mirror-neurolink-2026-08/neurolink-front-end-fabric.md)
5. [PEN and Mirror module models](source/docs/research/diva-workbench-pen-mirror-neurolink-2026-08/pen-and-mirror.md)
6. [Companion Node and experience boundaries](source/docs/research/diva-workbench-pen-mirror-neurolink-2026-08/companion-node.md)
7. [DIVA implementation evidence and references](source/docs/research/diva-workbench-pen-mirror-neurolink-2026-08/current-state-and-references.md)
8. [Security, proposed epics, gates, and unresolved decisions](source/docs/research/diva-workbench-pen-mirror-neurolink-2026-08/roadmap-and-gates.md)
9. [Original iteration summary](source/docs/logs/2026-08-diva-workbench-vision/v0.1.0-workbench-pen-mirror-neurolink-research/summary.md)
   ([acceptance](source/docs/logs/2026-08-diva-workbench-vision/v0.1.0-workbench-pen-mirror-neurolink-research/acceptance.md),
   [verification](source/docs/logs/2026-08-diva-workbench-vision/v0.1.0-workbench-pen-mirror-neurolink-research/verification.md),
   [release](source/docs/logs/2026-08-diva-workbench-vision/v0.1.0-workbench-pen-mirror-neurolink-research/release.md))
10. [Workbench scope decision](source/docs/decisions/product-scope-and-workbench-2026-06-18.md)
    and [Alife disposition](source/docs/decisions/alife-feature-disposition-2026-06-18.md)

## What the source actually establishes

- **PEN** is an installable, authorized, supervised, revocable, and audited
  external capability unit. It can project tools, channels, agents, resources,
  events, or UI without defining one universal wire protocol.
- **Neuro-Link** is the owner/frontend interface for conversation,
  presentation, control, workspace binding, and state synchronization. The
  earlier hypothesis that Neuro-Link is a PEN transport was explicitly
  withdrawn in the original iteration summary.
- **Mirror** owns replaceable embodiment and presentation; **Workbench**
  composes product surfaces without becoming a second authority for memory,
  persona, sessions, or tasks. **Companion Node** combines a frontend, Mirror,
  sensor PENs, and local privacy boundaries on a device.
- Browser PEN was recommended as the first infrastructure slice in the DIVA
  proposal. Package/instance/entity identity, process isolation, module kinds,
  the first frontend, and the first PEN were listed as decisions to settle
  before production work.

These are **DIVA research findings and proposals**. Their Rust Manager paths,
Laputa/BML ownership assumptions, Neuro-Link prototype, packaging approach,
and original epic sequence describe DIVA at the source revision. Adoption in
VIVY requires reconciliation with VIVY's current Module/Port/Host/Recipe
contracts, single `Service.Run` and Journal, policy boundaries, and the
VIVY/Studio split. This migration changes none of those contracts and claims
no PEN implementation in VIVY.

The source files are copied without prose changes. Three links pointing to
documents outside this bounded archive were changed to pinned DIVA source
URLs: two related research packages in `current-state-and-references.md`,
and the root `TODOLIST.md` link in `acceptance.md` (three links total).
All internal links among the imported files retain their original relative
paths.
