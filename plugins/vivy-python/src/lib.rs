//! An auxiliary persistent CPython tool. Native coding tools remain available.

use rig_harness::prelude::*;
use rig_harness::rig_core::{
    serve::{ErasedHandler, adapters::ToolAdapter},
    tool::{ToolContext, ToolExecutionError},
};
use rig_harness::rig_ecs::effects::Handler;
use rig_harness::rig_tools::{Live, OutputLimit};
use serde::Deserialize;
use serde_json::{Value, json};
use std::collections::HashMap;
use std::io::{BufRead, BufReader, Read, Write};
use std::path::PathBuf;
use std::process::{ChildStdin, Stdio};
use std::sync::{
    Arc, Mutex,
    atomic::{AtomicBool, AtomicU64, Ordering},
    mpsc,
};
use std::time::Instant;
use vivy_process::{Control, OwnedChild};

#[cfg(feature = "managed-skills")]
pub mod managed;
mod management;
#[cfg(feature = "memory-tools")]
mod memory_tools;
mod worker;
#[cfg(feature = "memory-tools")]
pub use memory_tools::MemoryToolsPlugin;
use vivy_basics::workbench_status::{MemoryState, PythonState, WorkbenchStatus};
use worker::Worker;

#[derive(Resource, Clone, Deserialize)]
#[serde(default, deny_unknown_fields)]
struct PythonSettings {
    executable: String,
    connections: HashMap<String, Value>,
    memory: bool,
    memory_root: Option<PathBuf>,
    #[serde(skip)]
    skills: Value,
    #[serde(skip)]
    agent: String,
    #[serde(skip)]
    project_root: PathBuf,
    #[serde(skip)]
    session: String,
    #[serde(skip)]
    usage: Arc<Mutex<Vec<Value>>>,
    #[cfg(feature = "managed-skills")]
    #[serde(skip)]
    selected: Option<managed::Publication>,
    #[cfg(feature = "managed-skills")]
    #[serde(skip)]
    user_home: PathBuf,
    #[cfg(feature = "managed-skills")]
    #[serde(skip)]
    managed_requirements: Vec<String>,
    #[cfg(feature = "managed-skills")]
    #[serde(skip)]
    stage_only: bool,
    #[cfg(feature = "managed-skills")]
    #[serde(skip)]
    pinned: bool,
}
impl Default for PythonSettings {
    fn default() -> Self {
        Self {
            executable: "python".into(),
            connections: HashMap::new(),
            memory: false,
            memory_root: None,
            skills: json!([]),
            agent: "standalone".into(),
            project_root: PathBuf::new(),
            session: "unsaved".into(),
            usage: Arc::default(),
            #[cfg(feature = "managed-skills")]
            selected: None,
            #[cfg(feature = "managed-skills")]
            user_home: PathBuf::new(),
            #[cfg(feature = "managed-skills")]
            managed_requirements: Vec::new(),
            #[cfg(feature = "managed-skills")]
            stage_only: false,
            #[cfg(feature = "managed-skills")]
            pinned: false,
        }
    }
}

#[derive(Resource, Default)]
struct Workbenches(HashMap<Entity, Arc<Workbench>>);

#[derive(Default)]
pub struct PythonPlugin;
#[derive(SystemSet, Clone, Debug, PartialEq, Eq, Hash)]
pub struct BindWorkbench;
impl Plugin for PythonPlugin {
    fn build(&self, app: &mut App) {
        app.add_setting::<PythonSettings>("python")
            .init_resource::<Workbenches>();
        management::install(app);
        #[cfg(feature = "managed-skills")]
        managed::install(app);
        app.add_tool_per_call(
            PythonTool::for_directory(PathBuf::new()),
            ToolOptions::default(),
        );
        app.add_systems(
            AgentPass,
            bind_calls
                .in_set(PrepareSystems::Edit)
                .in_set(BindWorkbench),
        );
        app.add_systems(First, (register_credentials, prune_workers).chain());
        app.add_systems(
            Last,
            stop_workers
                .in_set(OnAppExitSystems)
                .run_if(on_message::<rig_harness::AppExit>),
        );
    }
}

