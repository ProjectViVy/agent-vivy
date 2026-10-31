//! Names, never values, shared by provider configuration and process launchers.
use std::{
    collections::BTreeSet,
    ffi::OsString,
    path::Path,
    process::Command,
    sync::{
        OnceLock, RwLock,
        atomic::{AtomicU64, Ordering},
    },
};

static GENERATION: AtomicU64 = AtomicU64::new(0);
pub fn credential_policy_generation() -> u64 {
    GENERATION.load(Ordering::Acquire)
}

fn registered() -> &'static RwLock<BTreeSet<OsString>> {
    static NAMES: OnceLock<RwLock<BTreeSet<OsString>>> = OnceLock::new();
    NAMES.get_or_init(Default::default)
}

pub fn credential_variables(text: &str) -> Result<Vec<OsString>, String> {
    let config: toml::Table =
        toml::from_str(text).map_err(|_| "settings.toml is not valid TOML".to_owned())?;
    let mut names = BTreeSet::new();
    if let Some(instances) = config
        .get("providers")
        .and_then(|v| v.get("instances"))
        .and_then(|v| v.as_table())
    {
        for instance in instances.values() {
            if let Some(credential) = instance.get("credential") {
                if credential.get("source").and_then(|v| v.as_str()) == Some("env") {
                    if let Some(name) = credential.get("name").and_then(|v| v.as_str()) {
                        if !name.is_empty() && !name.contains('=') && !name.contains('\0') {
                            names.insert(OsString::from(name));
                        }
                    }
                }
            }
        }
    }
    Ok(names.into_iter().collect())
}

/// Even an invalid configuration must not expose a name referenced in its text
/// to a describe/build process. It can still be repaired in the agent's UI.
pub fn variables_from_file(path: &Path) -> Vec<OsString> {
    let Ok(text) = std::fs::read_to_string(path) else {
        return vec![];
    };
    credential_variables(&text).unwrap_or_else(|_| {
        std::env::vars_os()
            .map(|(name, _)| name)
            .filter(|name| text.contains(name.to_string_lossy().as_ref()))
            .collect()
    })
}

/// Monotonic for the life of this process: removing a provider does not expose
/// its previous credential to subsequently started tools.
pub fn register_credential_variables(names: &[OsString]) {
    let mut registered = registered().write().unwrap_or_else(|e| e.into_inner());
    let previous = registered.len();
    registered.extend(names.iter().cloned());
    if registered.len() != previous {
        GENERATION.fetch_add(1, Ordering::Release);
    }
}

pub fn filter_credentials(command: &mut Command, names: &[OsString]) {
    let mut all = registered()
        .read()
        .unwrap_or_else(|e| e.into_inner())
        .clone();
    all.extend(names.iter().cloned());
    let upper: BTreeSet<_> = all
        .iter()
        .map(|v| v.to_string_lossy().to_ascii_uppercase())
        .collect();
    let present = std::env::vars_os()
        .map(|(k, _)| k)
        .chain(command.get_envs().map(|(k, _)| k.to_owned()))
        .collect::<Vec<_>>();
    for name in present {
        let key = name.to_string_lossy().to_ascii_uppercase();
        if key.starts_with("RIG_SECRET_") || upper.contains(&key) {
            all.insert(name);
        }
    }
    for name in all {
        command.env_remove(name);
    }
}
