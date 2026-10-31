//! Context files and system-prompt files as Pi reads them, in place of
//! Rig's `AgentsMdPlugin`:
//!
//! - In `RIG_HOME` and in the working directory and every directory above
//!   it, the first of `AGENTS.override.md`, `AGENTS.md`, `AGENTS.MD`,
//!   `CLAUDE.md`, `CLAUDE.MD`: an override replaces the others only in its
//!   own directory. The user's comes first, then from the outermost
//!   directory down to the working one, so the nearest is read last.
//! - `SYSTEM.md` replaces the default system prompt and `APPEND_SYSTEM.md`
//!   adds to it: the project's `.rig/` file wins over `RIG_HOME`'s, and two
//!   files of one name are not combined. An agent with a prompt of its own
//!   keeps it.
//! - `--system-prompt <text|path>` stands in for `SYSTEM.md`;
//!   `--append-system-prompt <text|path>`, repeatable, for `APPEND_SYSTEM.md`.
//!   What is appended comes right after the prompt, before the context files.
//!
//! The files are read once at start, so the prompt stays the same for the
//! session and the provider's prompt cache keeps it.

use std::path::{Path, PathBuf};

use bevy_log::warn;
use rig_harness::prelude::*;

/// The context files looked for in each directory, the first found used.
pub const CONTEXT_FILES: [&str; 5] = [
    "AGENTS.override.md",
    "AGENTS.md",
    "AGENTS.MD",
    "CLAUDE.md",
    "CLAUDE.MD",
];

const SYSTEM_FLAG: &str = "system-prompt";
const APPEND_FLAG: &str = "append-system-prompt";

/// Context files, `SYSTEM.md`, `APPEND_SYSTEM.md` and their flags.
#[derive(Default)]
pub struct ContextFilesPlugin;

/// The prompt that replaces the default one, when any.
#[derive(Resource, Clone, Debug, Default)]
pub struct ReplacedPrompt(pub Option<String>);

impl Plugin for ContextFilesPlugin {
    fn build(&self, app: &mut App) {
        app.init_resource::<ProjectRoot>()
            .add_flag(
                Flag::new(
                    SYSTEM_FLAG,
                    "Replace the system prompt with this text or file",
                )
                .value("TEXT|PATH"),
            )
            .add_flag(
                Flag::new(
                    APPEND_FLAG,
                    "Add this text or file to the system prompt; repeatable",
                )
                .value("TEXT|PATH"),
            )
            .add_systems(AgentPass, replace_prompt.in_set(PrepareSystems::Edit));
    }

    fn finish(&self, app: &mut App) {
        let world = app.world_mut();
        let home = RigHome::of(world).root().to_path_buf();
        let root = world.resource::<ProjectRoot>().0.clone();
        let flags: Vec<(String, Option<String>)> = world
            .get_resource::<Invoked>()
            .map(|invoked| invoked.args.flags.clone())
            .unwrap_or_default();
        let given = |name: &str| -> Vec<String> {
            flags
                .iter()
                .filter(|(flag, _)| flag == name)
                .filter_map(|(_, value)| value.as_deref().map(|value| text_or_file(value, &root)))
                .collect()
        };
        let trust = world.get_resource::<crate::trust::ProjectTrust>().cloned();
        let allowed = |path: &Path| trust.as_ref().is_none_or(|trust| trust.allows(path));
        let files = context_files(&root, &home);
        world.spawn((
            Name::new("prompt:project_instructions"),
            PromptSection::new(
                PromptSection::ORDER_PROJECT,
                "project_instructions",
                section(&files),
            ),
        ));
        let system = given(SYSTEM_FLAG)
            .pop()
            .or_else(|| prompt_file_if(&root, &home, "SYSTEM.md", allowed));
        world.insert_resource(ReplacedPrompt(system));
        // As Pi: the flags, when given, stand in for the file.
        let mut appended = given(APPEND_FLAG);
        if appended.is_empty() {
            appended.extend(prompt_file_if(&root, &home, "APPEND_SYSTEM.md", allowed));
        }
        world.spawn((
            Name::new("prompt:append_system"),
            PromptSection::new(
                PromptSection::ORDER_ROLE + 50,
                "append_system",
                appended.join("\n\n"),
            ),
        ));
    }
}

/// The context files that apply in `dir`, the order the model reads them:
/// `home`'s, then from the outermost directory above `dir` down to `dir`.
/// A file reached twice, such as through a symlink, is listed once.
pub fn context_files(dir: &Path, home: &Path) -> Vec<PathBuf> {
    let mut places: Vec<&Path> = dir.ancestors().collect();
    places.push(home);
    places.reverse();
    let mut files = Vec::new();
    let mut seen = Vec::new();
    for place in places {
        let Some(file) = CONTEXT_FILES
            .iter()
            .map(|name| place.join(name))
            .find(|file| file.is_file())
        else {
            continue;
        };
        let real = file.canonicalize().unwrap_or_else(|_| file.clone());
        if !seen.contains(&real) {
            seen.push(real);
            files.push(file);
        }
    }
    files
}

/// The project's `.rig/<name>`, else `home`'s `<name>`, read; none when
/// neither is there or readable.
pub fn prompt_file(root: &Path, home: &Path, name: &str) -> Option<String> {
    prompt_file_if(root, home, name, |_| true)
}

