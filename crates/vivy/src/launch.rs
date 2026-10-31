//! Launches the compiled species and honors Rig's existing session-switch protocol.

use crate::Result;
use rig_harness_protocol::{AgentDescription, Home, RELOAD_EXIT_CODE, SessionId, StartReason, env};
use std::fs::{self, OpenOptions};
use std::path::Path;
use std::process::{Command, Stdio};
use vivy_process::session::{EXECUTION_LOCK, SessionGuard, clear_launcher_context};

/// Pi's session options, which the launcher handles itself: the rest of
/// the command line goes to the species.
#[derive(Debug, Default, PartialEq)]
struct SessionOptions {
    /// `--no-session`: the sessions this run makes are deleted at its end.
    ephemeral: bool,
    /// `--fork <id|path>`: a copy of that session, resumed.
    fork: Option<String>,
}

/// Takes `--no-session`, `--fork <id>`, `--session <id|path>` (Pi's name
/// for `--resume`) and `--export <id> [output]` (that session's `/export`,
/// in print mode) out of `args`.
fn session_options(args: &[String]) -> Result<(Vec<String>, SessionOptions)> {
    let mut rest = Vec::new();
    let mut options = SessionOptions::default();
    let mut resumed = false;
    let mut args = args.iter();
    while let Some(arg) = args.next() {
        match arg.as_str() {
            "--no-session" => options.ephemeral = true,
            "--fork" | "--session" => {
                let value = args
                    .next()
                    .ok_or_else(|| format!("{arg} needs a session id or path"))?;
                let value = session_name(value);
                if arg == "--fork" {
                    options.fork = Some(value);
                } else {
                    resumed = true;
                    rest.extend(["--resume".to_owned(), value]);
                }
            }
            "--export" => {
                let session = args.next().ok_or("--export needs a session id or path")?;
                let mut line = "/export".to_owned();
                if let Some(output) = args.clone().next().filter(|next| !next.starts_with('-')) {
                    line = format!("/export {output}");
                    args.next();
                }
                resumed = true;
                rest.extend([
                    "--resume".to_owned(),
                    session_name(session),
                    "-p".to_owned(),
                    line,
                ]);
            }
            "--resume" | "-r" => {
                resumed = true;
                rest.push(arg.clone());
            }
            _ => rest.push(arg.clone()),
        }
    }
    if options.fork.is_some() && (resumed || options.ephemeral) {
        return Err("--fork cannot be combined with --resume, --session or --no-session".into());
    }
    Ok((rest, options))
}

/// A session given as a path to its directory is named by that directory.
fn session_name(value: &str) -> String {
    let path = Path::new(value);
    match path.file_name() {
        Some(name) if path.is_dir() => name.to_string_lossy().into_owned(),
        _ => value.to_owned(),
    }
}

/// Copies the session directory `from` to `to`, without what belongs to
/// the process running it: locks, a pending switch, its name.
fn copy_session(from: &Path, to: &Path) -> std::io::Result<()> {
    const LEFT_OUT: [&str; 7] = [
        ".rig-memory.lock",
        "lock",
        "launcher.lock",
        EXECUTION_LOCK,
        "switch",
        "deleting",
        "name",
    ];
    fs::create_dir_all(to)?;
    for entry in fs::read_dir(from)? {
        let entry = entry?;
        let name = entry.file_name();
        if LEFT_OUT.iter().any(|left| name == *left) {
            continue;
        }
        let target = to.join(&name);
        if entry.file_type()?.is_dir() {
            copy_session(&entry.path(), &target)?;
        } else {
            fs::copy(entry.path(), target)?;
        }
    }
    Ok(())
}

