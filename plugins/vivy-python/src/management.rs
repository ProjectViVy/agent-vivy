//! Explicit preparation is a cancellable owned child, never an implicit tool side effect.
use super::*;
use std::path::Path;
use vivy_basics::skills::{TrustedSkillInventory, refresh_inventory};
pub(super) mod initialization;
mod status;
pub(super) mod toolchain;

const CORE: &str = include_str!("../requirements-core.lock");
const MEMORY: &str = include_str!("../requirements-memory.lock");

struct Preparation {
    cancel: Arc<Cancellation>,
    result: Mutex<mpsc::Receiver<Result<String, String>>>,
    stage: Arc<Mutex<String>>,
    checking: bool,
    last_stage: String,
}
#[derive(Resource, Default)]
struct Preparations(
    HashMap<Entity, Preparation>,
    std::collections::HashSet<Entity>,
    u64,
);

/// Typed desktop entry points use the same initialization state and owned jobs.
pub fn desktop_report(world: &World, agent: Entity) -> vivy_basics::setup::Report {
    initialization::report(world, agent)
}
pub fn desktop_prepare(
    world: &mut World,
    agent: Entity,
    managed: bool,
    memory: bool,
) -> Result<(), String> {
    initialization::approve_python(world, agent, managed, memory)
}
pub fn desktop_cancel(world: &mut World, agent: Entity) -> Result<(), String> {
    let mut jobs = world.resource_mut::<Preparations>();
    let job = jobs
        .0
        .get_mut(&agent)
        .ok_or("no Python initialization is running")?;
    job.cancel.stop();
    Ok(())
}

pub(super) fn install(app: &mut App) {
    initialization::install(app);
    app.init_resource::<Preparations>()
        .add_command("python", "Python status | prepare | reset | close", command)
        .add_systems(First, (poll, initialization::tick, status::publish).chain())
        .add_systems(
            Last,
            stop_preparations
                .in_set(OnAppExitSystems)
                .run_if(on_message::<rig_harness::AppExit>),
        )
        .add_observer(cancel_turn);
}

pub(super) fn skill_metadata(inventory: Option<&TrustedSkillInventory>) -> Value {
    json!(
        inventory
            .map(|i| i
                .0
                .iter()
                .filter(|s| !s.manual)
                .map(|s| json!({"name":s.name,"description":s.description,"file":s.file}))
                .collect::<Vec<_>>())
            .unwrap_or_default()
    )
}

pub(super) fn prepared_python(root: &Path) -> Result<Option<String>, String> {
    let file = root.join("current.json");
    let bytes = match std::fs::read(&file) {
        Ok(bytes) => bytes,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(None),
        Err(e) => return Err(format!("Cannot read prepared Python manifest: {e}")),
    };
    let value: Value = serde_json::from_slice(&bytes)
        .map_err(|e| format!("Invalid Python manifest; run /python prepare: {e}"))?;
    let path = value["executable"]
        .as_str()
        .ok_or("Python manifest has no executable; run /python prepare")?;
    if !Path::new(path).is_file() {
        return Err("Prepared Python executable is missing; run /python prepare".into());
    }
    Ok(Some(path.to_owned()))
}

fn command(In(args): In<CommandArgs>, world: &mut World) {
    let action = args.args.trim();
    #[cfg(feature = "managed-skills")]
    if action.starts_with("rollback ") {
        let fields: Vec<_> = action.split_whitespace().collect();
        let result = (|| -> std::io::Result<crate::managed::Publication> {
            if fields.len() != 3 {
                return Err(std::io::Error::other(
                    "Use /python rollback <expected-publication> <retained-target>",
                ));
            }
            let store = crate::managed::PublicationStore::new(
                &world.resource::<ProjectRoot>().0,
                RigHome::of(world).root(),
            );
            let update = format!(
                "rollback:{}",
                vivy_basics::setup::fingerprint(format!("{}:{}", fields[1], fields[2]).as_bytes())
            );
            store.rollback(fields[1], fields[2], &update)
        })();
        world.write_message(match result {
            Ok(p)=>Notice::info(args.agent,format!("Rollback published {}. Newly planned calls use the retained code/environment; worker variables reset at adoption.",p.id)),
            Err(e)=>Notice::error(args.agent,format!("Rollback failed: {e}")),
        });
        return;
    }
    if matches!(action, "reset" | "close" | "prepare" | "status" | "") {
        let mut jobs = world.resource_mut::<Preparations>();
        jobs.2 = jobs.2.wrapping_add(1);
        if matches!(action, "reset" | "close" | "prepare") {
            jobs.1.remove(&args.agent);
        }
    }
    if action == "reset" || action == "prepare" {
        if world.contains_resource::<TrustedSkillInventory>() {
            refresh_inventory(world);
        }
    }
    if action == "close" || action == "reset" {
        world
            .resource_mut::<initialization::Initialization>()
            .report
            .phase = None;
        if let Some(job) = world.resource_mut::<Preparations>().0.remove(&args.agent) {
            job.cancel.stop();
        }
        if let Some(worker) = world.resource_mut::<Workbenches>().0.remove(&args.agent) {
            worker.retire();
        }
        world.write_message(Notice::info(args.agent, "Python closed; variables reset. Explicit artifacts and committed memories are preserved. Next use starts fresh; changed dependencies require /python prepare."));
    } else if action == "prepare" {
        start_preparation(world, args.agent);
    } else if action.is_empty() || action == "status" {
        initialization::refresh(world);
        let report = initialization::report(world, args.agent);
        world.write_message(Notice::info(args.agent, format!("Python initialization: {}. /setup python configures or retries; worker lifecycle is shown separately.", serde_json::to_string(&report).unwrap_or_default())));
    } else {
        world.write_message(Notice::error(
            args.agent,
            "Use /python status, prepare, reset or close.",
        ));
    }
}

