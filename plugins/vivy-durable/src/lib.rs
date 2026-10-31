//! Opt-in persistent-agent runtime and diagnostic probes, selected at compilation.
//! The runtime requires the local Rig checkpoint APIs and targets ordinary process
//! crashes. It does not promise OS-crash or power-loss durability.

#[cfg(feature = "d0-probe")]
mod probe;
#[cfg(feature = "d0-probe")]
pub use probe::D0ProbePlugin;

#[cfg(feature = "checkpoint-probe")]
mod checkpoint_probe;
#[cfg(feature = "checkpoint-probe")]
pub use checkpoint_probe::CheckpointProbePlugin;

#[cfg(feature = "runtime")]
pub mod runtime;
#[cfg(feature = "runtime")]
pub use runtime::DurablePlugin;
#[cfg(feature = "assistant-context")]
pub use runtime::assistant::AssistantContextPlugin;
