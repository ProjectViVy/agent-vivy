//! Continuously drain a worker; only the active request can complete its waiter.
use super::*;
use std::fs::{self, File};
use std::path::Path;

static GENERATION: AtomicU64 = AtomicU64::new(1);
pub(super) fn diagnostic_tail(mut stream: impl Read) -> String {
    let mut tail = Vec::new();
    let mut buffer = [0; 4096];
    while let Ok(count) = stream.read(&mut buffer) {
        if count == 0 {
            break;
        }
        tail.extend_from_slice(&buffer[..count]);
        if tail.len() > 16384 {
            tail.drain(..tail.len() - 16384);
        }
    }
    let mut text = String::from_utf8_lossy(&tail).into_owned();
    for (key, value) in std::env::vars() {
        if value.len() >= 4
            && ["KEY", "TOKEN", "SECRET", "PASSWORD", "CREDENTIAL"]
                .iter()
                .any(|word| key.to_uppercase().contains(word))
        {
            text = text.replace(&value, "[redacted]");
        }
    }
    text
}
const SOURCES: &[(&str, &str)] = &[
    (
        "__init__.py",
        include_str!("../python/vivy_workbench/__init__.py"),
    ),
    ("repl.py", include_str!("../python/vivy_workbench/repl.py")),
    (
        "validation.py",
        include_str!("../python/vivy_workbench/validation.py"),
    ),
    ("mcp.py", include_str!("../python/vivy_workbench/mcp.py")),
    (
        "errors.py",
        include_str!("../python/vivy_workbench/errors.py"),
    ),
    (
        "artifacts.py",
        include_str!("../python/vivy_workbench/artifacts.py"),
    ),
    (
        "capabilities.py",
        include_str!("../python/vivy_workbench/capabilities.py"),
    ),
    (
        "prepare.py",
        include_str!("../python/vivy_workbench/prepare.py"),
    ),
    (
        "memory.py",
        include_str!("../python/vivy_workbench/memory.py"),
    ),
];

pub(super) fn package(root: &Path) -> Result<PathBuf, String> {
    use std::hash::{Hash, Hasher};
    let mut hash = std::collections::hash_map::DefaultHasher::new();
    SOURCES.hash(&mut hash);
    let path = root.join("runtime").join(format!("{:016x}", hash.finish()));
    let modules = path.join("vivy_workbench");
    fs::create_dir_all(&modules).map_err(|e| e.to_string())?;
    for (name, text) in SOURCES {
        let target = modules.join(name);
        if fs::read_to_string(&target).ok().as_deref() != Some(text) {
            fs::write(target, text).map_err(|e| e.to_string())?;
        }
    }
    Ok(path)
}

type Input = Arc<Mutex<ChildStdin>>;
type Reply = Result<(String, bool), String>;
#[derive(Clone)]
pub(super) struct ControlPort {
    input: Input,
    pending: Arc<Mutex<HashMap<String, mpsc::Sender<Result<Value, String>>>>>,
    sequence: Arc<AtomicU64>,
    generation: u64,
    failure: Arc<Mutex<Option<String>>>,
}
impl ControlPort {
    pub fn request(&self, action: &str, server: &str) -> Result<Value, String> {
        if let Some(error) = self.failure.lock().unwrap().clone() {
            return Err(error);
        }
        let id = format!(
            "control:{}:{}",
            self.generation,
            self.sequence.fetch_add(1, Ordering::Relaxed)
        );
        let (tx, rx) = mpsc::channel();
        {
            let mut pending = self.pending.lock().unwrap();
            if pending.len() >= 16 {
                return Err("MCP management queue is full".into());
            }
            pending.insert(id.clone(), tx);
        }
        let result = send(
            &self.input,
            json!({"type":"mcp_control","id":id,"action":action,"server":server}),
        )
        .and_then(|_| {
            rx.recv_timeout(Duration::from_secs(620))
                .map_err(|e| format!("MCP management result uncertain: {e}"))?
        });
        self.pending.lock().unwrap().remove(&id);
        result
    }
}
struct Pending {
    id: String,
    sender: mpsc::Sender<Reply>,
    tail: String,
    text: File,
    path: PathBuf,
    bytes: usize,
    live: Option<Live>,
    usage: Arc<Mutex<Vec<Value>>>,
}