pub fn run(binary: &Path, args: &[String]) -> Result<u8> {
    let (args, options) = session_options(args)?;
    let args = args.as_slice();
    if !binary.is_file() {
        return Err(format!("the coding species is not built at {}; run `vivy build recipes/coding-agent.yaml` first", binary.display()).into());
    }
    let home = Home::from_env();
    let here = std::env::current_dir()?;
    let run = home.run(std::process::id());
    fs::create_dir_all(run.path())?;
    let run_lock = OpenOptions::new()
        .create(true)
        .truncate(false)
        .read(true)
        .write(true)
        .open(run.lock())?;
    run_lock.try_lock()?;
    let description_file = run.path().join("description");
    let mut describe = Command::new(binary);
    let names = vivy_process::variables_from_file(&home.settings());
    vivy_process::filter_credentials(&mut describe, &names);
    for (name, _) in std::env::vars_os() {
        if name
            .to_string_lossy()
            .to_ascii_uppercase()
            .starts_with("RIG_SECRET_")
        {
            describe.env_remove(name);
        }
    }
    clear_launcher_context(&mut describe);
    let status = describe
        .env(env::DESCRIBE, &description_file)
        .stdout(Stdio::null())
        .status()?;
    if !status.success() {
        return Err("the compiled species could not describe its command line".into());
    }
    let report = AgentDescription::parse(&fs::read_to_string(&description_file)?);
    let invocation = report.invocation(args)?;
    let resume = report.resume(args);
    let mut session = resume.map(|name| home.find(&name)).transpose()?;
    if let Some(original) = &options.fork {
        let original = home.find(original)?;
        let source = SessionGuard::direct(home.session(&original))?;
        vivy_process::session::require_ordinary_history(source.directory.path())?;
        let copy = SessionId::generate();
        let target = SessionGuard::direct(home.session(&copy))?;
        copy_session(source.directory.path(), target.directory.path())?;
        session = Some(copy);
    }
    let mut made = Vec::new();
    let reason = if session.is_some() {
        StartReason::Resumed
    } else {
        StartReason::New
    };
    // The agent receives console interrupts itself and performs its own cleanup.
    ctrlc::set_handler(|| {})?;
    let code = run_sessions(
        binary,
        &home,
        &run,
        &here,
        &invocation,
        (session, reason),
        &mut made,
    );
    if options.ephemeral {
        for id in &made {
            if let Err(error) = discard_ephemeral(home.session(id)) {
                eprintln!("vivy: could not fully remove ephemeral session {id}: {error}");
            }
        }
    }
    code
}

fn discard_ephemeral(directory: rig_harness_protocol::SessionDir) -> std::io::Result<()> {
    if let Some(retired) = retire_ephemeral(directory)? {
        if let Err(error) = fs::remove_dir_all(&retired) {
            eprintln!("vivy: cleanup remains at {}", retired.display());
            return Err(error);
        }
    }
    Ok(())
}

fn retire_ephemeral(
    directory: rig_harness_protocol::SessionDir,
) -> std::io::Result<Option<std::path::PathBuf>> {
    if !directory.path().try_exists()? {
        return Ok(None);
    }
    SessionGuard::direct(directory)?.retire().map(Some)
}

