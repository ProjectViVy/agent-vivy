# Acceptance

1. Run `just tui` from a project directory. It builds `vivy-code.exe` without the browser UI and opens VIVY CODE against that project.
2. Start a second `just tui` while the first remains active, and optionally keep the web Vivy process running. No process reports `organism lease held`.
3. Confirm each process creates a different `<shared-data-root>/code-instances/<instance>/vivy.db`; sessions created in one instance do not appear in another.
4. Confirm provider/model selection and credentials match the shared Vivy `settings.yaml` or frozen provider environment variables.
5. Confirm closing one TUI does not stop or alter any other TUI or the web process.

Studio is outside this acceptance path.
