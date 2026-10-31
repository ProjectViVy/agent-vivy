# Workbench source and dependency provenance

`python/vivy_workbench/repl.py` and `mcp.py` adapt Prime Agent's runtime at commit
`c4c238ca025e382d96fb5f37e6328ca77fb82ef3`, from
`prime-agent-runtime/src/rlm/repl.py` and `mcp.py`. Copyright and MIT terms are
retained in `PRIME-LICENSE`. Changes remove the second agent/factory, snapshots,
persistent bash, marketplace and OAuth surfaces; replace the host bridge; adapt
MCP SDK 1.27.0; and add complete artifact preservation and native memory shutdown.
The remaining package, Rust dispatcher/integration and acceptance fixtures are
owned by this repository. No Prime checkout is required at build or runtime.

`requirements-core.lock` and `requirements-memory.lock` pin transitive packages
and hashes for the CPython 3.13 baseline. The optional memory lock installs the
full upstream MemPalace 3.12.0 source archive at
`f41e8e75ddaadf5588d5127715fa531d79eadb45` and Chroma 1.5.8, without a source fork.
`memory.py` is a bounded consumer of that engine; it rebinds the upstream state
path seam because `MEMPALACE_CONFIG_DIR` alone does not isolate upstream locks.
Recheck this seam and collection/embedding contracts when updating the pin.

Rust embeds the owned Python sources and lock files, exports them into the owned
workbench directory, and launches an installed/prepared interpreter. It does not
import a reference checkout. The `pyproject.toml` is also usable for development
packaging; explicit preparation uses the lock files for reproducibility. Skill
packages are installed editable under those constraints; their additional
dependencies are not claimed to be fully hash-locked.
