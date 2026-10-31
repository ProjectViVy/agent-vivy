//! Observes existing Store writes without strengthening their guarantees.
use super::{Checkpoint, KEY, Trace};
use rig_memory::{Files, LogWriteError, Store};
use serde_json::{Value, json};
use std::{io, sync::Arc};

#[derive(Clone)]
pub(super) struct ObservedStore {
    inner: Arc<dyn Store>,
    trace: Trace,
}

impl ObservedStore {
    pub fn new(inner: Arc<dyn Store>, trace: Trace) -> Self {
        Self { inner, trace }
    }
}

impl Files for ObservedStore {
    fn write_file(&self, name: &str, bytes: &[u8]) -> io::Result<()> {
        self.inner.write_file(name, bytes)
    }
    fn append_file(&self, name: &str, bytes: &[u8]) -> io::Result<()> {
        self.inner.append_file(name, bytes)
    }
    fn read_file(&self, name: &str) -> io::Result<Vec<u8>> {
        self.inner.read_file(name)
    }
    fn remove_file(&self, name: &str) -> io::Result<()> {
        self.inner.remove_file(name)
    }
    fn files(&self) -> io::Result<Vec<String>> {
        self.inner.files()
    }
}

impl Store for ObservedStore {
    fn logs(&self) -> io::Result<Vec<String>> {
        self.inner.logs()
    }
    fn read_log(&self, name: &str) -> io::Result<Vec<u8>> {
        self.inner.read_log(name)
    }
    fn append_log(&self, name: &str, len: u64, bytes: &[u8]) -> Result<(), LogWriteError> {
        if let Err(error) = self.inner.append_log(name, len, bytes) {
            self.trace
                .event(json!({"event":"append_failed","log":name,"length":len}));
            return Err(error);
        }
        let records: Vec<Value> = bytes
            .split(|b| *b == b'\n')
            .filter(|b| !b.is_empty())
            .filter_map(|line| serde_json::from_slice(line).ok())
            .collect();
        let checkpoints: Vec<Checkpoint> = records
            .iter()
            .filter(|record| record["type"] == "state" && record["key"] == KEY)
            .filter_map(|record| {
                serde_json::from_value::<super::ProbeState>(record["value"].clone()).ok()
            })
            .flat_map(|state| state.operations)
            .collect();
        self.trace
            .event(json!({"event":"append_observed","log":name,"start":len,
            "end":len + bytes.len() as u64,"checkpoints":checkpoints,
            "record_types":records.iter().map(|r| r["type"].clone()).collect::<Vec<_>>()}));
        let mut data = self.trace.0.lock().unwrap_or_else(|e| e.into_inner());
        // Only the latest checkpoint per logical operation is useful to this
        // bounded experiment. Failed writes never enter this list.
        for checkpoint in checkpoints {
            data.written
                .retain(|(log, cp)| log != name || cp.operation != checkpoint.operation);
            data.written.push((name.to_owned(), checkpoint));
        }
        Ok(())
    }
    fn replace_log(&self, name: &str, len: u64, bytes: &[u8]) -> Result<(), LogWriteError> {
        self.inner.replace_log(name, len, bytes)?;
        self.trace
            .0
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .written
            .retain(|(log, _)| log != name);
        self.trace
            .event(json!({"event":"replace_observed","log":name}));
        Ok(())
    }
    fn destroy(&self) -> io::Result<()> {
        self.inner.destroy()
    }
}