/// [`prompt_file`] of the files `allowed` lets load.
pub fn prompt_file_if(
    root: &Path,
    home: &Path,
    name: &str,
    allowed: impl Fn(&Path) -> bool,
) -> Option<String> {
    [root.join(".rig").join(name), home.join(name)]
        .into_iter()
        .find(|file| file.is_file() && allowed(file))
        .and_then(|file| match std::fs::read_to_string(&file) {
            Ok(text) => Some(text.trim().to_owned()),
            Err(error) => {
                warn!("{} not read: {error}", file.display());
                None
            }
        })
        .filter(|text| !text.is_empty())
}

/// A flag's value: the file it names, relative to `root`, when there is
/// one, else the text itself.
pub fn text_or_file(value: &str, root: &Path) -> String {
    let path = root.join(value);
    match path.is_file().then(|| std::fs::read_to_string(&path)) {
        Some(Ok(text)) => text.trim().to_owned(),
        _ => value.to_owned(),
    }
}

/// The prompt section's text: each readable file, with its path. Empty,
/// which leaves the section out, when there is none.
fn section(files: &[PathBuf]) -> String {
    let texts: Vec<String> = files
        .iter()
        .filter_map(|file| match std::fs::read_to_string(file) {
            Ok(text) if !text.trim().is_empty() => Some(format!(
                "<file path=\"{}\">\n{}\n</file>",
                file.display(),
                text.trim()
            )),
            Ok(_) => None,
            Err(error) => {
                warn!("{} not read: {error}", file.display());
                None
            }
        })
        .collect();
    if texts.is_empty() {
        return String::new();
    }
    format!(
        "Instructions from the user and the project, in context files (AGENTS.md or CLAUDE.md): \
         the user's own first, then from the outermost directory down to the working directory. \
         Follow them; where two disagree, the later file wins.\n\n{}",
        texts.join("\n\n")
    )
}

/// Model calls the kernel just built from an agent: its turns' and side calls.
type BuiltCalls = (Added<Admitted>, Or<(With<TurnCall>, With<SideCall>)>);

/// Puts the replacing prompt in each model call built for an agent with no
/// prompt of its own, after every plugin composed it.
pub(crate) fn replace_prompt(
    mut calls: Query<(&mut ModelRequest, &ForAgent), BuiltCalls>,
    own: Query<&SystemPrompt>,
    replaced: Res<ReplacedPrompt>,
) {
    let Some(text) = &replaced.0 else {
        return;
    };
    for (mut call, &ForAgent(agent)) in &mut calls {
        if own.get(agent).is_ok_and(|own| !own.0.is_empty()) {
            continue;
        }
        if let Some(prompt) = &mut call.prompt {
            prompt.role = text.clone();
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn tree(name: &str) -> PathBuf {
        let dir = std::env::temp_dir().join(format!("vivy-context-{name}-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&dir);
        std::fs::create_dir_all(dir.join("home")).unwrap();
        std::fs::create_dir_all(dir.join("repo/sub")).unwrap();
        dir
    }

    #[test]
    fn reads_one_file_per_directory_in_pis_order() {
        let dir = tree("order");
        std::fs::write(dir.join("home/AGENTS.md"), "user").unwrap();
        std::fs::write(dir.join("repo/CLAUDE.md"), "repo claude").unwrap();
        std::fs::write(dir.join("repo/sub/AGENTS.md"), "sub agents").unwrap();
        std::fs::write(dir.join("repo/sub/AGENTS.override.md"), "sub override").unwrap();
        let files = context_files(&dir.join("repo/sub"), &dir.join("home"));
        let names: Vec<PathBuf> = files
            .iter()
            .map(|f| f.strip_prefix(&dir).unwrap().to_path_buf())
            .collect();
        std::fs::remove_dir_all(&dir).unwrap();
        assert_eq!(
            names,
            [
                PathBuf::from("home/AGENTS.md"),
                PathBuf::from("repo/CLAUDE.md"),
                PathBuf::from("repo/sub/AGENTS.override.md")
            ]
        );
    }

    #[test]
    fn the_projects_prompt_file_wins_and_flags_take_text_or_files() {
        let dir = tree("prompt");
        let root = dir.join("repo");
        std::fs::create_dir_all(root.join(".rig")).unwrap();
        std::fs::write(dir.join("home/SYSTEM.md"), "home system").unwrap();
        assert_eq!(
            prompt_file(&root, &dir.join("home"), "SYSTEM.md").as_deref(),
            Some("home system")
        );
        std::fs::write(root.join(".rig/SYSTEM.md"), "project system\n").unwrap();
        assert_eq!(
            prompt_file(&root, &dir.join("home"), "SYSTEM.md").as_deref(),
            Some("project system")
        );
        assert_eq!(
            prompt_file(&root, &dir.join("home"), "APPEND_SYSTEM.md"),
            None
        );
        std::fs::write(root.join("extra.md"), "from a file\n").unwrap();
        assert_eq!(text_or_file("extra.md", &root), "from a file");
        assert_eq!(text_or_file("Be brief.", &root), "Be brief.");
        std::fs::remove_dir_all(&dir).unwrap();
    }
}