fn bind_calls(
    mut calls: Query<
        (Entity, &ToolRequest, &ForAgent, &mut CallHandler),
        (ToolCalls, Added<Admitted>),
    >,
    places: PerCallTools,
    workspace: Res<ProjectWorkspace>,
    settings: Res<PythonSettings>,
    mut workers: ResMut<Workbenches>,
    wake: Res<Wake>,
    mut commands: Commands,
    scratch: Option<Res<Scratch>>,
    skills: Option<Res<vivy_basics::skills::TrustedSkillInventory>>,
    ids: Query<&AgentId>,
    root: Res<ProjectRoot>,
    paths: Option<Res<SessionPaths>>,
    #[cfg(feature = "managed-skills")] pins: Query<&managed::PinnedCall>,
    #[cfg(feature = "managed-skills")] home: Option<Res<RigHome>>,
) {
    for (call, request, agent, mut handler) in &mut calls {
        let name = request.call.function.name.as_str();
        #[cfg(feature = "memory-tools")]
        let memory = memory_tools::is_tool(name);
        #[cfg(not(feature = "memory-tools"))]
        let memory = false;
        if name != PythonTool::NAME && !memory {
            continue;
        }
        if memory && (ids.get(agent.0).is_err() || paths.as_ref().and_then(|p| p.0.id()).is_none())
        {
            commands.entity(call).insert(Outcome::refused(
                "Memory tools require a saved session and stable agent identity",
            ));
            continue;
        }
        let place = places.place(Some(agent.0), workspace.0.clone());
        let state = workers.0.entry(agent.0).or_default().clone();
        let (output, live) = CallOutput::new(&wake);
        commands.entity(call).insert(output);
        let mut configured = settings.clone();
        configured.skills = management::skill_metadata(skills.as_deref());
        configured.agent = ids
            .get(agent.0)
            .map(|id| id.0.clone())
            .unwrap_or_else(|_| format!("{:?}", agent.0));
        configured.project_root = root.0.clone();
        configured.session = paths
            .as_ref()
            .and_then(|p| p.0.id())
            .map(|id| id.to_string())
            .unwrap_or_else(|| "unsaved".into());
        #[cfg(feature = "managed-skills")]
        if let Ok(pin) = pins.get(call) {
            configured.pinned = true;
            configured.user_home = home
                .as_ref()
                .map(|h| h.0.root().to_path_buf())
                .unwrap_or_else(|| RigHome::default().0.root().to_path_buf());
            configured.usage = pin.usage.clone();
            if let Some(id) = &pin.publication {
                match managed::PublicationStore::new(&root.0, &configured.user_home).selected(id) {
                    Ok(selected) => configured.selected = Some(selected),
                    Err(e) => {
                        commands.entity(call).insert(Outcome::refused(format!(
                            "Pinned Skill publication is unavailable: {e}"
                        )));
                        continue;
                    }
                }
            }
        }
        let tool = PythonTool {
            state,
            dir: place.base,
            settings: configured,
            live: Some(live),
            spill: scratch.as_ref().and_then(|scratch| scratch.spill()),
        };
        #[cfg(feature = "memory-tools")]
        if let Some(bound) = memory_tools::bind(name, &tool) {
            *handler = CallHandler(bound);
            continue;
        }
        *handler = CallHandler(Handler(ErasedHandler::new(ToolAdapter::new(tool))));
    }
}

fn register_credentials(settings: Res<PythonSettings>) {
    let names = settings
        .connections
        .values()
        .flat_map(|connection| {
            ["env", "headers"]
                .into_iter()
                .filter_map(move |key| connection.get(key).and_then(Value::as_object))
        })
        .flat_map(|map| {
            map.values()
                .filter_map(|value| value.get("env").and_then(Value::as_str))
                .map(std::ffi::OsString::from)
        })
        .collect::<Vec<_>>();
    vivy_process::register_credential_variables(&names);
}

