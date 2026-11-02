//! Project trust as Pi has it: a working folder's own resources load only
//! once the user trusts it. Protected here: the project's `.rig/prompts`,
//! `.rig/skills`, `.rig/agents`, `.agents/skills` (in the working directory or above),
//! `.rig/SYSTEM.md` and `.rig/APPEND_SYSTEM.md`. Context files (AGENTS.md,
//! CLAUDE.md) load regardless, as in Pi.
//!
//! The decision, first that applies: `--approve` or `--no-approve`; a saved
//! decision for the working directory or the nearest directory above it
//! (`RIG_HOME/trust.json`, written by `/trust`); else `[trust] default` in
//! `RIG_HOME/settings.toml`: `"always"`, `"never"` or `"ask"` (the default).
//! "ask" asks in the terminal view at start, and trusting restarts the
//! session so the resources load; a run nobody can answer skips them and
//! says so. Trust is not a sandbox: tools still act with the user's rights.

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use bevy_log::warn;
use rig_harness::prelude::*;
use serde::Deserialize;

const APPROVE: &str = "approve";
const NO_APPROVE: &str = "no-approve";
const FILE: &str = "trust.json";

/// `[trust]` in `RIG_HOME/settings.toml`; the user's alone, never a project's.
#[derive(Resource, Deserialize, Clone, Debug)]
#[serde(default, deny_unknown_fields)]
pub struct TrustSettings {
    /// `"ask"`, `"always"` or `"never"`.
    pub default: String,
}

impl Default for TrustSettings {
    fn default() -> Self {
        Self {
            default: "ask".to_owned(),
        }
    }
}

/// Whether this run loads the project's protected resources, and which
/// were found.
#[derive(Resource, Clone, Debug, Default)]
pub struct ProjectTrust {
    pub trusted: bool,
    /// No flag, saved decision or setting decided: the user is asked.
    pub undecided: bool,
    pub protected: Vec<PathBuf>,
}

impl ProjectTrust {
    fn from_decision(decided: Option<bool>, protected: Vec<PathBuf>) -> Self {
        Self {
            // Empty discovery is not permission to load resources created later.
            trusted: decided.unwrap_or(false),
            undecided: decided.is_none() && !protected.is_empty(),
            protected,
        }
    }

    /// Whether `path`, a resource a plugin would load, may be loaded: one
    /// that is not a protected resource always may.
    pub fn allows(&self, path: &Path) -> bool {
        self.trusted
            || !self
                .protected
                .iter()
                .any(|protected| path.starts_with(protected))
    }
}

/// Whether `path` may be loaded: as `ProjectTrust` says when the trust
/// plugin is in the build, else always.
pub fn allowed(world: &World, path: &Path) -> bool {
    world
        .get_resource::<ProjectTrust>()
        .is_none_or(|trust| trust.allows(path))
}

/// Decides project trust as the run starts; `/trust` saves a decision.
#[derive(Default)]
pub struct TrustPlugin;

impl Plugin for TrustPlugin {
    fn build(&self, app: &mut App) {
        app.init_resource::<ProjectRoot>()
            .add_flag(Flag::new(
                APPROVE,
                "Trust this project's resources for this run",
            ))
            .add_flag(Flag::new(
                NO_APPROVE,
                "Do not load this project's resources for this run",
            ))
            .add_setting::<TrustSettings>("trust")
            .add_command(
                "trust",
                "Trust this project from now on: /trust [yes|no|forget]",
                trust,
            )
            .add_systems(Update, ask_at_start)
            .add_systems(Update, answered);
        let world = app.world_mut();
        let home = RigHome::of(world).root().to_path_buf();
        let root = world.resource::<ProjectRoot>().0.clone();
        let flags: Vec<String> = world
            .get_resource::<Invoked>()
            .map(|invoked| {
                invoked
                    .args
                    .flags
                    .iter()
                    .map(|(flag, _)| flag.clone())
                    .collect()
            })
            .unwrap_or_default();
        let setting = world.resource::<TrustSettings>().default.clone();
        let saved = saved_decision(&read_decisions(&home), &root);
        let protected = protected(&root);
        let decided = if flags.iter().any(|flag| flag == NO_APPROVE) {
            Some(false)
        } else if flags.iter().any(|flag| flag == APPROVE) {
            Some(true)
        } else if saved.is_some() {
            saved
        } else {
            match setting.as_str() {
                "always" => Some(true),
                "never" => Some(false),
                "ask" => None,
                other => {
                    warn!("trust: [trust] default `{other}` is not ask, always or never; asking");
                    None
                }
            }
        };
        world.insert_resource(ProjectTrust::from_decision(decided, protected));
    }
}

