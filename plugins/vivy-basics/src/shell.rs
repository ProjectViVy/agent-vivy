//! A Git Bash shell with Windows process-tree ownership; Rig's shell elsewhere.

#[cfg(not(windows))]
#[derive(Default)]
pub struct ShellTool;
#[cfg(not(windows))]
impl rig_harness::prelude::Plugin for ShellTool {
    fn build(&self, app: &mut rig_harness::prelude::App) {
        use rig_harness::prelude::*;
        use rig_harness::rig_tools::Shell;
        let processes = app.world_mut().get_resource_or_init::<Processes>().clone();
        // Each call runs with the launcher in the agent's directory.
        app.add_live_tool_per_call(
            Shell {
                launcher: processes.launcher(),
                ..Shell::default()
            },
            ToolOptions::default(),
        );
    }
}

#[cfg(windows)]
pub use native::ShellTool;

#[cfg(windows)]
mod native {
    use rig_harness::prelude::*;
    use rig_harness::rig_core::{
        completion::ToolAnnotations,
        tool::{ExitStatus, ToolContext, ToolExecutionError},
    };
    use rig_harness::rig_tools::{
        Live, OutputLimit, Shell, ShellArgs, Spill,
        workspace::{Local, Workspace},
    };
    use serde::Deserialize;
    use std::ffi::OsString;
    use std::io::{Read, Write};
    use std::path::{Path, PathBuf};
    use std::process::Stdio;
    use std::sync::{Arc, mpsc};
    use std::time::{Duration, Instant};
    use vivy_process::{AbortGuard, OwnedChild};

    #[derive(Resource, Default, Deserialize, Clone)]
    #[serde(default, deny_unknown_fields)]
    struct ShellSettings {
        executable: String,
    }

    #[derive(Default)]
    pub struct ShellTool;
    impl Plugin for ShellTool {
        fn build(&self, app: &mut App) {
            app.add_setting::<ShellSettings>("shell");
            let executable = app.world().resource::<ShellSettings>().executable.clone();
            let processes = app.world_mut().get_resource_or_init::<Processes>().clone();
            app.add_live_tool_per_call(
                NativeShell {
                    executable,
                    processes,
                    ..NativeShell::for_directory(PathBuf::new())
                },
                ToolOptions::default(),
            );
        }
    }

    #[derive(Clone)]
    struct NativeShell {
        dir: PathBuf,
        workspace: Arc<dyn Workspace>,
        confine: bool,
        live: Option<Live>,
        spill: Option<Spill>,
        executable: String,
        processes: Processes,
    }
    impl NativeShell {
        fn for_directory(dir: PathBuf) -> Self {
            Self {
                dir,
                workspace: Arc::new(Local),
                confine: false,
                live: None,
                spill: None,
                executable: String::new(),
                processes: Processes::default(),
            }
        }
    }

    impl PerCallTool for NativeShell {
        /// Each call runs in the agent's own directory, keeping the
        /// configured shell and the process owner from the registration.
        fn at(&self, place: CallPlace) -> Self {
            Self {
                dir: place.base,
                workspace: place.workspace,
                confine: place.confine,
                live: place.live,
                spill: place.spill,
                executable: self.executable.clone(),
                processes: self.processes.clone(),
            }
        }
    }

    fn bash(executable: &str) -> Result<PathBuf, ToolExecutionError> {
        let mut candidates = Vec::new();
        if !executable.is_empty() {
            candidates.push(PathBuf::from(executable));
        } else {
            for path in std::env::split_paths(&std::env::var_os("PATH").unwrap_or_default()) {
                if path.join("git.exe").is_file() {
                    for ancestor in path.ancestors().take(3) {
                        candidates.push(ancestor.join("bin/bash.exe"));
                    }
                }
            }
            if let Some(root) = std::env::var_os("ProgramFiles") {
                candidates.push(PathBuf::from(root).join("Git/bin/bash.exe"));
            }
        }
        candidates.into_iter().find(|path| path.is_file()).ok_or_else(|| ToolExecutionError::other(
            "Git Bash was not found; install Git for Windows or set [shell].executable in RIG_HOME/settings.toml"))
    }

