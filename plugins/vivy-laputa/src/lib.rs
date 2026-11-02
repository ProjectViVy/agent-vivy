//! Workspace persona authority; it owns neither task context nor memory.
use rig_memory::{DirFiles, Files};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::{
    fs::{self, File, OpenOptions},
    io,
    path::{Path, PathBuf},
};

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Persona {
    pub schema_version: u32,
    pub persona_id: String,
    pub revision: u64,
    pub text: String,
    pub last_update_id: String,
    pub sources: Vec<String>,
    pub parent_update_id: String,
}

pub struct PersonaStore {
    root: PathBuf,
}
impl PersonaStore {
    pub fn new(workspace: &Path) -> Self {
        Self {
            root: workspace.join(".vivy/laputa"),
        }
    }
    fn files(&self) -> DirFiles {
        DirFiles::new(&self.root)
    }
    fn empty(&self) -> Persona {
        Persona {
            schema_version: 1,
            persona_id: hash(self.root.to_string_lossy().as_bytes()),
            revision: 0,
            text: String::new(),
            last_update_id: String::new(),
            sources: vec![],
            parent_update_id: String::new(),
        }
    }
    pub fn read(&self) -> io::Result<Persona> {
        let bytes = match self.files().read_file("persona.json") {
            Ok(b) => b,
            Err(e) if e.kind() == io::ErrorKind::NotFound => return Ok(self.empty()),
            Err(e) => return Err(e),
        };
        let current: Persona = serde_json::from_slice(&bytes).map_err(io::Error::other)?;
        if current.schema_version != 1 || current.revision == 0 || current.last_update_id.is_empty()
        {
            return Err(invalid("invalid persona schema or revision"));
        }
        if self.history(&current.last_update_id)? != current {
            return Err(invalid(
                "persona differs from its immutable published revision",
            ));
        }
        Ok(current)
    }
    fn history(&self, id: &str) -> io::Result<Persona> {
        serde_json::from_slice(&self.files().read_file(&history_name(id))?)
            .map_err(io::Error::other)
    }
    fn lock(&self) -> io::Result<File> {
        fs::create_dir_all(&self.root)?;
        let f = OpenOptions::new()
            .create(true)
            .truncate(false)
            .read(true)
            .write(true)
            .open(self.root.join("writer.lock"))?;
        f.lock()?;
        Ok(f)
    }
    fn ancestors(&self, current: &Persona) -> io::Result<Vec<Persona>> {
        let mut all = vec![];
        let mut next = current.clone();
        while next.revision > 0 {
            let id = next.parent_update_id.clone();
            let rev = next.revision;
            all.push(next);
            if rev == 1 {
                if !id.is_empty() {
                    return Err(invalid("invalid initial persona parent"));
                }
                break;
            }
            next = self.history(&id)?;
            if next.revision + 1 != rev || next.last_update_id != id {
                return Err(invalid("invalid persona history chain"));
            }
        }
        Ok(all)
    }
    /// Only activated ancestry counts; a prepared revision is not adoption.
    pub fn activated(&self, id: &str) -> io::Result<Option<Persona>> {
        Ok(self
            .ancestors(&self.read()?)?
            .into_iter()
            .find(|p| p.last_update_id == id))
    }
    /// The activated chain only; prepared but unpublished revisions stay private.
    pub fn revisions(&self) -> io::Result<Vec<Persona>> {
        self.ancestors(&self.read()?)
    }
    pub fn update(
        &self,
        expected: u64,
        id: &str,
        text: &str,
        sources: Vec<String>,
    ) -> io::Result<Persona> {
        let _lock = self.lock()?;
        self.update_locked(expected, id, text, sources)
    }
    fn update_locked(
        &self,
        expected: u64,
        id: &str,
        text: &str,
        sources: Vec<String>,
    ) -> io::Result<Persona> {
        if id.trim().is_empty() {
            return Err(invalid("an update ID is required"));
        }
        let current = self.read()?;
        for prior in self.ancestors(&current)? {
            if prior.last_update_id == id {
                return if prior.revision
                    == expected
                        .checked_add(1)
                        .ok_or_else(|| invalid("revision overflow"))?
                    && prior.text == text
                    && prior.sources == sources
                {
                    Ok(prior)
                } else {
                    Err(invalid("update ID already has different content"))
                };
            }
        }
        if current.revision != expected {
            return Err(invalid("persona revision conflict"));
        }
        let next = Persona {
            revision: expected
                .checked_add(1)
                .ok_or_else(|| invalid("revision overflow"))?,
            text: text.into(),
            last_update_id: id.into(),
            sources,
            parent_update_id: current.last_update_id.clone(),
            ..current
        };
        let name = history_name(id);
        let bytes = serde_json::to_vec_pretty(&next).map_err(io::Error::other)?;
        match self.files().read_file(&name) {
            Ok(old) if old != bytes => {
                return Err(invalid(
                    "update ID belongs to a different prepared revision",
                ));
            }
            Ok(_) => (),
            Err(e) if e.kind() == io::ErrorKind::NotFound => {
                self.files().write_file(&name, &bytes)?
            }
            Err(e) => return Err(e),
        }
        self.files().write_file("persona.json", &bytes)?;
        Ok(next)
    }
    pub fn rollback(&self, expected: u64, target: u64, id: &str) -> io::Result<Persona> {
        let _lock = self.lock()?;
        let current = self.read()?;
        let text = if target == 0 {
            String::new()
        } else {
            self.ancestors(&current)?
                .into_iter()
                .find(|p| p.revision == target)
                .ok_or_else(|| invalid("persona rollback revision is not published"))?
                .text
        };
        self.update_locked(expected, id, &text, vec![format!("rollback:{target}")])
    }
}

