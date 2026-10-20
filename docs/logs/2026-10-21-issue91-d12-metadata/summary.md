# D12 migration metadata candidate

Status: implementation in progress; exact integrated acceptance remains open.

Accepted D03 c065a82b was integrated into the isolated data lane at c2c32ff5.
The root checkout was not edited. Released numbered SQL001..036 is unchanged.

The active manifest has no checksum field or computation. Final metadata is
exactly version/name/applied_at. Truly pristine initialization commits all DDL
and metadata in one transaction; later compatible target migrations retain
per-migration transactions. Read-only startup checks reject old metadata with
upgrade_required and malformed/unknown/partial targets before writes.

Target checks derive required table columns, types, primary keys and required uniqueness from the
embedded SQL catalog. Metadata structure and D03 private-binding shape are
checked independently of the numeric marker. Current036_tool_operation_private_binding
is a candidate barrier, not D14's complete final target T; old-lineage036_notebook_content
remains unsupported. No new migration number is allocated for metadata alone.

Legacy normalization/backfill writers and their ALTER metadata SQL are removed
from the runtime. A separate finite artifact retains frozen source recognition,
released017 repair and metadata sealing; D14 owns source-copy orchestration,
remaining business conversions and final assembly/release acceptance.

This continues the [BML candidate](../2026-10-21-issue91-d12-bml/summary.md).
D09 now has its accepted D03 input; D10 still needs D08/D12 and compatible
Laputa, D11 still needs D08 and reviewed hash-free INOFY delivery.
