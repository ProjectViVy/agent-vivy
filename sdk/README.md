# vivy-sdk

Separate binary for cutting the next species body. Not a subcommand of
`vivy.exe`. Lives in this tree so the contract cannot fork from the
gateway, but it is built and shipped on its own — it may later embed
source snapshots or a Go toolchain and will be large.

```text
go build -o vivy-sdk.exe ./sdk

vivy-sdk verify plugins/hello-fs
vivy-sdk pack --with hello-fs --out dist/hello-fs
vivy-sdk inspect-artifact dist/hello-fs
```

| Path | Who may import it |
|---|---|
| `sdk/plugin` | User plugins only (`agent-vivy/sdk/plugin`) |
| `sdk/tui` | Shared presentation and durable-stream state for terminal face modules; no kernel authority |
| `sdk/internal` | This binary only |
| `sdk/main.go` | The `vivy-sdk` entry |

Pack still needs the species module root and a `go` toolchain (bundled
or host). A machine that only has `vivy.exe` cannot compile a generation.

Authors do not run this as the product. **Vivy Studio** invokes it
(`docs/architecture/VIVY-STUDIO.md`). After the ST-6 venue switch,
packing is done from Studio, not from an external shell as the primary
loop.
