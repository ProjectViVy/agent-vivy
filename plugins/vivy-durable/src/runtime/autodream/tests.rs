use super::*;
use crate::runtime::tests::until;
use rig_core::test_utils::MockCompletionModel;
use rig_harness_test_support::{MemoryStore, app_on, calls, connect, first_agent, reply};
use std::path::PathBuf;

struct Fixture(PathBuf);
#[test]
fn malformed_saved_dream_cannot_be_restored_as_an_unowned_runtime() {
    let mut state = DurableAgentState::default();
    state.evolution.dream = true;
    assert!(serde_json::from_value::<DurableAgentState>(json!(state)).is_err());
}
impl Fixture {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "vivy-dream-{}-{}",
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_nanos()
        ));
        std::fs::create_dir_all(&path).unwrap();
        Self(path)
    }
}
impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}
fn app(disk: &MemoryStore, model: &MockCompletionModel, root: &Fixture) -> (App, Entity) {
    let (mut app, _) = app_on(disk);
    app.insert_resource(ProjectRoot(root.0.clone()));
    app.insert_resource(vivy_laputa::PersonaSource(root.0.join("laputa")));
    app.insert_resource(RigHome(rig_harness::harness_protocol::Home::at(
        root.0.join("home"),
    )));
    app.add_plugins((
        AgentPlugin,
        SessionLogPlugin,
        DurablePlugin,
        assistant::AssistantContextPlugin,
        vivy_laputa::LaputaPlugin,
    ));
    let connection = connect(model).unwrap();
    app.add_observer(move |e: On<Add<Agent>>, mut commands: Commands| {
        commands.entity(e.entity).insert(connection.clone());
    });
    app.finish();
    app.update();
    let agent = first_agent(app.world_mut()).unwrap();
    (app, agent)
}

