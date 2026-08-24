# hello-fs

Sample user plugin for S5. It exposes `hello_stat`, which reads a
workspace-relative path through `sdk/plugin.Env`.

```
vivy-sdk verify plugins/hello-fs
```

This directory is source only. Pack it into a new generation:

```
vivy-sdk pack --with hello-fs --out dist/hello-fs
```
