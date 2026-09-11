# Acceptance

Pull request 19 is ready to merge when:

1. The default generated Assembly contains the Tool, Observer, Status, Context, Skill, and MCP Hosts and the typed Context/Skill/MCP Providers alongside the P5 declarative Provider Profiles.
2. The sealed Manifest reports Provider Profile, Context Source, and Skill Source identities from the same compiled plan.
3. The public Port evidence ledger contains the completed P3/P4 and P5 evidence without parallel registries.
4. The existing OpenAI-compatible and Claude adapters remain behind the single ModelHost, keep raw native model IDs, and expose no Secrets.
5. The cron scheduler regression observes both durable settlement and asynchronous active-run cleanup without assuming they are atomic.
6. The complete `just ci` gate, Generation pack/inspect path, and split UI smoke pass.
