//! Immutable managed Skill revisions, validated publications and call selection.
use rig_memory::{DirFiles, Files};
use serde::{Deserialize, Serialize};
use serde_json::{Value, json};
use std::{
    collections::BTreeMap,
    fs::{self, File, OpenOptions},
    io,
    path::{Path, PathBuf},
};
use vivy_basics::setup::fingerprint as hash;
pub mod desktop;
pub(crate) mod preparation;
mod validation;
pub use preparation::{PendingRequirements, request_dependencies, staged_environment};
pub use validation::{ValidationCase, ValidationJob, ValidationSpec};

/// Set by the durable host before Python's per-call handler is prepared.
#[derive(rig_harness::prelude::Component, Clone, Default)]
pub struct PinnedCall {
    pub publication: Option<String>,
    pub usage: std::sync::Arc<std::sync::Mutex<Vec<Value>>>,
}

#[derive(rig_harness::prelude::Resource, Default)]
struct ActivePublication(Option<String>);
pub(super) fn install(app: &mut rig_harness::prelude::App) {
    use rig_harness::prelude::*;
    app.init_resource::<ActivePublication>()
        .add_systems(First, refresh);
    app.init_resource::<PendingRequirements>();
}
fn refresh(world: &mut rig_harness::prelude::World) {
    use rig_harness::prelude::*;
    use vivy_basics::skills::{PublishedSkills, Skill, TrustedSkillInventory, refresh_inventory};
    let Some(root) = world.get_resource::<ProjectRoot>().map(|p| p.0.clone()) else {
        return;
    };
    let store = PublicationStore::new(&root, RigHome::of(world).root());
    let current = store.current().ok();
    let id = current.as_ref().map(|p| p.id.clone());
    if world.resource::<ActivePublication>().0 == id {
        return;
    }
    let skills = current
        .and_then(|p| p.skills().ok())
        .unwrap_or_default()
        .into_iter()
        .map(|s| Skill {
            name: s.name,
            description: s.description,
            file: PathBuf::from(s.file),
            manual: false,
        })
        .collect();
    world.insert_resource(PublishedSkills(skills));
    world.resource_mut::<ActivePublication>().0 = id;
    if world.contains_resource::<TrustedSkillInventory>() {
        refresh_inventory(world);
    }
}

fn invalid(message: impl Into<String>) -> io::Error {
    io::Error::new(io::ErrorKind::InvalidData, message.into())
}
fn encode(value: &impl Serialize) -> io::Result<Vec<u8>> {
    serde_json::to_vec_pretty(value).map_err(io::Error::other)
}
fn identifier(value: &str) -> bool {
    !value.is_empty()
        && value.len() <= 128
        && value
            .bytes()
            .all(|b| b.is_ascii_alphanumeric() || b == b'_' || b == b'-')
}

#[derive(Clone, Debug, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct SkillRevision {
    pub name: String,
    pub description: String,
    pub file: String,
    pub revision: String,
    pub module: String,
    pub binding: String,
    pub dependencies: Vec<String>,
}

#[derive(Clone, Debug, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct Publication {
    pub id: String,
    pub manifest: Value,
}
impl Publication {
    pub fn skills(&self) -> io::Result<Vec<SkillRevision>> {
        serde_json::from_value(
            self.manifest
                .get("managed_skills")
                .cloned()
                .unwrap_or(json!([])),
        )
        .map_err(io::Error::other)
    }
    pub fn executable(&self) -> io::Result<&str> {
        self.manifest["executable"]
            .as_str()
            .ok_or_else(|| invalid("publication has no executable"))
    }
    pub fn environment(&self) -> &str {
        self.manifest["identity"].as_str().unwrap_or("")
    }
    pub fn scope(&self) -> &str {
        self.manifest["scope"].as_str().unwrap_or("")
    }
}