fn prune_workers(mut workers: ResMut<Workbenches>, agents: Query<(), With<Agent>>) {
    workers.0.retain(|agent, worker| {
        if agents.contains(*agent) {
            worker.refresh_credential_policy();
            true
        } else {
            worker.retire();
            false
        }
    });
}
fn stop_workers(workers: Res<Workbenches>) {
    for worker in workers.0.values() {
        worker.retire();
    }
}

#[derive(Default)]
struct Workbench {
    status: Mutex<WorkbenchStatus>,
    worker: Mutex<Option<Worker>>,
    control: Mutex<Option<Control>>,
    active_cancel: Mutex<Option<Arc<Cancellation>>>,
    retired: AtomicBool,
    policy_generation: AtomicU64,
    publication: Mutex<Option<String>>,
}
impl Workbench {
    fn retire(&self) {
        self.retired.store(true, Ordering::SeqCst);
        self.stop();
    }
    fn refresh_credential_policy(&self) {
        let generation = vivy_process::credential_policy_generation();
        if self.policy_generation.load(Ordering::Acquire) != generation {
            self.stop();
            if let Ok(mut worker) = self.worker.try_lock() {
                *worker = None;
                self.policy_generation.store(generation, Ordering::Release);
            }
        }
    }
    fn stop(&self) {
        self.stopped_status(PythonState::Stopped);
        if let Some(cancel) = self
            .active_cancel
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .take()
        {
            cancel.stop();
            return;
        }
        if let Ok(mut slot) = self.worker.try_lock() {
            if let Some(mut worker) = slot.take() {
                worker.close();
            }
        }
        if let Some(control) = self
            .control
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .as_ref()
        {
            control.kill();
        }
    }

    fn stopped_status(&self, python: PythonState) {
        let mut status = self.status.lock().unwrap_or_else(|e| e.into_inner());
        status.python = python;
        if status.memory != MemoryState::Off {
            status.memory = MemoryState::Unchecked;
        }
    }
}
impl Drop for Workbench {
    fn drop(&mut self) {
        self.stop();
    }
}

