# Managed ancestor test isolation

Synthetic project instruction fixtures now create their own Git root marker so managed `/tmp/.git` or `/workspace/.git` ancestors do not change their project boundary. Production instruction discovery is unchanged. The external SDK consumer's synthetic module explicitly disables VCS stamping on its build; no product provenance check is disabled.

No release: test-only isolated branch checkpoint.
