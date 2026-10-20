# D06 generation and D01 scoped acceptance

Implementation in progress from e2c1518e on codex/issue91-d06.

The SDK is moving to explicit source/ref/root/Trust selection and one opaque
pack-owned Generation identity across Go, UI, Manifest and Inspect. Source,
lock, asset and catalog hash producers are retired; native dependency and
Git integrity remain with their original owners. Existing feature selection,
Eino typed runtime and Journal/Policy/Host boundaries are retained.

Independent Studio and go-host lanes are integrated through their branches.
No tenant data is accessed. No push or product-wide release acceptance is authorized.

Core Generation field/column retirement must sequence after active D03 migration
036: the manifest requires contiguous IDs. D06 does not copy D03 working files
or write dummy hash values into the existing core schema. Full D06 closure
requires that shared-owner integration; #93 extraction and D14 closure remain separate.
