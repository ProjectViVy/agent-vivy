# Acceptance

A diagnostics client reads `one` from a log containing `one` and `two`, then the writer appends `three`. Resuming the issued cursor with a one-record page returns `two` with `gap:false` and another page available; the following page returns `three` with `gap:false` and no further records. Previously consumed lines do not reappear solely because the log grows.

If the file is replaced, or shrinks even while the cursor remains within the new file size, resuming reports `gap:true` and restarts from the first surviving record. Existing pagination, filtering, clipping, and GUI append/read behavior continues to pass. Focused automated checks passed; integrated control-plane smoke and full CI remain with the integrating root lane, and cross-compilation does not claim Windows or Darwin runtime execution.
