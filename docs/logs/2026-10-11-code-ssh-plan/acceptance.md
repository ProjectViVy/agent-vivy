# Planning acceptance

| Requested outcome | Delivery / limit |
| --- | --- |
| Check whether #15 has a detailed plan | Baseline research found proposal/checklist scope without an SSH Story package; this branch now provides the linked package |
| Plugin + lightweight remote worker | SSH-D1/SSH-C1 define source boundaries, ownership, transport, receipts, jobs and resource semantics |
| Pure plugin; avoid internal | No internal code changed; public plugin imports internal/Eino are forbidden; H1-H6 explicitly gate integration and any generic exception |
| Detailed action plan through Supermanagement | Four Epics, nine Story plans, immediate dependency DAG, readiness/ownership, exact worker interfaces, concrete tasks/tests and requirement/evidence coverage |
| New code-ssh branch | Remote branch created from ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666; reviewed plan docs are published there with lease protection |
| Preserve local/optional scope | Default/local behavior and Recipes unchanged; physical omission and no-SSH startup are mandatory SSH-08 evidence |

Planning acceptance does not approve product implementation or a Host exception.
No feature or full zero-internal feasibility is claimed. SSH-00 may conclude
exception-required; dependent integration remains blocked. A01-A10, seven
artifacts, full CI and real GUI/SSH acceptance remain implementation obligations.

The new TODO row is an open issue/gap pointer. The Story index alone owns
execution states/dependencies. No issue/PR was closed, merged or commented on.
