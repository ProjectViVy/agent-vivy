# Verification

At the dependency handoff: independent WSL `go mod tidy` passed. `go list -m`
confirmed INOFY v0.0.0-20261021084605-6f1397e417fc and Eino v0.9.21; the
task-owned sibling Laputa is clean at9acc999301d5d575da2cea902a1c6d213763f381.
The expected RED compile probe reports the removed StrategyDigest,
DefinitionDigest, ArtifactDigest and UsedCatalogDigest downstream callers.
No empty placeholder, constant hash or compatibility implementation is added.
Required acceptance gates will run after adaptation, not at this checkpoint.
