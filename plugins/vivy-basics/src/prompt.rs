//! Recipe-selected roles, with human-editable Markdown overrides.
use rig_harness::prelude::*;
use std::{io, path::Path};

#[derive(Resource)]
struct DefaultRole {
    file: &'static str,
    text: &'static str,
}

#[derive(Component)]
struct SelectedRole(String);

/// Coding guidance selected at build time by the recipe.
#[derive(Default)]
pub struct VivyPromptPlugin;

impl Plugin for VivyPromptPlugin {
    fn build(&self, app: &mut App) {
        app.init_resource::<ProjectRoot>()
            .init_resource::<RigHome>();
        app.insert_resource(DefaultRole {
            file: "CODING.md",
            text: include_str!("../../../prompts/CODING.md"),
        });
        app.add_systems(AgentPass, default_prompt.in_set(PrepareSystems::Compose));
    }
}

/// The persistent assistant gets its base identity from Laputa's persona files.
/// Read when forming a new operation. The caller retains it for retries.
/// Explicit project refusal also covers files created after startup discovery.
pub fn load(world: &mut World, name: &str, fallback: &str) -> io::Result<String> {
    let home = RigHome::of(world).root().to_path_buf();
    let root = world
        .get_resource::<ProjectRoot>()
        .map(|r| r.0.clone())
        .unwrap_or_else(|| std::env::current_dir().unwrap_or_default());
    let trusted = world
        .get_resource::<crate::trust::ProjectTrust>()
        .is_none_or(|trust| trust.trusted);
    read(&root, &home, trusted, name, fallback)
}

/// Live retries retain the selection on their turn. Durable recovery restores
/// the entire saved request later; it must not fail on a newly edited file first.
pub fn load_role(
    world: &mut World,
    call: Entity,
    name: &str,
    fallback: &str,
) -> io::Result<String> {
    let turn = world.get::<CallOf>(call).map(|of| of.0);
    let retry = turn
        .and_then(|e| world.get::<rig_harness::rig_ecs::turn::Recovery>(e))
        .is_some_and(|r| r.retries > 0);
    if retry {
        return Ok(turn
            .and_then(|e| world.get::<SelectedRole>(e))
            .map(|r| r.0.clone())
            .unwrap_or_else(|| fallback.trim().into()));
    }
    let text = load(world, name, fallback)?;
    if let Some(turn) = turn {
        world.entity_mut(turn).insert(SelectedRole(text.clone()));
    }
    Ok(text)
}

fn read(root: &Path, home: &Path, trusted: bool, name: &str, fallback: &str) -> io::Result<String> {
    let paths = trusted
        .then(|| root.join(".rig").join(name))
        .into_iter()
        .chain(std::iter::once(home.join(name)));
    for path in paths {
        let text = match std::fs::read_to_string(&path) {
            Ok(text) => text,
            Err(e) if e.kind() == io::ErrorKind::NotFound => continue,
            Err(e) => {
                return Err(io::Error::new(
                    e.kind(),
                    format!("Cannot read prompt {}: {e}", path.display()),
                ));
            }
        };
        let text = text.trim_start_matches('\u{feff}').trim();
        if text.is_empty() {
            return Err(io::Error::new(
                io::ErrorKind::InvalidData,
                format!(
                    "Prompt {} is empty; restore its text or remove the override",
                    path.display()
                ),
            ));
        }
        return Ok(text.to_owned());
    }
    Ok(fallback.trim().to_owned())
}

