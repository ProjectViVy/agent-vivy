# Acceptance

1. Resolve an ordinary project-relative PNG/JPEG/GIF/WebP. Metadata is
   returned and `turn/start` can re-resolve the same content.
2. Resolve `photo.png:secret`, a drive/UNC path, traversal, or an absolute
   path. It is rejected before opening a file.
3. Resolve `.env`, `credentials.json`, key material, VCS metadata, or an
   innocuous symlink to one of those targets. Sensitive lexical paths and all
   links are rejected; the client error contains neither supplied path nor
   host root, and an external UNC target is never probed.
4. Replace the named entry between validation and post-read identity checking.
   The resolver returns `file changed while reading` and discards all bytes.
5. On Windows, address `credentials.json` through its available 8.3 alias. It
   is still rejected as sensitive. On Unix, replace a checked regular file
   with a FIFO immediately before open; resolution returns without blocking.
6. Existing count, 5 MiB size, regular-file, content-MIME, and display-name
   bounds remain enforced.
