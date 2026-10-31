//! Session ownership before any harness/plugin can open writable state.
//! This coordinates trusted hosts; it is not a boundary against hostile code.
use rig_harness_protocol::{Home, SessionDir, SessionId, env, lock_file};
use std::{fs::File, io, path::Path, process::Command};

// Excluded when copying sessions written by earlier generated builds.
pub const EXECUTION_LOCK: &str = ".vivy-execution.lock";

/// History mutation of grouped checkpoints needs lifecycle-aware rewriting.
/// Refuse it before a destination or source log can be changed. This is format
/// detection only; normal open still validates every complete record and schema.
pub fn require_ordinary_history(directory: &Path) -> io::Result<()> {
    use std::io::Read;
    let logs = match std::fs::read_dir(directory.join("logs")) {
        Ok(logs) => logs,
        Err(error) if error.kind() == io::ErrorKind::NotFound => return Ok(()),
        Err(error) => return Err(error),
    };
    for entry in logs {
        let entry = entry?;
        if entry.path().extension().is_none_or(|e| e != "jsonl") {
            continue;
        }
        let mut bytes = Vec::new();
        File::open(entry.path())?.take(96).read_to_end(&mut bytes)?;
        if bytes.starts_with(b"{\"format\":\"rig-checkpoint/") {
            return Err(io::Error::new(
                io::ErrorKind::Unsupported,
                "fork, clone, rewind, import and history rewriting are unsupported for checkpoint sessions",
            ));
        }
    }
    Ok(())
}

/// Keep alive until the app and its cleanup have finished. A launcher retains
/// the session lease while its child holds the execution lock.
pub struct SessionGuard {
    pub directory: SessionDir,
    _lease: Option<File>,
    execution: Option<File>,
}

fn busy() -> io::Error {
    io::Error::new(io::ErrorKind::WouldBlock, "this session is already running")
}

fn handoff_error() -> io::Error {
    io::Error::new(
        io::ErrorKind::PermissionDenied,
        "invalid or stale launcher handoff",
    )
}

impl SessionGuard {
    /// Acquire both locks in a fixed order, for standalone execution or a
    /// launcher about to prepare/copy a session. Drop releases both on errors.
    pub fn direct(directory: SessionDir) -> io::Result<Self> {
        let lease = directory.hold()?.ok_or_else(busy)?;
        Self::with_execution(directory, Some(lease))
    }

    fn with_execution(directory: SessionDir, lease: Option<File>) -> io::Result<Self> {
        // Stable namespace outside the deletable session tree. Keep these small
        // lock files: unlinking them would let another process lock a new inode.
        let parent = directory.path().parent().ok_or_else(handoff_error)?;
        let mut root_name = parent.file_name().ok_or_else(handoff_error)?.to_os_string();
        root_name.push(".vivy-locks");
        let root = parent.with_file_name(root_name);
        std::fs::create_dir_all(&root)?;
        let name = directory.path().file_name().ok_or_else(handoff_error)?;
        let execution = lock_file(&root.join(name), true)?;
        match execution.try_lock() {
            Ok(()) => Ok(Self {
                directory,
                _lease: lease,
                execution: Some(execution),
            }),
            Err(std::fs::TryLockError::WouldBlock) => Err(busy()),
            Err(std::fs::TryLockError::Error(error)) => Err(error),
        }
    }

    /// Used only by the launcher immediately before starting its child. The
    /// session lease stays held across the entire run and subsequent cleanup.
    pub fn hand_off_execution(&mut self) {
        self.execution.take();
    }

    /// Detach an owned ephemeral directory before recursive deletion. On
    /// Windows, release its internal file handle before rename while the
    /// external execution lock still excludes every guarded entrant.
    pub fn retire(mut self) -> io::Result<std::path::PathBuf> {
        if self.execution.is_none() || self._lease.is_none() {
            return Err(handoff_error());
        }
        let retired = self
            .directory
            .path()
            .with_file_name(format!(".vivy-deleted-{}", SessionId::generate()));
        self._lease.take();
        std::fs::rename(self.directory.path(), &retired)?;
        Ok(retired)
    }
}

