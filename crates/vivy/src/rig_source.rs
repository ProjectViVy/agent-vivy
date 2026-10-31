//! Explicit local Rig source overrides for isolated development builds.

use serde_json::Value;
use std::{
    collections::BTreeMap,
    path::{Path, PathBuf},
    process::Command,
};

pub struct DevRig {
    pub root: PathBuf,
    packages: BTreeMap<String, PathBuf>,
}

fn metadata(manifest: &Path, no_deps: bool) -> crate::Result<Value> {
    let mut command = Command::new(std::env::var_os("CARGO").unwrap_or_else(|| "cargo".into()));
    command
        .args([
            "metadata",
            "--offline",
            "--format-version",
            "1",
            "--manifest-path",
        ])
        .arg(manifest);
    if no_deps {
        command.arg("--no-deps");
    } else {
        let rustc = Command::new("rustc").arg("-vV").output()?;
        if !rustc.status.success() {
            return Err("cannot determine the Rust host target".into());
        }
        let version = String::from_utf8(rustc.stdout)?;
        let host = version
            .lines()
            .find_map(|line| line.strip_prefix("host: "))
            .ok_or("rustc did not report its host target")?;
        command.args(["--filter-platform", host]);
    }
    let names =
        vivy_process::variables_from_file(&rig_harness_protocol::Home::from_env().settings());
    vivy_process::filter_credentials(&mut command, &names);
    for (name, _) in std::env::vars_os() {
        if name
            .to_string_lossy()
            .to_ascii_uppercase()
            .starts_with("RIG_SECRET_")
        {
            command.env_remove(name);
        }
    }
    let output = command.output()?;
    if !output.status.success() {
        return Err(format!(
            "cannot resolve {}: {}",
            manifest.display(),
            String::from_utf8_lossy(&output.stderr)
        )
        .into());
    }
    Ok(serde_json::from_slice(&output.stdout)?)
}

impl DevRig {
    pub fn discover(path: &Path) -> crate::Result<Self> {
        let root = path.canonicalize()?;
        let data = metadata(&root.join("Cargo.toml"), true)?;
        let members = data["workspace_members"]
            .as_array()
            .ok_or("Rig workspace has no members")?;
        let mut packages = BTreeMap::new();
        for package in data["packages"]
            .as_array()
            .ok_or("Rig workspace has no packages")?
        {
            let name = package["name"].as_str().ok_or("package name missing")?;
            if (name == "rig" || name.starts_with("rig-")) && members.contains(&package["id"]) {
                let manifest = Path::new(
                    package["manifest_path"]
                        .as_str()
                        .ok_or("manifest path missing")?,
                );
                let directory = manifest
                    .parent()
                    .ok_or("manifest has no directory")?
                    .canonicalize()?;
                if !directory.starts_with(&root) {
                    return Err(format!("Rig package {name} is outside its checkout").into());
                }
                packages.insert(name.to_owned(), directory);
            }
        }
        if !packages.contains_key("rig-harness") {
            return Err("--rig-path must contain the rig-harness workspace package".into());
        }
        Ok(Self { root, packages })
    }

    pub fn patches(&self, source: &str) -> String {
        let mut text = String::new();
        let mut sources = vec![source];
        let upstream = "https://github.com/0xPlaygrounds/rig";
        if source != upstream {
            sources.push(upstream);
        }
        for source in sources {
            text.push_str(&format!("\n[patch.{source:?}]\n"));
            for (name, path) in &self.packages {
                text.push_str(&format!(
                    "{name} = {{ path = {:?} }}\n",
                    path.to_string_lossy()
                ));
            }
        }
        text
    }

    pub fn verify(&self, manifest: &Path) -> crate::Result<()> {
        self.check_metadata(&metadata(manifest, false)?)
    }

    fn check_metadata(&self, metadata: &Value) -> crate::Result<()> {
        for package in metadata["packages"]
            .as_array()
            .ok_or("resolved packages missing")?
        {
            let name = package["name"].as_str().unwrap_or_default();
            if name != "rig" && !name.starts_with("rig-") {
                continue;
            }
            let manifest = PathBuf::from(
                package["manifest_path"]
                    .as_str()
                    .ok_or("manifest path missing")?,
            );
            let manifest = manifest.canonicalize().unwrap_or(manifest);
            if !package["source"].is_null() || !manifest.starts_with(&self.root) {
                return Err(format!(
                    "mixed Rig sources: {name} did not resolve inside {}",
                    self.root.display()
                )
                .into());
            }
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;

    #[test]
    fn discovery_uses_workspace_packages_and_requires_the_harness() {
        let dir = std::env::temp_dir().join(format!(
            "vivy-rig-source-{}-{}",
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_nanos()
        ));
        fs::create_dir_all(dir.join("crates/rig-harness/src")).unwrap();
        fs::write(
            dir.join("Cargo.toml"),
            "[workspace]\nmembers=[\"crates/rig-harness\"]\nresolver=\"3\"\n",
        )
        .unwrap();
        fs::write(
            dir.join("crates/rig-harness/Cargo.toml"),
            "[package]\nname=\"rig-harness\"\nversion=\"0.1.0\"\nedition=\"2024\"\n",
        )
        .unwrap();
        fs::write(dir.join("crates/rig-harness/src/lib.rs"), "").unwrap();
        let source = DevRig::discover(&dir).unwrap();
        let patches = source.patches("https://github.com/0xPlaygrounds/rig");
        assert!(patches.contains("[patch.\"https://github.com/0xPlaygrounds/rig\"]"));
        assert!(patches.contains("rig-harness = { path = "));
        fs::write(
            dir.join("crates/rig-harness/Cargo.toml"),
            "[package]\nname=\"unrelated\"\nversion=\"0.1.0\"\nedition=\"2024\"\n",
        )
        .unwrap();
        assert!(DevRig::discover(&dir).is_err());
        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn a_resolved_git_copy_cannot_mix_with_the_local_rig() {
        let source = DevRig {
            root: std::path::PathBuf::from("C:/rig"),
            packages: Default::default(),
        };
        let metadata = serde_json::json!({"packages":[{"name":"rig-core","manifest_path":"C:/cache/rig-core/Cargo.toml","source":"git+https://example.test/rig"}]});
        assert!(source.check_metadata(&metadata).is_err());
        let metadata = serde_json::json!({"packages":[{"name":"rig-core","manifest_path":"C:/rig/crates/rig-core/Cargo.toml","source":null}]});
        assert!(source.check_metadata(&metadata).is_ok());
    }
}
