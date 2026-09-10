# hello-fs

Sample v1 Module. It exposes `hello_stat`, which reads a workspace-relative
path through the focused `sdk/port/toolworld.Host` contract.

```
vivy-sdk verify plugins/hello-fs
```

This directory is source only. Pack it into a new generation:

```
vivy-sdk pack --recipe recipes/hello-fs.vivy.yml --source plugins/hello-fs --output dist/hello-fs
```