/// Inspect an editable role through the same project/home/default precedence.
pub fn inspect_file(
    root: &Path,
    home: &Path,
    trusted: bool,
    name: &str,
) -> io::Result<(String, String, String)> {
    let fallback = match name {
        "RECAP.md" => include_str!("../../../prompts/RECAP.md"),
        "AUTODREAM.md" => include_str!("../../../prompts/AUTODREAM.md"),
        _ => return Err(io::Error::other("unsupported editable prompt")),
    };
    let text = read(root, home, trusted, name, fallback)?;
    let source = if trusted && root.join(".rig").join(name).exists() {
        "workspace"
    } else if home.join(name).exists() {
        "home"
    } else {
        "default"
    };
    let revision = crate::setup::fingerprint(format!("{source}\0{text}").as_bytes());
    Ok((source.into(), text, revision))
}

/// Replace the effective override with optimistic conflict detection.
pub fn save_file(
    root: &Path,
    home: &Path,
    trusted: bool,
    name: &str,
    revision: &str,
    text: &str,
) -> io::Result<()> {
    if text.trim().is_empty() || text.len() > 256 * 1024 {
        return Err(io::Error::other(
            "prompt must be nonempty and at most 256 KiB",
        ));
    }
    std::fs::create_dir_all(home)?;
    let lock = std::fs::OpenOptions::new()
        .create(true)
        .truncate(false)
        .write(true)
        .open(home.join("prompt.lock"))?;
    lock.lock()?;
    let (source, _, current) = inspect_file(root, home, trusted, name)?;
    if current != revision {
        return Err(io::Error::other("prompt changed; reload before saving"));
    }
    let directory = if source == "workspace" {
        root.join(".rig")
    } else {
        home.to_path_buf()
    };
    std::fs::create_dir_all(&directory)?;
    rig_harness::rig_tools::fs::write_atomic(&directory.join(name), text.as_bytes())
}

