//! Atomic settings edits and instance-scoped API keys.
use super::config::{ProvidersConfig, parse, valid_id};
use rig_harness::rig_core::providers::chatgpt::auth::{FileTokenStore, TokenStore};
use std::{fs, path::PathBuf};

#[derive(Clone, Debug, bevy_ecs::prelude::Resource)]
pub struct SettingsFile {
    pub path: PathBuf,
    pub(crate) snapshot: String,
}
impl SettingsFile {
    pub fn open(path: PathBuf) -> Result<Self, String> {
        let snapshot = read(&path)?;
        parse(&snapshot)?;
        Ok(Self { path, snapshot })
    }
    pub fn current(&self) -> Result<ProvidersConfig, String> {
        parse(&self.snapshot)
    }
    pub fn save(&mut self, config: &ProvidersConfig) -> Result<(), String> {
        self.save_with_key(config, None)
    }
    pub(crate) fn save_with_key(
        &mut self,
        config: &ProvidersConfig,
        edit: Option<(&rig_harness::harness_protocol::Home, &str, Option<&str>)>,
    ) -> Result<(), String> {
        config.validate()?;
        if let Some(parent) = self.path.parent() {
            fs::create_dir_all(parent).map_err(|e| format!("create settings directory: {e}"))?;
        }
        let lock_path = self.path.with_extension("toml.lock");
        let lock = fs::OpenOptions::new()
            .create(true)
            .truncate(false)
            .read(true)
            .write(true)
            .open(lock_path)
            .map_err(|e| format!("open settings lock: {e}"))?;
        lock.try_lock()
            .map_err(|_| "settings are being changed by another process; retry".to_owned())?;
        if read(&self.path)? != self.snapshot {
            return Err("settings.toml changed outside this editor; reload and retry".into());
        }
        let mut document: toml_edit::DocumentMut = self
            .snapshot
            .parse()
            .map_err(|e| format!("repair settings.toml before saving: {e}"))?;
        let encoded =
            toml::to_string_pretty(config).map_err(|e| format!("encode provider settings: {e}"))?;
        let table = encoded
            .parse::<toml_edit::DocumentMut>()
            .map_err(|e| format!("format provider settings: {e}"))?
            .into_table();
        document["providers"] = toml_edit::Item::Table(table);
        document.remove("gateway");
        let text = document.to_string();
        let backup = if let Some((home, id, key)) = edit {
            let store = FileTokenStore::new(home.auth().join("provider-keys"));
            let previous = bevy_tasks::block_on(store.load(id))
                .map_err(|e| format!("read previous API key for {id}: {e}"))?;
            write_key(home, id, key)?;
            Some((store, id, previous))
        } else {
            None
        };
        if let Err(error) = rig_harness::rig_tools::fs::write_atomic(&self.path, text.as_bytes()) {
            if let Some((store, id, previous)) = backup {
                let restored = match previous {
                    Some(record) => bevy_tasks::block_on(store.save(id, &record)),
                    None => bevy_tasks::block_on(store.delete(id)),
                };
                if let Err(rollback) = restored {
                    return Err(format!(
                        "save settings.toml: {error}; restore previous API key failed: {rollback}"
                    ));
                }
            }
            return Err(format!("save settings.toml: {error}"));
        }
        self.snapshot = text;
        Ok(())
    }
}

pub(crate) fn read(path: &std::path::Path) -> Result<String, String> {
    match fs::read_to_string(path) {
        Ok(v) => Ok(v),
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(String::new()),
        Err(e) => Err(format!("read settings.toml: {e}")),
    }
}

pub(crate) fn write_key(
    home: &rig_harness::harness_protocol::Home,
    id: &str,
    key: Option<&str>,
) -> Result<(), String> {
    if !valid_id(id) {
        return Err("invalid provider instance ID".into());
    }
    let store = FileTokenStore::new(home.auth().join("provider-keys"));
    if let Some(key) = key {
        if key.trim().is_empty() || key.chars().any(char::is_control) {
            return Err("API key cannot be empty or contain control characters".into());
        }
        let record = serde_json::to_string(&super::providers::SavedKey { key: key.into() })
            .map_err(|e| e.to_string())?;
        bevy_tasks::block_on(store.save(id, &record))
            .map_err(|e| format!("save API key for {id}: {e}"))
    } else {
        bevy_tasks::block_on(store.delete(id)).map_err(|e| format!("remove API key for {id}: {e}"))
    }
}
