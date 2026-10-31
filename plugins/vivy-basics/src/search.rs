//! Rig's `search` and `list` tools, which Rig's own coding tools leave to
//! the shell. `search` finds a regular expression in file contents, or
//! file names with `files`, honouring `.gitignore` (Pi's `grep` and
//! `find`); `list` lists a directory tree (Pi's `ls`). Each call works in
//! its agent's directory, as `read` does.

use rig_harness::prelude::*;
use rig_harness::rig_tools::{List, Search};

/// The `search` and `list` tools.
#[derive(Default)]
pub struct SearchTools;

impl Plugin for SearchTools {
    fn build(&self, app: &mut App) {
        // Both build each call for where it works: the workspace, the
        // calling agent's directory and its confinement.
        app.add_tool_per_call(Search::default(), ToolOptions::default());
        app.add_tool_per_call(List::default(), ToolOptions::default());
    }
}
