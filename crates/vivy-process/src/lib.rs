//! Process ownership shared by the Windows shell and Python workbench.

use std::io;
use std::process::{Child, Command};
use std::sync::Arc;

mod credentials;
pub mod session;
pub use credentials::{
    credential_policy_generation, credential_variables, filter_credentials,
    register_credential_variables, registered_credential_variables, variables_from_file,
};

/// A command without agent-only state or model credentials.
pub fn command(program: impl AsRef<std::ffi::OsStr>) -> Command {
    let mut command = Command::new(program);
    filter_credentials(&mut command, &[]);
    for (name, _) in std::env::vars_os() {
        let key = name.to_string_lossy().to_ascii_uppercase();
        if key.starts_with("RIG_SECRET_")
            || [
                "RIG_SESSION",
                "RIG_RUN",
                "RIG_START",
                "RIG_NOTICE",
                "RIG_DESCRIBE",
                "RIG_BUILD_FAILURE",
                "RIG_LAUNCHER",
            ]
            .contains(&key.as_str())
        {
            command.env_remove(name);
        }
    }
    command
}

#[derive(Clone)]
pub struct Control(Arc<platform::Owner>);

impl Control {
    pub fn kill(&self) {
        self.0.kill();
    }
}

pub struct OwnedChild {
    pub child: Child,
    control: Control,
}

impl OwnedChild {
    pub fn spawn(command: &mut Command) -> io::Result<Self> {
        let (child, owner) = platform::spawn(command)?;
        Ok(Self {
            child,
            control: Control(Arc::new(owner)),
        })
    }

    pub fn control(&self) -> Control {
        self.control.clone()
    }
}

impl Drop for OwnedChild {
    fn drop(&mut self) {
        self.control.kill();
        let _ = self.child.wait();
    }
}

/// Kept in the calling future, not the blocking I/O task: dropping a tool
/// future must stop its process even while that task waits for output.
pub struct AbortGuard(Option<Control>);
impl AbortGuard {
    pub fn new(control: Control) -> Self {
        Self(Some(control))
    }
    pub fn disarm(&mut self) {
        self.0 = None;
    }
}
impl Drop for AbortGuard {
    fn drop(&mut self) {
        if let Some(control) = &self.0 {
            control.kill();
        }
    }
}

#[cfg(windows)]
mod platform {
    use super::*;
    use std::os::windows::{io::AsRawHandle, process::CommandExt};
    use windows_sys::Win32::{
        Foundation::*,
        System::{Diagnostics::ToolHelp::*, JobObjects::*, Threading::*},
    };

    pub struct Owner(usize);
    impl Owner {
        pub fn kill(&self) {
            unsafe {
                TerminateJobObject(self.0 as HANDLE, 130);
            }
        }
    }
    impl Drop for Owner {
        fn drop(&mut self) {
            unsafe {
                CloseHandle(self.0 as HANDLE);
            }
        }
    }

    pub fn spawn(command: &mut Command) -> io::Result<(Child, Owner)> {
        // No user code executes until the suspended primary thread has been
        // attached to the job. Descendants cannot escape an assignment race.
        unsafe {
            let job = CreateJobObjectW(std::ptr::null(), std::ptr::null());
            if job.is_null() {
                return Err(io::Error::last_os_error());
            }
            let owner = Owner(job as usize);
            let mut info: JOBOBJECT_EXTENDED_LIMIT_INFORMATION = std::mem::zeroed();
            info.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
            if SetInformationJobObject(
                job,
                JobObjectExtendedLimitInformation,
                (&info as *const JOBOBJECT_EXTENDED_LIMIT_INFORMATION).cast(),
                std::mem::size_of_val(&info) as u32,
            ) == 0
            {
                return Err(io::Error::last_os_error());
            }
            let mut child = command
                .creation_flags(CREATE_SUSPENDED | CREATE_NO_WINDOW)
                .spawn()?;
            let attached = AssignProcessToJobObject(job, child.as_raw_handle() as HANDLE);
            let resumed = if attached == 0 {
                Err(io::Error::last_os_error())
            } else {
                resume(child.id())
            };
            if let Err(error) = resumed {
                let _ = child.kill();
                let _ = child.wait();
                return Err(error);
            }
            Ok((child, owner))
        }
    }

    unsafe fn resume(pid: u32) -> io::Result<()> {
        unsafe {
            let snapshot = CreateToolhelp32Snapshot(TH32CS_SNAPTHREAD, 0);
            if snapshot == INVALID_HANDLE_VALUE {
                return Err(io::Error::last_os_error());
            }
            let mut thread: THREADENTRY32 = std::mem::zeroed();
            thread.dwSize = std::mem::size_of_val(&thread) as u32;
            let mut found = Thread32First(snapshot, &mut thread);
            let mut result = Err(io::Error::new(
                io::ErrorKind::NotFound,
                "suspended process has no primary thread",
            ));
            while found != 0 {
                if thread.th32OwnerProcessID == pid {
                    let handle = OpenThread(THREAD_SUSPEND_RESUME, 0, thread.th32ThreadID);
                    if handle.is_null() {
                        result = Err(io::Error::last_os_error());
                    } else {
                        result = if ResumeThread(handle) == u32::MAX {
                            Err(io::Error::last_os_error())
                        } else {
                            Ok(())
                        };
                        CloseHandle(handle);
                    }
                    break;
                }
                found = Thread32Next(snapshot, &mut thread);
            }
            CloseHandle(snapshot);
            result
        }
    }
}

#[cfg(unix)]
mod platform {
    use super::*;
    use std::os::unix::process::CommandExt;
    pub struct Owner(u32);
    impl Owner {
        pub fn kill(&self) {
            unsafe {
                libc::kill(-(self.0 as i32), libc::SIGKILL);
            }
        }
    }
    pub fn spawn(command: &mut Command) -> io::Result<(Child, Owner)> {
        let child = command.process_group(0).spawn()?;
        let owner = Owner(child.id());
        Ok((child, owner))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::{BufRead, BufReader};
    use std::process::Stdio;

    #[test]
    fn cancellation_stops_descendants_and_cleans_up_the_parent() {
        let mut command = command("python");
        command.args(["-u", "-c", "import subprocess,sys,time; p=subprocess.Popen([sys.executable,'-c','import time; time.sleep(60)']); print(p.pid,flush=True); time.sleep(60)"])
            .stdout(Stdio::piped());
        let mut process = OwnedChild::spawn(&mut command).unwrap();
        let mut line = String::new();
        BufReader::new(process.child.stdout.take().unwrap())
            .read_line(&mut line)
            .unwrap();
        let descendant: u32 = line.trim().parse().unwrap();
        let control = process.control();
        control.kill();
        assert!(!process.child.wait().unwrap().success());
        #[cfg(windows)]
        unsafe {
            use windows_sys::Win32::{
                Foundation::CloseHandle,
                System::Threading::{OpenProcess, PROCESS_SYNCHRONIZE, WaitForSingleObject},
            };
            let handle = OpenProcess(PROCESS_SYNCHRONIZE, 0, descendant);
            if !handle.is_null() {
                assert_eq!(WaitForSingleObject(handle, 3000), 0);
                CloseHandle(handle);
            }
        }
        #[cfg(not(windows))]
        let _ = descendant;
    }

    #[test]
    fn ordinary_exit_is_preserved() {
        let mut command = command("python");
        command.args(["-c", "raise SystemExit(7)"]);
        assert_eq!(
            OwnedChild::spawn(&mut command)
                .unwrap()
                .child
                .wait()
                .unwrap()
                .code(),
            Some(7)
        );
    }
}