#[test]
fn skill_job_waits_resumes_uses_published_code_and_collects_actual_feedback() {
    use vivy_python::managed::PublicationStore;
    let root = Fixture::new();
    let output = vivy_process::command("python")
        .args(["-I", "-c", "import sys; print(sys.executable)"])
        .output()
        .unwrap();
    let manifest = json!({"identity":"fixture-env","scope":"fixture","executable":String::from_utf8(output.stdout).unwrap().trim(),"skills":[],"packages":{},"imports":[]});
    let bytes = serde_json::to_vec(&manifest).unwrap();
    let base = vivy_basics::setup::fingerprint(&bytes);
    fs::create_dir_all(root.0.join(".vivy/workbench")).unwrap();
    fs::write(root.0.join(".vivy/workbench/current.json"), bytes).unwrap();
    vivy_basics::setup::save(
        &root.0.join("home"),
        &root.0,
        "python",
        &vivy_basics::setup::Decision {
            grant: Some("fixture".into()),
            environment: Some(base.clone()),
            ..Default::default()
        },
    )
    .unwrap();
    let draft = root.0.join("draft");
    fs::create_dir_all(draft.join("src/calculator")).unwrap();
    fs::write(
        draft.join("SKILL.md"),
        "---\nname: calculator\ndescription: synthetic calculator\n---\nUse calculator(value).\n",
    )
    .unwrap();
    fs::write(
        draft.join("pyproject.toml"),
        "[project]\nname='calculator'\nversion='0.1.0'\n",
    )
    .unwrap();
    fs::write(
        draft.join("src/calculator/__init__.py"),
        "import time\ndef run(value):\n time.sleep(0.3)\n return value+2\n",
    )
    .unwrap();
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([
        calls(
            "candidate",
            "candidate_submit",
            json!({"target":"skill","source":draft,"name":"calculator","expected_base":base,"checks":{"cases":[{"args":[3],"expected":5}]}}),
        ),
        calls("wait-job", "validation_wait", json!({"job":"job:1"})),
        calls(
            "finish",
            "task_control",
            json!({"action":"finish_silently","task":"task:1","generation":0}),
        ),
        calls(
            "use-skill",
            "python_repl",
            json!({"code":"print(calculator(3))"}),
        ),
        reply("Skill result observed"),
        reply("Synthetic recap"),
        reply("Feedback collected, no further change"),
    ]);
    let (mut app, _) = app_on(&disk);
    app.insert_resource(ProjectRoot(root.0.clone()))
        .insert_resource(vivy_laputa::PersonaSource(root.0.join("laputa")))
        .insert_resource(RigHome(rig_harness::harness_protocol::Home::at(
            root.0.join("home"),
        )));
    app.add_plugins((
        AgentPlugin,
        SessionLogPlugin,
        DurablePlugin,
        assistant::AssistantContextPlugin,
        vivy_laputa::LaputaPlugin,
        vivy_basics::skills::TrustedSkillsPlugin,
        vivy_python::PythonPlugin,
    ));
    let connection = connect(&model).unwrap();
    app.add_observer(move |e: On<Add<Agent>>, mut commands: Commands| {
        commands.entity(e.entity).insert(connection.clone());
    });
    app.finish();
    app.update();
    let main = first_agent(app.world_mut()).unwrap();
    request(app.world_mut(), main, "skill-run", "Build calculator").unwrap();
    until(&mut app, |w| {
        committed(w, main).is_some_and(|s| s.evolution.requests[0].finished)
    });
    let dream = dream_agent(app.world_mut(), main).unwrap();
    let job = committed(app.world(), dream).unwrap().evolution.jobs[0].clone();
    assert_eq!(job.phase, "published", "{job:#?}");
    let mut malformed = json!(committed(app.world(), dream).unwrap());
    malformed["evolution"]["jobs"][0]["task"] = json!("missing-owner");
    assert!(serde_json::from_value::<DurableAgentState>(malformed).is_err());
    assert!(
        job.delivered.is_some(),
        "slow checks must deliver a committed completion to the waiting owner: {job:#?}"
    );
    submit(app.world_mut(), main, "test", "use", "USE_SKILL").unwrap();
    until(&mut app, |w| {
        committed(w, main)
            .is_some_and(|s| s.tasks[0].status == Status::Completed && s.active.is_none())
    });
    let state = committed(app.world(), main).unwrap();
    let used = state
        .operations
        .iter()
        .find(|o| !o.usage.is_empty())
        .expect("actual callable usage missing");
    assert_eq!(
        used.usage[0]["revision"],
        job.skill.as_ref().unwrap().revision
    );
    assert_eq!(used.usage[0]["outcome"], "ok");
    assert_eq!(
        used.publication.as_deref(),
        Some(
            PublicationStore::new(&root.0, &root.0.join("home"))
                .current()
                .unwrap()
                .id
                .as_str()
        )
    );
    request(
        app.world_mut(),
        main,
        "feedback",
        "Review actual calculator use",
    )
    .unwrap();
    until(&mut app, |w| {
        committed(w, main).is_some_and(|s| s.evolution.requests[1].finished)
    });
    let evidence = &committed(app.world(), dream).unwrap().evolution.evidence;
    assert!(evidence.values().any(|e| {
        fs::read_to_string(&e.path)
            .unwrap()
            .contains(&job.skill.as_ref().unwrap().revision)
    }));
}