/// Produced by the host after checking frozen code in the selected environment.
/// It is not trusted merely because a file in the project contains these fields.
#[derive(Clone, Debug, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ValidationReceipt {
    pub passed: bool,
    pub revision: String,
    pub environment: String,
    pub checks: String,
    pub evidence: Value,
    pub spec: Option<ValidationSpec>,
}
impl ValidationReceipt {
    pub fn passed(
        skill: &SkillRevision,
        publication: &Publication,
        checks: &str,
        evidence: Value,
    ) -> Self {
        Self {
            passed: true,
            revision: skill.revision.clone(),
            environment: publication.environment().into(),
            checks: checks.into(),
            evidence,
            spec: None,
        }
    }
}

#[derive(Clone)]
pub struct PublicationStore {
    workspace: PathBuf,
    root: PathBuf,
    home: PathBuf,
}
impl PublicationStore {
    pub fn new(workspace: &Path, home: &Path) -> Self {
        Self {
            workspace: workspace.to_owned(),
            root: workspace.join(".vivy/workbench"),
            home: home.to_owned(),
        }
    }
    pub fn root(&self) -> &Path {
        &self.root
    }
    fn files(&self) -> DirFiles {
        DirFiles::new(&self.root)
    }
    fn local(&self) -> DirFiles {
        let canonical = self
            .workspace
            .canonicalize()
            .unwrap_or_else(|_| self.workspace.clone());
        DirFiles::new(
            self.home
                .join("initialization")
                .join(hash(canonical.to_string_lossy().as_bytes()))
                .join("publications"),
        )
    }
    pub fn lock(&self) -> io::Result<File> {
        fs::create_dir_all(&self.root)?;
        let file = OpenOptions::new()
            .create(true)
            .truncate(false)
            .read(true)
            .write(true)
            .open(self.root.join("prepare.lock"))?;
        file.try_lock().map_err(|e| {
            io::Error::new(
                io::ErrorKind::WouldBlock,
                format!("Python preparation/publication is busy: {e}"),
            )
        })?;
        Ok(file)
    }
    fn decode(&self, bytes: &[u8]) -> io::Result<Publication> {
        let manifest: Value = serde_json::from_slice(bytes).map_err(io::Error::other)?;
        let publication = Publication {
            id: hash(bytes),
            manifest,
        };
        if publication.environment().is_empty() || !Path::new(publication.executable()?).is_file() {
            return Err(invalid("publication environment is missing"));
        }
        Ok(publication)
    }
    fn trusted(&self, publication: &Publication) -> io::Result<()> {
        let decision =
            vivy_basics::setup::read(&self.home, &self.workspace, "python").map_err(invalid)?;
        if decision.permits(publication.scope())
            && decision.environment.as_deref() == Some(&publication.id)
        {
            return Ok(());
        }
        let record: Value = serde_json::from_slice(
            &self
                .local()
                .read_file(&format!("{}.json", publication.id))
                .map_err(|e| io::Error::new(io::ErrorKind::PermissionDenied, format!(
                    "Workbench publication {} has no local validation record after its installation scope changed: {e}", publication.id
                )))?,
        )
        .map_err(io::Error::other)?;
        if record["publication"] != publication.id || record["scope"] != publication.scope() {
            return Err(invalid(
                "publication has no matching local validation record",
            ));
        }
        Ok(())
    }
    pub fn current(&self) -> io::Result<Publication> {
        let p = self.decode(&self.files().read_file("current.json")?)?;
        self.trusted(&p)?;
        Ok(p)
    }
    pub fn retained(&self, id: &str) -> io::Result<Publication> {
        if id.len() != 64 || !id.bytes().all(|b| b.is_ascii_hexdigit()) {
            return Err(invalid("invalid publication identity"));
        }
        let p = self.decode(&self.files().read_file(&format!("publications/{id}.json"))?)?;
        if p.id != id {
            return Err(invalid("retained publication digest mismatch"));
        }
        self.trusted(&p)?;
        Ok(p)
    }
    pub fn selected(&self, id: &str) -> io::Result<Publication> {
        let current = self.current()?;
        let p = if current.id == id {
            current
        } else {
            self.retained(id)?
        };
        for skill in p.skills()? {
            self.verify(&skill)?;
        }
        Ok(p)
    }
    fn keep(&self, p: &Publication, bytes: &[u8], validation: &Value) -> io::Result<()> {
        if hash(bytes) != p.id {
            return Err(invalid("publication bytes changed"));
        }
        self.files()
            .write_file(&format!("publications/{}.json", p.id), bytes)?;
        self.local().write_file(
            &format!("{}.json", p.id),
            &encode(&json!({"publication":p.id,"scope":p.scope(),"validation":validation}))?,
        )?;
        Ok(())
    }
    pub fn retain_current(&self) -> io::Result<Publication> {
        let _lock = self.lock()?;
        let current = self.current()?;
        self.keep(
            &current,
            &self.files().read_file("current.json")?,
            &json!({"retained":true}),
        )?;
        Ok(current)
    }
    /// A prepared environment is locally validated, but has no activation lineage.
    pub fn stage(&self, expected: &str, mut manifest: Value) -> io::Result<Publication> {
        let _lock = self.lock()?;
        let current = self.current()?;
        if current.id != expected {
            return Err(invalid("stale preparation base"));
        }
        let scope = manifest["scope"]
            .as_str()
            .ok_or_else(|| invalid("missing preparation scope"))?
            .to_owned();
        let decision =
            vivy_basics::setup::read(&self.home, &self.workspace, "python").map_err(invalid)?;
        if !decision.permits(&scope) {
            return Err(invalid("staged dependencies require /setup python consent"));
        }
        self.keep(
            &current,
            &self.files().read_file("current.json")?,
            &json!({"retained":true}),
        )?;
        manifest["managed_skills"] = current
            .manifest
            .get("managed_skills")
            .cloned()
            .unwrap_or(json!([]));
        manifest["managed_validations"] = current
            .manifest
            .get("managed_validations")
            .cloned()
            .unwrap_or(json!({}));
        manifest["staged_base"] = json!(expected);
        let bytes = encode(&manifest)?;
        let p = self.decode(&bytes)?;
        self.keep(&p, &bytes, &json!({"environment_prepared":true}))?;
        Ok(p)
    }
    pub fn ready_stage(&self, p: &Publication) -> io::Result<()> {
        let _lock = self.lock()?;
        let selected = self.retained(&p.id)?;
        if selected != *p || p.manifest["staged_base"] != self.current()?.id {
            return Err(invalid("stale staged environment"));
        }
        let bytes = self
            .files()
            .read_file(&format!("publications/{}.json", p.id))?;
        self.files().write_file(
            &format!("staged/{}.json", hash(p.scope().as_bytes())),
            &bytes,
        )
    }
    pub fn staged(&self, scope: &str) -> io::Result<Option<Publication>> {
        let bytes = match self
            .files()
            .read_file(&format!("staged/{}.json", hash(scope.as_bytes())))
        {
            Ok(bytes) => bytes,
            Err(e) if e.kind() == io::ErrorKind::NotFound => return Ok(None),
            Err(e) => return Err(e),
        };
        let p = self.decode(&bytes)?;
        self.trusted(&p)?;
        if p.scope() != scope {
            return Err(invalid("staged scope mismatch"));
        }
        Ok(Some(p))
    }
    pub fn freeze(&self, name: &str, draft: &Path, budget: u64) -> io::Result<SkillRevision> {
        if !identifier(name) {
            return Err(invalid("invalid managed Skill name"));
        }
        let entries = source_files(draft, budget)?;
        let metadata = entries
            .get("pyproject.toml")
            .ok_or_else(|| invalid("managed Skill needs pyproject.toml"))?;
        let document: toml::Value =
            toml::from_str(std::str::from_utf8(metadata).map_err(io::Error::other)?)
                .map_err(io::Error::other)?;
        if document
            .get("project")
            .and_then(|p| p.get("dynamic"))
            .is_some()
        {
            return Err(invalid(
                "dynamic package metadata is unsupported for managed Skills",
            ));
        }
        if let Some(backend) = document
            .get("build-system")
            .and_then(|b| b.get("build-backend"))
            .and_then(|b| b.as_str())
        {
            if !["setuptools.build_meta", "hatchling.build"].contains(&backend) {
                return Err(invalid(
                    "native/custom build hooks are unsupported for managed Skills",
                ));
            }
        }
        let package = document
            .get("project")
            .and_then(|p| p.get("name"))
            .and_then(|n| n.as_str())
            .ok_or_else(|| invalid("Skill project.name is required"))?;
        let options = document
            .get("tool")
            .and_then(|v| v.get("vivy"))
            .and_then(|v| v.get("skill"));
        let default_module = package.replace('-', "_");
        let module = options
            .and_then(|o| o.get("module"))
            .and_then(|m| m.as_str())
            .unwrap_or(&default_module);
        let binding = options
            .and_then(|o| o.get("binding"))
            .and_then(|m| m.as_str())
            .unwrap_or(module);
        let python_identifier =
            |s: &str| identifier(s) && !s.contains('-') && !s.as_bytes()[0].is_ascii_digit();
        if !python_identifier(module) || !python_identifier(binding) {
            return Err(invalid(
                "managed module and binding must be simple Python identifiers",
            ));
        }
        if !entries.contains_key(&format!("src/{module}/__init__.py")) {
            return Err(invalid("managed Skill needs src/import_name/__init__.py"));
        }
        let instructions = std::str::from_utf8(
            entries
                .get("SKILL.md")
                .ok_or_else(|| invalid("managed Skill needs SKILL.md"))?,
        )
        .map_err(io::Error::other)?;
        let description = instructions
            .lines()
            .find_map(|l| l.strip_prefix("description:"))
            .unwrap_or(name)
            .trim()
            .to_owned();
        let dependencies = document
            .get("project")
            .and_then(|p| p.get("dependencies"))
            .map(|v| {
                v.as_array()
                    .ok_or_else(|| invalid("dependencies must be an array"))
            })
            .transpose()?
            .into_iter()
            .flatten()
            .map(|v| {
                v.as_str()
                    .map(str::to_owned)
                    .ok_or_else(|| invalid("dependency must be text"))
            })
            .collect::<io::Result<Vec<_>>>()?;
        let revision = hash(&serde_json::to_vec(&entries).map_err(io::Error::other)?);
        let target = self.root.join("skill-revisions").join(name).join(&revision);
        if target.exists() {
            if source_files(&target, budget)? != entries {
                return Err(invalid("frozen Skill revision was modified"));
            }
        } else {
            let used =
                crate::management::toolchain::directory_bytes(&self.root.join("skill-revisions"))?;
            let added = entries.values().map(|v| v.len() as u64).sum::<u64>();
            if used.saturating_add(added) > budget {
                return Err(invalid(
                    "managed revision storage budget exhausted; retained revisions were preserved",
                ));
            }
            for (path, bytes) in &entries {
                DirFiles::new(&target).write_file(path, bytes)?;
            }
        }
        Ok(SkillRevision {
            name: name.into(),
            description,
            file: target.join("SKILL.md").to_string_lossy().into_owned(),
            revision,
            module: module.into(),
            binding: binding.into(),
            dependencies,
        })
    }
    pub fn verify(&self, skill: &SkillRevision) -> io::Result<()> {
        let expected = self
            .root
            .join("skill-revisions")
            .join(&skill.name)
            .join(&skill.revision)
            .join("SKILL.md");
        if !identifier(&skill.name) || Path::new(&skill.file) != expected {
            return Err(invalid("Skill is outside its managed revision"));
        }
        let entries = source_files(expected.parent().unwrap(), 256 * 1024 * 1024)?;
        if hash(&serde_json::to_vec(&entries).map_err(io::Error::other)?) != skill.revision {
            return Err(invalid("frozen Skill source digest mismatch"));
        }
        Ok(())
    }
    pub fn activated(&self, update: &str) -> io::Result<Option<Publication>> {
        let mut current = self.current()?;
        let mut seen = std::collections::BTreeSet::new();
        loop {
            if !seen.insert(current.id.clone()) {
                return Err(invalid("publication history cycle"));
            }
            if current.manifest["update_id"] == update {
                return Ok(Some(current));
            }
            let Some(parent) = current.manifest["managed_parent"].as_str() else {
                return Ok(None);
            };
            current = self.retained(parent)?;
        }
    }
    pub fn publish(
        &self,
        expected: &str,
        skill: &SkillRevision,
        receipt: &ValidationReceipt,
        update: &str,
    ) -> io::Result<Publication> {
        self.publish_in(expected, &self.current()?, skill, receipt, update)
    }
    pub fn publish_in(
        &self,
        expected: &str,
        environment: &Publication,
        skill: &SkillRevision,
        receipt: &ValidationReceipt,
        update: &str,
    ) -> io::Result<Publication> {
        let _lock = self.lock()?;
        if update.is_empty() {
            return Err(invalid("publication requires an update ID"));
        }
        if let Some(prior) = self.activated(update)? {
            if prior.manifest["managed_parent"] != expected
                || !prior.skills()?.iter().any(|s| s == skill)
                || prior.manifest["validation"] != serde_json::to_value(receipt)?
            {
                return Err(invalid("update ID already belongs to another publication"));
            }
            return Ok(prior);
        }
        let current = self.current()?;
        if current.id != expected {
            return Err(invalid("stale publication base"));
        }
        self.verify(skill)?;
        let selected = self.selected(&environment.id)?;
        if selected != *environment
            || (selected.id != current.id && selected.manifest["staged_base"] != expected)
        {
            return Err(invalid(
                "validation environment does not belong to this base",
            ));
        }
        if !receipt.passed
            || receipt.revision != skill.revision
            || receipt.environment != environment.environment()
            || receipt.checks.is_empty()
            || receipt.evidence["assertions"].as_u64().unwrap_or(0) == 0
        {
            return Err(invalid("candidate lacks matching successful validation"));
        }
        let mut skills = current.skills()?;
        let admitted: Vec<String> = serde_json::from_value(
            environment
                .manifest
                .get("managed_requirements")
                .cloned()
                .unwrap_or(json!([])),
        )?;
        if skill.dependencies.iter().any(|d| !admitted.contains(d)) {
            return Err(invalid(
                "candidate dependencies require staged /setup python preparation",
            ));
        }
        skills.retain(|s| s.name != skill.name);
        if skills
            .iter()
            .any(|s| s.binding == skill.binding || s.module == skill.module)
        {
            return Err(invalid("managed Skill binding/module collision"));
        }
        for ordinary in current.manifest["skills"].as_array().into_iter().flatten() {
            if ordinary["name"] == skill.name {
                return Err(invalid("managed Skill collides with unmanaged discovery"));
            }
        }
        skills.push(skill.clone());
        skills.sort_by(|a, b| a.name.cmp(&b.name));
        let mut next = environment.manifest.clone();
        next["managed_skills"] = serde_json::to_value(skills)?;
        if !next["managed_validations"].is_object() {
            next["managed_validations"] = json!({});
        }
        next["managed_validations"][&skill.name] = serde_json::to_value(receipt)?;
        next["validation"] = serde_json::to_value(receipt)?;
        self.activate(&current, next, update)
    }
    fn activate(
        &self,
        current: &Publication,
        mut next: Value,
        update: &str,
    ) -> io::Result<Publication> {
        let old = self.files().read_file("current.json")?;
        if hash(&old) != current.id {
            return Err(invalid("publication changed while locked"));
        }
        self.keep(current, &old, &json!({"retained":true}))?;
        next["managed_parent"] = json!(current.id);
        next.as_object_mut().unwrap().remove("staged_base");
        next["update_id"] = json!(update);
        let bytes = encode(&next)?;
        let publication = self.decode(&bytes)?;
        self.keep(&publication, &bytes, &next["validation"])?;
        self.files().write_file("current.json", &bytes)?;
        Ok(publication)
    }
    pub fn rollback(&self, expected: &str, target: &str, update: &str) -> io::Result<Publication> {
        let _lock = self.lock()?;
        if let Some(prior) = self.activated(update)? {
            if prior.manifest["rollback_target"] == target
                && prior.manifest["managed_parent"] == expected
            {
                return Ok(prior);
            }
            return Err(invalid("rollback update ID conflict"));
        }
        let current = self.current()?;
        if current.id != expected {
            return Err(invalid("stale rollback base"));
        }
        // A staged but never activated manifest cannot become a rollback target.
        let mut ancestor = current.clone();
        while ancestor.id != target {
            ancestor = self.retained(
                ancestor.manifest["managed_parent"]
                    .as_str()
                    .ok_or_else(|| invalid("rollback target was not activated"))?,
            )?;
        }
        for skill in ancestor.skills()? {
            self.verify(&skill)?;
        }
        let mut next = ancestor.manifest;
        next["rollback_target"] = json!(target);
        self.activate(&current, next, update)
    }
}