fn start_preparation(world: &mut World, agent: Entity) {
    let root = world.resource::<ProjectRoot>().0.clone();
    let checking: Vec<_> = world
        .resource::<Preparations>()
        .0
        .iter()
        .filter(|(_, job)| job.checking)
        .map(|(agent, _)| *agent)
        .collect();
    for checking in checking {
        if let Some(job) = world.resource_mut::<Preparations>().0.remove(&checking) {
            job.cancel.stop();
        }
    }
    if !world.resource::<Preparations>().0.is_empty() {
        world.write_message(Notice::info(
            agent,
            "Python preparation is already running.",
        ));
        return;
    }
    let mut settings = world.resource::<PythonSettings>().clone();
    settings.skills = skill_metadata(world.get_resource::<TrustedSkillInventory>());
    #[cfg(feature = "managed-skills")]
    {
        settings.managed_requirements = crate::managed::preparation::requirements(world);
        settings.stage_only = world
            .get_resource::<crate::managed::PendingRequirements>()
            .is_some_and(|p| !p.0.is_empty());
        let store = crate::managed::PublicationStore::new(&root, RigHome::of(world).root());
        settings.selected = if store.root().join("current.json").exists() {
            match store.current() {
                Ok(base) => Some(base),
                Err(error) => {
                    let cause = format!(
                        "Cannot prepare over an invalid or untrusted publication: {error}. Restore its validated manifest/local setup record before retrying."
                    );
                    world
                        .resource_mut::<initialization::Initialization>()
                        .complete(false, Err(cause.clone()), false);
                    world.write_message(Notice::error(agent, cause));
                    return;
                }
            }
        } else {
            None
        };
        if let Some(published) = world.get_resource::<vivy_basics::skills::PublishedSkills>() {
            settings.skills.as_array_mut().unwrap().retain(|row| {
                !published
                    .0
                    .iter()
                    .any(|s| row["file"].as_str() == s.file.to_str())
            });
        }
    }
    let cancel = Arc::<Cancellation>::default();
    let token = cancel.clone();
    let (sender, receiver) = mpsc::channel();
    let wake = world.resource::<Wake>().clone();
    let home = RigHome::of(world).root().to_path_buf();
    let scope = initialization::scope(world);
    let reuse = vivy_basics::setup::decision(world, "python")
        .ok()
        .and_then(|d| d.environment)
        .is_some_and(|identity| {
            Some(identity) == initialization::publication_identity(&root.join(".vivy/workbench"))
        });
    let stage = Arc::new(Mutex::new("toolchain".to_owned()));
    let progress = stage.clone();
    let managed = initialization::managed(world);
    initialization::preparing(world);
    std::thread::spawn(move || {
        let _ = sender.send(prepare(
            &root, &home, &settings, &scope, managed, reuse, &token, &progress,
        ));
        wake.wake();
    });
    world.resource_mut::<Preparations>().0.insert(
        agent,
        Preparation {
            cancel,
            result: Mutex::new(receiver),
            stage,
            checking: false,
            last_stage: String::new(),
        },
    );
    world.write_message(Notice::info(agent, "Preparing Python. /python close cancels. New preparation is adopted at the next call boundary; existing variables will reset."));
}

