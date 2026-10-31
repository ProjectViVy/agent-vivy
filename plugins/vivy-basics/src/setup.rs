//! Project-scoped initialization consent and the shared setup entry point.
use bevy_ecs::system::RunSystemOnce;
use rig_harness::pick::{Picked, Picker};
use rig_harness::prelude::*;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::{
    collections::{BTreeMap, BTreeSet},
    io::Write,
    path::{Path, PathBuf},
};

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum OutcomeState {
    #[default]
    NotInit,
    Error,
    Ready,
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize)]
pub struct Report {
    pub outcome: OutcomeState,
    pub phase: Option<String>,
    pub detail: String,
    pub scope: String,
}
#[derive(Component, Clone, Debug, PartialEq, Eq)]
pub struct ProviderStatus(pub Report);
impl Report {
    pub fn ready(detail: impl Into<String>) -> Self {
        Self {
            outcome: OutcomeState::Ready,
            detail: detail.into(),
            ..Self::default()
        }
    }
    pub fn error(detail: impl Into<String>) -> Self {
        Self {
            outcome: OutcomeState::Error,
            detail: detail.into(),
            ..Self::default()
        }
    }
    pub fn skip(&mut self) {
        self.phase = None;
    }
}

#[derive(Clone, Debug, Default, Serialize, Deserialize)]
#[serde(default)]
pub struct Decision {
    pub grant: Option<String>,
    pub skipped: bool,
    pub enabled: bool,
    pub scope: String,
    pub managed: bool,
    pub failed_scope: String,
    pub last_error: Option<String>,
    pub environment: Option<String>,
}
impl Decision {
    pub fn permits(&self, scope: &str) -> bool {
        self.grant.as_deref() == Some(scope)
    }
}
pub fn fingerprint(bytes: &[u8]) -> String {
    format!("{:x}", Sha256::digest(bytes))
}
fn file(home: &Path, project: &Path, key: &str) -> PathBuf {
    let canonical = project
        .canonicalize()
        .unwrap_or_else(|_| project.to_path_buf());
    home.join("initialization")
        .join(fingerprint(canonical.to_string_lossy().as_bytes()))
        .join(format!("{key}.json"))
}
pub fn read(home: &Path, project: &Path, key: &str) -> Result<Decision, String> {
    match std::fs::read(file(home, project, key)) {
        Ok(bytes) => serde_json::from_slice(&bytes)
            .map_err(|e| format!("Cannot read initialization consent: {e}")),
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(Decision::default()),
        Err(e) => Err(format!("Cannot read initialization consent: {e}")),
    }
}
pub fn save(home: &Path, project: &Path, key: &str, value: &Decision) -> Result<(), String> {
    let path = file(home, project, key);
    (|| -> std::io::Result<()> {
        std::fs::create_dir_all(path.parent().unwrap())?;
        let lock = std::fs::OpenOptions::new()
            .create(true)
            .truncate(false)
            .write(true)
            .open(path.with_extension("lock"))?;
        lock.try_lock().map_err(std::io::Error::other)?;
        let temp = path.with_extension(format!("{}.tmp", std::process::id()));
        let mut stream = std::fs::File::create(&temp)?;
        stream.write_all(&serde_json::to_vec(value)?)?;
        stream.sync_all()?;
        drop(stream);
        std::fs::rename(temp, path)?;
        Ok(())
    })()
    .map_err(|e| format!("Cannot save initialization consent: {e}"))
}
pub fn decision(world: &World, key: &str) -> Result<Decision, String> {
    read(
        RigHome::of(world).root(),
        &world.resource::<ProjectRoot>().0,
        key,
    )
}
pub fn remember(world: &World, key: &str, value: &Decision) -> Result<(), String> {
    save(
        RigHome::of(world).root(),
        &world.resource::<ProjectRoot>().0,
        key,
        value,
    )
}

#[derive(Clone, Copy)]
pub struct Item {
    pub key: &'static str,
    pub title: &'static str,
    pub report: fn(&World, Entity) -> Report,
    pub open: fn(&mut World, Entity),
}
#[derive(Resource, Default)]
struct Registry(BTreeMap<u8, Item>);
#[derive(Resource, Default)]
struct Startup {
    seen: BTreeSet<&'static str>,
    finished: bool,
}

