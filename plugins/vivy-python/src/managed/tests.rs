use super::*;

struct Fixture {
    root: PathBuf,
    home: PathBuf,
}
impl Fixture {
    fn manifest(&self, value: Value) -> Publication {
        let bytes = serde_json::to_vec(&value).unwrap();
        fs::write(self.root.join(".vivy/workbench/current.json"), &bytes).unwrap();
        let mut decision = vivy_basics::setup::read(&self.home, &self.root, "python").unwrap();
        decision.environment = Some(hash(&bytes));
        vivy_basics::setup::save(&self.home, &self.root, "python", &decision).unwrap();
        self.store().current().unwrap()
    }
    fn new() -> Self {
        let root = std::env::temp_dir().join(format!(
            "vivy-d4-{}-{}",
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_nanos()
        ));
        let home = root.join("user");
        fs::create_dir_all(root.join(".vivy/workbench")).unwrap();
        let manifest = json!({"identity":"env-1","executable":std::env::current_exe().unwrap(),"scope":"fixture","skills":[],"packages":{},"imports":[]});
        let bytes = serde_json::to_vec(&manifest).unwrap();
        fs::write(root.join(".vivy/workbench/current.json"), &bytes).unwrap();
        let decision = vivy_basics::setup::Decision {
            grant: Some("fixture".into()),
            environment: Some(hash(&bytes)),
            ..Default::default()
        };
        vivy_basics::setup::save(&home, &root, "python", &decision).unwrap();
        Self { root, home }
    }
    fn store(&self) -> PublicationStore {
        PublicationStore::new(&self.root, &self.home)
    }
    fn python(&self) {
        let output = vivy_process::command("python")
            .args(["-I", "-c", "import sys; print(sys.executable)"])
            .output()
            .unwrap();
        let mut value = self.store().current().unwrap().manifest;
        value["executable"] = json!(String::from_utf8(output.stdout).unwrap().trim());
        let bytes = serde_json::to_vec(&value).unwrap();
        fs::write(self.store().root().join("current.json"), &bytes).unwrap();
        let mut decision = vivy_basics::setup::read(&self.home, &self.root, "python").unwrap();
        decision.environment = Some(hash(&bytes));
        vivy_basics::setup::save(&self.home, &self.root, "python", &decision).unwrap();
    }
    fn draft(&self, value: u8) -> PathBuf {
        let path = self.root.join("draft");
        fs::create_dir_all(path.join("src/calculator")).unwrap();
        fs::write(
            path.join("SKILL.md"),
            "---\nname: calculator\ndescription: fixture calculator\n---\nCall calculator(1).\n",
        )
        .unwrap();
        fs::write(
            path.join("pyproject.toml"),
            "[project]\nname='calculator'\nversion='0.1.0'\n",
        )
        .unwrap();
        fs::write(
            path.join("src/calculator/__init__.py"),
            format!("def run(value): return value+{value}\n"),
        )
        .unwrap();
        path
    }
}

#[test]
fn validation_rejects_an_unmanaged_binding_conflict_before_activation() {
    let f = Fixture::new();
    f.python();
    let store = f.store();
    let ordinary = f.root.join("ordinary");
    fs::create_dir_all(&ordinary).unwrap();
    fs::write(ordinary.join("SKILL.md"), "Ordinary math instructions").unwrap();
    fs::write(
        ordinary.join("pyproject.toml"),
        "[project]\nname='math'\nversion='1.0'\n[tool.vivy.skill]\nbinding='calculator'\n",
    )
    .unwrap();
    let mut manifest = store.current().unwrap().manifest;
    manifest["skills"] = json!([{"name":"different-display-name","description":"ordinary math","file":ordinary.join("SKILL.md")}]);
    let base = f.manifest(manifest);
    let skill = store.freeze("calculator", &f.draft(1), 1_000_000).unwrap();
    let spec: ValidationSpec =
        serde_json::from_value(json!({"cases":[{"args":[3],"expected":4}]})).unwrap();
    let result = validation::run(
        &store,
        &base,
        &skill,
        &spec,
        &f.root.join("conflict-check"),
        &crate::Cancellation::default(),
    )
    .unwrap();
    assert!(
        !result.passed,
        "isolated candidate checks missed a real worker binding conflict"
    );
    assert_eq!(store.current().unwrap().id, base.id);
}

