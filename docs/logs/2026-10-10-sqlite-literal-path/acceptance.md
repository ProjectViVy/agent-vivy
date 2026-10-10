# Acceptance

Distinct supported filesystem paths select distinct Journal/snapshot databases even when they contain URI delimiters or percent characters. Invalid OS filenames fail instead of being interpreted as a different database name.

This is a storage boundary unit, not complete S11 cross-profile memory acceptance. Memory backends and read-only fixture observation paths still need their own literal-path checks, and the broader local memory-loop work remains open.
