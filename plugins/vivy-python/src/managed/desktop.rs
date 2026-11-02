//! Candidate editing and validation reuse the immutable publication owner.
use super::*;
use rig_harness::prelude::*;
use std::sync::{Arc, Mutex};

#[derive(Clone, Default, Resource)]
pub struct Desktop(Arc<Mutex<State>>);
#[derive(Default)]
struct State {
    running: BTreeMap<String, Arc<ValidationJob>>,
    validated: BTreeMap<String, (Publication, ValidationReceipt)>,
}
#[derive(Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct Candidate {
    base: String,
    skill: SkillRevision,
}
#[derive(Debug, Serialize, Deserialize)]
#[cfg_attr(feature = "protocol-types", derive(ts_rs::TS))]
pub struct CandidateView {
    pub id: String,
    pub name: String,
    pub base: String,
    pub revision: String,
    pub dependencies: Vec<String>,
    pub files: BTreeMap<String, String>,
    pub state: String,
    pub result: Option<String>,
}
#[derive(Debug, Serialize, Deserialize)]
#[cfg_attr(feature = "protocol-types", derive(ts_rs::TS))]
pub struct PublicationView {
    pub id: String,
    pub environment: String,
    pub skills: Vec<String>,
}
#[derive(Debug, Serialize, Deserialize)]
#[cfg_attr(feature = "protocol-types", derive(ts_rs::TS))]
pub struct ManagedView {
    pub current: Option<PublicationView>,
    pub history: Vec<PublicationView>,
    pub candidates: Vec<CandidateView>,
    pub issue: Option<String>,
}

