# Acceptance

How a human can tell this worked:

1. **Lane landed**: `git log --oneline -3` on main shows the fast-forward past 3c25562 containing all VC lane work; `git log main..feat/vc1a-bash-tool` and `git log main..feat/rb1-rollback-research` are both empty. Both worktrees are gone.
2. **Walkthrough**: `go test ./internal/runtime/ -run TestVC1Walkthrough -v` passes — it replays the full read-code → grep → multiedit → bash-test → final-answer loop against a real workspace and asserts the multiedit result carries a diff and the file_versions chain exists.
3. **TODO board**: `docs/TODO.md` §0.1 rows VC-0, VC-1, VC-2 read `DONE 2026-09-02` with closure notes; §10 has the closure row pointing back here.
4. **Fixed defects visible**: `internal/tools/multiedit.go` declares `edits` with `Type: "array"`; `internal/storage/{sqlite,postgres}/fileversions.go` normalize nil content to `[]byte{}` before insert.