/// The project's protected resources that exist: in `root`'s `.rig`, and
/// `.agents/skills` in `root` or a directory above it.
pub fn protected(root: &Path) -> Vec<PathBuf> {
    let mut found: Vec<PathBuf> = [
        "prompts",
        "skills",
        "agents",
        "SYSTEM.md",
        "APPEND_SYSTEM.md",
        "CODING.md",
        "RECAP.md",
        "AUTODREAM.md",
    ]
    .iter()
    .map(|name| root.join(".rig").join(name))
    .filter(|path| path.exists())
    .collect();
    found.extend(
        root.ancestors()
            .map(|dir| dir.join(".agents/skills"))
            .filter(|path| path.is_dir()),
    );
    found
}

/// The saved decisions, by directory.
pub fn read_decisions(home: &Path) -> BTreeMap<PathBuf, bool> {
    std::fs::read_to_string(home.join(FILE))
        .ok()
        .and_then(|text| serde_json::from_str(&text).ok())
        .unwrap_or_default()
}

/// The decision saved for `dir` or the nearest directory above it.
pub fn saved_decision(decisions: &BTreeMap<PathBuf, bool>, dir: &Path) -> Option<bool> {
    let dir = dir.canonicalize().unwrap_or_else(|_| dir.to_path_buf());
    dir.ancestors()
        .find_map(|place| decisions.get(place).copied())
}

/// Saves `decision` for `dir`; `None` forgets it.
pub fn save_decision(home: &Path, dir: &Path, decision: Option<bool>) -> std::io::Result<()> {
    let mut decisions = read_decisions(home);
    let dir = dir.canonicalize().unwrap_or_else(|_| dir.to_path_buf());
    match decision {
        Some(decision) => decisions.insert(dir, decision),
        None => decisions.remove(&dir),
    };
    std::fs::create_dir_all(home)?;
    std::fs::write(
        home.join(FILE),
        serde_json::to_string_pretty(&decisions).unwrap_or_default(),
    )
}

fn trust(
    In(args): In<CommandArgs>,
    home: Res<RigHome>,
    root: Res<ProjectRoot>,
    mut notices: MessageWriter<Notice>,
) {
    let decision = match args.args.trim() {
        "" | "yes" => Some(true),
        "no" => Some(false),
        "forget" => None,
        other => {
            notices.write(Notice::error(
                args.agent,
                format!("/trust takes yes, no or forget, not `{other}`."),
            ));
            return;
        }
    };
    let text = match decision {
        Some(true) => "This project is trusted from the next start on.",
        Some(false) => "This project's resources stay unloaded from the next start on.",
        None => "The decision for this project is forgotten; the next start asks again.",
    };
    match save_decision(home.0.root(), &root.0, decision) {
        Ok(()) => notices.write(Notice::info(args.agent, text)),
        Err(why) => notices.write(Notice::error(
            args.agent,
            format!("Could not save the decision: {why}."),
        )),
    };
}

/// The start question, on the main agent.
#[derive(Component)]
struct TrustQuestion;

/// The start question once answered.
type AnsweredQuestions = (With<TrustQuestion>, Added<Outcome>);

fn ask_at_start(
    mut trust: ResMut<ProjectTrust>,
    agents: MainAgentQuery,
    answerers: Answerers,
    mut done: Local<bool>,
    mut commands: Commands,
    mut notices: MessageWriter<Notice>,
) {
    if *done || !trust.undecided {
        *done = true;
        return;
    }
    let Some(agent) = main_agent(&agents) else {
        return;
    };
    *done = true;
    let listed: Vec<String> = trust
        .protected
        .iter()
        .map(|path| path.display().to_string())
        .collect();
    if !answerers.can_answer(Some(agent)) {
        trust.undecided = false;
        let text = format!(
            "Project resources not loaded, as this project is not trusted: {}. Run with --approve, or /trust.",
            listed.join(", ")
        );
        notices.write(Notice::info(agent, text));
        return;
    }
    let ask = Ask::new("Trust this project?")
        .body(format!(
            "It has resources that change what the agent is told and can do:\n{}",
            listed.join("\n")
        ))
        .choice("always", "Trust it, and remember")
        .choice("no", "Not now (--approve trusts one run)")
        .choice("never", "Never, and remember");
    commands.spawn((ask, ForAgent(agent), TrustQuestion));
}