fn candidate_file(id: &str) -> io::Result<String> {
    if id.len() != 64 || !id.bytes().all(|b| b.is_ascii_hexdigit()) {
        return Err(invalid("invalid candidate identity"));
    }
    Ok(format!("desktop-candidates/{id}.json"))
}
fn candidate(store: &PublicationStore, id: &str) -> io::Result<Candidate> {
    let bytes = store.files().read_file(&candidate_file(id)?)?;
    if hash(&bytes) != id {
        return Err(invalid(
            "candidate metadata changed; preserve it for recovery",
        ));
    }
    let c: Candidate = serde_json::from_slice(&bytes)?;
    store.verify(&c.skill)?;
    Ok(c)
}
fn view(p: &Publication) -> io::Result<PublicationView> {
    Ok(PublicationView {
        id: p.id.clone(),
        environment: p.environment().into(),
        skills: p.skills()?.into_iter().map(|s| s.name).collect(),
    })
}
impl Desktop {
    pub fn read(&self, store: &PublicationStore) -> io::Result<ManagedView> {
        let mut result = ManagedView {
            current: None,
            history: vec![],
            candidates: vec![],
            issue: None,
        };
        match store.current() {
            Ok(mut p) => {
                result.current = Some(view(&p)?);
                let mut seen = std::collections::BTreeSet::new();
                loop {
                    if !seen.insert(p.id.clone()) || seen.len() > 256 {
                        return Err(invalid(
                            "publication history is cyclic or exceeds 256 entries",
                        ));
                    }
                    result.history.push(view(&p)?);
                    let Some(parent) = p.manifest["managed_parent"].as_str() else {
                        break;
                    };
                    p = store.retained(parent)?;
                }
            }
            Err(e) => result.issue = Some(e.to_string()),
        }
        let directory = store.root.join("desktop-candidates");
        if directory.exists() {
            for entry in fs::read_dir(directory)? {
                if result.candidates.len() >= 128 {
                    return Err(invalid("candidate inventory exceeds 128 entries"));
                }
                let path = entry?.path();
                if path.extension().and_then(|s| s.to_str()) != Some("json") {
                    continue;
                }
                let id = path
                    .file_stem()
                    .and_then(|s| s.to_str())
                    .ok_or_else(|| invalid("invalid candidate path"))?;
                let c = candidate(store, id)?;
                let state = self.0.lock().unwrap();
                let receipt = state.validated.get(id).map(|(_, r)| r);
                result.candidates.push(CandidateView {
                    id: id.into(),
                    name: c.skill.name,
                    base: c.base,
                    revision: c.skill.revision,
                    dependencies: c.skill.dependencies,
                    files: BTreeMap::new(),
                    state: if state.running.contains_key(id) {
                        "validating"
                    } else if receipt.is_some_and(|r| r.passed) {
                        "validated"
                    } else if receipt.is_some() {
                        "failed"
                    } else {
                        "candidate"
                    }
                    .into(),
                    result: receipt.map(|r| {
                        let text = rig_harness::rig_core::redact::text(&r.evidence.to_string())
                            .into_owned();
                        let mut end = text.len().min(4096);
                        while !text.is_char_boundary(end) {
                            end -= 1;
                        }
                        if end < text.len() {
                            format!("{}\n[Validation evidence preview truncated]", &text[..end])
                        } else {
                            text
                        }
                    }),
                });
            }
        }
        Ok(result)
    }
    pub fn open(&self, store: &PublicationStore, id: &str) -> io::Result<CandidateView> {
        let c = candidate(store, id)?;
        let mut row = self
            .read(store)?
            .candidates
            .into_iter()
            .find(|c| c.id == id)
            .ok_or_else(|| invalid("candidate not found"))?;
        row.files = source_files(Path::new(&c.skill.file).parent().unwrap(), 512 * 1024)?
            .into_iter()
            .map(|(p, b)| Ok((p, String::from_utf8(b).map_err(io::Error::other)?)))
            .collect::<io::Result<_>>()?;
        Ok(row)
    }
    pub fn import(
        &self,
        store: &PublicationStore,
        base: &str,
        name: &str,
        source: &Path,
    ) -> io::Result<()> {
        if store.current()?.id != base {
            return Err(invalid("stale publication base"));
        }
        // Admission must satisfy the same bounded text contract as the editor.
        for (_, bytes) in source_files(source, 512 * 1024)? {
            std::str::from_utf8(&bytes)
                .map_err(|e| invalid(format!("desktop candidates require UTF-8 text: {e}")))?;
        }
        let skill = store.freeze(name, source, 512 * 1024)?;
        let bytes = encode(&Candidate {
            base: base.into(),
            skill,
        })?;
        let id = hash(&bytes);
        store.files().write_file(&candidate_file(&id)?, &bytes)
    }
    pub fn edit(
        &self,
        store: &PublicationStore,
        id: &str,
        files: &BTreeMap<String, String>,
    ) -> io::Result<()> {
        let c = candidate(store, id)?;
        let mut bytes = 0;
        for (path, text) in files {
            bytes += text.len();
            if bytes > 512 * 1024
                || path.is_empty()
                || path.contains(['\\', ':'])
                || !Path::new(path)
                    .components()
                    .all(|c| matches!(c, std::path::Component::Normal(_)))
            {
                return Err(invalid(
                    "candidate edits require relative regular paths and at most 512 KiB",
                ));
            }
        }
        let digest = hash(&encode(files)?);
        let staging = store.root.join("desktop-edits").join(digest);
        let target = DirFiles::new(&staging);
        for (path, text) in files {
            target.write_file(path, text.as_bytes())?;
        }
        self.import(store, &c.base, &c.skill.name, &staging)
    }
    pub fn validate(
        &self,
        store: &PublicationStore,
        id: &str,
        spec: ValidationSpec,
        environment: Option<Publication>,
    ) -> io::Result<()> {
        let c = candidate(store, id)?;
        let current = store.current()?;
        if current.id != c.base {
            return Err(invalid("stale candidate base"));
        }
        let environment = environment.unwrap_or(current);
        let job = {
            let mut state = self.0.lock().unwrap();
            if !state.running.is_empty() {
                return Err(invalid("another desktop validation is running"));
            }
            let job = Arc::new(ValidationJob::start(
                store.clone(),
                environment.clone(),
                c.skill,
                spec,
                store.root.join("desktop-validation").join(id),
                None,
            )?);
            state.validated.remove(id);
            state.running.insert(id.into(), job.clone());
            job
        };
        let result = loop {
            if let Some(result) = job.poll() {
                break result;
            }
            std::thread::sleep(Duration::from_millis(25));
        };
        let mut state = self.0.lock().unwrap();
        state.running.remove(id);
        match result {
            Ok(receipt) => {
                state.validated.insert(id.into(), (environment, receipt));
                Ok(())
            }
            Err(e) => Err(invalid(e)),
        }
    }
    pub fn cancel(&self, id: &str) -> io::Result<()> {
        self.0
            .lock()
            .unwrap()
            .running
            .get(id)
            .ok_or_else(|| invalid("candidate validation is not running"))?
            .cancel();
        Ok(())
    }
    pub fn publish(&self, store: &PublicationStore, id: &str, update: &str) -> io::Result<()> {
        let c = candidate(store, id)?;
        let state = self.0.lock().unwrap();
        let (environment, receipt) = state.validated.get(id).ok_or_else(|| {
            invalid("validate this candidate in the current core instance before publishing")
        })?;
        store.publish_in(&c.base, environment, &c.skill, receipt, update)?;
        Ok(())
    }
}
pub fn dependencies(world: &mut World, id: &str) -> io::Result<()> {
    let store = PublicationStore::new(
        &world.resource::<ProjectRoot>().0,
        RigHome::of(world).root(),
    );
    let candidate = candidate(&store, id)?;
    request_dependencies(world, &candidate.skill.dependencies)?;
    Ok(())
}
