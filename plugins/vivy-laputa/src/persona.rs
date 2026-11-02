//! Laputa's six core persona files, stored as real Markdown documents.
use rig_memory::{DirFiles, Files};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::{
    collections::BTreeMap,
    fs::{self, File, OpenOptions},
    io,
    path::{Path, PathBuf},
};

pub const MISSION: &str = "MISSION.MD";
pub const IDENTITY: &str = "IDENTITY.MD";
pub const RELATIONSHIP: &str = "RELATIONSHIP.MD";
pub const REDLINE: &str = "REDLINE.MD";
pub const DREAM: &str = "DREAM.MD";
pub const DARK: &str = "DARK.MD";

pub const ASSEMBLY_FILES: [&str; 6] = [MISSION, IDENTITY, RELATIONSHIP, REDLINE, DREAM, DARK];
pub const AUTO_FILES: [&str; 4] = [IDENTITY, RELATIONSHIP, DREAM, DARK];
pub const OPTIONAL_FILES: [&str; 2] = [DREAM, DARK];
pub const MAX_FILE_BYTES: usize = 32 * 1024;
pub const MAX_PERSONA_BYTES: usize = 64 * 1024;
pub const PERSONA_ID: &str = "main";

pub fn required(name: &str) -> bool {
    !OPTIONAL_FILES.contains(&name)
}