fn require_held(path: &Path) -> io::Result<()> {
    let file = match lock_file(path, false) {
        Ok(file) => file,
        Err(error) if error.kind() == io::ErrorKind::NotFound => return Err(handoff_error()),
        Err(error) => return Err(error),
    };
    match file.try_lock() {
        Err(std::fs::TryLockError::WouldBlock) => Ok(()),
        Ok(()) => Err(handoff_error()),
        Err(std::fs::TryLockError::Error(error)) => Err(error),
    }
}

fn delegated(home: &Home, id: &SessionId, run: &Path, parent: u32) -> io::Result<SessionGuard> {
    let expected = home.run(parent);
    // Canonicalization accepts Windows path spelling differences without
    // allowing an inherited run belonging to a different parent process.
    if !run.is_dir()
        || !expected.path().is_dir()
        || run.canonicalize()? != expected.path().canonicalize()?
    {
        return Err(handoff_error());
    }
    require_held(&expected.lock())?;
    let directory = home.session(id);
    require_held(&directory.launcher_lock())?;
    SessionGuard::with_execution(directory, None)
}

/// Describing a generated binary opens no session. Otherwise select the same
/// session that Rig will use and guard it before building the application.
pub fn enter_from_env() -> io::Result<Option<SessionGuard>> {
    if std::env::var_os(env::DESCRIBE).is_some_and(|path| !path.is_empty()) {
        return Ok(None);
    }
    let home = Home::from_env();
    let selected =
        SessionId::from_env().map_err(|why| io::Error::new(io::ErrorKind::InvalidInput, why))?;
    let run = std::env::var_os(env::RUN);
    let launcher = std::env::var_os(env::LAUNCHER);
    let guard = match (run, launcher) {
        (None, Some(_)) if selected.is_none() => {
            SessionGuard::direct(home.session(&SessionId::generate()))?
        }
        (None, None) => {
            SessionGuard::direct(home.session(&selected.unwrap_or_else(SessionId::generate)))?
        }
        (Some(run), Some(_)) => delegated(
            &home,
            &selected.ok_or_else(handoff_error)?,
            Path::new(&run),
            parent_process_id()?,
        )?,
        _ => return Err(handoff_error()),
    };
    Ok(Some(guard))
}

/// A new launcher invocation must not reuse its caller's control channel.
/// Model credentials and RIG_HOME are configured independently and retained.
pub fn clear_launcher_context(command: &mut Command) {
    for key in
        env::AGENT_ONLY
            .iter()
            .copied()
            .chain([env::LAUNCHER, env::DESCRIBE, env::PROCESS_LEDGER])
    {
        command.env_remove(key);
    }
}

#[cfg(unix)]
fn parent_process_id() -> io::Result<u32> {
    Ok(unsafe { libc::getppid() } as u32)
}

