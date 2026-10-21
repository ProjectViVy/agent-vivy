# Minimal kernel with real protected tools

Follow the toolchain setup in [the examples index](../README.md). From the
repository root, in a Bash shell, with `DEEPSEEK_API_KEY` already exported:

```sh
export VIVY_USER_HOME="$(mktemp -d /tmp/vivy-minimal.XXXXXX)"
export VIVY_CONFIG="$VIVY_USER_HOME/config.json"
export VIVY_ADDR=127.0.0.1:8787
export VIVY_PROVIDER=deepseek
export VIVY_MODEL=deepseek-flash
export VIVY_API_BASE=https://api.deepseek.com
python3 - <<'PY'
import json, os, pathlib
home = pathlib.Path(os.environ['VIVY_USER_HOME'])
(home / 'workspace').mkdir()
(home / 'workspace' / 'nonce.txt').write_text('minimal-example-42\n')
config = {'server': {'addr': os.environ['VIVY_ADDR']},
          'storage': {'backend': 'sqlite', 'sqlite': {'path': str(home / 'vivy.db')}},
          'runtime': {'workspace_root': str(home / 'workspace'), 'cache_warming': 'off'},
          'tools': {'enabled': ['read_file', 'list_dir']}}
pathlib.Path(os.environ['VIVY_CONFIG']).write_text(json.dumps(config))
PY
printf 'Example home: %s\n' "$VIVY_USER_HOME"
go run ./sdk pack --recipe recipes/minimal.vivy.yml --output "$VIVY_USER_HOME/artifact"
go run ./sdk inspect-artifact "$VIVY_USER_HOME/artifact"
"$VIVY_USER_HOME/artifact/vivy"
```

Inspect must show protected-tools and provider-profiles, with no selected UI
root. Leave the gateway running. In another terminal, from the repository root, export VIVY_USER_HOME to the exact
disposable path printed in the first shell (do not create another home):

```sh
go run ./examples/minimal/bind-workspace --addr 127.0.0.1:8787 --workspace "$VIVY_USER_HOME/workspace"
go run -tags=vivy_headless ./cmd/vivy tui --addr 127.0.0.1:8787
```

The bootstrap prints the confirmed Session ID and workspace. The existing live
terminal opens the newest Session, titled `Minimal example`; select that Session
in its picker if needed. Do not create a new unbound Session for this exchange.
Ask: `Use list_dir on ., then read_file on nonce.txt and quote its content.`
The real tool output must contain `minimal-example-42`; the next model answer
uses that output. A completed Run returns to idle. Escape/cancel uses the
existing terminal controller, not another agent loop.

Stop both processes with Ctrl+C. Delete only the exact disposable directory
printed by `printf '%s\n' "$VIVY_USER_HOME"` after inspecting that path. No
tenant installation or source database is changed. Choose another fresh home
to rerun; pack outputs must not already exist.

Deterministic acceptance without a key:

```sh
go test -tags=vivy_headless ./sdk/internal -run '^TestMinimalCausalExchange$' -count=1 -timeout 5m
just headless-compile
```