#[test]
fn a_pinned_worker_uses_the_memory_scope_of_its_validated_manifest() {
    use crate::{PythonArgs, PythonTool};
    use rig_harness::rig_core::tool::{Tool, ToolContext};
    let f = Fixture::new();
    f.python();
    let mut manifest = f.store().current().unwrap().manifest;
    let old = f.root.join("old-palace");
    let new = f.root.join("unadmitted-palace");
    manifest["memory_ready"] = json!(true);
    manifest["memory_root"] = json!(old);
    let selected = f.manifest(manifest);
    let mut tool = PythonTool::for_directory(f.root.clone());
    tool.settings.selected = Some(selected);
    tool.settings.user_home = f.home.clone();
    tool.settings.memory = true;
    tool.settings.memory_root = Some(new);
    let code = format!(
        "import os\nassert os.environ.get('VIVY_WORKBENCH_MEMORY_ROOT')=={}, os.environ.get('VIVY_WORKBENCH_MEMORY_ROOT')\nprint('PINNED_MEMORY_ROOT')",
        json!(old)
    );
    let result = futures::executor::block_on(tool.call(
        &mut ToolContext::default(),
        PythonArgs {
            code,
            timeout_secs: None,
        },
    ))
    .unwrap();
    assert!(result.contains("PINNED_MEMORY_ROOT"), "{result}");
    tool.state.retire();
}

#[test]
fn setup_scope_expansion_retains_the_initial_publication_before_changing_grant() {
    let f = Fixture::new();
    let store = f.store();
    let base = store.current().unwrap();
    let mut world = rig_harness::prelude::World::new();
    world.insert_resource(rig_harness::prelude::ProjectRoot(f.root.clone()));
    world.insert_resource(rig_harness::prelude::RigHome(
        rig_harness::harness_protocol::Home::at(f.home.clone()),
    ));
    crate::management::initialization::retain_before_approval(&world).unwrap();
    let mut decision = vivy_basics::setup::read(&f.home, &f.root, "python").unwrap();
    decision.grant = Some("expanded".into());
    vivy_basics::setup::save(&f.home, &f.root, "python", &decision).unwrap();
    assert_eq!(store.selected(&base.id).unwrap(), base);
    assert_eq!(store.retained(&base.id).unwrap(), base);
}

#[test]
fn validation_exercises_same_assertions_on_old_and_new_code() {
    let f = Fixture::new();
    f.python();
    let store = f.store();
    let base = store.current().unwrap();
    let old = store
        .freeze("calculator", &f.draft(1), 256 * 1024 * 1024)
        .unwrap();
    let old_publication = store
        .publish(
            &base.id,
            &old,
            &ValidationReceipt::passed(&old, &base, "bootstrap", json!({"assertions":1})),
            "bootstrap",
        )
        .unwrap();
    let candidate = store
        .freeze("calculator", &f.draft(2), 256 * 1024 * 1024)
        .unwrap();
    let spec: ValidationSpec = serde_json::from_value(
        json!({"cases":[{"args":[3],"expected":5}],"require_regression":true,"replay_safe":true}),
    )
    .unwrap();
    let job = ValidationJob::start(
        store.clone(),
        old_publication.clone(),
        candidate.clone(),
        spec.clone(),
        f.root.join("scratch"),
        None,
    )
    .unwrap();
    let receipt = loop {
        if let Some(result) = job.poll() {
            break result.unwrap();
        }
        std::thread::sleep(std::time::Duration::from_millis(10));
    };
    assert!(receipt.passed, "{:?}", receipt.evidence);
    assert_eq!(receipt.evidence["old"]["passed"], false);
    assert_eq!(receipt.evidence["new"]["passed"], true);
    let mut failing = spec;
    failing.cases[0].expected = json!(99);
    let job = ValidationJob::start(
        store,
        old_publication,
        candidate,
        failing,
        f.root.join("scratch-failed"),
        None,
    )
    .unwrap();
    let receipt = loop {
        if let Some(result) = job.poll() {
            break result.unwrap();
        }
        std::thread::sleep(std::time::Duration::from_millis(10));
    };
    assert!(!receipt.passed);
}
impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.root);
    }
}