#[test]
fn explicit_dream_is_independent_persona_free_and_records_no_change() {
    let root = Fixture::new();
    vivy_laputa::PersonaStore::at(root.0.join("laputa"))
        .save_file(
            0,
            "",
            "initial",
            vivy_laputa::MISSION,
            "PERSONA_CANARY",
            vec![],
        )
        .unwrap();
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([
        reply("observed"),
        reply("Observed source recorded."),
        reply("No candidate is needed."),
    ]);
    let (mut app, main) = app(&disk, &model, &root);
    submit(app.world_mut(), main, "test", "one", "SOURCE_CANARY").unwrap();
    until(&mut app, |w| {
        committed(w, main)
            .is_some_and(|s| s.tasks[0].status == Status::Completed && s.active.is_none())
    });
    let id = request(
        app.world_mut(),
        main,
        "request-one",
        "Inspect the committed observation",
    )
    .unwrap();
    assert_eq!(
        request(
            app.world_mut(),
            main,
            "request-one",
            "Inspect the committed observation"
        )
        .unwrap(),
        id
    );
    until(&mut app, |w| {
        committed(w, main).is_some_and(|s| {
            s.evolution
                .requests
                .iter()
                .any(|r| r.id == id && r.finished)
        })
    });
    let requests = model.requests();
    assert!(
        serde_json::to_string(&requests[0])
            .unwrap()
            .contains("PERSONA_CANARY")
    );
    let dream_requests: Vec<_> = requests
        .iter()
        .map(|r| serde_json::to_string(r).unwrap())
        .filter(|r| r.contains("You are AutoDream"))
        .collect();
    assert_eq!(dream_requests.len(), 1);
    let dream = &dream_requests[0];
    assert!(!dream.contains("PERSONA_CANARY"), "{dream}");
    assert!(dream.contains("AutoDream"), "requests: {requests:#?}");
    let e = dream_agent(app.world_mut(), main).unwrap();
    assert!(app.world().get::<SpawnedBy>(e).is_none());
    let state = committed(app.world(), e).unwrap();
    assert!(!state.evolution.evidence.is_empty());
    assert!(state.evolution.windows[0].complete);
    assert!(model.request_count() <= 3);
}

#[test]
fn persona_candidate_is_adopted_and_completed_wait_returns_its_receipt() {
    let root = Fixture::new();
    vivy_laputa::PersonaStore::at(root.0.join("laputa"))
        .save_file(
            0,
            "",
            "initial",
            vivy_laputa::MISSION,
            "PERSONA_INITIAL",
            vec![],
        )
        .unwrap();
    let path = root.0.join("persona.txt");
    std::fs::write(&path, "NEW_PERSONA_CANARY").unwrap();
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([
        calls(
            "candidate",
            "candidate_submit",
            json!({"target":"persona","source":path,"expected_base":"1","evidence":[],"checks":{},"file":"IDENTITY.MD"}),
        ),
        calls("wait-job", "validation_wait", json!({"job":"job:1"})),
        calls(
            "finish",
            "task_control",
            json!({"action":"finish_silently","task":"task:1","generation":0}),
        ),
        reply("using the updated persona"),
        reply("summary"),
    ]);
    let (mut app, main) = app(&disk, &model, &root);
    request(
        app.world_mut(),
        main,
        "persona-run",
        "Apply the explicit persona candidate",
    )
    .unwrap();
    until(&mut app, |w| {
        committed(w, main).is_some_and(|s| s.evolution.requests[0].finished)
    });
    let dream = dream_agent(app.world_mut(), main).unwrap();
    let state = committed(app.world(), dream).unwrap();
    assert!(!state.evolution.jobs.is_empty(), "{state:#?}");
    assert_eq!(state.evolution.jobs[0].phase, "published");
    assert!(
        state.evolution.jobs[0].delivered.is_some()
            || state
                .operations
                .iter()
                .any(|o| o.request["function"]["name"] == "validation_wait"
                    && json!(o.outcome).to_string().contains("published"))
    );
    assert_eq!(
        vivy_laputa::PersonaStore::at(root.0.join("laputa"))
            .read()
            .unwrap()
            .file(vivy_laputa::IDENTITY),
        "NEW_PERSONA_CANARY"
    );
    submit(
        app.world_mut(),
        main,
        "test",
        "new-foreground",
        "hello after adoption",
    )
    .unwrap();
    until(&mut app, |w| {
        committed(w, main).is_some_and(|s| s.tasks[0].status == Status::Completed)
    });
    let requests = model.requests();
    let foreground = requests
        .iter()
        .map(|r| serde_json::to_string(r).unwrap())
        .find(|r| r.contains("hello after adoption"))
        .unwrap();
    assert!(foreground.contains("<persona>"));
    assert!(foreground.contains("NEW_PERSONA_CANARY"));
    for r in requests
        .iter()
        .map(|r| serde_json::to_string(r).unwrap())
        .filter(|r| r.contains("You are AutoDream"))
    {
        assert!(!r.contains("<persona>"));
    }
}