#[derive(Default)]
struct Cancellation {
    control: Mutex<Option<Control>>,
    target: Mutex<Option<(Arc<Mutex<ChildStdin>>, String)>>,
    cancelled: AtomicBool,
}
impl Cancellation {
    fn bind(&self, control: Control) {
        let mut slot = self.control.lock().unwrap_or_else(|e| e.into_inner());
        if self.cancelled.load(Ordering::SeqCst) {
            control.kill();
        }
        *slot = Some(control);
    }
    fn stop(&self) {
        let control = self.control.lock().unwrap_or_else(|e| e.into_inner());
        self.cancelled.store(true, Ordering::SeqCst);
        if let Some(control) = control.as_ref() {
            if let Some((input, id)) = self
                .target
                .lock()
                .unwrap_or_else(|e| e.into_inner())
                .as_ref()
            {
                let mut input = input.lock().unwrap_or_else(|e| e.into_inner());
                let _ = writeln!(input, "{}", json!({"type":"interrupt","id":id}));
                let _ = input.flush();
                let control = control.clone();
                std::thread::spawn(move || {
                    std::thread::sleep(Duration::from_millis(500));
                    control.kill();
                });
            } else {
                control.kill();
            }
        }
    }
    fn finish(&self) -> bool {
        let mut control = self.control.lock().unwrap_or_else(|e| e.into_inner());
        let accepted = !self.cancelled.load(Ordering::SeqCst);
        *control = None;
        *self.target.lock().unwrap_or_else(|e| e.into_inner()) = None;
        accepted
    }
}
struct CancelOnDrop(Arc<Cancellation>, bool);
impl Drop for CancelOnDrop {
    fn drop(&mut self) {
        if self.1 {
            self.0.stop();
        }
    }
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct PythonArgs {
    code: String,
    timeout_secs: Option<u64>,
}

#[derive(Clone)]
struct PythonTool {
    state: Arc<Workbench>,
    dir: PathBuf,
    settings: PythonSettings,
    live: Option<Live>,
    spill: Option<Spill>,
}
impl PythonTool {
    fn for_directory(dir: PathBuf) -> Self {
        Self {
            state: Arc::default(),
            dir,
            settings: PythonSettings::default(),
            live: None,
            spill: None,
        }
    }
}

impl PerCallTool for PythonTool {
    /// Each call runs in the calling agent's directory, sharing the worker
    /// that keeps the namespaces between the calls of one agent.
    fn at(&self, place: CallPlace) -> Self {
        Self {
            dir: place.base,
            ..self.clone()
        }
    }
}
impl Tool for PythonTool {
    const NAME: &'static str = "python_repl";
    type Args = PythonArgs;
    type Output = String;
    type Error = ToolExecutionError;
    fn description(&self) -> String {
        "Execute Python in a persistent, auxiliary workbench. Variables and imports persist between calls for this agent; top-level await works. Use the normal coding tools for file edits and Git Bash commands. Python state is NOT saved across exit/session switches. A timeout or cancellation stops the worker and all descendants and resets its variables. Default timeout: 60 seconds. No automatic package installation. Background tasks keep their original output identity. Use workbench.list_skills(), workbench.instructions(name), mcp.list_connections(), and workbench.artifact(value, media_type).".into()
    }
    fn parameters(&self) -> Value {
        json!({"type":"object", "properties":{"code":{"type":"string"},"timeout_secs":{"type":"integer","minimum":1}},"required":["code"],"additionalProperties":false})
    }
    async fn call(&self, _: &mut ToolContext, args: PythonArgs) -> Result<String, Self::Error> {
        let cancellation = Arc::<Cancellation>::default();
        let mut guard = CancelOnDrop(cancellation.clone(), true);
        let tool = self.clone();
        let result = blocking(move || tool.execute(args, &cancellation)).await;
        guard.1 = false;
        let (text, error) = result?;
        if error {
            Err(ToolExecutionError::other(text))
        } else {
            Ok(text)
        }
    }
}

impl PythonTool {
    fn execute(
        &self,
        args: PythonArgs,
        cancellation: &Arc<Cancellation>,
    ) -> Result<(String, bool), ToolExecutionError> {
        self.execute_frame(
            json!({"type":"execute","code":args.code}),
            args.timeout_secs,
            cancellation,
        )
    }

