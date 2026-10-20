# Restore the advanced development environment skill

Restored `vivy-advance-dev-env` from human-authored commit
`7b42b2920f72de6f37de8b9e4ff4bf6a110a762f` on the remake archive line to
`stable`, starting at `ce548925`. Its omission after the rollback removed the
explicit Windows/WSL/Docker venue guidance from the active skill catalog.

The four-file skill and its original 2026-10-17 delivery records are retained.
The executable helper is unchanged from that source. A restored-baseline note
identifies commands and launcher ports that stable does not yet provide, and
AGENTS.md requires agents to load the guide before environment or acceptance
work. Current owner instructions determine verification scope.

This cut restores agent guidance and host analysis. It does not import other
remake features, portable recipe infrastructure, a new minimal CI gate, or the
unfinished D02/D03 changes. No release or push is part of this delivery.
