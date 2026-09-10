# vivy-sdk

Separate binary for cutting the next species body. Not a subcommand of
`vivy.exe`. Lives in this tree so the contract cannot fork from the
gateway, but it is built and shipped on its own — it may later embed
source snapshots or a Go toolchain and will be large.

```text
go build -o vivy-sdk.exe ./sdk

vivy-sdk verify plugins/hello-fs
vivy-sdk pack --recipe recipes/default.vivy.yml --output dist/default-v1
vivy-sdk inspect-artifact dist/default-v1
```

| Path | Who may import it |
|---|---|
| `sdk/module`, `sdk/port/*` | Module lifecycle and focused public Port contracts |
| `sdk/tui` | Shared presentation and durable-stream state for terminal face modules; no kernel authority |
| `sdk/internal` | This binary only |
| `sdk/main.go` | The `vivy-sdk` entry |

Pack still needs the species module root and a `go` toolchain (bundled
or host). A machine that only has `vivy.exe` cannot compile a generation.

Authors do not run this as the product. **Vivy Studio** invokes it
(`docs/architecture/VIVY-STUDIO.md`). After the ST-6 venue switch,
packing is done from Studio, not from an external shell as the primary
loop.