fn source_files(root: &Path, budget: u64) -> io::Result<BTreeMap<String, Vec<u8>>> {
    fn visit(
        root: &Path,
        dir: &Path,
        files: &mut BTreeMap<String, Vec<u8>>,
        remaining: &mut u64,
    ) -> io::Result<()> {
        #[cfg(windows)]
        {
            use std::os::windows::fs::MetadataExt;
            if fs::symlink_metadata(dir)?.file_attributes() & 0x400 != 0 {
                return Err(invalid("source reparse points are unsupported"));
            }
        }
        if fs::symlink_metadata(dir)?.file_type().is_symlink() {
            return Err(invalid("source symlinks are unsupported"));
        }
        for entry in fs::read_dir(dir)? {
            let entry = entry?;
            let path = entry.path();
            let meta = fs::symlink_metadata(&path)?;
            #[cfg(windows)]
            {
                use std::os::windows::fs::MetadataExt;
                if meta.file_attributes() & 0x400 != 0 {
                    return Err(invalid("source reparse points are unsupported"));
                }
            }
            if meta.file_type().is_symlink() {
                return Err(invalid("source symlinks are unsupported"));
            }
            if meta.is_dir() {
                if ["__pycache__", ".git", ".venv", ".pytest_cache"]
                    .contains(&entry.file_name().to_string_lossy().as_ref())
                {
                    continue;
                }
                visit(root, &path, files, remaining)?;
            } else if meta.is_file() {
                if matches!(
                    path.extension().and_then(|e| e.to_str()),
                    Some("pyc" | "pyd" | "so" | "dll" | "dylib")
                ) {
                    return Err(invalid(
                        "native/compiled files are unsupported in managed sources",
                    ));
                }
                *remaining = remaining
                    .checked_sub(meta.len())
                    .ok_or_else(|| invalid("managed source staging budget exhausted"))?;
                let bytes = fs::read(&path)?;
                if bytes.len() as u64 != meta.len() {
                    return Err(invalid("source changed during freeze"));
                }
                files.insert(
                    path.strip_prefix(root)
                        .map_err(io::Error::other)?
                        .to_string_lossy()
                        .replace('\\', "/"),
                    bytes,
                );
            }
        }
        Ok(())
    }
    let mut files = BTreeMap::new();
    let mut remaining = budget;
    visit(root, root, &mut files, &mut remaining)?;
    Ok(files)
}

#[cfg(test)]
mod tests;
