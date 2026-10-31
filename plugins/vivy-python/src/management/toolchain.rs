use super::*;

/// Owned, bounded subprocess for both bootstrap and the fixed Python workflow.
pub(crate) fn run(
    command: &mut std::process::Command,
    cancel: &Cancellation,
    progress: &Arc<Mutex<String>>,
    seconds: u64,
) -> Result<Value, String> {
    run_bounded(command, cancel, progress, seconds, None)
}
pub(crate) fn run_bounded(
    command: &mut std::process::Command,
    cancel: &Cancellation,
    progress: &Arc<Mutex<String>>,
    seconds: u64,
    budget: Option<(&Path, u64)>,
) -> Result<Value, String> {
    command
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());
    let mut child =
        OwnedChild::spawn(command).map_err(|e| format!("Initialization could not start: {e}"))?;
    cancel.bind(child.control());
    let errors = child
        .child
        .stderr
        .take()
        .ok_or("missing initialization stderr")?;
    let errors = std::thread::spawn(move || worker::diagnostic_tail(errors));
    let output = child
        .child
        .stdout
        .take()
        .ok_or("missing initialization stdout")?;
    let progress = progress.clone();
    let output = std::thread::spawn(move || {
        let mut reader = BufReader::new(output);
        let mut last = Value::Null;
        loop {
            let mut line = Vec::new();
            let count = reader
                .by_ref()
                .take(65537)
                .read_until(b'\n', &mut line)
                .unwrap_or(0);
            if count == 0 {
                break;
            }
            if count > 65536 {
                continue;
            }
            if let Ok(value) = serde_json::from_slice::<Value>(&line) {
                if value["type"] == "stage" {
                    if let Some(stage) = value["stage"].as_str() {
                        *progress.lock().unwrap_or_else(|e| e.into_inner()) =
                            stage.chars().take(80).collect();
                    }
                } else {
                    last = value;
                }
            }
        }
        last
    });
    let deadline = Instant::now() + Duration::from_secs(seconds);
    let status = loop {
        if let Some((root, limit)) = budget {
            let size = directory_bytes(root);
            if size.as_ref().map_or(true, |bytes| *bytes > limit) {
                child.control().kill();
                let _ = child.child.wait();
                let _ = output.join();
                let _ = errors.join();
                return Err(format!(
                    "Validation scratch budget exhausted or unreadable: {}",
                    size.map(|n| format!("{n} bytes, limit {limit}"))
                        .unwrap_or_else(|e| e.to_string())
                ));
            }
        }
        if Instant::now() >= deadline || cancel.cancelled.load(Ordering::SeqCst) {
            child.control().kill();
            let _ = child.child.wait();
            let _ = output.join();
            let _ = errors.join();
            return Err(if cancel.cancelled.load(Ordering::SeqCst) {
                "Initialization cancelled"
            } else {
                "Initialization timed out; previous environment preserved"
            }
            .into());
        }
        if let Some(status) = child.child.try_wait().map_err(|e| e.to_string())? {
            break status;
        }
        std::thread::sleep(Duration::from_millis(50));
    };
    // A finished parent may leave a descendant holding the pipes. Terminate the
    // owned tree before joining readers, including on successful parent exit.
    child.control().kill();
    let result = output.join().unwrap_or(Value::Null);
    let diagnostic = errors.join().unwrap_or_default();
    if !status.success() {
        return Err(format!(
            "Initialization failed ({status}); previous environment preserved. {diagnostic}"
        ));
    }
    Ok(result)
}

pub(crate) fn directory_bytes(root: &Path) -> std::io::Result<u64> {
    if !root.exists() {
        return Ok(0);
    }
    let mut total = 0u64;
    for item in std::fs::read_dir(root)? {
        let path = item?.path();
        let metadata = std::fs::symlink_metadata(&path)?;
        if metadata.file_type().is_symlink() {
            return Err(std::io::Error::other("staging symlinks are unsupported"));
        }
        total = total.saturating_add(if metadata.is_dir() {
            directory_bytes(&path)?
        } else {
            metadata.len()
        });
    }
    Ok(total)
}

pub(super) fn ensure(
    home: &Path,
    python: &str,
    managed: bool,
    cancel: &Cancellation,
    progress: &Arc<Mutex<String>>,
) -> Result<(String, String), String> {
    if !cfg!(windows) {
        return Ok((python.to_owned(), "uv".into()));
    }
    let root = home.join("toolchains");
    std::fs::create_dir_all(&root).map_err(|e| e.to_string())?;
    let script = root.join(format!(
        "bootstrap-{}.ps1",
        vivy_basics::setup::fingerprint(include_bytes!("../../bootstrap.ps1"))
    ));
    std::fs::write(&script, include_bytes!("../../bootstrap.ps1")).map_err(|e| e.to_string())?;
    let mut command = vivy_process::command("powershell.exe");
    command
        .args([
            "-NoProfile",
            "-NonInteractive",
            "-ExecutionPolicy",
            "Bypass",
            "-File",
        ])
        .arg(script)
        .arg("-Root")
        .arg(root)
        .arg("-Python")
        .arg(python);
    if managed {
        command.arg("-Managed");
    }
    let value = run(&mut command, cancel, progress, 300)?;
    Ok((
        value["python"]
            .as_str()
            .ok_or("Bootstrap did not return a Python executable")?
            .into(),
        value["uv"]
            .as_str()
            .ok_or("Bootstrap did not return uv")?
            .into(),
    ))
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn parent_exit_cannot_leave_a_descendant_holding_initialization_pipes() {
        let mut command = vivy_process::command("python");
        command.args(["-I", "-c", "import subprocess,sys;subprocess.Popen([sys.executable,'-I','-c','import time;time.sleep(4)']);print('{}',flush=True)"]);
        let start = Instant::now();
        run(&mut command, &Cancellation::default(), &Arc::default(), 1).unwrap();
        assert!(
            start.elapsed() < Duration::from_secs(2),
            "A descendant outlived the preparation deadline: {:?}",
            start.elapsed()
        );
    }
}