    fn execute_frame(
        &self,
        request: Value,
        timeout_secs: Option<u64>,
        cancellation: &Arc<Cancellation>,
    ) -> Result<(String, bool), ToolExecutionError> {
        let error = |why| {
            ToolExecutionError::other(format!("Python workbench stopped; variables reset: {why}"))
        };
        let mut slot = self.state.worker.lock().unwrap_or_else(|e| e.into_inner());
        if cancellation.cancelled.load(Ordering::SeqCst) {
            return Err(error("call cancelled before execution".into()));
        }
        {
            let mut active = self
                .state
                .active_cancel
                .lock()
                .unwrap_or_else(|e| e.into_inner());
            if self.state.retired.load(Ordering::SeqCst) {
                return Err(error(
                    "this workbench was closed/reset; queued code was not executed".into(),
                ));
            }
            *active = Some(cancellation.clone());
        }
        let error = |why| {
            self.state
                .stopped_status(if cancellation.cancelled.load(Ordering::SeqCst) {
                    PythonState::Stopped
                } else {
                    PythonState::Error
                });
            error(why)
        };
        let generation = vivy_process::credential_policy_generation();
        let project = if self.settings.project_root.as_os_str().is_empty() {
            &self.dir
        } else {
            &self.settings.project_root
        };
        let publication =
            management::prepared_python(&project.join(".vivy/workbench")).map_err(error)?;
        #[cfg(feature = "managed-skills")]
        if self.settings.pinned && self.settings.selected.is_none() && publication.is_some() {
            return Err(error("No workbench publication was selected for this operation; re-plan after setup completes".into()));
        }
        #[cfg(feature = "managed-skills")]
        let publication = if let Some(selected) = &self.settings.selected {
            managed::PublicationStore::new(project, &self.settings.user_home)
                .selected(&selected.id)
                .map_err(|e| error(e.to_string()))?;
            Some(selected.id.clone())
        } else {
            publication
        };
        {
            let mut previous = self
                .state
                .publication
                .lock()
                .unwrap_or_else(|e| e.into_inner());
            if *previous != publication {
                *slot = None;
                *previous = publication;
            }
        }
        if self.state.policy_generation.load(Ordering::Acquire) != generation {
            *slot = None;
            self.state
                .policy_generation
                .store(generation, Ordering::Release);
        }
        if slot.as_mut().is_some_and(Worker::stopped) {
            *slot = None;
        }
        let fresh = slot.is_none();
        if fresh {
            self.state
                .status
                .lock()
                .unwrap_or_else(|e| e.into_inner())
                .python = PythonState::Starting;
        }
        let mut worker = match slot.take() {
            Some(worker) => worker,
            None => Worker::start(&self.settings, &self.dir, cancellation).map_err(error)?,
        };
        cancellation.bind(worker.process.control());
        *self.state.status.lock().unwrap_or_else(|e| e.into_inner()) = WorkbenchStatus {
            python: PythonState::Running,
            memory: *worker.memory.lock().unwrap_or_else(|e| e.into_inner()),
        };
        *self.state.control.lock().unwrap_or_else(|e| e.into_inner()) =
            Some(worker.process.control());
        let timeout = Duration::from_secs(timeout_secs.unwrap_or(60).max(1));
        self.settings
            .usage
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .clear();
        let reply = worker
            .execute(
                request,
                timeout,
                self.live.as_ref(),
                cancellation,
                self.settings.usage.clone(),
            )
            .map_err(error)?;
        {
            let mut active = self
                .state
                .active_cancel
                .lock()
                .unwrap_or_else(|e| e.into_inner());
            if self.state.retired.load(Ordering::SeqCst) || !cancellation.finish() {
                return Err(error("call cancelled; worker closed".into()));
            }
            *self.state.status.lock().unwrap_or_else(|e| e.into_inner()) = WorkbenchStatus {
                python: PythonState::Ready,
                memory: *worker.memory.lock().unwrap_or_else(|e| e.into_inner()),
            };
            *slot = Some(worker);
            *active = None;
        }
        let (kept, dropped) = OutputLimit::default().tail(&reply.0);
        let mut text = if fresh {
            "[Fresh Python workbench; previous process/session variables are not restored.]\n"
                .to_owned()
        } else {
            String::new()
        };
        text.push_str(kept);
        if dropped > 0 {
            if let Some(spill) = &self.spill {
                let (path, mut file) = spill.create("python").map_err(|e| error(e.to_string()))?;
                file.write_all(reply.0.as_bytes())
                    .map_err(|e| error(e.to_string()))?;
                text.push_str(&format!(
                    "\n[bounded tool view: {}; complete text path is above]",
                    path.display()
                ));
            } else {
                text.push_str("\n[output truncated]");
            }
        }
        Ok((text, reply.1))
    }
}

#[cfg(test)]
mod tests {
    #[test]
    fn new_publication_is_adopted_on_the_next_call_without_manual_reset() {
        use super::*;
        let root = std::env::temp_dir().join(format!("vivy-adoption-{}", std::process::id()));
        std::fs::create_dir_all(root.join(".vivy/workbench")).unwrap();
        let tool = PythonTool::for_directory(root.clone());
        call(&tool, "old_environment_value=42", None).unwrap();
        let output = vivy_process::command("python")
            .args(["-I", "-c", "import sys;print(sys.executable)"])
            .output()
            .unwrap();
        let executable = String::from_utf8(output.stdout).unwrap();
        std::fs::write(
            root.join(".vivy/workbench/current.json"),
            json!({"executable":executable.trim()}).to_string(),
        )
        .unwrap();
        let result = call(
            &tool,
            "assert 'old_environment_value' not in globals(); print('ADOPTED')",
            None,
        )
        .unwrap();
        assert!(result.contains("ADOPTED") && result.contains("Fresh Python workbench"));
        tool.state.retire();
        std::fs::remove_dir_all(root).unwrap();
    }
    #[test]
    fn status_tracks_real_worker_failure_recovery_and_memory_unavailability() {
        use vivy_basics::workbench_status::{MemoryState, PythonState};
        let mut tool = PythonTool::for_directory(std::env::temp_dir());
        tool.settings.memory = true;
        tool.settings.memory_root =
            Some(std::env::temp_dir().join(format!("vivy-missing-assets-{}", std::process::id())));
        call(&tool, "value=42", None).unwrap();
        let observed = *tool.state.status.lock().unwrap();
        assert_eq!(observed.python, PythonState::Ready);
        assert_eq!(observed.memory, MemoryState::Error);
        assert!(call(&tool, "def broken(", None).is_err());
        assert_eq!(tool.state.status.lock().unwrap().memory, MemoryState::Error);
        assert!(call(&tool, "class UserObject:\n @property\n def closed(self): raise RuntimeError('status side effect')\nmemory=UserObject()\n42", None).unwrap().contains("42"));
        assert!(call(&tool, "raise ValueError('ordinary error')", None).is_err());
        assert_eq!(tool.state.status.lock().unwrap().python, PythonState::Ready);
        assert!(call(&tool, "import os;os._exit(7)", None).is_err());
        assert_eq!(tool.state.status.lock().unwrap().python, PythonState::Error);
        call(&tool, "42", None).unwrap();
        assert_eq!(tool.state.status.lock().unwrap().python, PythonState::Ready);
        tool.state.stop();
        assert_eq!(
            tool.state.status.lock().unwrap().python,
            PythonState::Stopped
        );
    }