/// Called by the actual compiled consumer, never a parallel capability catalog.
pub fn register(app: &mut App, order: u8, item: Item) {
    if !app.world().contains_resource::<Registry>() {
        app.init_resource::<ProjectRoot>()
            .init_resource::<Registry>()
            .init_resource::<Startup>()
            .add_command(
                "setup",
                "Initialize provider, Python and optional memory",
                command,
            )
            .add_systems(Update, startup);
    }
    app.world_mut()
        .resource_mut::<Registry>()
        .0
        .insert(order, item);
}
fn command(In(args): In<CommandArgs>, world: &mut World) {
    let items: Vec<Item> = world.resource::<Registry>().0.values().copied().collect();
    let can_answer = world
        .run_system_once(|answerers: Answerers| answerers.can_answer(None))
        .unwrap_or(false);
    if !can_answer {
        for item in &items {
            world.write_message(Notice::info(
                args.agent,
                format!(
                    "setup {}: {}",
                    item.key,
                    serde_json::to_string(&(item.report)(world, args.agent)).unwrap_or_default()
                ),
            ));
        }
        return;
    }
    if let Some(item) = items.iter().find(|item| item.key == args.args.trim()) {
        (item.open)(world, args.agent);
        return;
    }
    if !args.args.trim().is_empty() {
        world.write_message(Notice::error(
            args.agent,
            "That setup capability is not compiled into this agent.",
        ));
        return;
    }
    let mut picker = Picker::new("Initialization")
        .for_agent(args.agent)
        .body("Configure or retry an item. Skipped items remain available here.");
    for item in &items {
        let report = (item.report)(world, args.agent);
        picker = picker.item(
            format!(
                "{} · {} · {}",
                item.title,
                report
                    .phase
                    .as_deref()
                    .map(str::to_owned)
                    .unwrap_or_else(|| format!("{:?}", report.outcome)),
                report.detail
            ),
            format!("setup {}", item.key),
        );
    }
    world.spawn(picker);
}
fn startup(world: &mut World) {
    if world.resource::<Startup>().finished {
        return;
    }
    if world
        .get_resource::<crate::trust::ProjectTrust>()
        .is_some_and(|t| t.undecided)
    {
        return;
    }
    if world.query::<&Picker>().iter(world).next().is_some()
        || world
            .query_filtered::<&Ask, Without<Outcome>>()
            .iter(world)
            .next()
            .is_some()
    {
        return;
    }
    let agent = {
        let mut query = world.query_filtered::<Entity, (With<Agent>, Without<SpawnedBy>)>();
        query.iter(world).next()
    };
    let Some(agent) = agent else {
        return;
    };
    if world.get::<ActiveTurn>(agent).is_some() {
        return;
    }
    let can_answer = world
        .run_system_once(|answerers: Answerers| answerers.can_answer(None))
        .unwrap_or(false);
    let items: Vec<Item> = world.resource::<Registry>().0.values().copied().collect();
    for item in items {
        if world.resource::<Startup>().seen.contains(item.key) {
            continue;
        }
        let report = (item.report)(world, agent);
        if report.phase.is_some() {
            return;
        }
        let saved = decision(world, item.key);
        let skipped = saved
            .as_ref()
            .is_ok_and(|d| d.skipped && d.scope == report.scope);
        world.resource_mut::<Startup>().seen.insert(item.key);
        if report.outcome == OutcomeState::Ready || skipped {
            continue;
        }
        if !can_answer {
            world.write_message(Notice::info(
                agent,
                format!(
                    "setup {}: {}. Use /setup {} interactively or explicitly prepare.",
                    item.key,
                    serde_json::to_string(&report).unwrap_or_default(),
                    item.key
                ),
            ));
            continue;
        }
        let picker = Picker::new(format!("Set up {}?", item.title))
            .for_agent(agent)
            .body(format!(
                "{}\nSkipping is remembered for this project. /setup reopens configuration.",
                report.detail
            ))
            .item("Configure", "")
            .item("Skip for this project", "");
        world
            .spawn(picker)
            .observe(move |event: On<Picked>, mut commands: Commands| {
                let configure = event.chosen.as_ref().and_then(|c| c.items.first()) == Some(&0);
                let scope = report.scope.clone();
                commands.queue(move |world: &mut World| {
                    if configure {
                        (item.open)(world, agent);
                    } else {
                        let result = decision(world, item.key).and_then(|mut d| {
                            d.skipped = true;
                            d.scope = scope;
                            remember(world, item.key, &d)
                        });
                        if let Err(e) = result {
                            world.write_message(Notice::error(agent, e));
                        }
                    }
                });
            });
        return;
    }
    world.resource_mut::<Startup>().finished = true;
    world.write_message(Notice::info(
        agent,
        "Initialization checks complete. /setup shows configuration and retry options.",
    ));
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn consent_is_exact_project_and_scope_and_skip_survives_restart() {
        let root = std::env::temp_dir().join(format!("vivy-setup-{}", std::process::id()));
        let home = root.join("home");
        let project = root.join("project");
        std::fs::create_dir_all(&project).unwrap();
        let record = Decision {
            grant: Some("core-v1".into()),
            skipped: true,
            enabled: false,
            ..Default::default()
        };
        save(&home, &project, "python", &record).unwrap();
        let read = read(&home, &project, "python").unwrap();
        assert!(read.skipped);
        assert!(read.permits("core-v1"));
        assert!(!read.permits("core-v2"));
        assert!(
            !super::read(&home, &root, "python")
                .unwrap()
                .permits("core-v1")
        );
        assert!(
            super::read(&home, &project, "memory")
                .unwrap()
                .grant
                .is_none()
        );
        std::fs::remove_dir_all(&root).unwrap();
    }

    #[test]
    fn skip_preserves_failure_and_ready_is_not_a_grant() {
        let mut report = Report::error("database access denied");
        report.skip();
        assert_eq!(report.outcome, OutcomeState::Error);
        assert_eq!(report.detail, "database access denied");
        let mut report = Report::ready("validated");
        report.skip();
        assert_eq!(report.outcome, OutcomeState::Ready);
        assert!(!Decision::default().permits("anything"));
    }
}
