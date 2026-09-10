# Acceptance — Command output capture race

1. A successful direct command returns its bounded stdout before `Execute`
   returns.
2. A successful foreground Bash command returns its bounded stdout before the
   job reaches a terminal state.
3. Background jobs retain incremental reads, output bounds, timeout adoption,
   cancellation, and final status behavior.
