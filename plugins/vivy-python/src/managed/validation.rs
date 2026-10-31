//! A bounded validation process owned by Rust, independent of any Python cell.
use super::*;
use crate::{Cancellation, management::toolchain, worker};
use std::sync::{Arc, Mutex, mpsc};

#[derive(Clone, Debug, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ValidationCase {
    #[serde(default)]
    pub args: Vec<Value>,
    #[serde(default)]
    pub kwargs: BTreeMap<String, Value>,
    pub expected: Value,
}
#[derive(Clone, Debug, Serialize, Deserialize, PartialEq)]
#[serde(default, deny_unknown_fields)]
pub struct ValidationSpec {
    pub cases: Vec<ValidationCase>,
    pub timeout_secs: u64,
    pub require_regression: bool,
    pub replay_safe: bool,
    pub output_budget_bytes: u64,
}
impl Default for ValidationSpec {
    fn default() -> Self {
        Self {
            cases: vec![],
            timeout_secs: 60,
            require_regression: false,
            replay_safe: false,
            output_budget_bytes: 256 * 1024 * 1024,
        }
    }
}
impl ValidationSpec {
    pub fn validate(&self) -> io::Result<()> {
        if self.cases.is_empty()
            || self.cases.len() > 16
            || self.timeout_secs == 0
            || self.timeout_secs > 3600
            || self.output_budget_bytes == 0
            || self.output_budget_bytes > 256 * 1024 * 1024
        {
            return Err(invalid(
                "validation requires 1..16 callable assertions, 1..3600 seconds and at most 256 MiB of scratch output",
            ));
        }
        if encode(self)?.len() > 1024 * 1024 {
            return Err(invalid("validation inputs exceed 1 MiB"));
        }
        Ok(())
    }
    pub fn digest(&self) -> String {
        hash(&serde_json::to_vec(self).expect("serializable checks"))
    }
}

pub struct ValidationJob {
    cancel: Arc<Cancellation>,
    receiver: Mutex<mpsc::Receiver<Result<ValidationReceipt, String>>>,
}
impl ValidationJob {
    pub fn start(
        store: PublicationStore,
        publication: Publication,
        skill: SkillRevision,
        spec: ValidationSpec,
        scratch: PathBuf,
        wake: Option<rig_harness::prelude::Wake>,
    ) -> io::Result<Self> {
        spec.validate()?;
        store.verify(&skill)?;
        store.selected(&publication.id)?;
        let cancel = Arc::<Cancellation>::default();
        let owned = cancel.clone();
        let (sender, receiver) = mpsc::channel();
        std::thread::spawn(move || {
            let result = run(&store, &publication, &skill, &spec, &scratch, &owned);
            let _ = sender.send(result);
            if let Some(wake) = wake {
                wake.wake();
            }
        });
        Ok(Self {
            cancel,
            receiver: Mutex::new(receiver),
        })
    }
    pub fn poll(&self) -> Option<Result<ValidationReceipt, String>> {
        match self
            .receiver
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .try_recv()
        {
            Ok(result) => Some(result),
            Err(mpsc::TryRecvError::Empty) => None,
            Err(mpsc::TryRecvError::Disconnected) => {
                Some(Err("validation worker exited without an outcome".into()))
            }
        }
    }
    pub fn cancel(&self) {
        self.cancel.stop();
    }
}
impl Drop for ValidationJob {
    fn drop(&mut self) {
        self.cancel();
    }
}

pub(super) fn run(
    store: &PublicationStore,
    publication: &Publication,
    skill: &SkillRevision,
    spec: &ValidationSpec,
    scratch: &Path,
    cancel: &Cancellation,
) -> Result<ValidationReceipt, String> {
    let evaluate = |revision: &SkillRevision, label: &str| -> Result<Value, String> {
        store.verify(revision).map_err(|e| e.to_string())?;
        let dir = scratch.join(label);
        fs::create_dir_all(&dir).map_err(|e| e.to_string())?;
        let runtime = worker::package(&dir)?;
        let request = dir.join("check.json");
        let mut inventory = publication.manifest["skills"]
            .as_array()
            .cloned()
            .unwrap_or_default();
        for other in publication.skills().map_err(|e| e.to_string())? {
            if other.name != revision.name {
                inventory.push(json!(other));
            }
        }
        inventory.push(json!(revision));
        fs::write(
            &request,
            encode(&json!({"skill":revision,"inventory":inventory,"checks":spec}))
                .map_err(|e| e.to_string())?,
        )
        .map_err(|e| e.to_string())?;
        let mut command =
            vivy_process::command(publication.executable().map_err(|e| e.to_string())?);
        let bootstrap = format!(
            "import sys;sys.path.insert(0,{});from vivy_workbench.validation import main;main({})",
            json!(runtime),
            json!(request)
        );
        command
            .args(["-I", "-u", "-c", &bootstrap])
            .current_dir(&dir)
            .env("PYTHONDONTWRITEBYTECODE", "1")
            .env("TMP", &dir)
            .env("TEMP", &dir);
        let result = toolchain::run_bounded(
            &mut command,
            cancel,
            &Arc::default(),
            spec.timeout_secs,
            Some((scratch, spec.output_budget_bytes)),
        )?;
        store.verify(revision).map_err(|e| e.to_string())?;
        for other in publication.skills().map_err(|e| e.to_string())? {
            store.verify(&other).map_err(|e| e.to_string())?;
        }
        if result["assertions"].as_u64() != Some(spec.cases.len() as u64)
            || !result["passed"].is_boolean()
        {
            return Err("validation process did not return the required assertion outcomes".into());
        }
        Ok(result)
    };
    let new = evaluate(skill, "new")?;
    let previous = publication
        .skills()
        .map_err(|e| e.to_string())?
        .into_iter()
        .find(|s| s.name == skill.name);
    let old = if let Some(old) = previous {
        evaluate(&old, "old")?
    } else {
        Value::Null
    };
    let passed = new["passed"] == true && (!spec.require_regression || old["passed"] == false);
    Ok(ValidationReceipt {
        passed,
        revision: skill.revision.clone(),
        environment: publication.environment().into(),
        checks: spec.digest(),
        evidence: json!({"assertions":spec.cases.len(),"new":new,"old":old}),
        spec: Some(spec.clone()),
    })
}