    #[test]
    fn closing_a_workbench_rejects_already_bound_queued_handlers() {
        let tool = PythonTool::for_directory(std::env::temp_dir());
        let marker = std::env::temp_dir().join(format!("vivy-retire-{}", std::process::id()));
        let queued_marker = marker.with_extension("queued");
        let _ = std::fs::remove_file(&marker);
        let _ = std::fs::remove_file(&queued_marker);
        let active = tool.clone();
        let code = format!(
            "import pathlib,asyncio;pathlib.Path({}).write_text('started');await asyncio.sleep(30)",
            json!(marker)
        );
        let running = std::thread::spawn(move || call(&active, &code, None));
        let deadline = Instant::now() + Duration::from_secs(5);
        while !marker.exists() && Instant::now() < deadline {
            std::thread::sleep(Duration::from_millis(10));
        }
        assert!(marker.exists());
        let queued = tool.clone();
        let code = format!(
            "import pathlib;pathlib.Path({}).write_text('must not run')",
            json!(queued_marker)
        );
        let waiting = std::thread::spawn(move || call(&queued, &code, None));
        tool.state.retire();
        assert!(running.join().unwrap().is_err());
        let queued_result = waiting.join().unwrap();
        let wrote = queued_marker.exists();
        let _ = std::fs::remove_file(marker);
        let _ = std::fs::remove_file(queued_marker);
        assert!(
            queued_result.is_err() && !wrote,
            "retired handler restarted a worker and executed queued code"
        );
        assert!(
            call(&PythonTool::for_directory(std::env::temp_dir()), "42", None)
                .unwrap()
                .contains("42")
        );
    }
    #[test]
    fn a_large_unicode_exception_preserves_the_namespace() {
        let tool = PythonTool::for_directory(std::env::temp_dir());
        call(&tool, "value=42", None).unwrap();
        assert!(call(&tool, "raise ValueError(chr(0x1f600) * 100000)", None).is_err());
        assert!(call(&tool, "value", None).unwrap().contains("42"));
    }
    #[test]
    fn an_idle_crash_starts_fresh_on_the_next_call() {
        let tool = PythonTool::for_directory(std::env::temp_dir());
        call(&tool, "value=42", None).unwrap();
        {
            let mut state = tool.state.worker.lock().unwrap();
            let worker = state.as_mut().unwrap();
            worker.process.control().kill();
            worker.process.child.wait().unwrap();
        }
        let reply = call(
            &tool,
            "assert 'value' not in globals();print('RECOVERED_IDLE')",
            None,
        )
        .unwrap();
        assert!(reply.contains("RECOVERED_IDLE"));
        assert!(reply.contains("Fresh Python"));
    }
    #[test]
    fn cancellation_allows_the_running_cell_to_unwind_before_closing() {
        use futures::{FutureExt, poll};
        let tool = PythonTool::for_directory(std::env::temp_dir());
        let marker = std::env::temp_dir().join(format!("vivy-unwind-{}", std::process::id()));
        let started = marker.with_extension("started");
        let _ = std::fs::remove_file(&marker);
        let _ = std::fs::remove_file(&started);
        let code = format!(
            "import asyncio,pathlib\npathlib.Path({}).write_text('started')\ntry:\n await asyncio.sleep(60)\nfinally:\n pathlib.Path({}).write_text('unwound')",
            json!(started),
            json!(marker)
        );
        futures::executor::block_on(async {
            let mut context = ToolContext::default();
            let mut future = tool
                .call(
                    &mut context,
                    PythonArgs {
                        code,
                        timeout_secs: None,
                    },
                )
                .boxed();
            assert!(poll!(future.as_mut()).is_pending());
            let deadline = Instant::now() + Duration::from_secs(5);
            while !started.exists() && Instant::now() < deadline {
                std::thread::sleep(Duration::from_millis(20));
            }
            assert!(started.exists(), "cell did not start");
            drop(future);
        });
        let deadline = Instant::now() + Duration::from_secs(3);
        while !marker.exists() && Instant::now() < deadline {
            std::thread::sleep(Duration::from_millis(20));
        }
        assert_eq!(std::fs::read_to_string(&marker).unwrap(), "unwound");
        std::fs::remove_file(marker).unwrap();
        std::fs::remove_file(started).unwrap();
    }
    #[test]
    fn late_output_cannot_complete_or_pollute_the_next_call() {
        let tool = PythonTool::for_directory(std::env::temp_dir());
        call(&tool, "import asyncio\nasync def later():\n await asyncio.sleep(.1)\n print('OLD_CALL_MARKER')\ntask=asyncio.create_task(later())", None).unwrap();
        let reply = call(&tool, "await task\n'CURRENT_CALL_MARKER'", None).unwrap();
        assert!(reply.contains("CURRENT_CALL_MARKER"));
        assert!(!reply.contains("OLD_CALL_MARKER"));
    }

