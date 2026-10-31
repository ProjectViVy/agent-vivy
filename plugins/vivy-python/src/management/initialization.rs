use super::*;
use rig_harness::pick::{Picked, Picker};
use vivy_basics::setup::{self, Item, OutcomeState, Report};

#[derive(Resource, Default)]
pub(super) struct Initialization {
    pub report: Report,
    checked: bool,
    attempted: bool,
    loaded: bool,
    managed: bool,
    pub memory_ready: bool,
    scope: String,
    pending_save: bool,
    blocked: Option<(String, String)>,
}
impl Initialization {
    pub fn complete(&mut self, checking: bool, result: Result<String, String>, cancelled: bool) {
        self.report.phase = None;
        if cancelled {
            self.attempted = true;
            return;
        }
        self.pending_save = !checking;
        self.report = match result {
            Ok(text) if checking => serde_json::from_str::<Value>(&text)
                .map(|value| Report {
                    outcome: match value["outcome"].as_str() {
                        Some("ready") => OutcomeState::Ready,
                        Some("not_init") => OutcomeState::NotInit,
                        _ => OutcomeState::Error,
                    },
                    detail: value["detail"]
                        .as_str()
                        .unwrap_or("Invalid detection result")
                        .into(),
                    ..Default::default()
                })
                .unwrap_or_else(|e| Report::error(e.to_string())),
            Ok(text) => Report::ready(text),
            Err(text) => Report::error(text),
        };
        self.memory_ready = self.report.outcome == OutcomeState::Ready;
    }
}

pub(super) fn install(app: &mut App) {
    app.init_resource::<Initialization>();
    setup::register(
        app,
        20,
        Item {
            key: "python",
            title: "Python workbench",
            report,
            open: open_python,
        },
    );
    setup::register(
        app,
        30,
        Item {
            key: "memory",
            title: "MemPalace (optional)",
            report: memory_report,
            open: open_memory,
        },
    );
}
pub(crate) fn scope(world: &World) -> String {
    let settings = world.resource::<PythonSettings>();
    let skills = skill_metadata(world.get_resource::<TrustedSkillInventory>());
    let mut bytes = Vec::from(if settings.memory {
        MEMORY.as_bytes()
    } else {
        CORE.as_bytes()
    });
    bytes.extend_from_slice(include_bytes!("../../bootstrap.ps1"));
    bytes.extend_from_slice(settings.executable.as_bytes());
    bytes.extend_from_slice(format!("{:?}", settings.memory_root).as_bytes());
    let variables: Vec<_> = std::env::vars().collect();
    bytes.extend_from_slice(
        source_identity(variables.iter().map(|(k, v)| (k.as_str(), v.as_str()))).as_bytes(),
    );
    for row in skills.as_array().into_iter().flatten() {
        if let Some(file) = row["file"].as_str() {
            #[cfg(feature = "managed-skills")]
            if world
                .get_resource::<vivy_basics::skills::PublishedSkills>()
                .is_some_and(|p| p.0.iter().any(|s| s.file == Path::new(file)))
            {
                continue;
            }
            let path = Path::new(file)
                .parent()
                .unwrap_or(Path::new(file))
                .join("pyproject.toml");
            if let Ok(content) = std::fs::read(&path) {
                bytes.extend_from_slice(path.to_string_lossy().as_bytes());
                bytes.extend(content);
            }
        }
    }
    #[cfg(feature = "managed-skills")]
    {
        let dependencies = crate::managed::preparation::requirements(world);
        if !dependencies.is_empty() {
            bytes.extend(serde_json::to_vec(&dependencies).expect("dependency strings"));
        }
    }
    setup::fingerprint(&bytes)
}
fn source_identity<'a>(variables: impl IntoIterator<Item = (&'a str, &'a str)>) -> String {
    let sources: std::collections::BTreeMap<_, _> = variables
        .into_iter()
        .filter(|(key, _)| {
            key.starts_with("UV_INDEX")
                || (key.starts_with("PIP_")
                    && (key.contains("INDEX") || key.ends_with("FIND_LINKS")))
                || matches!(
                    *key,
                    "UV_DEFAULT_INDEX"
                        | "UV_EXTRA_INDEX_URL"
                        | "UV_FIND_LINKS"
                        | "UV_NO_INDEX"
                        | "UV_INSECURE_HOST"
                        | "UV_PYTHON_INSTALL_MIRROR"
                        | "UV_PYTHON_DOWNLOADS_JSON_URL"
                )
        })
        .collect();
    setup::fingerprint(format!("{sources:?}").as_bytes())
}
pub(super) fn publication_identity(root: &Path) -> Option<String> {
    std::fs::read(root.join("current.json"))
        .ok()
        .map(|bytes| setup::fingerprint(&bytes))
}
pub(super) fn report(world: &World, _: Entity) -> Report {
    let init = world.resource::<Initialization>();
    let mut report = init.report.clone();
    report.scope = if init.scope.is_empty() {
        scope(world)
    } else {
        init.scope.clone()
    };
    if !init.checked {
        report.phase = Some("checking".into());
    }
    if init.pending_save {
        report.phase = Some("publishing".into());
    }
    if let Some(job) = world.resource::<Preparations>().0.values().next() {
        report.phase = Some(job.stage.lock().unwrap_or_else(|e| e.into_inner()).clone());
    }
    report
}
fn memory_report(world: &World, agent: Entity) -> Report {
    if !world.resource::<PythonSettings>().memory {
        return Report {
            // Memory shares Python's preparer and grant. Do not offer the next
            // setup step until the running preparation and local receipt finish.
            phase: report(world, agent).phase,
            detail:
                "Disabled. Enable to install native MemPalace and download its embedding model."
                    .into(),
            scope: "memory-off".into(),
            ..Default::default()
        };
    }
    report(world, agent)
}
pub(super) fn managed(world: &World) -> bool {
    world.resource::<Initialization>().managed
}
pub(crate) fn refresh(world: &mut World) {
    let mut init = world.resource_mut::<Initialization>();
    init.checked = false;
    init.scope.clear();
}
pub(super) fn preparing(world: &mut World) {
    let scope = scope(world);
    let mut init = world.resource_mut::<Initialization>();
    init.scope = scope;
    init.checked = true;
    init.attempted = true;
    init.report.phase = Some("preparing".into());
}

