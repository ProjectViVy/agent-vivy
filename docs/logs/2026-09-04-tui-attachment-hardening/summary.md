# TUI attachment path hardening

## Delivered

- Rejected colon-bearing paths on every host, closing Windows drive-relative
  and alternate-data-stream spellings before filesystem access.
- Applied the existing project-context sensitive-path policy to the requested
  path. Credential files, VCS metadata, and key material cannot enter image
  attachments; every symlink/junction is rejected before its target is probed.
- Walked every path component with rooted `Lstat`, required a regular file
  before opening, used non-blocking rooted opens on Unix, and compared the pre-open file,
  opened handle, end-of-read handle, and post-read named entry. Replacement,
  escape, or mutation during the read fails closed.
- On Windows, derived the final normalized path from the opened file handle
  and reapplied the sensitive-path policy, preventing NTFS 8.3 aliases from
  bypassing credential and key-material exclusions.
- Preserved the existing content-sniffed MIME allowlist, count/size bounds,
  server-side re-resolution for `turn/start`, and non-disclosing RPC errors.

## Scope

This is Vivy kernel/RPC attachment hardening. It does not change Studio,
tenant journals, attachment formats, image-model capability rules, or TUI
presentation.
