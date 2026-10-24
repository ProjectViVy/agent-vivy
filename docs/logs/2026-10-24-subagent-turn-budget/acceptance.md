# Acceptance

1. Ask Vivy to delegate a multi-step research or reading task to a sub-agent.
   The sub-agent may inspect many files in sequence and still return an answer;
   it is no longer cut off after eight model/tool cycles.
2. If a sub-agent genuinely runs away, it is still stopped at 32 turns per
   agent, and the parent chat continues with a model-visible
   `sub-agent failed: ... tool-call limit ...` result instead of ending.
3. A parent plus its delegated children stay inside the shared run-tree
   breakers (128 model calls, 256 tool calls, 2048 events); exhausting them
   fails only the affected run, not the conversation.
4. An operator can set `runtime.max_tool_turns` in `config.yaml` and see the
   same ceiling applied to the parent and to every delegated child.
