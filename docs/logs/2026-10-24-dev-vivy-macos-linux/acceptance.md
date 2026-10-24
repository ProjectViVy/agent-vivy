# Acceptance

On macOS or Linux, from the repository root:

1. Run `just dev-vivy` (or `just dev`).
2. The first run prepares Laputa, installs UI dependencies if needed, and
   builds the headless backend with the Go version pinned by `go.mod`.
3. The terminal prints
   `split loop ready: http://127.0.0.1:3015  (control plane http://127.0.0.1:8787)`
   and opens the browser unless `--no-browser` / `-NoBrowser` is passed.
4. Editing Go code and running `just dev-vivy` again stops the previous
   backend and Vite and restarts them; no manual port cleanup is needed.
5. Ctrl+C stops both processes and exits without leaving `tail`, `node`, or
   backend processes behind.
6. Dev state (settings and `vivy.db`) lives under `.workspace/dev/home`, so
   the daily `~/.vivy` journal is untouched. Export `VIVY_USER_HOME` to use a
   different home.