pub fn auto(name: &str) -> bool {
    AUTO_FILES.contains(&name)
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Persona {
    pub schema_version: u32,
    pub persona_id: String,
    pub revision: u64,
    pub files: BTreeMap<String, String>,
    pub last_update_id: String,
    pub sources: Vec<String>,
    pub parent_update_id: String,
    pub sources_digest: String,
    /// Files on disk differ from the published revision; never stored.
    #[serde(default, skip_serializing)]
    pub unpublished: bool,
}

impl Persona {
    /// Fixed-order assembly; empty bodies are skipped, nothing is trimmed.
    pub fn text(&self) -> String {
        let mut parts = Vec::new();
        for name in ASSEMBLY_FILES {
            if let Some(body) = self.files.get(name)
                && !body.is_empty()
            {
                parts.push(body.as_str());
            }
        }
        parts.join("\n\n")
    }

    pub fn file(&self, name: &str) -> &str {
        self.files.get(name).map(String::as_str).unwrap_or("")
    }
}

#[derive(Deserialize)]
struct StoredV1 {
    revision: u64,
    text: String,
    last_update_id: String,
    sources: Vec<String>,
    parent_update_id: String,
}

#[derive(Deserialize)]
struct Probe {
    schema_version: u32,
}

pub struct PersonaStore {
    root: PathBuf,
}
impl PersonaStore {
    pub fn new(workspace: &Path) -> Self {
        Self {
            root: workspace.join(".vivy/laputa"),
        }
    }
    /// The directory holding the six editable source files.
    pub fn directory(&self) -> PathBuf {
        self.root.join("personas").join(PERSONA_ID)
    }
    fn files(&self) -> DirFiles {
        DirFiles::new(&self.root)
    }
    fn sources(&self) -> DirFiles {
        DirFiles::new(self.directory())
    }
    fn seed(&self) -> Persona {
        let files = defaults();
        Persona {
            schema_version: 2,
            persona_id: PERSONA_ID.into(),
            revision: 0,
            sources_digest: digest_files(&files),
            files,
            last_update_id: String::new(),
            sources: Vec::new(),
            parent_update_id: String::new(),
            unpublished: false,
        }
    }
    pub fn read(&self) -> io::Result<Persona> {
        let Some(mut loaded) = self.load()? else {
            return Ok(self.seed());
        };
        if loaded.legacy {
            loaded.persona.unpublished = false;
            return Ok(loaded.persona);
        }
        let directory = self.directory();
        if !directory.is_dir() {
            return Err(invalid(format!(
                "persona source directory is missing: {}",
                directory.display()
            )));
        }
        let mut files = BTreeMap::new();
        for name in ASSEMBLY_FILES {
            match fs::read(directory.join(name)) {
                Ok(bytes) => {
                    files.insert(name.to_string(), body(name, &bytes)?);
                }
                Err(e) if e.kind() == io::ErrorKind::NotFound && !required(name) => {
                    files.insert(name.to_string(), String::new());
                }
                Err(e) => {
                    return Err(io::Error::new(
                        e.kind(),
                        format!("persona file {name} cannot be read: {e}"),
                    ));
                }
            }
        }
        validate_files(&files)?;
        loaded.persona.unpublished = digest_files(&files) != loaded.persona.sources_digest;
        loaded.persona.files = files;
        Ok(loaded.persona)
    }
    fn load(&self) -> io::Result<Option<Loaded>> {
        let bytes = match self.files().read_file("persona.json") {
            Ok(bytes) => bytes,
            Err(e) if e.kind() == io::ErrorKind::NotFound => return Ok(None),
            Err(e) => return Err(e),
        };
        let probe: Probe = serde_json::from_slice(&bytes).map_err(io::Error::other)?;
        match probe.schema_version {
            1 => {
                let old: StoredV1 = serde_json::from_slice(&bytes).map_err(io::Error::other)?;
                if old.revision == 0 || old.last_update_id.is_empty() {
                    return Err(invalid("invalid persona schema or revision"));
                }
                let persona = self.migrate(&old);
                if self.history(&persona.last_update_id)? != persona {
                    return Err(invalid(
                        "persona differs from its immutable published revision",
                    ));
                }
                Ok(Some(Loaded {
                    persona,
                    legacy: true,
                }))
            }
            2 => {
                let persona: Persona = serde_json::from_slice(&bytes).map_err(io::Error::other)?;
                check(&persona)?;
                if self.history(&persona.last_update_id)? != persona {
                    return Err(invalid(
                        "persona differs from its immutable published revision",
                    ));
                }
                Ok(Some(Loaded {
                    persona,
                    legacy: false,
                }))
            }
            _ => Err(invalid("unsupported persona schema version")),
        }
    }
    fn history(&self, id: &str) -> io::Result<Persona> {
        let bytes = self.files().read_file(&history_name(id))?;
        let probe: Probe = serde_json::from_slice(&bytes).map_err(io::Error::other)?;
        match probe.schema_version {
            1 => {
                let old: StoredV1 = serde_json::from_slice(&bytes).map_err(io::Error::other)?;
                if old.last_update_id != id || old.revision == 0 {
                    return Err(invalid("invalid persona history entry"));
                }
                Ok(self.migrate(&old))
            }
            2 => {
                let persona: Persona = serde_json::from_slice(&bytes).map_err(io::Error::other)?;
                check(&persona)?;
                if persona.last_update_id != id {
                    return Err(invalid("invalid persona history entry"));
                }
                Ok(persona)
            }
            _ => Err(invalid("unsupported persona schema version")),
        }
    }
    fn migrate(&self, old: &StoredV1) -> Persona {
        let mut files = defaults();
        if !old.text.is_empty() {
            files.insert(IDENTITY.into(), old.text.clone());
        }
        Persona {
            schema_version: 2,
            persona_id: PERSONA_ID.into(),
            revision: old.revision,
            sources_digest: digest_files(&files),
            files,
            last_update_id: old.last_update_id.clone(),
            sources: old.sources.clone(),
            parent_update_id: old.parent_update_id.clone(),
            unpublished: false,
        }
    }
    fn lock(&self) -> io::Result<File> {
        fs::create_dir_all(&self.root)?;
        let f = OpenOptions::new()
            .create(true)
            .truncate(false)
            .read(true)
            .write(true)
            .open(self.root.join("writer.lock"))?;
        f.lock()?;
        Ok(f)
    }
    fn ancestors(&self, current: &Persona) -> io::Result<Vec<Persona>> {
        let mut all = Vec::new();
        let mut next = current.clone();
        while next.revision > 0 {
            let id = next.parent_update_id.clone();
            let rev = next.revision;
            all.push(next);
            if rev == 1 {
                if !id.is_empty() {
                    return Err(invalid("invalid initial persona parent"));
                }
                break;
            }
            next = self.history(&id)?;
            if next.revision + 1 != rev || next.last_update_id != id {
                return Err(invalid("invalid persona history chain"));
            }
        }
        Ok(all)
    }
    fn find_update(&self, current: &Persona, id: &str) -> io::Result<Option<Persona>> {
        Ok(self
            .ancestors(current)?
            .into_iter()
            .find(|p| p.last_update_id == id))
    }
    /// Only activated ancestry counts; a prepared revision is not adoption.
    pub fn activated(&self, id: &str) -> io::Result<Option<Persona>> {
        self.find_update(&self.read()?, id)
    }
    /// The activated chain only; prepared but unpublished revisions stay private.
    pub fn revisions(&self) -> io::Result<Vec<Persona>> {
        self.ancestors(&self.read()?)
    }
    pub fn save_file(
        &self,
        expected: u64,
        expected_update_id: &str,
        id: &str,
        name: &str,
        text: &str,
        sources: Vec<String>,
    ) -> io::Result<Persona> {
        let _lock = self.lock()?;
        self.save_locked(expected, expected_update_id, None, id, name, text, sources)
    }
    /// Fails closed when the target file changed after a candidate was prepared.
    pub fn save_file_checked(
        &self,
        expected: u64,
        expected_update_id: &str,
        expected_file_digest: &str,
        id: &str,
        name: &str,
        text: &str,
        sources: Vec<String>,
    ) -> io::Result<Persona> {
        let _lock = self.lock()?;
        self.save_locked(
            expected,
            expected_update_id,
            Some(expected_file_digest),
            id,
            name,
            text,
            sources,
        )
    }
    fn save_locked(
        &self,
        expected: u64,
        expected_update_id: &str,
        expected_file_digest: Option<&str>,
        id: &str,
        name: &str,
        text: &str,
        sources: Vec<String>,
    ) -> io::Result<Persona> {
        if id.trim().is_empty() {
            return Err(invalid("an update ID is required"));
        }
        if !ASSEMBLY_FILES.contains(&name) {
            return Err(invalid(format!("unknown persona file: {name}")));
        }
        let fresh = self.load()?.is_none_or(|loaded| loaded.legacy);
        let current = self.read()?;
        let mut files = current.files.clone();
        files.insert(name.into(), text.into());
        validate_files(&files)?;
        if let Some(prior) = self.find_update(&current, id)? {
            return if prior.files == files && prior.sources == sources {
                Ok(prior)
            } else {
                Err(invalid("update ID already has different content"))
            };
        }
        if current.revision != expected || current.last_update_id != expected_update_id {
            return Err(invalid("persona revision conflict"));
        }
        if let Some(expected_digest) = expected_file_digest
            && digest_text(current.file(name)) != expected_digest
        {
            return Err(invalid(
                "persona file is stale: it changed after the candidate was prepared",
            ));
        }
        let next = Persona {
            revision: expected
                .checked_add(1)
                .ok_or_else(|| invalid("revision overflow"))?,
            files: files.clone(),
            last_update_id: id.into(),
            sources,
            parent_update_id: current.last_update_id.clone(),
            sources_digest: digest_files(&files),
            unpublished: false,
            ..current
        };
        if fresh {
            for name in ASSEMBLY_FILES {
                self.sources().write_file(name, files[name].as_bytes())?;
            }
        } else {
            self.sources().write_file(name, text.as_bytes())?;
        }
        self.publish(&next)
    }
    pub fn rollback(&self, expected: u64, target: u64, id: &str) -> io::Result<Persona> {
        let _lock = self.lock()?;
        let current = self.read()?;
        let sources = vec![format!("rollback:{target}")];
        if let Some(prior) = self.find_update(&current, id)? {
            return if prior.sources == sources {
                Ok(prior)
            } else {
                Err(invalid("update ID already has different content"))
            };
        }
        if current.revision != expected {
            return Err(invalid("persona revision conflict"));
        }
        let files = if target == 0 {
            defaults()
        } else {
            self.ancestors(&current)?
                .into_iter()
                .find(|p| p.revision == target)
                .ok_or_else(|| invalid("persona rollback revision is not published"))?
                .files
        };
        validate_files(&files)?;
        let next = Persona {
            revision: expected
                .checked_add(1)
                .ok_or_else(|| invalid("revision overflow"))?,
            files: files.clone(),
            last_update_id: id.into(),
            sources,
            parent_update_id: current.last_update_id.clone(),
            sources_digest: digest_files(&files),
            unpublished: false,
            ..current
        };
        for name in ASSEMBLY_FILES {
            self.sources().write_file(name, files[name].as_bytes())?;
        }
        self.publish(&next)
    }
    fn publish(&self, next: &Persona) -> io::Result<Persona> {
        let name = history_name(&next.last_update_id);
        let bytes = serde_json::to_vec_pretty(next).map_err(io::Error::other)?;
        match self.files().read_file(&name) {
            Ok(old) if old != bytes => {
                return Err(invalid(
                    "update ID belongs to a different prepared revision",
                ));
            }
            Ok(_) => (),
            Err(e) if e.kind() == io::ErrorKind::NotFound => {
                self.files().write_file(&name, &bytes)?
            }
            Err(e) => return Err(e),
        }
        self.files().write_file("persona.json", &bytes)?;
        let published = self.read()?;
        if published != *next {
            return Err(invalid("persona publication did not round-trip"));
        }
        Ok(published)
    }
}

struct Loaded {
    persona: Persona,
    legacy: bool,
}

fn check(persona: &Persona) -> io::Result<()> {
    if persona.schema_version != 2
        || persona.persona_id != PERSONA_ID
        || persona.revision == 0
        || persona.last_update_id.is_empty()
    {
        return Err(invalid("invalid persona schema or revision"));
    }
    validate_files(&persona.files)?;
    if persona.sources_digest != digest_files(&persona.files) {
        return Err(invalid("persona source digest does not match its revision"));
    }
    Ok(())
}

fn validate_files(files: &BTreeMap<String, String>) -> io::Result<()> {
    for name in files.keys() {
        if !ASSEMBLY_FILES.contains(&name.as_str()) {
            return Err(invalid(format!("unknown persona file: {name}")));
        }
    }
    let mut total = 0;
    for name in ASSEMBLY_FILES {
        let Some(body) = files.get(name) else {
            return Err(invalid(format!("required persona file is missing: {name}")));
        };
        if body.len() > MAX_FILE_BYTES {
            return Err(invalid(format!("{name} exceeds the per-file byte budget")));
        }
        total += body.len();
    }
    if total > MAX_PERSONA_BYTES {
        return Err(invalid("persona exceeds the assembled byte budget"));
    }
    if files
        .get(MISSION)
        .is_none_or(|mission| mission.trim().is_empty())
    {
        return Err(invalid("MISSION.MD must not be blank"));
    }
    Ok(())
}

fn body(name: &str, bytes: &[u8]) -> io::Result<String> {
    if bytes.len() > MAX_FILE_BYTES {
        return Err(invalid(format!("{name} exceeds the per-file byte budget")));
    }
    String::from_utf8(bytes.to_vec()).map_err(|_| invalid(format!("{name} is not valid UTF-8")))
}

fn defaults() -> BTreeMap<String, String> {
    ASSEMBLY_FILES
        .into_iter()
        .map(|name| (name.to_string(), default_body(name).to_string()))
        .collect()
}

fn default_body(name: &str) -> &'static str {
    match name {
        MISSION => include_str!("../../../prompts/persona/MISSION.MD"),
        IDENTITY => include_str!("../../../prompts/persona/IDENTITY.MD"),
        RELATIONSHIP => include_str!("../../../prompts/persona/RELATIONSHIP.MD"),
        REDLINE => include_str!("../../../prompts/persona/REDLINE.MD"),
        DREAM => include_str!("../../../prompts/persona/DREAM.MD"),
        DARK => include_str!("../../../prompts/persona/DARK.MD"),
        _ => "",
    }
}

pub fn digest(bytes: &[u8]) -> String {
    format!("{:x}", Sha256::digest(bytes))
}

pub fn digest_text(text: &str) -> String {
    digest(text.as_bytes())
}

pub fn digest_files(files: &BTreeMap<String, String>) -> String {
    let mut hash = Sha256::new();
    for name in ASSEMBLY_FILES {
        let body = files.get(name).map(String::as_str).unwrap_or("");
        hash.update(name.as_bytes());
        hash.update([0]);
        hash.update((body.len() as u64).to_be_bytes());
        hash.update(body.as_bytes());
    }
    format!("{:x}", hash.finalize())
}

fn invalid(text: impl ToString) -> io::Error {
    io::Error::new(io::ErrorKind::InvalidData, text.to_string())
}

fn history_name(id: &str) -> String {
    format!("revisions/{}.json", digest(id.as_bytes()))
}
