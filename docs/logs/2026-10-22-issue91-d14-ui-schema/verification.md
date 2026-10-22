# Verification

Candidate base: a12748a7114bb54fd0bdf663bfb1f01fcec90802. Fresh independent Linux checkout: /home/mastwet/work/vivy-issue91-d14-ui/agent-vivy, with its own sibling Laputa at 9acc999301d5d575da2cea902a1c6d213763f381. Existing D10 Linux clone was preserved.

- RED: go test ./internal/domain -run TestDeliverablePresentedSchemaMatchesCurrentMetadata -count=1 failed because the serialized current Deliverable lacked the schema's required sha256 field.
- GREEN: the same regression passed after removing the obsolete field from the closed schema; the regression also rejects that retired property.
- PASS: pnpm --dir ui install --offline --frozen-lockfile in the fresh disposable clone.
- PASS: pnpm --dir ui typecheck, including its normal SDK stage-ui pretypecheck. Generated sources were produced by the SDK; no generated source was hand-edited.
- PASS: pnpm --dir ui exec vitest run ../plugins/vivy-persona/ui/vivy-persona/src src/generated/ui/vivy-workflow src/lib/demo-api.test.ts. Seven files, 50 tests. Existing workflow test harness emits React act environment warnings; no test failed or skipped.
- PASS: node --test scripts/check-i18n-cross-face.test.js. Eight tests.
- RED then GREEN: node scripts/check-i18n-cross-face.js initially reported two retired classifications and three current unclassified keys; exact source-aligned correction passed all 13 shared semantic units and English/Chinese projections.
- PASS: git diff --check; gofmt on the new dedicated schema test; focused source residue search over assigned consumers returned no obsolete DTO/helper names.

Automatic approval review rejected an attempted hard reset of the earlier dirty WSL clone because its state had not been verified. The rejected command did not execute. Read-only inspection confirmed dirty files, and a new unused disposable clone was created without resetting or deleting the prior clone. No approval block remains.

Heavy whole-candidate, native, browser and release gates were intentionally not run concurrently with the parent lane's active acceptance. The parent owns regenerated assembly integration and final audit/gates. Task transfer scripts, bundle and temporary output logs were removed after recording evidence.
