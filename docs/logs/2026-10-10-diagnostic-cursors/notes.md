# Diagnostic cursor continuity

Ordinary appends changed the file mtime, which the reader treated as a new file. A client that read `one` from `one/two`, then appended `three`, received `one` again with `gap:true`. The cursor also carried the previous size without using it, so truncation could go unnoticed when the remaining file still extended beyond the cursor offset and retained its timestamp.

Keep the existing cursor envelope and compare the opened file identity, observed size, and offset. Same-file growth resumes from the cursor; a changed identity, shrinking size, or offset beyond EOF restarts with a gap. A same-size rewrite with a changed mtime remains invalidated. Windows obtains the actual file index from the opened handle rather than using creation time. Unix retains its inode identity. Previously issued Windows cursors reset once because the identity source changes. Scanning, record clipping, filtering, and pagination bounds remain unchanged.

Regression coverage exercises appends while a page is outstanding, truncation above the cursor offset with unchanged mtime, and replacement preserving size and mtime, in addition to the existing pagination/truncation coverage.

File metadata cannot detect a file truncated and regrown to the same or a larger size between reads, or an in-place rewrite that preserves both size and mtime. Unsupported platforms retain the existing zero-identity fallback. No content fingerprints or additional reads were introduced.