fn invalid(text: &str) -> io::Error {
    io::Error::new(io::ErrorKind::InvalidData, text)
}
fn hash(bytes: &[u8]) -> String {
    format!("{:x}", Sha256::digest(bytes))
}
fn history_name(id: &str) -> String {
    format!("revisions/{}.json", hash(id.as_bytes()))
}

use rig_harness::prelude::*;
#[derive(Resource, Clone)]
pub struct PersonaSource(pub PathBuf);
#[derive(Component, Clone, Debug, Serialize, Deserialize, PartialEq, Eq)]
pub struct PersonaRevision {
    pub persona_id: String,
    pub revision: u64,
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
                "Persona show | update <revision> <file> | rollback <revision> <target>",
                command,
            );
    }
}

fn source(world: &mut World) {
    if !world.contains_resource::<PersonaSource>() {
        let root = world
            .get_resource::<ProjectRoot>()
            .map(|r| r.0.clone())
            .unwrap_or_else(|| std::env::current_dir().unwrap_or_default());
        world.insert_resource(PersonaSource(root));
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
        match PersonaStore::new(&source.0).read() {
            Ok(persona) => {
                if !persona.text.is_empty() {
                    prompt.role =
                        format!("<persona>\n{}\n</persona>\n\n{}", persona.text, prompt.role);
                }
                commands.entity(entity).insert(PersonaRevision {
                    persona_id: persona.persona_id,
                    revision: persona.revision,
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
    let store = PersonaStore::new(&world.resource::<PersonaSource>().0);
    let mut parts = args.args.splitn(3, char::is_whitespace);
    let action = parts.next().unwrap_or("");
    let result = (|| -> io::Result<Persona> {
        if matches!(action, "" | "show") {
            return store.read();
        }
        let revision = parts
            .next()
            .and_then(|s| s.parse::<u64>().ok())
            .ok_or_else(|| invalid("expected persona revision"))?;
        let value = parts
            .next()
            .ok_or_else(|| invalid("expected persona file or rollback target"))?
            .trim();
        let id = format!(
            "human:{}:{}",
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .map_err(io::Error::other)?
                .as_nanos()
        );
        match action {
            "update" => store.update(
                revision,
                &id,
                &fs::read_to_string(value.trim_matches('"'))?,
                vec!["human".into()],
            ),
            "rollback" => store.rollback(
                revision,
                value
                    .parse()
                    .map_err(|_| invalid("invalid rollback target"))?,
                &id,
            ),
            _ => Err(invalid(
                "use persona show, update <revision> <file>, or rollback <revision> <target>",
            )),
        }
    })();
    match result {
        Ok(p) => {
            world.write_message(Notice::info(
                args.agent,
                format!(
                    "Persona {} revision {}\n{}",
                    p.persona_id, p.revision, p.text
                ),
            ));
        }
        Err(e) => {
            world.write_message(Notice::error(args.agent, format!("Persona: {e}")));
        }
    }
}

#[cfg(test)]
mod tests;