    #[test]
    fn cancelling_a_queued_call_does_not_execute_it_or_stop_active_state() {
        let tool = PythonTool::for_directory(std::env::temp_dir());
        call(&tool, "answer=42", None).unwrap();
        let cancellation = Arc::<Cancellation>::default();
        cancellation.stop();
        assert!(
            tool.execute(
                PythonArgs {
                    code: "answer=0".into(),
                    timeout_secs: None
                },
                &cancellation
            )
            .is_err()
        );
        assert!(call(&tool, "answer", None).unwrap().contains("42"));
    }

    #[test]
    fn crashed_worker_recovers_on_the_next_call() {
        let tool = PythonTool::for_directory(std::env::temp_dir());
        assert!(call(&tool, "import os;os._exit(7)", None).is_err());
        assert!(call(&tool, "42", None).unwrap().contains("42"));
    }
    #[test]
    fn credential_policy_change_restarts_persistent_workers() {
        if std::env::var_os("VIVY_POLICY_TEST_CHILD").is_none() {
            let status = std::process::Command::new(std::env::current_exe().unwrap())
                .args([
                    "--exact",
                    "tests::credential_policy_change_restarts_persistent_workers",
                    "--nocapture",
                ])
                .env("VIVY_POLICY_TEST_CHILD", "1")
                .status()
                .unwrap();
            assert!(status.success());
            return;
        }
        let tool = PythonTool::for_directory(std::env::temp_dir());
        call(&tool, "marker_before_policy = 42", None).unwrap();
        vivy_process::register_credential_variables(&["VIVY_POLICY_EXPANSION_FIXTURE".into()]);
        let result = call(
            &tool,
            "'STALE_POLICY' if 'marker_before_policy' in globals() else 'FRESH_POLICY'",
            None,
        )
        .unwrap();
        assert!(result.contains("FRESH_POLICY"), "{result}");
    }
    use super::*;
    fn call(
        tool: &PythonTool,
        code: &str,
        timeout_secs: Option<u64>,
    ) -> Result<String, ToolExecutionError> {
        futures::executor::block_on(tool.call(
            &mut ToolContext::default(),
            PythonArgs {
                code: code.into(),
                timeout_secs,
            },
        ))
    }