    impl Tool for NativeShell {
        const NAME: &'static str = "shell";
        type Args = ShellArgs;
        type Output = String;
        type Error = ToolExecutionError;
        fn description(&self) -> String {
            format!(
                "{} Commands run in Git Bash on native Windows, not WSL.",
                Shell::default().description()
            )
        }
        fn parameters(&self) -> serde_json::Value {
            Shell::default().parameters()
        }
        fn annotations(&self) -> ToolAnnotations {
            Shell::default().annotations()
        }
        async fn call(
            &self,
            context: &mut ToolContext,
            args: ShellArgs,
        ) -> Result<String, Self::Error> {
            let executable = bash(&self.executable)?;
            let cwd = match args.cwd.as_deref() {
                Some(cwd) => {
                    self.workspace.place(Some(&self.dir), self.confine, cwd)?;
                    self.workspace.join(&self.dir, Path::new(cwd))
                }
                None => self.dir.clone(),
            };
            let mut command = self.processes.launcher().in_dir(cwd).command(&executable);
            let root = executable
                .parent()
                .and_then(Path::parent)
                .unwrap_or(Path::new("."));
            let paths = [root.join("usr/bin"), root.join("bin")]
                .into_iter()
                .chain(std::env::split_paths(
                    &std::env::var_os("PATH").unwrap_or_default(),
                ))
                .collect::<Vec<_>>();
            let path: OsString = std::env::join_paths(paths)
                .map_err(|error| ToolExecutionError::other(error.to_string()))?;
            command
                .args(["--noprofile", "--norc", "-c", &args.command])
                .env("PATH", path)
                .stdin(Stdio::null())
                .stdout(Stdio::piped())
                .stderr(Stdio::piped());
            let mut process = OwnedChild::spawn(&mut command).map_err(|error| {
                ToolExecutionError::other(format!("could not start Git Bash: {error}"))
            })?;
            let mut abort = AbortGuard::new(process.control());
            let (sender, receiver) = mpsc::sync_channel(16);
            let pipes: Vec<Box<dyn Read + Send>> = vec![
                Box::new(process.child.stdout.take().unwrap()),
                Box::new(process.child.stderr.take().unwrap()),
            ];
            for mut pipe in pipes {
                let sender = sender.clone();
                std::thread::spawn(move || {
                    let mut buffer = [0; 4096];
                    while let Ok(len) = pipe.read(&mut buffer) {
                        if len == 0 || sender.send(buffer[..len].to_vec()).is_err() {
                            break;
                        }
                    }
                });
            }
            drop(sender);
            let live = self.live.clone();
            let spill = self.spill.clone();
            let result = blocking(move || {
                let run = || -> std::io::Result<_> {
                    let deadline = args
                        .timeout_secs
                        .map(|seconds| Instant::now() + Duration::from_secs(seconds.max(1)));
                    let mut output = Vec::new();
                    let mut timed_out = false;
                    loop {
                        if deadline.is_some_and(|deadline| Instant::now() >= deadline) {
                            timed_out = true;
                            process.control().kill();
                            break;
                        }
                        match receiver.recv_timeout(Duration::from_millis(25)) {
                            Ok(bytes) => {
                                if let Some(live) = &live {
                                    live.line(&String::from_utf8_lossy(&bytes));
                                }
                                output.extend(bytes);
                                if output.len() > 10 * 1024 * 1024 {
                                    process.control().kill();
                                    break;
                                }
                            }
                            Err(mpsc::RecvTimeoutError::Disconnected) => {
                                if process.child.try_wait()?.is_some() {
                                    break;
                                }
                                std::thread::sleep(Duration::from_millis(25));
                            }
                            Err(mpsc::RecvTimeoutError::Timeout) => {
                                if process.child.try_wait()?.is_some() {
                                    process.control().kill();
                                }
                            }
                        }
                    }
                    let status = process.child.wait()?;
                    let text = String::from_utf8_lossy(&output);
                    let (tail, dropped) = OutputLimit::default().tail(&text);
                    let mut result = tail.to_owned();
                    if dropped > 0 {
                        if let Some(spill) = spill {
                            let (path, mut file) = spill.create("shell")?;
                            file.write_all(&output)?;
                            result.push_str(&format!("\n[full output: {}]", path.display()));
                        }
                    }
                    let exit = if timed_out {
                        ExitStatus::TimedOut
                    } else {
                        ExitStatus::Exited(status.code().unwrap_or(130))
                    };
                    result.push_str(&format!(
                        "\n[{}]",
                        if timed_out {
                            "timed out; process tree stopped".to_owned()
                        } else {
                            format!("exit code {}", status.code().unwrap_or(130))
                        }
                    ));
                    Ok::<_, std::io::Error>((result, exit))
                };
                run().map_err(|error| ToolExecutionError::other(error.to_string()))
            })
            .await?;
            abort.disarm();
            context.insert_result(result.1)?;
            Ok(result.0)
        }
    }
    #[cfg(test)]
    mod tests {
        use super::*;
        #[test]
        fn git_bash_runs_from_a_directory_with_spaces_and_preserves_errors() {
            let dir = std::env::temp_dir().join(format!("vivy shell {}", std::process::id()));
            std::fs::create_dir_all(&dir).unwrap();
            let shell = NativeShell::for_directory(dir.clone());
            let mut context = rig_harness::rig_core::tool::ToolContext::default();
            let result = futures::executor::block_on(shell.call(
                &mut context,
                rig_harness::rig_tools::ShellArgs::new("printf 'hello'; exit 7"),
            ))
            .unwrap();
            assert!(result.contains("hello"));
            assert!(result.contains("exit code 7"));
            std::fs::remove_dir(&dir).unwrap();
        }

        #[test]
        fn timeout_still_applies_after_a_command_closes_its_output() {
            let shell = NativeShell::for_directory(std::env::temp_dir());
            let mut context = rig_harness::rig_core::tool::ToolContext::default();
            let result = futures::executor::block_on(shell.call(
                &mut context,
                rig_harness::rig_tools::ShellArgs::new("exec 1>&- 2>&-; sleep 2").timeout_secs(1),
            ))
            .unwrap();
            assert!(result.contains("timed out"));
        }
    }
}
