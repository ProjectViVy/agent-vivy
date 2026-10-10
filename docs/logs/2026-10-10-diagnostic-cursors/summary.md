# Diagnostic cursor continuity

Log pagination now resumes after ordinary appends instead of repeating earlier records with a false gap. Replacing a file or shrinking it below its previously observed size restarts the read with a gap, including truncation above the current cursor offset. Windows uses the opened file's actual index for stable identity; existing Windows cursors reset once after this change.

The scope is the logging diagnostics reader, its platform identity helpers, and regression coverage. Record clipping, scan limits, filtering, GUI writes, and the RPC contract are unchanged. No release or push was performed. Metadata-only detection limits and verification evidence are recorded in `notes.md` and `verification.md`.