    #[test]
    fn persistent_workbench_and_independent_agents() {
        let tool = PythonTool::for_directory(std::env::temp_dir());
        call(&tool, "value = 40", None).unwrap();
        assert!(
            call(
                &tool,
                "import asyncio\nawait asyncio.sleep(0)\nvalue + 2",
                None
            )
            .unwrap()
            .contains("42")
        );
        assert!(call(&tool, "raise ValueError('expected')", None).is_err());
        assert!(call(&tool, "value", None).unwrap().contains("40"));
        let other = PythonTool::for_directory(std::env::temp_dir());
        assert!(call(&other, "value", None).is_err());
    }

    #[test]
    fn a_large_unicode_result_does_not_reset_the_workbench() {
        let tool = PythonTool::for_directory(std::env::temp_dir());
        call(&tool, "s = chr(0x1F600) * 600000; s", None).unwrap();
        assert!(call(&tool, "len(s)", None).unwrap().contains("600000"));
    }

    #[test]
    fn timeout_resets_variables_and_the_next_call_recovers() {
        let tool = PythonTool::for_directory(std::env::temp_dir());
        let start = std::time::Instant::now();
        assert!(
            call(&tool, "value = 42\nimport time\ntime.sleep(30)", Some(1))
                .unwrap_err()
                .to_string()
                .contains("reset")
        );
        assert!(start.elapsed().as_secs() < 5);
        assert!(
            call(
                &tool,
                "'fresh' if 'value' not in globals() else 'stale'",
                None
            )
            .unwrap()
            .contains("fresh")
        );
    }

    #[test]
    fn dropping_a_tool_future_stops_the_worker() {
        use futures::{FutureExt, poll};
        let tool = PythonTool::for_directory(std::env::temp_dir());
        call(&tool, "value = 1", None).unwrap();
        futures::executor::block_on(async {
            let mut context = ToolContext::default();
            let mut future = tool
                .call(
                    &mut context,
                    PythonArgs {
                        code: "import time\ntime.sleep(60)".into(),
                        timeout_secs: None,
                    },
                )
                .boxed();
            assert!(poll!(future.as_mut()).is_pending());
            std::thread::sleep(std::time::Duration::from_millis(100));
            drop(future);
        });
        let start = std::time::Instant::now();
        assert!(
            call(
                &tool,
                "'fresh' if 'value' not in globals() else 'stale'",
                None
            )
            .unwrap()
            .contains("fresh")
        );
        assert!(start.elapsed().as_secs() < 5);
    }
}
