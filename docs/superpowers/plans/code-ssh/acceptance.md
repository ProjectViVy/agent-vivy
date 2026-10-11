# Code SSH acceptance evidence

**Design:** [SSH-D1](../../specs/2026-10-11-code-ssh-design.md).
**Contract:** [SSH-C1](contracts.md). **State:** [index](index.md).
No product check below has been executed by this planning delivery. Evidence
must identify commit, command, fixture, actual result and environment. Mocks
cannot replace real SSH, Host authority, selected GUI or omission evidence.

| ID | Requirement / responsible Story | Required acceptance evidence |
| --- | --- | --- |
| A01 | R1/R4/R5/R7/R8; SSH-00 | H1-H6 ledger, exact authority path, replay/approval/egress tests, constructor ownership, decision and any approved generic seam evidence |
| A02 | R1/R3/R4/R5; SSH-01 | Strict framing/typed payload/digest/limits tests, identity conflict, version refusal; parser fuzz corpus and executed bounds |
| A03 | R2/R4/R5; SSH-02 | Root/symlink race containment, stale edits, per-file partial multiedit, concurrent duplicate admission and crash windows; no false settlement |
| A04 | R3/R4/R5; SSH-03 | Stdout/stderr/exit, bounded output, live jobs, cancel/timeout/EOF/lease, recycled PID protection and unknown descendant cleanup |
| A05 | R1-R5; SSH-04/05 | Real verified-key SSH via Host, scoped profile/egress, stable-ID resume, no blind retry, target freeze, core approval/Journal reconciliation |
| A06 | R1/R2/R5/R6; SSH-06 | Distinct local/remote AGENTS sentinel, remote preview, exact digest download, changed artifact refusal, tenant/session/root denial |
| A07 | R1/R3/R5/R6; SSH-07 | Split GUI at :3015 against selected backend, action authorization, local/remote indicator, connection/error/stale status, jobs/output/cancel, switch denial |
| A08 | R5/R7/R8; SSH-08 | Selected/omitted packs and Inspect, exact source closure, no plugin code/UI/Grants/assets on omission, local start with SSH absent, seven artifacts and full CI |
| A09 | R1-R3/R6/R9; SSH-08 | Real remote inspect → edit → test → build → present → download loop; both remote architectures and supported local client matrix recorded honestly |
| A10 | R4/R5/R9; SSH-08 | Live disconnect/crash/hostkey/auth/disk failure matrix and operator recovery; original operation not executed twice |

## Required failure matrix

| Injection | Observable result / evidence |
| --- | --- |
| Before remote send; auth or host key fails | Denied/unavailable, zero remote mutation, no local fallback |
| After durable admission; before side effect | Retained accepted/unknown evidence; reconciliation only; no automatic command retry |
| During rename, per-file multiedit or command | Completed/partial/unknown receipt states survive restart; duplicate identity never repeats |
| Remote completed; reply lost | Same identity returns retained result after reconnect; original side effect count equals one |
| Remote completed; local settlement fails | Local authority remains unsettled/unknown until approved reconciliation; remote success alone does not settle Journal |
| Worker dies; PID reused; daemon child escapes group | No unrelated process signaled; uncertain cleanup displayed unknown; isolation-required execution refused without backend |
| Stdout floods or stderr corrupts framing | Bounded output and explicit gap/limit; parser stays synchronized or closes unavailable |
| State directory full/read-only or receipt damaged | Stop new side effects; diagnostic unavailable/unknown, never manufacture a completed receipt |
| Root symlink race or artifact replacement | No outside-root access; changed/unavailable download, never newer bytes under old reference |
| Switch target/root/policy during approval/resume | Binding_changed/denied before remote call; existing operation stays attached to original binding |
| Plugin absent; OpenSSH absent | Local Generation packs/starts without plugin imports, assets, Grants or SSH runtime requirement |

## Seven artifacts and cost record

SSH-08 returns Definition (descriptor/catalog), SDK (exact supported public
interfaces), Provider (plugin), Consumer (ToolHost/ActionHost/ContextHost/Status
and UI projections), Failure Model (this matrix plus receipts), Conformance
(executed tests on these source hashes) and Inspect (sealed Generation report).
If H1-H6 require a generic Host change, that change supplies its own complete
seven-artifact evidence; plugin tests cannot attest an untested Host producer.

Record transport startup, read/edit, command launch/output, cancel and artifact
download observations with target OS/arch, network and payload size. These are
measurements, not invented SLOs. Record dependencies, worker installation,
retained receipt/output disk use and cleanup procedure. No performance threshold
or compression/pool/cache work is accepted without observed need.

Evidence logs use `docs/logs/YYYY-MM-DD-code-ssh-<story>/` with summary,
verification and acceptance. Update this matrix's links after execution; leave
blocked/unavailable cells unverified. Planning logs are separate and cannot close
any A01-A10 item. Failure cleanup is confined to owned fixtures/processes and
explicit test targets; do not delete an operator's project or retained receipts.
