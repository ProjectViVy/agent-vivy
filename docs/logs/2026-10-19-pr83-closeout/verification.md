# Verification

The tested runtime source was `8ec876d3035c0ed11e5ac5463b3e5abc29565506`.
Commands and outcomes below distinguish local runs from GitHub checks.

## Local Windows

- `just ci` — PASS, exit 0, Go 1.26.4, `VIVY_GOPROXY=direct`. This included the
  100-source DIVA overlay Memory Loop suite (946.095s), six `diva-recall`
  acceptance cases, and all 22 plugin modules.
- `go test -race -count=1 -timeout 20m -run '^(TestStreamCompleteReplyEndsCleanly|TestStreamCutOffIsAcceptedKnownBug)$' ./internal/provider`
  — PASS.
- `go test -race -count=1 -timeout 20m ./internal/provider` — PASS.

## Local Linux

- `just race-ci` — PASS, exit 0, on the exact source SHA above.
- `go test -race -count=10 ./internal/provider` — PASS.
- Full Linux race log: `/home/mastwet/pr83-verification/race-ci-8ec876d3-20261018T175453Z.log`;
  SHA-256 `478bf075599d691772cb81d8eec23fd7c83e10b1674d0c4fa32e02c546d6a2dc`.

## Actual Windows binaries and isolated databases

Command, run from a clean standalone acceptance checkout at the candidate SHA:

```powershell
node scripts/pending-run-binary-smoke.mjs --old .workspace/vivy-historical.exe --new .workspace/vivy-candidate.exe --output .workspace/final-pr83-matrix --go 'C:/Program Files/Go/bin/go.exe'
```

The historical binary is an unmodified build of `7b42b2920f72de6f37de8b9e4ff4bf6a110a762f`,
Eino v0.9.13, SHA-256
`52a6acb43c27d86cde23f42221ede4326436528ea7975e09c55590860ae26475`.
The candidate embeds source revision
`8ec876d3035c0ed11e5ac5463b3e5abc29565506`, Eino v0.9.21, `vcs.modified=false`,
SHA-256 `5a0a57833922bf7734ab7de85228fb70119e89e098d774a0a5463a3f0170ba7b`.

All three matrix cases passed their assertions:

- Historical pause → candidate upgrade: Approval, Question, and Plan were
  cancelled once; terminal and cancellation evidence were retained; checkpoints
  stayed unchanged; recovery made no model call.
- Candidate pause → direct historical rollback: the negative control confirmed
  this is unsupported because the old binary leaves the new pending requests
  behind across restart.
- Candidate pause → candidate cleanup → candidate restart → historical binary:
  all cleanup was complete before rollback; repeated recovery added no duplicate
  terminal or cancellation events and did not alter checkpoints.

The full machine-readable report is `binary-results.json` in this directory.
Its SHA-256 is `8501147d26be973ad1849534e47f17e1b0250d31d48928dc5c3fa60cdaab560f`.

## GitHub Actions

At PR #84 head `8ec876d3035c0ed11e5ac5463b3e5abc29565506`, run
[39151492199](https://github.com/ProjectViVy/agent-vivy/actions/runs/39151492199)
completed successfully for backend CI, UI CI, full UI browser smoke, Linux
backend CI, Linux race, Postgres integration, and aggregate `just ci`. These are
remote checks; they are separate from the local Windows/Linux results above.