#[test]
fn staged_dependencies_require_local_consent_and_do_not_activate() {
    let f = Fixture::new();
    let store = f.store();
    let base = store.current().unwrap();
    store.retain_current().unwrap();
    let mut manifest = base.manifest.clone();
    manifest["identity"] = json!("environment-two");
    manifest["scope"] = json!("expanded");
    manifest["managed_requirements"] = json!(["example==1"]);
    assert!(store.stage(&base.id, manifest.clone()).is_err());
    let mut decision = vivy_basics::setup::read(&f.home, &f.root, "python").unwrap();
    decision.grant = Some("expanded".into());
    vivy_basics::setup::save(&f.home, &f.root, "python", &decision).unwrap();
    let staged = store.stage(&base.id, manifest).unwrap();
    assert_eq!(store.current().unwrap().id, base.id);
    assert!(store.staged("expanded").unwrap().is_none());
    store.ready_stage(&staged).unwrap();
    assert_eq!(store.staged("expanded").unwrap().unwrap().id, staged.id);
    let draft = f.draft(1);
    fs::write(
        draft.join("pyproject.toml"),
        "[project]\nname='calculator'\nversion='0.1.0'\ndependencies=['example==1']\n",
    )
    .unwrap();
    let skill = store.freeze("calculator", &draft, 1_000_000).unwrap();
    let receipt = ValidationReceipt::passed(&skill, &staged, "fixture", json!({"assertions":1}));
    assert!(
        store
            .publish(&base.id, &skill, &receipt, "wrong-environment")
            .is_err()
    );
    let next = store
        .publish_in(&base.id, &staged, &skill, &receipt, "expanded-candidate")
        .unwrap();
    assert_eq!(next.environment(), "environment-two");
    assert_eq!(store.selected(&base.id).unwrap().environment(), "env-1");
}

#[test]
fn publication_freezes_sources_reconciles_ancestry_and_rolls_back() {
    let f = Fixture::new();
    let store = f.store();
    let base = store.current().unwrap();
    let first = store
        .freeze("calculator", &f.draft(1), 256 * 1024 * 1024)
        .unwrap();
    let validation = ValidationReceipt::passed(&first, &base, "checks-1", json!({"assertions":1}));
    let next = store
        .publish(&base.id, &first, &validation, "update-1")
        .unwrap();
    f.draft(2);
    assert!(
        fs::read_to_string(
            Path::new(&first.file)
                .parent()
                .unwrap()
                .join("src/calculator/__init__.py")
        )
        .unwrap()
        .contains("value+1")
    );
    let second = store
        .freeze("calculator", &f.draft(2), 256 * 1024 * 1024)
        .unwrap();
    let v2 = ValidationReceipt::passed(&second, &next, "checks-2", json!({"assertions":1}));
    let newer = store.publish(&next.id, &second, &v2, "update-2").unwrap();
    assert_eq!(
        store
            .publish(&base.id, &first, &validation, "update-1")
            .unwrap()
            .id,
        next.id
    );
    assert_eq!(store.current().unwrap().id, newer.id);
    let back = store.rollback(&newer.id, &next.id, "rollback-1").unwrap();
    assert_eq!(back.skills().unwrap()[0].revision, first.revision);
    assert_ne!(back.id, next.id);
}

#[test]
fn failed_stale_or_tampered_candidates_cannot_activate() {
    let f = Fixture::new();
    let store = f.store();
    let base = store.current().unwrap();
    let skill = store
        .freeze("calculator", &f.draft(1), 256 * 1024 * 1024)
        .unwrap();
    let mut validation =
        ValidationReceipt::passed(&skill, &base, "checks", json!({"assertions":1}));
    validation.passed = false;
    assert!(store.publish(&base.id, &skill, &validation, "bad").is_err());
    validation.passed = true;
    assert!(store.publish("stale", &skill, &validation, "bad").is_err());
    fs::write(
        Path::new(&skill.file)
            .parent()
            .unwrap()
            .join("src/calculator/__init__.py"),
        "raise Exception('tampered')",
    )
    .unwrap();
    assert!(store.publish(&base.id, &skill, &validation, "bad").is_err());
    assert_eq!(store.current().unwrap().id, base.id);
}

