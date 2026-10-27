# Trusted preset full-auto approval — acceptance

1. Start the split dev pair (`just run`, then `cd ui && pnpm dev`) and open
   `http://127.0.0.1:3015`.
2. Switch the chat permission selector to **Trusted / 信任** and confirm.
3. Ask the agent to write a file, patch a file, and run a shell command. Each call executes without
   an approval card; the run continues to completion.
4. The selector and Settings descriptions say effectful tools auto-run when governance allows,
   instead of "allowlisted tools auto-run, others still ask".
5. Hard stops still hold:
   - A command on the bash deny table is refused with a model-visible "did not run" result.
   - In Plan mode, effectful writes are still refused.
   - `create_goal` still asks for human authorization.
6. Switch back to **Smart / 智能** and repeat a write: the approval card appears as before.
7. An existing `config.yaml` that still contains `auto_approve_tools` starts normally; the field has
   no runtime effect.