#[cfg(windows)]
fn parent_process_id() -> io::Result<u32> {
    use windows_sys::Win32::{
        Foundation::{CloseHandle, INVALID_HANDLE_VALUE},
        System::Diagnostics::ToolHelp::*,
    };
    unsafe {
        let snapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
        if snapshot == INVALID_HANDLE_VALUE {
            return Err(io::Error::last_os_error());
        }
        let mut entry: PROCESSENTRY32W = std::mem::zeroed();
        entry.dwSize = std::mem::size_of_val(&entry) as u32;
        let mut found = Process32FirstW(snapshot, &mut entry);
        let mut result = Err(io::Error::new(
            io::ErrorKind::NotFound,
            "current process is absent from process snapshot",
        ));
        while found != 0 {
            if entry.th32ProcessID == std::process::id() {
                result = Ok(entry.th32ParentProcessID);
                break;
            }
            found = Process32NextW(snapshot, &mut entry);
        }
        CloseHandle(snapshot);
        result
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn grouped_history_mutation_is_refused_without_changing_any_bytes() {
        let fixture = Fixture::new();
        let dir = fixture.0.session(&SessionId::generate());
        std::fs::create_dir_all(dir.path().join("logs")).unwrap();
        let log = dir.path().join("logs/main.jsonl");
        let bytes = b"{\"format\":\"rig-checkpoint/1\"}\n";
        std::fs::write(&log, bytes).unwrap();
        assert!(require_ordinary_history(dir.path()).is_err());
        assert_eq!(std::fs::read(log).unwrap(), bytes);
    }
    struct Fixture(Home);
    impl Fixture {
        fn new() -> Self {
            let root = std::env::temp_dir().join(format!("vivy-lease-{}", SessionId::generate()));
            std::fs::create_dir_all(&root).unwrap();
            Self(Home::at(root))
        }
    }
    impl Drop for Fixture {
        fn drop(&mut self) {
            assert!(self.0.root().starts_with(std::env::temp_dir()));
            std::fs::remove_dir_all(self.0.root()).unwrap();
        }
    }
    #[test]
    fn direct_lock_refuses_duplicates_and_releases_on_drop() {
        let fixture = Fixture::new();
        let dir = fixture.0.session(&SessionId::generate());
        let guard = SessionGuard::direct(dir.clone()).unwrap();
        assert!(
            matches!(SessionGuard::direct(dir.clone()), Err(e) if e.kind() == io::ErrorKind::WouldBlock)
        );
        drop(guard);
        drop(SessionGuard::direct(dir).unwrap());
    }

    #[test]
    fn checked_handoff_allows_one_child_and_keeps_parent_lease() {
        let fixture = Fixture::new();
        let id = SessionId::generate();
        let run = fixture.0.run(1234);
        std::fs::create_dir_all(run.path()).unwrap();
        let running = lock_file(&run.lock(), true).unwrap();
        running.lock().unwrap();
        let mut owner = SessionGuard::direct(fixture.0.session(&id)).unwrap();
        assert!(delegated(&fixture.0, &id, run.path(), 1234).is_err());
        owner.hand_off_execution();
        assert!(delegated(&fixture.0, &id, run.path(), 9999).is_err());
        let child = delegated(&fixture.0, &id, run.path(), 1234).unwrap();
        assert!(delegated(&fixture.0, &id, run.path(), 1234).is_err());
        drop(child);
        assert!(SessionGuard::direct(fixture.0.session(&id)).is_err());
        drop(owner);
        assert!(delegated(&fixture.0, &id, run.path(), 1234).is_err());
        drop(SessionGuard::direct(fixture.0.session(&id)).unwrap());
    }

    #[test]
    fn execution_lock_blocks_reentry_after_launcher_loss_until_child_exits() {
        let fixture = Fixture::new();
        let id = SessionId::generate();
        let dir = fixture.0.session(&id);
        let mut parent = SessionGuard::direct(dir.clone()).unwrap();
        parent.hand_off_execution();
        let child = SessionGuard::with_execution(dir.clone(), None).unwrap();
        drop(parent);
        assert!(SessionGuard::direct(dir.clone()).is_err());
        drop(child);
        drop(SessionGuard::direct(dir).unwrap());
    }

    #[test]
    fn stale_run_is_not_a_delegated_lease() {
        let fixture = Fixture::new();
        let id = SessionId::generate();
        let run = fixture.0.run(1234);
        std::fs::create_dir_all(run.path()).unwrap();
        let _file = lock_file(&run.lock(), true).unwrap();
        let _owner = SessionGuard::direct(fixture.0.session(&id)).unwrap();
        assert!(delegated(&fixture.0, &id, run.path(), 1234).is_err());
        assert!(parent_process_id().unwrap() > 0);
    }
}
