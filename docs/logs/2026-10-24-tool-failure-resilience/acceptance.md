# Acceptance — tool-failure resilience

Product view: a tool or network failure must not end a run, and a fatal child must
say why.

1. **Public fetch through the operator proxy.** With `HTTPS_PROXY` set (the daily
   host uses `http://127.0.0.1:7897`), ask Vivy to fetch
   `https://github.com/search?q=agent-vivy&type=repositories`. Expected: the
   `web_fetch` result contains page content. Previously: the run failed with
   `proxyconnect tcp: public fetch: target address is private or local`.
2. **Delegation survives the same fetch.** Ask Vivy to delegate that fetch to a
   sub-agent. Expected: `child.completed` with a written conclusion in the parent's
   transcript. Previously: `sub-agent failed: The child task could not be completed.
   Please retry.`
3. **A broken proxy is a tool result, not a dead run.** Restart the split pair with
   `HTTPS_PROXY=http://127.0.0.1:1` and repeat (1) and (2). Expected: the Run
   Inspector shows a `tool.finished` row with `outcome=recoverable`,
   `reason=network_failed`, `effects=unknown`; the next model turn quotes the
   diagnostic; the run and the child both reach a terminal conclusion. No
   `sub-agent failed: ... Please retry.`
4. **Private targets are still refused.** Ask for `http://127.0.0.1:8787/` or any
   RFC1918 host through `web_fetch`. Expected: a refused result
   (`refused/policy_denied/not_executed`) that says the target is private or local,
   and the run continues. A redirect into a private host is refused on the hop, and
   the hop is never sent.
5. **A fatal child explains itself.** Force a runtime-integrity failure inside a
   child (or read the unit fixture). Expected: the parent's tool result carries the
   generic sentence plus `Cause: <bounded, redacted detail>`; `child.failed.detail`
   is ≤1024 bytes in the Journal. A provider-status child keeps its specific
   provider message and adds no `detail`.
6. **Governance refusals stay refusals.** A tool outside the run selection, a
   tool-hook `deny`, and `report_goal` from a run that does not own the session Goal
   each produce `did not run: ...` results with
   `refused/policy_denied/not_executed`, and the run still completes.