fn send(input: &Input, value: Value) -> Result<(), String> {
    let mut input = input.lock().unwrap_or_else(|e| e.into_inner());
    writeln!(input, "{value}")
        .and_then(|_| input.flush())
        .map_err(|e| e.to_string())
}

pub(super) struct Worker {
    pub management: ControlPort,
    pub process: OwnedChild,
    input: Input,
    pending: Arc<Mutex<Option<Pending>>>,
    failure: Arc<Mutex<Option<String>>>,
    directory: PathBuf,
    generation: u64,
    sequence: u64,
    pub memory: Arc<Mutex<MemoryState>>,
}

fn memory_state(event: &Value) -> MemoryState {
    match event["memory"].as_str() {
        Some("ready") => MemoryState::Ready,
        Some("off") => MemoryState::Off,
        Some("closed") => MemoryState::Closed,
        Some("error") => MemoryState::Error,
        _ => MemoryState::Unchecked,
    }
}

impl Worker {
    pub fn stopped(&mut self) -> bool {
        self.failure
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .is_some()
            || self.process.child.try_wait().ok().flatten().is_some()
    }
    pub fn start(
        settings: &PythonSettings,
        dir: &Path,
        cancellation: &Cancellation,
    ) -> Result<Self, String> {
        let project = if settings.project_root.as_os_str().is_empty() {
            dir
        } else {
            &settings.project_root
        };
        let root = project.join(".vivy/workbench");
        let generation = GENERATION.fetch_add(1, Ordering::Relaxed);
        // Agent IDs are opaque; never interpret them as filesystem paths.
        use std::hash::{Hash, Hasher};
        let mut hash = std::collections::hash_map::DefaultHasher::new();
        settings.agent.hash(&mut hash);
        let agent = format!("{:016x}", hash.finish());
        let mut session_hash = std::collections::hash_map::DefaultHasher::new();
        settings.session.hash(&mut session_hash);
        let timestamp = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .map_err(|e| e.to_string())?
            .as_nanos();
        let directory = root
            .join("artifacts")
            .join(format!("{:016x}", session_hash.finish()))
            .join(agent)
            .join(format!("{}-{generation}-{timestamp}", std::process::id()));
        let source = package(&directory)?;
        fs::create_dir_all(&directory).map_err(|e| e.to_string())?;
        let temporary = directory.join("tmp");
        fs::create_dir_all(&temporary).map_err(|e| e.to_string())?;
        let mut journal =
            File::create(directory.join("events.jsonl")).map_err(|e| e.to_string())?;
        let executable = super::management::prepared_python(&root)?
            .unwrap_or_else(|| settings.executable.clone());
        let skills = settings.skills.clone();
        #[cfg(feature = "managed-skills")]
        let (executable, skills) = if let Some(selected) = &settings.selected {
            let executable = selected.executable().map_err(|e| e.to_string())?.to_owned();
            let mut rows = selected.manifest["skills"]
                .as_array()
                .cloned()
                .unwrap_or_default();
            for skill in selected.skills().map_err(|e| e.to_string())? {
                let mut row = serde_json::to_value(skill).map_err(|e| e.to_string())?;
                row["publication"] = json!(selected.id);
                rows.push(row);
            }
            (executable, json!(rows))
        } else {
            (executable, skills)
        };
        let mut command = vivy_process::command(&executable);
        let bootstrap = format!(
            "import sys;sys.path.insert(0,{});from vivy_workbench.repl import main;main()",
            json!(source)
        );
        command
            .args(["-I", "-u", "-c", &bootstrap])
            .current_dir(dir)
            .env("VIVY_WORKBENCH_ARTIFACTS", &directory)
            .env("VIVY_WORKBENCH_SKILLS", skills.to_string())
            .env("TMP", &temporary)
            .env("TEMP", &temporary)
            .env("SQLITE_TMPDIR", &temporary)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());
        let memory_root = settings.memory.then(|| {
            settings
                .memory_root
                .clone()
                .unwrap_or_else(|| root.join("native-memory"))
        });
        #[cfg(feature = "managed-skills")]
        let memory_root = if let Some(selected) = &settings.selected {
            if selected.manifest["memory_ready"] == true {
                Some(PathBuf::from(
                    selected.manifest["memory_root"]
                        .as_str()
                        .filter(|s| !s.is_empty())
                        .ok_or("Selected publication has no validated native memory root")?,
                ))
            } else {
                None
            }
        } else {
            memory_root
        };
        if let Some(memory_root) = memory_root {
            command.env("VIVY_WORKBENCH_MEMORY_ROOT", memory_root);
        } else {
            command.env_remove("VIVY_WORKBENCH_MEMORY_ROOT");
        }
        let mut process =
            OwnedChild::spawn(&mut command).map_err(|e| format!("Could not start Python: {e}"))?;
        cancellation.bind(process.control());
        let input = Arc::new(Mutex::new(
            process.child.stdin.take().ok_or("missing stdin")?,
        ));
        let output = process.child.stdout.take().ok_or("missing stdout")?;
        let stderr = process.child.stderr.take().ok_or("missing stderr")?;
        // Drain all stderr, retaining only a bounded diagnostic. Never allow a full pipe to wedge exit.
        let diagnostics = std::thread::spawn(move || diagnostic_tail(stderr));
        let pending: Arc<Mutex<Option<Pending>>> = Arc::default();
        let failure = Arc::new(Mutex::new(None));
        let management = ControlPort {
            input: input.clone(),
            pending: Arc::default(),
            sequence: Arc::new(AtomicU64::new(1)),
            generation,
            failure: failure.clone(),
        };
        let control_pending = management.pending.clone();
        let (ready_send, ready) = mpsc::channel();
        let memory = Arc::new(Mutex::new(MemoryState::Unchecked));
        let reported_memory = memory.clone();
        let (active, failed, writer, control, settings) = (
            pending.clone(),
            failure.clone(),
            input.clone(),
            process.control(),
            settings.clone(),
        );
        std::thread::spawn(move || {
            let read = || -> Result<(), String> {
                let mut reader = BufReader::new(output);
                let mut journal_bytes = 0usize;
                loop {
                    let mut bytes = Vec::new();
                    reader
                        .by_ref()
                        .take(2 * 1024 * 1024 + 1)
                        .read_until(b'\n', &mut bytes)
                        .map_err(|e| e.to_string())?;
                    if bytes.is_empty() {
                        return Err("worker stdout closed".into());
                    }
                    if bytes.len() > 2 * 1024 * 1024 || !bytes.ends_with(b"\n") {
                        return Err("oversized or incomplete worker frame".into());
                    }
                    let event: Value = serde_json::from_slice(&bytes)
                        .map_err(|e| format!("invalid worker protocol: {e}"))?;
                    let kind = event["event"].as_str().ok_or("worker event missing type")?;
                    if kind == "control_result" {
                        if let Some(id) = event["id"].as_str() {
                            if let Some(sender) = control_pending.lock().unwrap().remove(id) {
                                let result = if let Some(error) = event["error"].as_str() {
                                    Err(rig_harness::rig_core::redact::text(error).into_owned())
                                } else {
                                    Ok(event["data"].clone())
                                };
                                let _ = sender.send(result);
                            }
                        }
                        continue;
                    }
                    if kind == "host_request" {
                        let result = host_reply(&settings, &event["data"]);
                        send(
                            &writer,
                            json!({"type":"host_reply","id":event["id"],"data":result}),
                        )?;
                        continue;
                    }
                    journal_bytes += bytes.len();
                    if journal_bytes > 256 * 1024 * 1024 {
                        return Err("worker event journal exceeded 256 MiB; artifact incomplete; reset required".into());
                    }
                    journal
                        .write_all(&bytes)
                        .map_err(|e| format!("artifact write failed: {e}"))?;
                    if kind == "ready" {
                        *reported_memory.lock().unwrap_or_else(|e| e.into_inner()) =
                            memory_state(&event);
                        let _ = ready_send.send(event);
                        continue;
                    }
                    if kind == "error" && event["id"].is_null() {
                        return Err("worker reported a protocol error".into());
                    }
                    let mut slot = active.lock().unwrap_or_else(|e| e.into_inner());
                    let Some(call) = slot.as_mut() else {
                        continue;
                    };
                    // Raw fd/thread output has unknown ownership: retain it in the journal only.
                    // Late output retains its old request ID and cannot enter the new call.
                    if event["id"].as_str() != Some(&call.id) {
                        continue;
                    }
                    if kind == "skill_usage" {
                        let mut usage = call.usage.lock().unwrap_or_else(|e| e.into_inner());
                        if usage.len() >= 1024 {
                            return Err("managed usage evidence exceeded 1024 entries in one call; full journal retained".into());
                        }
                        usage.push(event["data"].clone());
                        continue;
                    }
                    if kind == "done" {
                        // Early syntax/dispatch failures do not include a fresh observation.
                        if event.get("memory").is_some() {
                            *reported_memory.lock().unwrap_or_else(|e| e.into_inner()) =
                                memory_state(&event);
                        }
                        call.text.flush().map_err(|e| e.to_string())?;
                        journal.flush().map_err(|e| e.to_string())?;
                        let mut reply = std::mem::take(&mut call.tail);
                        reply.push_str(&format!("\n[complete text: {}; {} bytes; media_type=text/plain; complete=true; protocol and unattributed output: {}]", call.path.display(), call.bytes, call.path.parent().unwrap().join("events.jsonl").display()));
                        let _ = call.sender.send(Ok((reply, event["status"] != "ok")));
                        *slot = None;
                        continue;
                    }
                    let text = match kind {
                        "stdout" | "stderr" | "result" => {
                            event["text"].as_str().unwrap_or("").to_owned()
                        }
                        "error" => format!(
                            "{}: {}\n{}",
                            event["ename"].as_str().unwrap_or("Error"),
                            event["evalue"].as_str().unwrap_or(""),
                            event["traceback"]
                                .as_array()
                                .map(|rows| rows
                                    .iter()
                                    .filter_map(Value::as_str)
                                    .collect::<String>())
                                .unwrap_or_default()
                        ),
                        "artifact" | "display" => format!("\n[artifact: {}]\n", event["artifact"]),
                        _ => return Err(format!("unsupported worker event: {kind}")),
                    };
                    call.text
                        .write_all(text.as_bytes())
                        .map_err(|e| format!("artifact write failed: {e}"))?;
                    call.bytes += text.len();
                    call.tail.push_str(&text);
                    if call.tail.len() > 64 * 1024 {
                        let mut at = call.tail.len() - 64 * 1024;
                        while !call.tail.is_char_boundary(at) {
                            at += 1;
                        }
                        call.tail.drain(..at);
                    }
                    if let Some(live) = &call.live {
                        live.line(&text);
                    }
                }
            };
            if let Err(error) = read() {
                *failed.lock().unwrap_or_else(|e| e.into_inner()) = Some(error.clone());
                for (_, sender) in control_pending.lock().unwrap().drain() {
                    let _ = sender.send(Err(error.clone()));
                }
                if let Some(call) = active.lock().unwrap_or_else(|e| e.into_inner()).take() {
                    let _ = call.sender.send(Err(error));
                }
                control.kill();
            }
        });
        let ready = match ready.recv_timeout(Duration::from_secs(10)) {
            Ok(ready) => ready,
            Err(error) => {
                process.control().kill();
                let details = diagnostics.join().unwrap_or_default();
                return Err(format!("Python startup failed: {error}; {details}"));
            }
        };
        if ready["protocol"] != 1 {
            return Err("incompatible workbench protocol".into());
        }
        Ok(Self {
            management,
            process,
            input,
            pending,
            failure,
            directory,
            generation,
            sequence: 0,
            memory,
        })
    }

    pub fn execute(
        &mut self,
        mut request: Value,
        timeout: Duration,
        live: Option<&Live>,
        cancellation: &Cancellation,
        usage: Arc<Mutex<Vec<Value>>>,
    ) -> Reply {
        if let Some(error) = self
            .failure
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .clone()
        {
            return Err(error);
        }
        self.sequence += 1;
        let id = format!("{}:{}", self.generation, self.sequence);
        let path = self.directory.join(format!("{}.txt", self.sequence));
        let text = File::create(&path).map_err(|e| e.to_string())?;
        let (sender, receiver) = mpsc::channel();
        *cancellation
            .target
            .lock()
            .unwrap_or_else(|e| e.into_inner()) = Some((self.input.clone(), id.clone()));
        if cancellation.cancelled.load(Ordering::SeqCst) {
            return Err("call cancelled before dispatch".into());
        }
        *self.pending.lock().unwrap_or_else(|e| e.into_inner()) = Some(Pending {
            id: id.clone(),
            sender,
            tail: String::new(),
            text,
            path,
            bytes: 0,
            live: live.cloned(),
            usage,
        });
        request["id"] = json!(id);
        send(&self.input, request)?;
        match receiver.recv_timeout(timeout) {
            Ok(reply) => reply,
            Err(error) => {
                let _ = send(&self.input, json!({"type":"interrupt", "id":id}));
                // Give cooperative cancellation a bounded chance to run finally
                // blocks before OwnedChild enforces the process-tree deadline.
                let _ = receiver.recv_timeout(Duration::from_millis(500));
                Err(format!(
                    "execution interrupted, timed out or worker exited: {error}"
                ))
            }
        }
    }

    pub fn close(&mut self) {
        let _ = send(&self.input, json!({"type":"shutdown"}));
        let deadline = Instant::now() + Duration::from_secs(4);
        while Instant::now() < deadline {
            if self.process.child.try_wait().ok().flatten().is_some() {
                return;
            }
            std::thread::sleep(Duration::from_millis(20));
        }
        self.process.control().kill();
    }
}

