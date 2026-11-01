//! Bounded pipe IO. Slow readers never hold the ECS thread.
use crate::protocol::Frame;
use rig_harness::prelude::Wake;
use std::io::Write;
use std::sync::{
    Arc, Mutex,
    atomic::{AtomicU64, Ordering},
    mpsc,
};

pub struct Writer {
    sender: mpsc::SyncSender<Frame>,
    written: Arc<AtomicU64>,
    failure: Arc<Mutex<Option<String>>>,
}

impl Writer {
    pub fn start(mut sink: impl Write + Send + 'static, wake: Wake) -> Self {
        let (sender, receiver) = mpsc::sync_channel::<Frame>(64);
        let written = Arc::new(AtomicU64::new(0));
        let failure = Arc::new(Mutex::new(None));
        let (done, error) = (written.clone(), failure.clone());
        std::thread::spawn(move || {
            for frame in receiver {
                let result = serde_json::to_writer(&mut sink, &frame)
                    .map_err(std::io::Error::other)
                    .and_then(|_| sink.write_all(b"\n"))
                    .and_then(|_| sink.flush());
                if let Err(cause) = result {
                    *error.lock().unwrap() = Some(format!("desktop output pipe: {cause}"));
                    wake.wake();
                    return;
                }
                done.store(frame.seq, Ordering::Release);
                wake.wake();
            }
        });
        Self {
            sender,
            written,
            failure,
        }
    }

    pub fn send(&self, frame: Frame) -> Result<(), &'static str> {
        self.sender.try_send(frame).map_err(|e| match e {
            mpsc::TrySendError::Full(_) => {
                "desktop output queue overflow; reconnect and resynchronize"
            }
            mpsc::TrySendError::Disconnected(_) => "desktop output pipe disconnected",
        })
    }
    pub fn written(&self) -> u64 {
        self.written.load(Ordering::Acquire)
    }
    pub fn failure(&self) -> Option<String> {
        self.failure.lock().unwrap().clone()
    }
}
