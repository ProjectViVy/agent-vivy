//! A display snapshot published by the optional Python component, per agent.
//! The terminal never starts a worker or probes dependencies to obtain it.
use rig_harness::prelude::*;

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum PythonState {
    Unprepared,
    Prepared,
    #[default]
    Stopped,
    Starting,
    Running,
    Ready,
    Preparing,
    Checking,
    Error,
}

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum MemoryState {
    #[default]
    Off,
    Unchecked,
    Ready,
    Closed,
    Error,
}

#[derive(Component, Clone, Copy, Debug, Default, PartialEq, Eq)]
pub struct WorkbenchStatus {
    pub python: PythonState,
    pub memory: MemoryState,
}

impl WorkbenchStatus {
    pub fn urgent(self) -> bool {
        matches!(self.python, PythonState::Preparing | PythonState::Error)
            || self.memory == MemoryState::Error
    }
}