fn default_prompt(
    mut calls: Query<
        (Entity, Option<&CallOf>, &mut ModelRequest),
        (
            Added<Admitted>,
            Without<Restored>,
            Or<(With<TurnCall>, With<SideCall>)>,
        ),
    >,
    turns: Query<(
        Option<&rig_harness::rig_ecs::turn::Recovery>,
        Option<&SelectedRole>,
    )>,
    role: Res<DefaultRole>,
    root: Res<ProjectRoot>,
    home: Res<RigHome>,
    trust: Option<Res<crate::trust::ProjectTrust>>,
    replaced: Option<Res<crate::context::ReplacedPrompt>>,
    mut commands: Commands,
) {
    // SYSTEM.md and --system-prompt remain explicit role replacements.
    if replaced.is_some_and(|r| r.0.is_some()) {
        return;
    }
    let trusted = trust.is_none_or(|t| t.trusted);
    for (call, of, mut request) in &mut calls {
        let Some(prompt) = request.prompt.as_mut().filter(|p| p.role.is_empty()) else {
            continue;
        };
        let turn = of.and_then(|of| turns.get(of.0).ok());
        let retry = turn.is_some_and(|(recovery, _)| recovery.is_some_and(|r| r.retries > 0));
        let selected = if retry {
            Ok(turn
                .and_then(|(_, selected)| selected)
                .map(|s| s.0.clone())
                .unwrap_or_else(|| role.text.trim().into()))
        } else {
            read(&root.0, home.0.root(), trusted, role.file, role.text)
        };
        match selected {
            Ok(text) => {
                if !retry && let Some(of) = of {
                    commands.entity(of.0).insert(SelectedRole(text.clone()));
                }
                prompt.role = text;
            }
            Err(error) => {
                commands
                    .entity(call)
                    .insert(Outcome::refused(error.to_string()));
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use rig_core::test_utils::MockCompletionModel;
    use rig_harness_test_support::{MemoryStore, app_on, connect, first_agent, reply, run_until};

    #[test]
    fn role_composition_has_no_order_conflicts_with_file_tools() {
        let (mut app, _) = app_on(&MemoryStore::default());
        app.add_plugins((
            AgentPlugin,
            SessionLogPlugin,
            VivyPromptPlugin,
            rig_basics::coding_tools::ReadTool,
            rig_basics::coding_tools::AttachPlugin,
        ));
        app.finish();
        let conflicts = rig_harness::stage_conflicts(app.world_mut());
        assert!(
            !conflicts.iter().any(|c| c.contains("default_prompt")),
            "{conflicts:?}"
        );
    }

    #[test]
    fn markdown_edits_apply_to_new_calls_and_project_trust_is_respected() {
        let root =
            std::env::temp_dir().join(format!("vivy-editable-prompt-{}", std::process::id()));
        let home = root.join("home");
        std::fs::create_dir_all(root.join(".rig")).unwrap();
        std::fs::create_dir_all(&home).unwrap();
        std::fs::write(home.join("CODING.md"), "HOME-ROLE").unwrap();
        let model =
            MockCompletionModel::from_stream_turns([reply("one"), reply("two"), reply("three")]);
        let (mut app, wakes) = app_on(&MemoryStore::default());
        app.insert_resource(ProjectRoot(root.clone()));
        app.insert_resource(RigHome(rig_harness::harness_protocol::Home::at(home)));
        app.insert_resource(crate::trust::ProjectTrust::default());
        app.add_plugins((AgentPlugin, SessionLogPlugin, VivyPromptPlugin));
        app.finish();
        app.update();
        let agent = first_agent(app.world_mut()).unwrap();
        app.world_mut()
            .entity_mut(agent)
            .insert(connect(&model).unwrap());
        // Creating a file after trust discovery must not bypass that decision.
        std::fs::write(root.join(".rig/CODING.md"), "PROJECT-ONE").unwrap();
        for (index, expected) in ["HOME-ROLE", "PROJECT-ONE", "PROJECT-TWO"]
            .iter()
            .enumerate()
        {
            if index == 1 {
                app.world_mut()
                    .resource_mut::<crate::trust::ProjectTrust>()
                    .trusted = true;
            }
            if index == 2 {
                std::fs::write(root.join(".rig/CODING.md"), "PROJECT-TWO").unwrap();
            }
            app.world_mut()
                .trigger(Deliver::new(agent, "hello", DeliveryMode::Queue));
            run_until(&mut app, &wakes, |world| {
                !world.entity(agent).contains::<ActiveTurn>() && model.request_count() == index + 1
            });
            let sent = format!("{:?}", model.requests()[index].chat_history.first());
            assert!(sent.contains(expected), "{sent}");
        }
        std::fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn default_identity_reaches_the_model_without_replacing_a_custom_role() {
        let model = MockCompletionModel::from_stream_turns([reply("one"), reply("two")]);
        let (mut app, wakes) = app_on(&MemoryStore::default());
        app.add_plugins((AgentPlugin, SessionLogPlugin, VivyPromptPlugin));
        app.finish();
        app.update();
        let agent = first_agent(app.world_mut()).unwrap();
        app.world_mut()
            .entity_mut(agent)
            .insert(connect(&model).unwrap());
        app.world_mut()
            .trigger(Deliver::new(agent, "hello", DeliveryMode::Queue));
        run_until(&mut app, &wakes, |world| {
            !world.entity(agent).contains::<ActiveTurn>() && model.request_count() == 1
        });
        let first = format!("{:?}", model.requests()[0].chat_history);
        assert!(first.contains("You are Vivy, a coding agent"), "{first}");
        assert!(!first.contains("You are rig,"));

        app.world_mut()
            .entity_mut(agent)
            .insert(SystemPrompt("CUSTOM-OWNER-ROLE".into()));
        app.world_mut()
            .trigger(Deliver::new(agent, "again", DeliveryMode::Queue));
        run_until(&mut app, &wakes, |world| {
            !world.entity(agent).contains::<ActiveTurn>() && model.request_count() == 2
        });
        let second = format!("{:?}", model.requests()[1].chat_history);
        assert!(second.contains("CUSTOM-OWNER-ROLE"));
        assert!(!second.contains("You are Vivy,"));
    }
}