fn prepare(
    dir: &Path,
    home: &Path,
    settings: &PythonSettings,
    scope: &str,
    managed: bool,
    reuse: bool,
    cancel: &Cancellation,
    progress: &Arc<Mutex<String>>,
) -> Result<String, String> {
    let (python, uv) = toolchain::ensure(home, &settings.executable, managed, cancel, progress)?;
    let root = dir.join(".vivy/workbench");
    let timestamp = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map_err(|e| e.to_string())?
        .as_nanos();
    let source =
        worker::package(&root.join(format!("prepare-{}-{timestamp}", std::process::id())))?;
    let lock = source.join("requirements.lock");
    std::fs::write(&lock, if settings.memory { MEMORY } else { CORE })
        .map_err(|e| e.to_string())?;
    let memory_root = settings
        .memory_root
        .clone()
        .unwrap_or_else(|| root.join("native-memory"));
    #[cfg(not(feature = "managed-skills"))]
    let extra = String::new();
    #[cfg(feature = "managed-skills")]
    let extra = format!(
        ",managed_requirements=json.loads({}),publish={}",
        json!(json!(settings.managed_requirements).to_string()),
        if settings.selected.is_some() {
            "False"
        } else {
            "True"
        }
    );
    let code = format!(
        "import sys,os,json;sys.path.insert(0,{});from vivy_workbench.prepare import prepare;print(json.dumps(prepare({}, {}, python=sys.executable, skills=json.loads(os.environ['VIVY_WORKBENCH_SKILLS']), native_memory={}, memory_root={},scope={},uv={},reuse={}{})))",
        json!(source),
        json!(root),
        json!(lock),
        if settings.memory { "True" } else { "False" },
        json!(memory_root),
        json!(scope),
        json!(uv),
        if reuse { "True" } else { "False" },
        extra
    );
    let mut command = vivy_process::command(&python);
    // Preparation is noninteractive. Inheriting stdin can block spawning while
    // the JSON input reader owns Rust's stdin lock, and would expose user input.
    command
        .args(["-I", "-u", "-c", &code])
        .current_dir(dir)
        .env("VIVY_WORKBENCH_SKILLS", settings.skills.to_string())
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());
    let manifest = toolchain::run(&mut command, cancel, progress, 1800)?;
    #[cfg(feature = "managed-skills")]
    if let Some(base) = &settings.selected {
        return crate::managed::preparation::finish(
            &crate::managed::PublicationStore::new(dir, home),
            base,
            manifest,
            settings.stage_only,
            cancel,
        );
    }
    let _ = manifest;
    Ok(
        "Python environment prepared and validated. The next cell adopts it with fresh variables."
            .into(),
    )
}

fn poll(
    mut jobs: ResMut<Preparations>,
    agents: Query<(), With<Agent>>,
    mut notices: MessageWriter<Notice>,
    init: Option<ResMut<initialization::Initialization>>,
) {
    let mut init = init;
    let Preparations(running, failures, revision) = &mut *jobs;
    failures.retain(|agent| agents.contains(*agent));
    running.retain(|agent, job| {
        if !agents.contains(*agent) {
            job.cancel.stop();
            return false;
        }
        let stage = job.stage.lock().unwrap_or_else(|e| e.into_inner()).clone();
        if !job.checking && stage != job.last_stage {
            notices.write(Notice::info(*agent, format!("Python preparation: {stage}")));
            job.last_stage = stage;
        }
        match job
            .result
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .try_recv()
        {
            Ok(Ok(text)) => {
                *revision = revision.wrapping_add(1);
                failures.remove(agent);
                if let Some(init) = init.as_deref_mut() {
                    init.complete(job.checking, Ok(text.clone()), false);
                }
                if !job.checking {
                    notices.write(Notice::info(*agent, text));
                }
                false
            }
            Ok(Err(text)) => {
                *revision = revision.wrapping_add(1);
                if !job.cancel.cancelled.load(Ordering::SeqCst) {
                    failures.insert(*agent);
                }
                let cancelled = job.cancel.cancelled.load(Ordering::SeqCst);
                if let Some(init) = init.as_deref_mut() {
                    init.complete(job.checking, Err(text.clone()), cancelled);
                }
                notices.write(if cancelled {
                    Notice::info(
                        *agent,
                        "Python initialization cancelled; previous environment preserved.",
                    )
                } else {
                    Notice::error(*agent, text)
                });
                false
            }
            Err(mpsc::TryRecvError::Disconnected) => {
                if let Some(init) = init.as_deref_mut() {
                    init.complete(
                        job.checking,
                        Err("Initialization worker exited".into()),
                        false,
                    );
                }
                failures.insert(*agent);
                notices.write(Notice::error(*agent, "Python preparation worker exited."));
                false
            }
            Err(mpsc::TryRecvError::Empty) => true,
        }
    });
}
fn cancel_turn(event: On<StopTurn>, jobs: Res<Preparations>, workers: Res<Workbenches>) {
    if let Some(job) = jobs.0.get(&event.entity) {
        job.cancel.stop();
    }
    if let Some(worker) = workers.0.get(&event.entity) {
        worker.stop();
    }
}
fn stop_preparations(jobs: Res<Preparations>) {
    for job in jobs.0.values() {
        job.cancel.stop();
    }
}
