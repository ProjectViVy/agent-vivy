# D08 acceptance

The split browser at `http://127.0.0.1:3015` used an actual backend, a disposable
home, a synthetic initialized persona and a deterministic loopback mock provider.
No real account or token was used. The Windows browser observed the WSL servers.

| Case | Visible result | Durable evidence |
| --- | --- | --- |
| Approve | Write and final model response | File has the exact nonce; completed success outcome. |
| Deny | Denied tool row and final model response | File absent; refused / user_denied / not_executed. |
| Restart while awaiting approval | Same pending call survives hard backend termination; approval completes it | Its checkpoint is v2 and owns the same prompt Run ID before restart; one completed operation and one file afterward. |

All three tool call IDs and exact result values (through the existing untrusted
output wrapper) occur once as the final tool message in the next model request.
Each Run has one prompt snapshot and exactly one terminal. There are no remaining
pending approvals or nonterminal Runs. The page returns to completed / Online with
its input available. See `browser-result.json`, `browser-completed.png` and
`browser-restart.png`; the raw disposable database/model requests are not shipped.

Automated rejection cases cover missing/foreign snapshot, changed engine/schema/
composer/Generation, old format, policy drift and forged same-revision mask
content. Recovery retains admitted body/frame despite current mask/config edits.
Completed siblings do not execute effects again. Direct storage calls cannot
substitute arbitrary admitted mask values.

The private-policy/prompt canary tests verify that actual admitted data survives
recovery without appearing in public events. Host IDs/content and explicit
versions remain usable without the retired hash projections.

Final upgrade publication still requires D11/D12/D14 integration. Full-product CI
and human real-token proof remain D14. Roll back code with Git and data with the
approved quiescent backup procedure; never open a converted target with an old
executable. Validly framed opaque payload corruption is no longer independently
detected by a removed Vivy checksum; Eino owns native decoding.