#[test]
fn persona_candidate_cannot_target_a_human_only_file() {
    let root = Fixture::new();
    let path = root.0.join("persona.txt");
    std::fs::write(&path, "PROTECTED_CANARY").unwrap();
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([
        calls(
            "candidate",
            "candidate_submit",
            json!({"target":"persona","source":path,"expected_base":"0","evidence":[],"checks":{},"file":"MISSION.MD"}),
        ),
        calls(
            "finish",
            "task_control",
            json!({"action":"finish_silently","task":"task:1","generation":0}),
        ),
        reply("foreground after refusal"),
        reply("summary"),
    ]);
    let (mut app, main) = app(&disk, &model, &root);
    request(
        app.world_mut(),
        main,
        "persona-protected",
        "Try to evolve MISSION",
    )
    .unwrap();
    until(&mut app, |w| {
        committed(w, main).is_some_and(|s| s.evolution.requests[0].finished)
    });
    let dream = dream_agent(app.world_mut(), main).unwrap();
    let state = committed(app.world(), dream).unwrap();
    assert!(state.evolution.jobs.is_empty(), "{state:#?}");
    assert!(
        state.operations.iter().any(|o| json!(o.outcome)
            .to_string()
            .contains("may be evolved automatically")),
        "{state:#?}"
    );
    assert_eq!(
        vivy_laputa::PersonaStore::at(root.0.join("laputa"))
            .read()
            .unwrap()
            .revision,
        0
    );
}

#[test]
fn legacy_persona_job_without_a_target_file_cannot_publish() {
    let job = jobs::Job {
        id: "job:legacy".into(),
        operation: "operation".into(),
        task: "task:1".into(),
        generation: 0,
        target: "persona".into(),
        expected_base: "0".into(),
        evidence: vec![],
        skill: None,
        persona: Some("CANARY".into()),
        file: Some("IDENTITY.MD".into()),
        file_digest: Some("digest".into()),
        environment: None,
        checks: vivy_python::managed::ValidationSpec::default(),
        phase: "publishing".into(),
        attempt: 0,
        unknown_attempts: vec![],
        result: None,
        adoption: None,
        error: None,
        notice_step: None,
        ready: false,
        delivered: None,
        skipped: false,
        update: "update".into(),
    };
    assert!(jobs::persona_target(&job).is_ok());
    let mut value = serde_json::to_value(&job).unwrap();
    let object = value.as_object_mut().unwrap();
    object.remove("file");
    object.remove("file_digest");
    let restored: jobs::Job = serde_json::from_value(value).unwrap();
    assert!(restored.file.is_none());
    let error = jobs::persona_target(&restored).unwrap_err();
    assert!(
        error
            .to_string()
            .contains("legacy persona candidate has no target file"),
        "{error}"
    );
}

#[test]
fn persona_candidates_require_an_initialized_persona() {
    let root = Fixture::new();
    let path = root.0.join("persona.txt");
    std::fs::write(&path, "UNINITIALIZED_CANARY").unwrap();
    let disk = MemoryStore::default();
    let model = MockCompletionModel::from_stream_turns([
        calls(
            "candidate",
            "candidate_submit",
            json!({"target":"persona","source":path,"expected_base":"0","evidence":[],"checks":{},"file":"IDENTITY.MD"}),
        ),
        calls(
            "finish",
            "task_control",
            json!({"action":"finish_silently","task":"task:1","generation":0}),
        ),
        reply("foreground after refusal"),
        reply("summary"),
    ]);
    let (mut app, main) = app(&disk, &model, &root);
    request(
        app.world_mut(),
        main,
        "persona-uninitialized",
        "Try to evolve an absent persona",
    )
    .unwrap();
    until(&mut app, |w| {
        committed(w, main).is_some_and(|s| s.evolution.requests[0].finished)
    });
    let dream = dream_agent(app.world_mut(), main).unwrap();
    let state = committed(app.world(), dream).unwrap();
    assert!(state.evolution.jobs.is_empty(), "{state:#?}");
    assert!(
        state
            .operations
            .iter()
            .any(|o| json!(o.outcome).to_string().contains("initialized")),
        "{state:#?}"
    );
}
