# Minimal browser chat

Use [minimal's setup](../minimal/README.md) through creation of the disposable
config and nonce file, then replace its pack/run commands with:

```sh
pnpm --dir ui install --frozen-lockfile
go run ./sdk verify faces/minimal-chat
go run ./sdk pack --recipe recipes/minimal-chat.vivy.yml --output "$VIVY_USER_HOME/artifact"
go run ./sdk inspect-artifact "$VIVY_USER_HOME/artifact"
"$VIVY_USER_HOME/artifact/vivy"
```

Open `http://127.0.0.1:8787` for the packaged page. Inspect must select
`vivy/minimal-chat` / `vivy.minimal-chat.root` through `std/ui-root@v1`, with
PresentationHost and the protected tools/provider profiles. The selected root
uses FullUIHost and FaceClientStore; it adds no Face runner or RPC transport.

Enter the absolute `workspace` path below your disposable home before sending
the first message. Ask to list that directory and read nonce.txt. The page shows
your message, tool output, the final answer and Completed. While Running, Send
is disabled and Cancel interrupts that Run. A provider failure displays Failed
and an error. The draft remains editable; an admission failure preserves it.
Workspace binding stays with its Session. Read-only tools need no extra Grant;
effectful tools are not enabled by this example's config.

For split development of this selected root (gateway above stays running):

```sh
go run ./sdk stage-ui --recipe recipes/minimal-chat.vivy.yml --output "$VIVY_USER_HOME/ui"
export VIVY_UI_ASSEMBLY_ENTRY="$VIVY_USER_HOME/ui/assembly.ts"
export VIVY_BACKEND_ADDR=http://127.0.0.1:8787
pnpm --dir ui dev --host 127.0.0.1 --strictPort
```

Open `http://127.0.0.1:3015`. Use this split page for development verification.
The environment-selected generated Assembly keeps the same root as the packed
gateway; do not edit generated UI copies.

Deterministic browser gate, from another shell with Go/Node on PATH, after
stopping the above servers (the test binds 8798/8797 and Vite 3015):

```sh
export VIVY_MINIMAL_CHAT_ARTIFACT="$VIVY_USER_HOME/artifact"
export VIVY_UI_ASSEMBLY_ENTRY="$VIVY_USER_HOME/ui/assembly.ts"
# Chromium also needs the host's standard Playwright libraries and fonts.
pnpm --dir ui exec playwright install chromium
pnpm --dir ui exec playwright test --config playwright.minimal.config.ts
```

The test uses a deterministic loopback model and dummy credential, isolated
SQLite/workspaces/home, the actual packed gateway, protected list/read, exact
tool output in request three, cancel, provider failure and a 390px viewport.
It creates a fresh temp directory prefixed `vivy-minimal-chat-`; remove that
test-owned directory after inspection. Real-token human acceptance belongs to
D14. Stop the split pair before cleanup as in the minimal example.
