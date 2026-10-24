# Acceptance — child runs share their root run's workspace

How a human tells this worked, from the operator's point of view.

## 1. A sub-agent can read what the parent wrote

This is the complaint that started the lane: "子 agent 只能做隔离推理，不能替我访问
仓库文件."

Check, in the sandbox world (a session with **no** selected project directory):

1. Start the split pair (`just run`, then `cd ui; pnpm dev`) and open
   `http://127.0.0.1:3015`.
2. In one message ask for both steps, so they happen inside a single run:
   *"Use write_file to create proof.txt containing `written by the parent`, then
   use the agent tool to delegate: open proof.txt and report its exact content."*
3. Approve the `write_file` call when it asks.

Before: the sub-agent's `read_file` came back
`filesystem: read proof.txt: … /workspaces/child_<id>/proof.txt: no such file or
directory`, and the parent reported a sub-agent that could not find the file.

After: the sub-agent's answer quotes `written by the parent`.

## 2. One directory per conversation-run tree

`data/workspaces/` (or the configured `runtime.workspace_root`) gains exactly one
directory for the run above — `run_<parent id>` — holding `proof.txt`. No
`child_<id>` directory appears for the delegation, at any depth: a grandchild
lands in the same directory as its parent and its root.

Check: `ls <workspace_root>` before and after the run in §1.

## 3. Parent and child report the same workspace

In the Run Inspector (Settings → Vivy Features → Run Inspector → Children), the
child's workspace id equals the parent's run id. The same value is in the
Journal's `child.started` event, so a replayed session shows it too.

Check: open the child row for the run in §1 and compare its workspace id with the
parent's run id.

## 4. A writable sub-agent still asks first

Sharing the directory did not share the authority. A host-requested one-shot
child whose `tool_names` include a writable tool can now write into the parent's
workspace — and that write still stops at an approval, because the child inherits
the parent session's sandbox mode and approval policy. Nothing about the gate
moved.

Check: start a one-shot child with `write_file` from the Run Inspector's "Start
child run" form under the smart preset; the call appears as a pending approval
before any file exists, and the file appears only after approving it.

## 5. A failed or cancelled child never takes the directory with it

Cancel a child mid-flight, or let a child admission roll back. The parent's
directory and everything in it survive; only a run's *own* private directory is
ever reclaimed, and a child no longer owns one.

Check: run §1, cancel the child while it is working, then confirm `proof.txt` is
still in `run_<parent id>/`.

## 6. The other two worlds are untouched

A session with a selected project directory, and a `world: local` launch, already
gave every run the same directory; both behave exactly as before. Path
validation, symlink rejection and the workspace-name rules are unchanged.

Check: open a session bound to a real project root and confirm a delegated child
still reads and writes inside that project, and that nothing new appears under
`runtime.workspace_root`.

## 7. Old child directories are left alone

Directories that earlier builds already created as `<workspace_root>/child_<id>`
are inert scratch. This change does not migrate, adopt or delete them; they are
boarded as `WORKSPACE-ORPHAN-CHILD-DIRS` for an explicit cleanup decision.

Check: nothing disappears from `<workspace_root>` on upgrade, and no run fails
because an old `child_*` directory is still there.
