//! Laputa persona authority: six core files, assembly and injection.
mod onboarding;
mod persona;
pub use onboarding::{Draft, PROMPT, extract, fallback, parse, write};
pub use persona::{
    ASSEMBLY_FILES, AUTO_FILES, DARK, DREAM, IDENTITY, MAX_FILE_BYTES, MAX_PERSONA_BYTES, MISSION,
    OPTIONAL_FILES, PERSONA_ID, Persona, PersonaStore, REDLINE, RELATIONSHIP, auto, digest_files,
    digest_text, required, root_from,
};

use rig_harness::prelude::*;
use serde::{Deserialize, Serialize};
use std::{fs, io, path::PathBuf};

#[derive(Resource, Clone)]
pub struct PersonaSource(pub PersonaStore);
#[derive(Component, Clone, Debug, Serialize, Deserialize, PartialEq, Eq)]
pub struct PersonaRevision {
    pub persona_id: String,
    pub revision: u64,
    #[serde(default)]
    pub sources_digest: String,
}
/// Explicit internal roles such as Dream do not inherit conversational persona.
#[derive(Component)]
pub struct PersonaExcluded;
#[derive(Default)]
pub struct LaputaPlugin;
#[derive(SystemSet, Clone, Debug, PartialEq, Eq, Hash)]
pub struct PersonaCompose;
impl Plugin for LaputaPlugin {
    fn build(&self, app: &mut App) {
        app.add_systems(First, source)
            .add_systems(
                AgentPass,
                compose
                    .in_set(PrepareSystems::AdaptOutermost)
                    .in_set(PersonaCompose),
            )
            .add_command(
                "persona",
                "Persona show [NAME] | update <revision> <NAME> <file> | rollback <revision> <target>",
                command,
            );
    }
}

fn source(world: &mut World) {
    if !world.contains_resource::<PersonaSource>() {
        let workspace = world
            .get_resource::<ProjectRoot>()
            .map(|r| r.0.clone())
            .unwrap_or_else(|| std::env::current_dir().unwrap_or_default());
        world.insert_resource(PersonaSource(PersonaStore::global(&workspace)));
    }
}

fn compose(
    mut calls: Query<(Entity, &ForAgent, &mut ModelRequest), (Added<Admitted>, Without<Restored>)>,
    excluded: Query<(), With<PersonaExcluded>>,
    source: Res<PersonaSource>,
    mut commands: Commands,
) {
    for (entity, agent, mut request) in &mut calls {
        if excluded.contains(agent.0) {
            continue;
        }
        let Some(prompt) = &mut request.prompt else {
            continue;
        };
        match source.0.read() {
            Ok(persona) => {
                let text = persona.text();
                if !text.is_empty() {
                    prompt.role = if prompt.role.is_empty() {
                        format!("<persona>\n{text}\n</persona>")
                    } else {
                        format!("<persona>\n{text}\n</persona>\n\n{}", prompt.role)
                    };
                }
                commands.entity(entity).insert(PersonaRevision {
                    persona_id: persona.persona_id,
                    revision: persona.revision,
                    sources_digest: persona.sources_digest,
                });
            }
            Err(e) => {
                commands.entity(entity).insert(Outcome::refused(format!(
                    "Cannot load managed persona: {e}"
                )));
            }
        }
    }
}

fn command(In(args): In<CommandArgs>, world: &mut World) {
    source(world);
    let store = world.resource::<PersonaSource>().0.clone();
    let mut parts = args.args.split_whitespace();
    let action = parts.next().unwrap_or("");
    let result = (|| -> io::Result<String> {
        if matches!(action, "" | "show") {
            let persona = store.read()?;
            if let Some(name) = parts.next() {
                if !ASSEMBLY_FILES.contains(&name) {
                    return Err(io::Error::new(
                        io::ErrorKind::InvalidData,
                        format!("unknown persona file: {name}"),
                    ));
                }
                return Ok(format!(
                    "Persona {} revision {}\n{}\n{}",
                    persona.persona_id,
                    persona.revision,
                    name,
                    persona.file(name)
                ));
            }
            return Ok(format!(
                "Persona {} revision {}\n{}",
                persona.persona_id,
                persona.revision,
                persona.text()
            ));
        }
        let revision = parts
            .next()
            .and_then(|s| s.parse::<u64>().ok())
            .ok_or_else(|| invalid("expected persona revision"))?;
        let target = parts
            .next()
            .ok_or_else(|| invalid("expected persona file name or rollback target"))?;
        let id = format!(
            "human:{}:{}",
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .map_err(io::Error::other)?
                .as_nanos()
        );
        let persona = match action {
            "update" => {
                let path = parts
                    .next()
                    .ok_or_else(|| invalid("expected persona file path"))?;
                let current = store.read()?;
                store.save_file(
                    revision,
                    &current.last_update_id,
                    &id,
                    target,
                    &fs::read_to_string(path.trim_matches('"'))?,
                    vec!["human".into()],
                )?
            }
            "rollback" => store.rollback(
                revision,
                target
                    .parse()
                    .map_err(|_| invalid("invalid rollback target"))?,
                &id,
            )?,
            _ => {
                return Err(invalid(
                    "use persona show [NAME], update <revision> <NAME> <file>, or rollback <revision> <target>",
                ));
            }
        };
        Ok(format!(
            "Persona {} revision {}\n{}",
            persona.persona_id,
            persona.revision,
            persona.text()
        ))
    })();
    match result {
        Ok(text) => {
            world.write_message(Notice::info(args.agent, text));
        }
        Err(e) => {
            world.write_message(Notice::error(args.agent, format!("Persona: {e}")));
        }
    }
}

fn invalid(text: &str) -> io::Error {
    io::Error::new(io::ErrorKind::InvalidData, text)
}

#[cfg(test)]
mod tests;
