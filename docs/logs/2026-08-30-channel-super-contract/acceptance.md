# Super Channel contract: user acceptance

Date: 2026-08-30

| Acceptance item | Evidence | Result |
|---|---|---|
| Contract changed from proposal to adopted direction | `VIVY-CHANNEL-PACK.md` status line | PASS |
| Host remains in the kernel and is never a plugin | Contract §7; `SELF-EVOLVING-GATEWAY.md` §4.2; `VIVY-GATEWAY-AND-STUDIO.md` §4.2 | PASS |
| All five adapters in this batch are `plugins/` + `seam: channel` | Contract §1, §9, §14; `VIVY-ASSEMBLY.md` exception section | PASS |
| Do not add a new `channels:` recipe key / `RegisterChannels()` | Contract §1, §10; ASSEMBLY does not add the key | PASS |
| The envelope is the first shaped slice; A2A / NeuroLink later use the same Host | Contract §8, §15 | PASS |
| Face / ACP remain independent | Contract §2; ACP proposal cross-reference | PASS |
| Empty `allow_from` is fail-closed | Contract §7, §11, §18.8 | PASS |
| An independent `go.mod` is mandatory for this batch | Contract §9.1, §18.11; PLUGIN-SPEC directory section | PASS |
| The default body has no ears | Contract §10, §16 | PASS |
| No runtime code implemented | No changes to `internal/` / `sdk/` / `ui/` / `plugins/telegram`, etc. | PASS |
| Incomplete items remain on the board | `CH-A`/`CH-B`/`CH-C`/`UI-CHANNELS-BE` OPEN; `CH-0` DONE | PASS |
