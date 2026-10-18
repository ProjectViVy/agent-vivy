# Acceptance — provider agentic spike

A reviewer can tell the spike is complete when:

1. `docs/plans/provider-agentic/README.md` shows S0–S3 `DONE 2026-10-17` and S4
   `PROPOSED: GO-CONDITIONAL — awaiting owner signature`.
2. Each stage file ends with a "Result (2026-10-17)" section that answers its
   own exit-evidence list:
   - S0 §7: Q1–Q4 answered with numbers; verbatim requires; pin legality;
     one named unknown (product binary delta).
   - S1 §9: C1–C14 per component, three verdicts, re-run bridge search,
     "bridge required? — no".
   - S2 §9: A1–A6 × three protocols matrix, outbound bodies described,
     findings, "inexpressible: none".
   - S3 §7: recounted scale, per-site dispositions, consumer-checked deletion
     set, preserved list, Shape A decision, phases P0–P4.
   - S4 §8: outcome, evidence table against T1–T4, blockers B1–B7, upgrade and
     rollback (two checkpoint boundaries), deferral disposition, an **empty**
     signature row.
3. `docs/DEFER.MD` `PROVIDER-AGENTIC-MIGRATION` status names the proposed
   outcome and points to S4 §8.
4. `docs/TODO.md` §0.1 lists `STREAM-TRUNCATION-SILENT-SUCCESS` and
   `EINOEXT-CLAUDE-STREAM-RACE`.
5. `git diff e7dcf85 --stat` (upstream `main`) touches `docs/` only.

The owner's next action is to read S4 §8.3 (blockers B1, B2 first) and sign
or decline.