#[test]
fn published_history_alone_cannot_grant_local_interpreter_trust() {
    let f = Fixture::new();
    let store = f.store();
    let base = store.current().unwrap();
    let skill = store
        .freeze("calculator", &f.draft(1), 256 * 1024 * 1024)
        .unwrap();
    let check = ValidationReceipt::passed(&skill, &base, "checks", json!({"assertions":1}));
    store.publish(&base.id, &skill, &check, "update").unwrap();
    let stranger = PublicationStore::new(&f.root, &f.root.join("another-user"));
    assert!(stranger.current().is_err());
}

#[test]
fn shared_lock_and_staging_budget_prevent_partial_publication() {
    let f = Fixture::new();
    let store = f.store();
    let base = store.current().unwrap();
    assert!(store.freeze("calculator", &f.draft(1), 1).is_err());
    let skill = store
        .freeze("calculator", &f.draft(1), 256 * 1024 * 1024)
        .unwrap();
    let check = ValidationReceipt::passed(&skill, &base, "checks", json!({"assertions":1}));
    let _lock = store.lock().unwrap();
    assert!(store.publish(&base.id, &skill, &check, "update").is_err());
    assert_eq!(store.current().unwrap().id, base.id);
}

#[test]
fn calls_pin_code_even_when_the_interpreter_path_does_not_change() {
    use crate::{PythonArgs, PythonTool};
    use rig_harness::rig_core::tool::{Tool, ToolContext};
    let f = Fixture::new();
    let store = f.store();
    let output = vivy_process::command("python")
        .args(["-I", "-c", "import sys; print(sys.executable)"])
        .output()
        .unwrap();
    let mut value = store.current().unwrap().manifest;
    value["executable"] = json!(String::from_utf8(output.stdout).unwrap().trim());
    let bytes = serde_json::to_vec(&value).unwrap();
    fs::write(store.root().join("current.json"), &bytes).unwrap();
    let mut decision = vivy_basics::setup::read(&f.home, &f.root, "python").unwrap();
    decision.environment = Some(hash(&bytes));
    vivy_basics::setup::save(&f.home, &f.root, "python", &decision).unwrap();
    let base = store.current().unwrap();
    let v1 = store
        .freeze("calculator", &f.draft(1), 256 * 1024 * 1024)
        .unwrap();
    let p1 = store
        .publish(
            &base.id,
            &v1,
            &ValidationReceipt::passed(&v1, &base, "checks", json!({"assertions":1})),
            "one",
        )
        .unwrap();
    let v2 = store
        .freeze("calculator", &f.draft(2), 256 * 1024 * 1024)
        .unwrap();
    let p2 = store
        .publish(
            &p1.id,
            &v2,
            &ValidationReceipt::passed(&v2, &p1, "checks", json!({"assertions":1})),
            "two",
        )
        .unwrap();
    assert_eq!(p1.executable().unwrap(), p2.executable().unwrap());
    let mut tool = PythonTool::for_directory(f.root.clone());
    tool.settings.selected = Some(p1.clone());
    tool.settings.user_home = f.home.clone();
    let run = |tool: &PythonTool, code: &str| {
        futures::executor::block_on(tool.call(
            &mut ToolContext::default(),
            PythonArgs {
                code: code.into(),
                timeout_secs: None,
            },
        ))
        .unwrap()
    };
    assert!(run(&tool, "assert calculator(3)==4; old=1; print('OLD')").contains("OLD"));
    assert_eq!(
        tool.settings.usage.lock().unwrap()[0]["revision"],
        v1.revision
    );
    tool.settings.selected = Some(p2);
    assert!(
        run(
            &tool,
            "assert 'old' not in globals(); assert calculator(3)==5; print('NEW')"
        )
        .contains("NEW")
    );
    tool.settings.selected = Some(p1);
    assert!(run(&tool, "assert calculator(3)==4; print('RETRY_OLD')").contains("RETRY_OLD"));
    tool.state.retire();
}

#[test]
fn an_operation_that_selected_no_environment_does_not_adopt_a_later_one() {
    use crate::{PythonArgs, PythonTool};
    use rig_harness::rig_core::tool::{Tool, ToolContext};
    let f = Fixture::new();
    f.python();
    let mut tool = PythonTool::for_directory(f.root.clone());
    tool.settings.pinned = true;
    let result = futures::executor::block_on(tool.call(
        &mut ToolContext::default(),
        PythonArgs {
            code: "print('must not start')".into(),
            timeout_secs: None,
        },
    ));
    assert!(
        result
            .unwrap_err()
            .to_string()
            .contains("No workbench publication was selected")
    );
}