fn host_reply(settings: &PythonSettings, request: &Value) -> Value {
    let mut settings = settings.clone();
    if !settings.control_home.as_os_str().is_empty() {
        match crate::control::settings_connections(&settings.control_home) {
            Ok(connections) => settings.connections = connections,
            Err(error) => return json!({"ok":false,"error":error}),
        }
    }
    match request["kind"].as_str() {
        Some("mcp.list_connections") => {
            let mut names = settings.connections.keys().collect::<Vec<_>>();
            names.sort();
            json!({"ok":true,"result":{"connections":names.into_iter().map(|id| json!({"connectionId":id,"name":id,"enabled":settings.connections[id].get("enabled").and_then(Value::as_bool).unwrap_or(true)})).collect::<Vec<_>>()}})
        }
        Some("mcp.config") => {
            let name = request["payload"]["server"].as_str().unwrap_or("");
            match settings.connections.get(name).map(resolve_connection) {
                Some(Ok(config)) => json!({"ok":true,"result":config}),
                Some(Err(error)) => json!({"ok":false,"error":error}),
                None => json!({"ok":false,"error":"MCP connection is not configured"}),
            }
        }
        _ => json!({"ok":false,"error":"Unsupported workbench host request"}),
    }
}

fn resolve_connection(source: &Value) -> Result<Value, String> {
    let fields = source
        .as_object()
        .ok_or("MCP connection must be an object")?;
    if fields.keys().any(|name| {
        ![
            "type",
            "command",
            "args",
            "cwd",
            "url",
            "env",
            "headers",
            "enabled",
            "enabledTools",
            "disabledTools",
            "startupTimeoutMs",
            "callTimeoutMs",
        ]
        .contains(&name.as_str())
    }) {
        return Err(
            "Unsupported MCP connection field; OAuth/catalog configuration is not supported".into(),
        );
    }
    let mut config = source.clone();
    for field in ["env", "headers"] {
        if let Some(values) = config.get_mut(field) {
            let values = values
                .as_object_mut()
                .ok_or("MCP credential map must be an object")?;
            for value in values.values_mut() {
                let name = value
                    .get("env")
                    .and_then(Value::as_str)
                    .ok_or("Use {env = 'VARIABLE_NAME'} credential references")?;
                *value = json!(
                    std::env::var(name)
                        .map_err(|_| "MCP credential environment variable is unavailable")?
                );
            }
        }
    }
    Ok(config)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn configuration_rejects_inline_credentials_and_unsupported_auth() {
        assert!(
            resolve_connection(&json!({"type":"http","headers":{"Authorization":"inline-secret"}}))
                .is_err()
        );
        assert!(resolve_connection(&json!({"type":"http","oauth":{}})).is_err());
        let error = resolve_connection(&json!({"type":"http","headers":{"Authorization":{"env":"VIVY_MISSING_MCP_FIXTURE_927"}}})).unwrap_err();
        assert_eq!(error, "MCP credential environment variable is unavailable");
        assert!(resolve_connection(&json!({"type":"stdio","command":"python","args":[]})).is_ok());
    }
}