pub(super) fn tick(world: &mut World) {
    if !world.resource::<Preparations>().0.is_empty() {
        return;
    }
    let agent = world
        .query_filtered::<Entity, (With<Agent>, Without<SpawnedBy>)>()
        .iter(world)
        .next();
    let Some(agent) = agent else {
        return;
    };
    if world
        .get_resource::<vivy_basics::trust::ProjectTrust>()
        .is_some_and(|t| t.undecided)
    {
        return;
    }
    if !world.resource::<Initialization>().loaded {
        world.resource_mut::<Initialization>().loaded = true;
        if let Ok(decision) = setup::decision(world, "python") {
            let mut init = world.resource_mut::<Initialization>();
            init.managed = decision.managed;
            init.blocked = decision
                .last_error
                .map(|error| (decision.failed_scope, error));
        }
        match setup::decision(world, "memory") {
            Ok(decision) if decision.enabled => {
                world.resource_mut::<PythonSettings>().memory = true
            }
            Err(error) => {
                world.resource_mut::<Initialization>().report = Report::error(error);
                world.resource_mut::<Initialization>().checked = true;
                return;
            }
            _ => {}
        }
    }
    if world.resource::<Initialization>().pending_save {
        world.resource_mut::<Initialization>().pending_save = false;
        let init = world.resource::<Initialization>();
        let failure = (init.report.outcome == OutcomeState::Error)
            .then(|| (init.scope.clone(), init.report.detail.clone()));
        let result = setup::decision(world, "python").and_then(|mut decision| {
            decision.failed_scope = failure.as_ref().map(|f| f.0.clone()).unwrap_or_default();
            decision.last_error = failure.as_ref().map(|f| f.1.clone());
            if failure.is_none() {
                decision.environment = publication_identity(
                    &world.resource::<ProjectRoot>().0.join(".vivy/workbench"),
                );
            }
            setup::remember(world, "python", &decision)
        });
        world.resource_mut::<Initialization>().blocked = failure;
        if let Err(error) = result {
            world.resource_mut::<Initialization>().report = Report::error(error);
        }
    }
    let scope = if world.resource::<Initialization>().scope.is_empty()
        || world.is_resource_changed::<PythonSettings>()
        || world.is_resource_changed::<TrustedSkillInventory>()
    {
        scope(world)
    } else {
        world.resource::<Initialization>().scope.clone()
    };
    if world.resource::<Initialization>().scope != scope {
        let mut init = world.resource_mut::<Initialization>();
        init.scope = scope.clone();
        init.checked = false;
        init.attempted = false;
    }
    if !world.resource::<Initialization>().checked {
        world.resource_mut::<Initialization>().checked = true;
        let root = world.resource::<ProjectRoot>().0.join(".vivy/workbench");
        let settings = world.resource::<PythonSettings>().clone();
        let managed = managed(world);
        let approved_environment = setup::decision(world, "python")
            .ok()
            .and_then(|d| d.environment);
        #[cfg(feature = "managed-skills")]
        let approved_environment = crate::managed::PublicationStore::new(
            &world.resource::<ProjectRoot>().0,
            RigHome::of(world).root(),
        )
        .current()
        .ok()
        .map(|p| p.id)
        .or(approved_environment);
        let cancel = Arc::<Cancellation>::default();
        let token = cancel.clone();
        let stage = Arc::new(Mutex::new("checking".into()));
        let progress = stage.clone();
        let wake = world.resource::<Wake>().clone();
        let (sender, receiver) = mpsc::channel();
        std::thread::spawn(move || {
            let result = detect(
                &root,
                &settings,
                &scope,
                managed,
                approved_environment.as_deref(),
                &token,
                &progress,
            );
            let _ = sender.send(result);
            wake.wake();
        });
        world.resource_mut::<Preparations>().0.insert(
            agent,
            Preparation {
                cancel,
                result: Mutex::new(receiver),
                stage,
                checking: true,
                last_stage: String::new(),
            },
        );
        return;
    }
    if let Some((failed_scope, error)) = world.resource::<Initialization>().blocked.clone() {
        if failed_scope == scope {
            let mut init = world.resource_mut::<Initialization>();
            init.report = Report::error(error);
            init.attempted = true;
        }
    }
    let init = world.resource::<Initialization>();
    if init.attempted || init.report.outcome == OutcomeState::Ready {
        return;
    }
    let approved = setup::decision(world, "python").is_ok_and(|d| d.permits(&scope) && !d.skipped);
    let memory_approved = !world.resource::<PythonSettings>().memory
        || setup::decision(world, "memory").is_ok_and(|d| d.permits(&scope) && !d.skipped);
    if approved && memory_approved {
        start_preparation(world, agent);
    } else {
        world.resource_mut::<Initialization>().attempted = true;
    }
}
fn detect(
    root: &Path,
    settings: &PythonSettings,
    scope: &str,
    managed: bool,
    approved_environment: Option<&str>,
    cancel: &Cancellation,
    progress: &Arc<Mutex<String>>,
) -> Result<String, String> {
    // Do this in Rust: even `python -I` executes venv .pth files before Python
    // code can compare trust/scope. A repository copy cannot confer local consent.
    let bytes = match std::fs::read(root.join("current.json")) {
        Ok(bytes) => bytes,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => Vec::new(),
        Err(e) => return Err(format!("Cannot inspect prepared environment: {e}")),
    };
    if !bytes.is_empty() {
        let manifest: Value = serde_json::from_slice(&bytes)
            .map_err(|e| format!("Invalid prepared environment: {e}"))?;
        if manifest["scope"].as_str() != Some(scope)
            || approved_environment != Some(setup::fingerprint(&bytes).as_str())
        {
            return Ok(json!({"outcome":"not_init", "detail":"Environment scope/local validation changed. Review /setup python before automatic detection can execute it."}).to_string());
        }
    }
    if settings.executable != "python" && !managed {
        let mut command = vivy_process::command(&settings.executable);
        command.args(["-I", "-c", "import sys; assert sys.version_info >= (3,11)"]);
        toolchain::run(&mut command, cancel, progress, 15).map_err(|e| format!("Configured Python is not usable. Correct python.executable or select Use managed Python in /setup python. {e}"))?;
    }
    let Some(python) = prepared_python(root)? else {
        return Ok(json!({"outcome":"not_init", "detail":"Python dependencies are not prepared. Review /setup python."}).to_string());
    };
    let source = worker::package(root)?;
    let memory_root = settings
        .memory_root
        .clone()
        .unwrap_or_else(|| root.join("native-memory"));
    let code = format!(
        "import sys,json;sys.path.insert(0,{});from vivy_workbench.prepare import check;print(json.dumps(check({},scope={},native_memory={},memory_root={})))",
        json!(source),
        json!(root),
        json!(scope),
        if settings.memory { "True" } else { "False" },
        json!(memory_root)
    );
    let mut command = vivy_process::command(python);
    command.args(["-I", "-u", "-c", &code]);
    Ok(toolchain::run(&mut command, cancel, progress, 40)?.to_string())
}
fn open_python(world: &mut World, agent: Entity) {
    open(world, agent, false);
}
#[cfg(feature = "managed-skills")]
pub(crate) fn retain_before_approval(world: &World) -> Result<(), String> {
    let store = crate::managed::PublicationStore::new(
        &world.resource::<ProjectRoot>().0,
        RigHome::of(world).root(),
    );
    if store.root().join("current.json").exists() {
        store.retain_current().map_err(|e| {
            format!("Cannot retain the current publication before changing setup scope: {e}")
        })?;
    }
    Ok(())
}
fn open_memory(world: &mut World, agent: Entity) {
    open(world, agent, true);
}
fn open(world: &mut World, agent: Entity, memory: bool) {
    if world.contains_resource::<TrustedSkillInventory>() {
        refresh_inventory(world);
    }
    let native = memory || world.resource::<PythonSettings>().memory;
    let current = if memory {
        memory_report(world, agent)
    } else {
        report(world, agent)
    };
    if current.phase.is_some() {
        world.write_message(Notice::info(
            agent,
            "Python initialization is still running. Wait for it to finish, then reopen /setup.",
        ));
        return;
    }
    let picker = Picker::new(if memory { "Initialize MemPalace" } else { "Initialize Python" }).for_agent(agent)
        .body(format!("{}\nInstall locked dependencies and trusted Python Skills. If missing, download verified uv 0.11.7 and Python 3.13.12 into the application toolchain directory. No system PATH or registry changes.{}\nApproval permits future repair of this same scope only.", current.detail, if native { " MemPalace is included: download embeddinggemma and validate a disposable local palace." } else { " Memory remains optional." }))
        .item("Approve and prepare", "")
        .item("Use managed Python and prepare", "")
        .item("Skip for this project", "");
    world
        .spawn(picker)
        .observe(move |event: On<Picked>, mut commands: Commands| {
            let choice = event.chosen.as_ref().and_then(|c| c.items.first()).copied();
            commands.queue(move |world: &mut World| {
                let key = if memory { "memory" } else { "python" };
                let result = (|| -> Result<(), String> {
                    let mut decision = setup::decision(world, key)?;
                    if matches!(choice, Some(0 | 1)) {
                        // A picker can predate another preparation. Recheck at
                        // admission, before changing either settings or grants.
                        if !world.resource::<Preparations>().0.is_empty()
                            || world.resource::<Initialization>().pending_save
                        {
                            return Err("Python initialization is still running. Wait for it to finish, then reopen /setup; the existing grant is unchanged.".into());
                        }
                        #[cfg(feature = "managed-skills")]
                        retain_before_approval(world)?;
                        if memory {
                            if !world.resource::<PythonSettings>().memory {
                                let mut init = world.resource_mut::<Initialization>();
                                init.memory_ready = false;
                                if init.report.outcome == OutcomeState::Ready {
                                    init.report.outcome = OutcomeState::NotInit;
                                }
                            }
                            world.resource_mut::<PythonSettings>().memory = true;
                            decision.enabled = true;
                        }
                        if choice == Some(1) {
                            world.resource_mut::<Initialization>().managed = true;
                            decision.managed = true;
                        }
                        let scope = scope(world);
                        decision.grant = Some(scope.clone());
                        decision.skipped = false;
                        decision.scope = scope.clone();
                        setup::remember(world, key, &decision)?;
                        if native && !memory {
                            let mut native_decision = setup::decision(world, "memory")?;
                            native_decision.grant = Some(scope.clone());
                            native_decision.scope = scope.clone();
                            native_decision.skipped = false;
                            setup::remember(world, "memory", &native_decision)?;
                        }
                        // The memory confirmation explicitly includes its Python dependency set.
                        if memory {
                            let mut python = setup::decision(world, "python")?;
                            python.grant = Some(scope.clone());
                            python.skipped = false;
                            python.scope = scope;
                            python.managed = managed(world);
                            setup::remember(world, "python", &python)?;
                        }
                        start_preparation(world, agent);
                    } else {
                        decision.skipped = true;
                        decision.scope = if memory {
                            memory_report(world, agent).scope
                        } else {
                            scope(world)
                        };
                        setup::remember(world, key, &decision)?;
                    }
                    Ok(())
                })();
                if let Err(error) = result {
                    world.write_message(Notice::error(agent, error));
                }
            });
        });
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn memory_setup_waits_for_python_preparation_and_its_local_receipt() {
        let mut app = App::new();
        app.init_resource::<PythonSettings>()
            .init_resource::<Initialization>()
            .init_resource::<Preparations>();
        let agent = app.world_mut().spawn_empty().id();
        app.world_mut().resource_mut::<Initialization>().checked = true;
        app.world_mut().resource_mut::<Initialization>().scope = "core".into();
        let (_sender, receiver) = mpsc::channel();
        app.world_mut().resource_mut::<Preparations>().0.insert(
            agent,
            Preparation {
                cancel: Arc::default(),
                result: Mutex::new(receiver),
                stage: Arc::new(Mutex::new("dependencies".into())),
                checking: false,
                last_stage: String::new(),
            },
        );
        assert_eq!(
            memory_report(app.world(), agent).phase.as_deref(),
            Some("dependencies")
        );
        app.world_mut().resource_mut::<Preparations>().0.clear();
        app.world_mut().resource_mut::<Initialization>().complete(
            false,
            Ok("prepared".into()),
            false,
        );
        assert!(
            memory_report(app.world(), agent).phase.is_some(),
            "the local validation receipt is not saved yet"
        );
        app.world_mut()
            .resource_mut::<Initialization>()
            .pending_save = false;
        assert!(memory_report(app.world(), agent).phase.is_none());
    }

    #[test]
    fn an_already_open_memory_picker_cannot_replace_an_inflight_python_grant() {
        use rig_harness::pick::Chosen;
        let root = std::env::temp_dir().join(format!("vivy-setup-race-{}", std::process::id()));
        let home = root.join("home");
        std::fs::create_dir_all(&root).unwrap();
        let mut app = App::new();
        app.insert_resource(ProjectRoot(root.clone()))
            .insert_resource(RigHome(rig_harness::harness_protocol::Home::at(&home)))
            .init_resource::<PythonSettings>()
            .init_resource::<Initialization>()
            .init_resource::<Preparations>()
            .add_message::<Notice>();
        let agent = app.world_mut().spawn_empty().id();
        app.world_mut().resource_mut::<Initialization>().checked = true;
        app.world_mut().resource_mut::<Initialization>().scope = "core".into();
        setup::save(
            &home,
            &root,
            "python",
            &setup::Decision {
                grant: Some("core".into()),
                scope: "core".into(),
                ..Default::default()
            },
        )
        .unwrap();
        open_memory(app.world_mut(), agent);
        let picker = app
            .world_mut()
            .query_filtered::<Entity, With<Picker>>()
            .single(app.world())
            .unwrap();
        let (_sender, receiver) = mpsc::channel();
        app.world_mut().resource_mut::<Preparations>().0.insert(
            agent,
            Preparation {
                cancel: Arc::default(),
                result: Mutex::new(receiver),
                stage: Arc::new(Mutex::new("dependencies".into())),
                checking: false,
                last_stage: String::new(),
            },
        );
        app.world_mut().trigger(Picked {
            entity: picker,
            chosen: Some(Chosen::item(0)),
        });
        app.world_mut().flush();
        let grant = setup::read(&home, &root, "python").unwrap();
        let memory = setup::read(&home, &root, "memory").unwrap();
        let enabled = app.world().resource::<PythonSettings>().memory;
        std::fs::remove_dir_all(&root).unwrap();
        assert!(
            !enabled,
            "a late picker changed the running preparation's scope"
        );
        assert_eq!(grant.grant.as_deref(), Some("core"));
        assert!(!memory.enabled);
    }

    #[test]
    fn alternate_package_sources_change_the_authorized_scope() {
        let a = source_identity([("UV_EXTRA_INDEX_URL", "https://one.invalid/simple")]);
        let b = source_identity([("UV_EXTRA_INDEX_URL", "https://two.invalid/simple")]);
        assert_ne!(a, b);
        assert_ne!(
            source_identity([("UV_FIND_LINKS", "C:/one")]),
            source_identity([("UV_FIND_LINKS", "C:/two")])
        );
        assert_eq!(
            source_identity([("UV_CACHE_DIR", "C:/one")]),
            source_identity([("UV_CACHE_DIR", "C:/two")])
        );
    }
    #[test]
    fn mismatched_publication_is_not_executed_during_detection() {
        let root =
            std::env::temp_dir().join(format!("vivy-untrusted-detect-{}", std::process::id()));
        let environment = root.join("venv");
        let marker = root.join("untrusted-started");
        std::fs::create_dir_all(&root).unwrap();
        assert!(
            vivy_process::command("python")
                .args(["-I", "-m", "venv", "--without-pip"])
                .arg(&environment)
                .status()
                .unwrap()
                .success()
        );
        let executable = environment.join(if cfg!(windows) {
            "Scripts/python.exe"
        } else {
            "bin/python"
        });
        let output = vivy_process::command(&executable)
            .args([
                "-I",
                "-c",
                "import sysconfig;print(sysconfig.get_path('purelib'))",
            ])
            .output()
            .unwrap();
        let site = PathBuf::from(String::from_utf8(output.stdout).unwrap().trim());
        std::fs::write(
            site.join("untrusted.pth"),
            format!("import pathlib;pathlib.Path({}).touch()\n", json!(marker)),
        )
        .unwrap();
        std::fs::write(
            root.join("current.json"),
            json!({"executable":executable,"scope":"old-untrusted-scope","packages":{}})
                .to_string(),
        )
        .unwrap();
        for scope in ["current-trusted-scope", "old-untrusted-scope"] {
            let result = detect(
                &root,
                &PythonSettings::default(),
                scope,
                false,
                None,
                &Cancellation::default(),
                &Arc::default(),
            )
            .unwrap();
            assert_eq!(
                serde_json::from_str::<Value>(&result).unwrap()["outcome"],
                "not_init"
            );
        }
        let executed = marker.exists();
        std::fs::remove_dir_all(&root).unwrap();
        assert!(
            !executed,
            "Detection executed an old environment before checking its scope"
        );
    }
    #[test]
    fn cancelled_retry_preserves_cause_and_success_clears_failure() {
        let mut init = Initialization::default();
        init.report = Report::error("database refused access");
        init.complete(false, Err("cancelled".into()), true);
        assert_eq!(init.report.detail, "database refused access");
        assert_eq!(init.report.outcome, OutcomeState::Error);
        assert!(!init.pending_save);
        init.complete(false, Ok("validated".into()), false);
        assert_eq!(init.report.outcome, OutcomeState::Ready);
        assert!(init.pending_save);
    }
}