#[test]
fn python_preparer_and_rust_publisher_share_the_native_lock() {
    let f = Fixture::new();
    f.python();
    let store = f.store();
    let runtime = crate::worker::package(&f.root.join("lock-probe")).unwrap();
    let code = format!(
        "import sys;sys.path.insert(0,{});from vivy_workbench.prepare import environment_lock\ntry:\n with environment_lock({}): raise AssertionError('overlapping locks')\nexcept RuntimeError as e:\n assert 'busy' in str(e)\n",
        json!(runtime),
        json!(store.root())
    );
    let lock = store.lock().unwrap();
    let out = vivy_process::command("python")
        .args(["-I", "-c", &code])
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    drop(lock);
    let code = format!(
        "import sys,time;sys.path.insert(0,{});from vivy_workbench.prepare import environment_lock\nwith environment_lock({}):\n print('locked',flush=True)\n time.sleep(30)\n",
        json!(runtime),
        json!(store.root())
    );
    let mut command = vivy_process::command("python");
    command
        .args(["-I", "-u", "-c", &code])
        .stdout(std::process::Stdio::piped());
    let mut child = vivy_process::OwnedChild::spawn(&mut command).unwrap();
    use std::io::BufRead;
    let mut ready = String::new();
    std::io::BufReader::new(child.child.stdout.take().unwrap())
        .read_line(&mut ready)
        .unwrap();
    assert!(ready.contains("locked"));
    assert!(store.lock().is_err());
    child.control().kill();
    child.child.wait().unwrap();
    assert!(store.lock().is_ok());
}

#[test]
fn validation_scratch_quota_stops_an_owned_process() {
    let f = Fixture::new();
    f.python();
    let store = f.store();
    let base = store.current().unwrap();
    let draft = f.draft(1);
    fs::write(draft.join("src/calculator/__init__.py"),"from pathlib import Path\nimport time\ndef run(value):\n Path('oversized').write_bytes(b'x'*600000)\n time.sleep(30)\n return value+1\n").unwrap();
    let skill = store.freeze("calculator", &draft, 1_000_000).unwrap();
    let spec: ValidationSpec = serde_json::from_value(
        json!({"cases":[{"args":[3],"expected":4}],"output_budget_bytes":400000}),
    )
    .unwrap();
    let job = ValidationJob::start(store, base, skill, spec, f.root.join("bounded"), None).unwrap();
    let start = std::time::Instant::now();
    let result = loop {
        if let Some(result) = job.poll() {
            break result;
        }
        assert!(start.elapsed().as_secs() < 5);
        std::thread::sleep(std::time::Duration::from_millis(10));
    };
    assert!(result.unwrap_err().contains("budget"));
}

#[test]
fn manual_preparation_preserves_sources_and_revalidates_retained_callable_checks() {
    let f = Fixture::new();
    f.python();
    let store = f.store();
    let base = store.current().unwrap();
    let skill = store.freeze("calculator", &f.draft(1), 1_000_000).unwrap();
    let checks: ValidationSpec =
        serde_json::from_value(json!({"cases":[{"args":[3],"expected":4}]})).unwrap();
    let receipt = validation::run(
        &store,
        &base,
        &skill,
        &checks,
        &f.root.join("first-check"),
        &crate::Cancellation::default(),
    )
    .unwrap();
    let active = store
        .publish(&base.id, &skill, &receipt, "initial")
        .unwrap();
    let mut changed = active.manifest.clone();
    changed["identity"] = json!("rebuilt-environment");
    preparation::finish(
        &store,
        &active,
        changed,
        false,
        &crate::Cancellation::default(),
    )
    .unwrap();
    let next = store.current().unwrap();
    assert_eq!(next.skills().unwrap(), vec![skill]);
    assert_ne!(next.id, active.id);
    assert_eq!(next.manifest["managed_parent"], active.id);
    assert_eq!(
        next.manifest["managed_validations"]["calculator"]["environment"],
        "rebuilt-environment"
    );
}