fn answered(
    mut trust: ResMut<ProjectTrust>,
    asks: Query<(Entity, &Outcome), AnsweredQuestions>,
    home: Res<RigHome>,
    root: Res<ProjectRoot>,
    paths: Option<Res<SessionPaths>>,
    mut commands: Commands,
    mut notices: MessageWriter<Notice>,
) {
    for (ask, outcome) in &asks {
        trust.undecided = false;
        commands.entity(ask).remove::<TrustQuestion>();
        let answer = Answer::of(outcome).unwrap_or_default();
        let remembered = if answer.is("always") {
            Some(true)
        } else if answer.is("never") {
            Some(false)
        } else {
            None
        };
        if let Some(decision) = remembered
            && let Err(why) = save_decision(home.0.root(), &root.0, Some(decision))
        {
            notices.write(Notice::error(
                None,
                format!("Could not save the decision: {why}."),
            ));
        }
        // The resources load as the agent starts: start this session again,
        // or a new one while nothing was saved in this one yet.
        if answer.is("always") {
            let saved = paths.as_ref().filter(|paths| paths.0.is_saved());
            let again = saved.and_then(|paths| paths.0.id());
            commands.trigger(Exit(ExitReason::Switch(again)));
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn empty_resources_do_not_grant_trust_to_future_resources() {
        for decided in [None, Some(false)] {
            let trust = ProjectTrust::from_decision(decided, vec![]);
            assert!(
                !trust.trusted,
                "Empty discovery must not grant trust: {decided:?}"
            );
            assert!(!trust.undecided, "Nothing to approve at startup");
        }
        assert!(ProjectTrust::from_decision(Some(true), vec![]).trusted);
    }

    #[test]
    fn the_nearest_saved_decision_applies() {
        let dir = std::env::temp_dir().join(format!("vivy-trust-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&dir);
        let (home, outer) = (dir.join("home"), dir.join("outer"));
        let inner = outer.join("inner");
        std::fs::create_dir_all(&inner).unwrap();
        assert_eq!(saved_decision(&read_decisions(&home), &inner), None);
        save_decision(&home, &outer, Some(true)).unwrap();
        assert_eq!(saved_decision(&read_decisions(&home), &inner), Some(true));
        save_decision(&home, &inner, Some(false)).unwrap();
        assert_eq!(saved_decision(&read_decisions(&home), &inner), Some(false));
        save_decision(&home, &inner, None).unwrap();
        assert_eq!(saved_decision(&read_decisions(&home), &inner), Some(true));
        std::fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn finds_the_protected_resources() {
        let dir = std::env::temp_dir().join(format!("vivy-protected-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&dir);
        let work = dir.join("work");
        std::fs::create_dir_all(work.join(".rig/prompts")).unwrap();
        std::fs::create_dir_all(dir.join(".agents/skills")).unwrap();
        std::fs::write(work.join("AGENTS.md"), "not protected").unwrap();
        // Limit the assertion to this fixture; real ancestor skills are valid.
        let found: Vec<_> = protected(&work)
            .into_iter()
            .filter(|path| path.starts_with(&dir))
            .collect();
        std::fs::remove_dir_all(&dir).unwrap();
        assert_eq!(
            found,
            [work.join(".rig/prompts"), dir.join(".agents/skills")]
        );
        let trust = ProjectTrust {
            trusted: false,
            undecided: false,
            protected: found,
        };
        assert!(!trust.allows(&work.join(".rig/prompts/review.md")));
        assert!(
            !trust.allows(&dir.join(".agents/skills/demo/SKILL.md")),
            "a parent's skills too"
        );
        assert!(trust.allows(&work.join("AGENTS.md")));
        assert!(trust.allows(Path::new("/home/user/.rig/skills")));
    }
}