/// Runs the species in `session`, then in each session it switches to.
fn run_sessions(
    binary: &Path,
    home: &Home,
    run: &rig_harness_protocol::RunDir,
    here: &Path,
    invocation: &rig_harness_protocol::Invocation,
    (mut session, mut reason): (Option<SessionId>, StartReason),
    made: &mut Vec<SessionId>,
) -> Result<u8> {
    loop {
        let id = session.take().unwrap_or_else(SessionId::generate);
        let dir = home.session(&id);
        let mut session_guard = SessionGuard::direct(dir.clone())?;
        let cwd = if reason == StartReason::Resumed {
            dir.working_directory()
                .ok_or("the resumed session's working directory is unavailable")?
        } else {
            here.to_path_buf()
        };
        if reason == StartReason::New {
            fs::write(dir.directory(), cwd.to_string_lossy().as_bytes())?;
            made.push(id.clone());
        }
        if dir.switch().is_file() {
            fs::remove_file(dir.switch())?;
        }
        let mut command = Command::new(binary);
        clear_launcher_context(&mut command);
        session_guard.hand_off_execution();
        let status = command
            .args(invocation.to_args())
            .current_dir(&cwd)
            .env(env::HOME, home.root())
            .env(env::SESSION, id.as_str())
            .env(env::LAUNCHER, std::env::current_exe()?)
            .env(env::RUN, run.path())
            .env(env::START, reason.as_str())
            .status()?;
        if status.code() == Some(i32::from(RELOAD_EXIT_CODE)) && dir.switch().is_file() {
            // A print prompt has already been consumed. Replaying `/new` on
            // every restart would create an endless chain of sessions.
            if invocation.is_headless() {
                return Ok(RELOAD_EXIT_CODE);
            }
            let target = fs::read_to_string(dir.switch())?;
            fs::remove_file(dir.switch())?;
            session = if target.trim().is_empty() {
                None
            } else {
                Some(home.find(target.trim())?)
            };
            reason = if session.is_some() {
                StartReason::Resumed
            } else {
                StartReason::New
            };
            continue;
        }
        return Ok(status
            .code()
            .and_then(|code| u8::try_from(code).ok())
            .unwrap_or(130));
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn ephemeral_cleanup_cannot_remove_a_session_taken_by_another_owner() {
        let root = std::env::temp_dir().join(format!("vivy-ephemeral-{}", SessionId::generate()));
        let dir = rig_harness_protocol::SessionDir::at(root.join("sessions/session"));
        let owner = SessionGuard::direct(dir.clone()).unwrap();
        fs::write(dir.path().join("evidence"), "keep").unwrap();
        let result = discard_ephemeral(dir.clone());
        assert!(result.is_err(), "cleanup removed an actively owned session");
        assert_eq!(
            fs::read_to_string(dir.path().join("evidence")).unwrap(),
            "keep"
        );
        drop(owner);
        discard_ephemeral(dir.clone()).unwrap();
        assert!(!dir.path().exists());
        assert!(root.starts_with(std::env::temp_dir()));
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn ephemeral_retirement_separates_cleanup_from_a_new_owner() {
        let root = std::env::temp_dir().join(format!("vivy-retire-{}", SessionId::generate()));
        let original = root.join("sessions/session");
        fs::create_dir_all(&original).unwrap();
        fs::write(original.join("old"), "old state").unwrap();
        let directory = rig_harness_protocol::SessionDir::at(&original);
        let retired = retire_ephemeral(directory.clone()).unwrap().unwrap();
        assert!(
            !original.exists(),
            "cleanup still targets the live session pathname"
        );
        let next_owner = SessionGuard::direct(directory).unwrap();
        fs::write(original.join("new"), "new state").unwrap();
        fs::remove_dir_all(retired).unwrap();
        assert_eq!(
            fs::read_to_string(original.join("new")).unwrap(),
            "new state"
        );
        drop(next_owner);
        assert!(root.starts_with(std::env::temp_dir()));
        fs::remove_dir_all(root).unwrap();
    }

    fn strings(args: &[&str]) -> Vec<String> {
        args.iter().map(|arg| arg.to_string()).collect()
    }

    #[test]
    fn pi_session_options_are_taken_out() {
        let (rest, options) = session_options(&strings(&["--no-session", "-p", "hi"])).unwrap();
        assert_eq!(rest, ["-p", "hi"]);
        assert!(options.ephemeral);
        let (rest, options) = session_options(&strings(&["--session", "abc", "-p", "hi"])).unwrap();
        assert_eq!(rest, ["--resume", "abc", "-p", "hi"]);
        assert_eq!(options, SessionOptions::default());
        let (rest, options) = session_options(&strings(&["--fork", "abc"])).unwrap();
        assert!(rest.is_empty());
        assert_eq!(options.fork.as_deref(), Some("abc"));
        assert!(session_options(&strings(&["--fork", "a", "--resume", "b"])).is_err());
        assert!(session_options(&strings(&["--fork"])).is_err());
        let (rest, _) = session_options(&strings(&["--export", "abc", "out.html"])).unwrap();
        assert_eq!(rest, ["--resume", "abc", "-p", "/export out.html"]);
        let (rest, _) = session_options(&strings(&["--export", "abc"])).unwrap();
        assert_eq!(rest, ["--resume", "abc", "-p", "/export"]);
    }

    #[test]
    fn a_copied_session_leaves_the_running_process_behind() {
        let dir = std::env::temp_dir().join(format!("vivy-copy-{}", std::process::id()));
        let (from, to) = (dir.join("from"), dir.join("to"));
        fs::create_dir_all(from.join("logs")).unwrap();
        for file in [
            "logs/session.jsonl",
            "directory",
            "started",
            ".rig-memory.lock",
            "launcher.lock",
            ".vivy-execution.lock",
            "name",
            "switch",
        ] {
            fs::write(from.join(file), file).unwrap();
        }
        copy_session(&from, &to).unwrap();
        let mut copied: Vec<String> = [
            "logs/session.jsonl",
            "directory",
            "started",
            ".rig-memory.lock",
            "launcher.lock",
            ".vivy-execution.lock",
            "name",
            "switch",
        ]
        .into_iter()
        .filter(|file| to.join(file).exists())
        .map(str::to_owned)
        .collect();
        copied.sort();
        fs::remove_dir_all(&dir).unwrap();
        assert_eq!(copied, ["directory", "logs/session.jsonl", "started"]);
        assert_eq!(
            session_name(dir.to_str().unwrap()),
            dir.to_str().unwrap(),
            "a path that is gone is a name"
        );
    }
}
