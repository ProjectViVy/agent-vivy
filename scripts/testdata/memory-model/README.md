# Native memory model checks

Python generates/replays synthetic JSON traces and models observable outcomes.
Go test-only overlays call the real BML SQLite store and pinned ACTMEM APIs. Python
does not write storage files. No production hooks, authority formats, dependencies,
or source pins are changed. These checks do not exercise the LLM/evolution runtime.

## Run

From the Vivy repository root, use Python 3.10+ and the Go toolchain required by the
modules (validated with Python 3.10.12 and Go 1.26.4 on Linux):

```sh
python3 -m unittest discover -s scripts -p test_memory_model.py -v
python3 scripts/memory_model.py --target bml --cases 20 --steps 100
python3 scripts/memory_model.py --target bml --go-regressions --race
python3 scripts/memory_model.py --target recap --cases 5 --steps 40
python3 scripts/memory_model.py --target recap --go-regressions
python3 scripts/memory_model.py --replay scripts/testdata/memory-model/recap-cancel.json
```

A clean sibling `../laputa` at `laputa-source.lock.json` is required, including for
BML-only runs so every receipt has the same source boundary. The script checks its
commit, cleanliness and ACTMEM module identity. This is not a replacement for the
official bootstrap, remote validation, or whole-product `just ci` gate.

**The locked Laputa revision `0e0027c5` fails before-replacement cancellation.**
Recap/default-all commands intentionally return failure for that defect. They do
not skip it, bless it as an expected pass, or apply a dependency fix. After-replace
uncertainty must still fail closed. A diagnostic fix overlay is a control only.

## Protocol and guarantees

- `create`, `update`, `delete`, `snapshot`: owned BML records. Content uses the
  synthetic FTS token `needle`; the narrow reference model assumes that token is
  present in every live record. Snapshots compare visible search and store revision.
  Replays compare the original native mutation outcome, not current target state.
- `commit`, `read`: ACTMEM Recap with revision CAS and stable operation identities.
- `reopen`: clean close/open within the same process and root.
- `restart`: clean close, process exit, fresh process on the same root.
- `crash`: immediate process exit without `Close`, then fresh process on the same
  root. This checks process-death recovery, **not** power-loss durability, torn
  writes or a crash during an uncommitted mutation.
- `cancel_recovery`: uses ACTMEM's existing unexported `recapFault` test seam. Before
  replacement, it cancels the context and lets native code observe cancellation;
  after replacement, it returns a cancellation fault after the filesystem rename.
  The bridge verifies that the requested boundary was actually reached.

Each worker consumes JSON requests and emits one normalized JSON object per input.
Malformed output, incorrect result counts, timeouts and unexpected exit statuses
fail closed. An explicit crash alone permits exit 23. Processes, builds and traces
are bounded; all store roots and binaries live in automatically cleaned temporary
directories. Python uses only the standard library. `--race` instruments workers;
it does not substitute for concurrent ownership tests or native Windows checks.

## Failure evidence and replay

Exit 0 means all selected comparisons passed; exit 1 means a model/native-test or
worker failure; exit 2 means invalid input, source precondition, or build failure.
Mismatch/worker receipts default to ignored `.workspace/memory-model/<target>-<seed>.json`.
`--output DIR` changes that location. Receipts retain the complete exact trace,
seed, first mismatch index (zero-based, or -1 for transport failure), expected and
actual values, source commits, dirty flag, Go version and harness digest. They
contain synthetic content only. Replay creates a **fresh** root rather than using
leftover database state:

```sh
python3 scripts/memory_model.py --replay .workspace/memory-model/recap-17.json
```

The small checked-in cancellation trace is a curated regression. Generated
receipts are not automatically minimized; there is no claim of a globally shortest
failure trace. A saved receipt can be replayed against a later dependency to check
whether its old failure is fixed; its source metadata is evidence, not permission
to override the current lock. Native regressions are embedded in the same Go
overlay sources and run without Python reference-model assertions. They are opt-in
and are not silently registered into the existing aggregate CI gate.
