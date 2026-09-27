# Acceptance

- Selecting Plan collaboration keeps the separately selected execution profile: a writable profile can still authorize its effectful action, while a read-only profile denies that same action.
- A model request after Plan entry receives the advisory guidance once; subsequent active-Plan requests retain it and Plan exit removes it, using the existing scripted Service/Engine coverage.
- Historical runs whose `run.started` payload has `mode: "plan"` and no collaboration contract version recover with the hard Plan profile for both runtime tool approvals and direct shell approvals. Effectful work stays blocked after resume.
- A human reviewer can verify the stored `run.started` and approval payloads remain the source of legacy mode evidence; no journal schema or historical event rewrite is introduced.
