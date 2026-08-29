# acceptance — vivy-studio submodule migration

A human can tell it worked when:

1. **Remote shell repo is real**  
   Open https://github.com/ProjectViVy/vivy-studio — `main` has README plus
   `dsh-vivy-studio/`, `dsh-vivy-console/`, `dsh-plugin-hub/`, and community
   plugin directories (not an empty repo).

2. **Host no longer vendors Studio blobs**  
   On branch `feat/vivy-studio-submodule`:
   ```text
   git ls-files -s studio
   # → 160000 <sha> studio
   git ls-files studio | measure
   # → Count = 1
   ```

3. **First-boot install works**  
   ```text
   git submodule deinit -f studio
   # empty studio/
   just ensure-studio
   # restores studio/dsh-vivy-studio/package.json and siblings
   ```
   Or a fresh clone without `--recurse-submodules`, then
   `.\launch-vivy-studio.ps1` (calls ensure first).

4. **Paths humans already know still work**  
   Edit `studio/dsh-vivy-studio/theme.css` (after ensure). Launch still uses
   root `launch-vivy-studio.ps1` and profile `vivy-studio` with
   `DSH_HOME=data/studio-home`.

5. **Species lifecycle CLI still in host**  
   `cmd/vivy-studio/` and `just studio` still build `vivy-studio.exe` here;
   they were not moved to the shell repo.

6. **Commit workflow**  
   Shell changes: commit/push inside `studio/` (vivy-studio repo).  
   Host only: `git add studio && git commit` to bump the gitlink.
