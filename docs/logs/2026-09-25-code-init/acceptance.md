# Acceptance

1. With a configured model provider, start `vivy-code` in a project with no `AGENTS.md`. `/help` should list `/init`. Enter `/init`, complete any governed write approval, and check that the agent inspected the repository before creating a short, project-specific root `AGENTS.md`.
2. Ask a follow-up question about a newly written rule in the same session. Vivy should apply it without restarting. This also applies when launching from a subdirectory that has ancestor rules.
3. Run `/init` again. Vivy should read the existing file and offer an explicit suggested change; it must leave the file untouched until the user approves a later edit.
4. In a Vivy terminal with no code project capability, `/help` should not advertise `/init`.
