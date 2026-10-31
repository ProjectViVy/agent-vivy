//! Cached environment metadata plus live worker observations; no periodic imports.
use super::*;

pub(super) fn publish(
    root: Res<ProjectRoot>,
    settings: Res<PythonSettings>,
    jobs: Res<Preparations>,
    workers: Res<Workbenches>,
    initialization: Option<Res<initialization::Initialization>>,
    agents: Query<(Entity, Option<&WorkbenchStatus>), With<Agent>>,
    mut cached: Local<Option<(PathBuf, u64, PythonState)>>,
    mut commands: Commands,
) {
    if cached
        .as_ref()
        .is_none_or(|(path, revision, _)| path != &root.0 || *revision != jobs.2)
    {
        let state = match prepared_python(&root.0.join(".vivy/workbench")) {
            Ok(Some(_)) => PythonState::Prepared,
            Ok(None) => PythonState::Unprepared,
            Err(_) => PythonState::Error,
        };
        *cached = Some((root.0.clone(), jobs.2, state));
    }
    let environment = cached.as_ref().unwrap().2;
    for (agent, previous) in &agents {
        let mut next = WorkbenchStatus {
            python: environment,
            memory: if settings.memory {
                MemoryState::Unchecked
            } else {
                MemoryState::Off
            },
        };
        if let Some(worker) = workers.0.get(&agent) {
            // A call holds this lock while running. Observing the footer must never wait for it.
            if let Ok(mut slot) = worker.worker.try_lock() {
                if slot.as_mut().is_some_and(Worker::stopped) {
                    *slot = None;
                    worker.stopped_status(PythonState::Error);
                }
            }
            next = *worker.status.lock().unwrap_or_else(|e| e.into_inner());
            if settings.memory && next.memory == MemoryState::Off {
                next.memory = MemoryState::Unchecked;
            }
        }
        if jobs.1.contains(&agent) || environment == PythonState::Error {
            next.python = PythonState::Error;
        }
        if let Some(job) = jobs.0.get(&agent) {
            next.python = if job.cancel.cancelled.load(Ordering::SeqCst) {
                PythonState::Stopped
            } else {
                PythonState::Preparing
            };
        }
        if let Some(init) = &initialization {
            use vivy_basics::setup::OutcomeState;
            if init.report.outcome == OutcomeState::Error {
                next.python = PythonState::Error;
            }
            if settings.memory && next.memory != MemoryState::Error {
                next.memory = if init.report.outcome == OutcomeState::Error {
                    MemoryState::Error
                } else if init.memory_ready {
                    MemoryState::Ready
                } else {
                    MemoryState::Unchecked
                };
            }
            if let Some(job) = jobs.0.get(&agent) {
                if !job.cancel.cancelled.load(Ordering::SeqCst) {
                    next.python = if job.checking {
                        PythonState::Checking
                    } else {
                        PythonState::Preparing
                    };
                }
            }
        }
        if previous != Some(&next) {
            commands.entity(agent).insert(next);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn publishes_preparation_outcomes_and_never_waits_on_a_running_cell() {
        let root = std::env::temp_dir().join(format!("vivy-footer-publish-{}", std::process::id()));
        let mut app = App::new();
        app.insert_resource(ProjectRoot(root))
            .insert_resource(PythonSettings::default())
            .init_resource::<Preparations>()
            .init_resource::<Workbenches>()
            .add_message::<Notice>()
            .add_systems(First, (poll, publish).chain());
        let agent = app.world_mut().spawn(Agent).id();
        let second = app.world_mut().spawn(Agent).id();
        app.update();
        assert_eq!(
            app.world().get::<WorkbenchStatus>(agent).unwrap().python,
            PythonState::Unprepared
        );
        let state = Arc::new(Workbench::default());
        state.status.lock().unwrap().python = PythonState::Running;
        app.world_mut()
            .resource_mut::<Workbenches>()
            .0
            .insert(agent, state.clone());
        let _held_by_call = state.worker.lock().unwrap();
        app.update();
        assert_eq!(
            app.world().get::<WorkbenchStatus>(agent).unwrap().python,
            PythonState::Running
        );
        let (send, receive) = mpsc::channel();
        app.world_mut().resource_mut::<Preparations>().0.insert(
            agent,
            Preparation {
                cancel: Arc::default(),
                result: Mutex::new(receive),
                stage: Arc::default(),
                checking: false,
                last_stage: String::new(),
            },
        );
        app.update();
        assert_eq!(
            app.world().get::<WorkbenchStatus>(agent).unwrap().python,
            PythonState::Preparing
        );
        send.send(Err("synthetic install failure".into())).unwrap();
        app.update();
        assert_eq!(
            app.world().get::<WorkbenchStatus>(agent).unwrap().python,
            PythonState::Error
        );
        assert_eq!(
            app.world().get::<WorkbenchStatus>(second).unwrap().python,
            PythonState::Unprepared
        );
        app.update();
        assert_eq!(
            app.world().get::<WorkbenchStatus>(agent).unwrap().python,
            PythonState::Error
        );
        app.world_mut()
            .resource_mut::<Preparations>()
            .1
            .remove(&agent);
        app.update();
        assert_eq!(
            app.world().get::<WorkbenchStatus>(agent).unwrap().python,
            PythonState::Running
        );
    }
}
