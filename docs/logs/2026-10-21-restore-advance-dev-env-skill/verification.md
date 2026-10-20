# Verification

Base: `stable@ce548925`. Source: `7b42b292`. No product source, dependency,
recipe, database, or tenant home changed.

- `node --check .agents/skills/vivy-advance-dev-env/scripts/advance-dev-env.mjs`
  passed; the helper diff against the source commit is empty.
- `advance-dev-env.mjs --help` passed.
- Windows `--check --json` passed as a read-only host analysis: Debian WSL and
  its Docker `26.1.5+dfsg1` are detected. Missing `scripts/doctor.mjs` leaves
  repository gates UNKNOWN. Exit zero is not a product CI pass.
- `--apply --offline` refused the missing doctor with exit 1 before setup.
- The same helper was copied to a task-owned WSL temporary directory and run
  with `--check --no-doctor --repo /home/mastwet/work/agent-vivy`. It detected
  WSL and Docker, marked Windows acceptance EXTERNAL, and left repository
  gates UNKNOWN. That Linux checkout was `5715950d`; this proves host analysis,
  not same-candidate product acceptance. Its temporary copy was removed.
- `python -X utf8 .../skill-creator/scripts/quick_validate.py
  .agents/skills/vivy-advance-dev-env` passed: Skill is valid.
- `git diff --check` and staged diff checks passed.

An initial inline WSL probe failed before copying the helper because Windows
argument quoting lost a shell variable. A saved LF shell script avoided that
boundary and passed. The initial validator used Windows GBK and failed to
read UTF-8; process-local Python UTF-8 mode passed without changing global
settings or file encoding.

No full CI, race, PostgreSQL, Docker packaging, or product browser acceptance
was run for this agent-guidance restoration. Root AGENTS.md permits focused
checks for instruction-only changes; none of those product gates is claimed.
The restored 2026-10-17 records describe the original source delivery only.
