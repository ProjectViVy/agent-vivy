# Issue58 Task1a expansion budget fix

A negative `BudgetChars` reached a negative rune-slice bound after native source
export and panicked. BoundBackend.Expand now rejects negative values with the
existing `invalid_schema` contract error before export or slicing. Zero still
selects the 1000-character default, and 4000 remains the inclusive maximum.

This is the single Important finding from the independent Task1a review. No
native BML, dependency, lifecycle, factory, source authorization, Task1b or Task2
behavior changes. The prepared adapter is not yet activated as LIFE projection.
