# Acceptance

A maintainer can read `AGENTS.md` and confirm that future implementation work
must make decisions in this order:

1. preserve Vivy's unified architecture and authoritative paths;
2. inspect and prefer pinned Eino/EinoExt native capabilities;
3. add custom overlapping machinery only with documented evidence.

The rule also makes clear that Eino reuse must stay behind Vivy's existing
runtime/provider boundary rather than spreading Eino types into domain and
product layers.
