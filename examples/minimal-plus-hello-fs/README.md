# Minimal plus hello-fs

Use [minimal's setup](../minimal/README.md) through creation of the disposable
config/nonce. Keep those shell variables. From the repository root:

```sh
python3 - <<'PY'
import json, os, pathlib
path = pathlib.Path(os.environ['VIVY_CONFIG'])
config = json.loads(path.read_text())
config['tools']['enabled'].append('hello_stat')
path.write_text(json.dumps(config))
PY
go run ./sdk verify plugins/hello-fs
go run ./sdk pack --recipe examples/minimal-plus-hello-fs/recipe.vivy.yml --output "$VIVY_USER_HOME/artifact"
go run ./sdk inspect-artifact "$VIVY_USER_HOME/artifact"
"$VIVY_USER_HOME/artifact/vivy"
```

The Recipe explicitly selects the build-owned source `vivy/hello-fs`, pinned by
its `repo:plugins/hello-fs` descriptor in the SDK source catalog; no directory
discovery or external source flag is needed. Inspect must show std/tool-world
Provider `vivy.hello-fs`, its sole ToolHost consumer and effective fs.read Grant.
It extends the nine-module minimal with one ToolWorld and no optional UI Host.

In another terminal, set VIVY_USER_HOME to the same disposable path, then bind
and connect through existing owners:

```sh
go run ./examples/minimal/bind-workspace --addr 127.0.0.1:8787 --workspace "$VIVY_USER_HOME/workspace"
go run -tags=vivy_headless ./cmd/vivy tui --addr 127.0.0.1:8787
```

Select the confirmed `Minimal example` Session (newest on startup), rather than
creating a new unbound Session. Ask: `Use hello_stat on nonce.txt, then report the result.`
The tool returns `nonce.txt 19` for the exact file created by minimal's setup.
The next model answer uses this real host-mediated file result. Absolute paths
and `../` escapes are rejected. This Module cannot shadow read_file or bypass
Policy, sandbox, workspace ownership or the durable operation path.

Stop both processes and clean only the inspected disposable directory, as in
minimal. A new run needs a fresh pack output. No plugin installation or change
to default/lite Recipes is involved.
