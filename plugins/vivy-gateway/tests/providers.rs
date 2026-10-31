#![cfg(all(feature = "openai", feature = "anthropic"))]
use rig_harness::prelude::*;
use vivy_gateway::store::SettingsFile;
use vivy_gateway::{
    ProvidersPlugin,
    config::{Credential, ProvidersConfig, parse},
};

const CONFIG: &str = r#"
[providers]
default_model = "company/org/model-1"
[providers.instances.company]
protocol = "openai"
base_url = "http://127.0.0.1:9/v1"
credential = { source = "none" }
[[providers.instances.company.models]]
id = "org/model-1"
context_window = 4000
max_output_tokens = 500
image_input = true
[providers.instances.messages]
protocol = "anthropic"
base_url = "http://127.0.0.1:9"
credential = { source = "none" }
[[providers.instances.messages.models]]
id = "org/model-1"
"#;

fn isolate(app: &mut App) {
    use std::sync::atomic::{AtomicU64, Ordering};
    static NEXT: AtomicU64 = AtomicU64::new(0);
    app.insert_resource(rig_harness::services::RigHome(
        rig_harness::harness_protocol::Home::at(std::env::temp_dir().join(format!(
            "vivy-providers-memory-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ))),
    ));
}

#[test]
fn parses_two_instances_and_rejects_protocols_and_unsafe_ids() {
    let config = parse(CONFIG).unwrap();
    assert_eq!(config.instances.len(), 2);
    assert_eq!(config.default_model.as_deref(), Some("company/org/model-1"));
    assert_eq!(config.instances["company"].credential, Credential::None);
    assert!(parse(&CONFIG.replace("protocol = \"openai\"", "protocol = \"gemini\"")).is_err());
    assert!(parse(&CONFIG.replace("instances.company", "instances.\"../escape\"")).is_err());
    assert!(parse(&CONFIG.replace("context_window = 4000", "context_window = 0")).is_err());
}

#[test]
fn registers_only_configured_instances_and_preserves_model_ids() {
    let mut app = App::new();
    isolate(&mut app);
    app.insert_resource(Settings::from_toml(CONFIG));
    app.add_plugins(ProvidersPlugin);
    let models = app.world().resource::<Models>();
    for instance in ["company", "messages"] {
        let connection = models.connect(&format!("{instance}/org/model-1")).unwrap();
        assert_eq!(connection.spec.id, "org/model-1");
        assert_eq!(connection.spec.provider.vendor(), instance);
    }
    let row = models
        .0
        .catalog()
        .resolve("company/org/model-1")
        .unwrap()
        .shared();
    assert_eq!(
        (row.context_window, row.max_output_tokens),
        (Some(4000), Some(500))
    );
    assert!(row.input.image && row.tools);
    assert!(models.connect("deepseek/deepseek-chat").is_err());
    assert!(models.connect("gcp.gemini/anything").is_err());
}

#[test]
fn explicit_missing_environment_credential_never_uses_another_source() {
    let text = CONFIG.replace(
        "credential = { source = \"none\" }",
        "credential = { source = \"env\", name = \"VIVY_TEST_MISSING_PROVIDER_KEY_739\" }",
    );
    let mut app = App::new();
    app.insert_resource(Settings::from_toml(&text));
    isolate(&mut app);
    app.add_plugins(ProvidersPlugin);
    let why = app
        .world()
        .resource::<Models>()
        .connect("company/org/model-1")
        .err()
        .unwrap();
    assert!(why.contains("VIVY_TEST_MISSING_PROVIDER_KEY_739"), "{why}");
}

#[test]
fn legacy_gateway_migrates_without_overriding_an_explicit_instance() {
    let legacy = "[gateway]\nbase_url = \"http://127.0.0.1:9/v1\"\nmodel = \"org/legacy\"\nmodels = [\"org/other\"]\ncontext_window = 9000\n";
    let config = parse(legacy).unwrap();
    assert_eq!(config.default_model.as_deref(), Some("gateway/org/legacy"));
    assert_eq!(config.instances["gateway"].models.len(), 2);
    assert_eq!(
        config.instances["gateway"].models[0].context_window,
        Some(9000)
    );
    let merged = parse(&format!(
        "{legacy}\n{}",
        CONFIG.replace("instances.company", "instances.gateway")
    ))
    .unwrap();
    assert_eq!(merged.instances["gateway"].models[0].id, "org/model-1");
}

fn scratch() -> std::path::PathBuf {
    use std::sync::atomic::{AtomicU64, Ordering};
    static N: AtomicU64 = AtomicU64::new(0);
    let path = std::env::temp_dir().join(format!(
        "vivy-providers-{}-{}",
        std::process::id(),
        N.fetch_add(1, Ordering::Relaxed)
    ));
    std::fs::create_dir_all(&path).unwrap();
    path
}

#[test]
fn atomic_settings_keep_unrelated_sections_and_refuse_external_edits() {
    let dir = scratch();
    let path = dir.join("settings.toml");
    let original =
        format!("# keep this comment\n[unrelated]\nanswer = 42 # keep this too\n{CONFIG}");
    std::fs::write(&path, &original).unwrap();
    let mut file = SettingsFile::open(path.clone()).unwrap();
    let mut config = parse(&original).unwrap();
    config.instances.get_mut("company").unwrap().name = Some("Company account".into());
    file.save(&config).unwrap();
    let text = std::fs::read_to_string(&path).unwrap();
    assert!(
        text.contains("[providers.instances.company]"),
        "provider instances should remain editable TOML tables"
    );
    assert!(text.contains("# keep this comment") && text.contains("42 # keep this too"));
    assert_eq!(
        parse(&text).unwrap().instances["company"].name.as_deref(),
        Some("Company account")
    );
    std::fs::write(&path, format!("{text}\n# outside edit\n")).unwrap();
    assert!(file.save(&config).unwrap_err().contains("changed"));
    assert!(
        std::fs::read_to_string(&path)
            .unwrap()
            .contains("# outside edit")
    );
    std::fs::remove_dir_all(dir).unwrap();
}

#[test]
fn an_unrelated_setting_error_does_not_disable_valid_providers() {
    #[derive(Default, serde::Deserialize, bevy_ecs::prelude::Resource)]
    #[allow(dead_code)]
    struct Other {
        answer: u32,
    }
    let mut app = App::new();
    app.insert_resource(Settings::from_toml(&format!(
        "{CONFIG}\n[unrelated]\nanswer='bad'\n"
    )));
    app.add_setting::<Other>("unrelated");
    isolate(&mut app);
    app.add_plugins(ProvidersPlugin);
    assert!(
        app.world()
            .resource::<Models>()
            .connect("company/org/model-1")
            .is_ok()
    );
}

#[test]
fn bootstrap_import_is_initial_only_and_can_be_persisted_without_the_key() {
    let mut config = parse("").unwrap();
    assert!(config.bootstrap("http://127.0.0.1:9/v1", "org/first", None));
    assert_eq!(config.default_model.as_deref(), Some("gateway/org/first"));
    assert!(!config.bootstrap(
        "http://127.0.0.1:8/v1",
        "org/replacement",
        Some("deepseek".into())
    ));
    assert_eq!(config.instances["gateway"].models[0].id, "org/first");
    assert_eq!(
        config.instances["gateway"].credential,
        Credential::Env {
            name: "RIG_SECRET_GATEWAY".into()
        }
    );
}

#[test]
fn reload_preserves_valid_state_and_refreshes_idle_children_before_active_parent() {
    use rig_harness::{harness_protocol::Home, rig_ecs::agent::TurnOf, services::RigHome};
    let dir = scratch();
    std::fs::write(dir.join("settings.toml"), CONFIG).unwrap();
    let mut app = App::new();
    app.add_message::<Notice>();
    app.insert_resource(RigHome(Home::at(&dir)));
    app.insert_resource(Settings::from_toml(CONFIG));
    app.add_plugins(ProvidersPlugin);
    let connected = app
        .world()
        .resource::<Models>()
        .connect("company/org/model-1")
        .unwrap();
    let parent = app
        .world_mut()
        .spawn((
            Agent,
            ModelChoice("company/org/model-1".into()),
            connected.clone(),
        ))
        .id();
    let child = app
        .world_mut()
        .spawn((
            Agent,
            ModelChoice("company/org/model-1".into()),
            connected,
            SpawnedBy(parent),
        ))
        .id();
    let turn = app.world_mut().spawn(TurnOf(parent)).id();
    assert!(app.world().get::<ActiveTurn>(parent).is_some());
    let mut config = vivy_gateway::config::parse(CONFIG).unwrap();
    config.instances.get_mut("company").unwrap().models[0].context_window = Some(8000);
    vivy_gateway::save_config(app.world_mut(), config.clone()).unwrap();
    assert_eq!(
        app.world()
            .get::<Connection>(parent)
            .unwrap()
            .spec
            .context_window,
        Some(4000)
    );
    assert_eq!(
        app.world()
            .get::<Connection>(child)
            .unwrap()
            .spec
            .context_window,
        Some(8000)
    );
    app.world_mut().despawn(turn);
    app.update();
    assert_eq!(
        app.world()
            .get::<Connection>(parent)
            .unwrap()
            .spec
            .context_window,
        Some(8000)
    );
    std::fs::write(dir.join("settings.toml"), "[providers broken").unwrap();
    assert!(vivy_gateway::reload(app.world_mut()).is_err());
    assert_eq!(
        app.world()
            .resource::<vivy_gateway::ProvidersState>()
            .config,
        config
    );
    assert_eq!(
        app.world()
            .get::<Connection>(parent)
            .unwrap()
            .spec
            .context_window,
        Some(8000)
    );
    let mut file = SettingsFile::open(dir.join("other.toml")).unwrap();
    file.save(&config).unwrap();
    std::fs::copy(dir.join("other.toml"), dir.join("settings.toml")).unwrap();
    vivy_gateway::reload(app.world_mut()).unwrap();
    config.instances.remove("company");
    vivy_gateway::save_config(app.world_mut(), config).unwrap();
    assert!(app.world().get::<Connection>(parent).is_none());
    assert_eq!(
        app.world().get::<ModelChoice>(parent).unwrap().0,
        "company/org/model-1"
    );
    std::fs::remove_dir_all(dir).unwrap();
}

#[test]
fn malformed_endpoint_is_rejected_before_a_request_or_save() {
    assert!(parse(&CONFIG.replace("http://127.0.0.1:9/v1", "http://127.0.0.1:bad/v1")).is_err());
    assert!(
        parse(&CONFIG.replace(
            "http://127.0.0.1:9/v1",
            "https://user:secret@example.com/v1"
        ))
        .is_err()
    );
}

#[test]
fn failed_login_does_not_overwrite_a_previous_key_on_external_settings_conflict() {
    use rig_harness::{harness_protocol::Home, services::RigHome};
    let dir = scratch();
    std::fs::write(dir.join("settings.toml"), CONFIG).unwrap();
    let mut app = App::new();
    app.insert_resource(RigHome(Home::at(&dir)));
    app.insert_resource(Settings::from_toml(CONFIG));
    app.add_plugins(ProvidersPlugin);
    vivy_gateway::save_key(app.world_mut(), "company", "previous-fixture").unwrap();
    let key_path = dir.join("auth/provider-keys/company.json");
    let previous = std::fs::read(&key_path).unwrap();
    let path = dir.join("settings.toml");
    let external = format!(
        "{}\n# external edit\n",
        std::fs::read_to_string(&path).unwrap()
    );
    std::fs::write(&path, external).unwrap();
    assert!(vivy_gateway::save_key(app.world_mut(), "company", "new-fixture").is_err());
    assert_eq!(std::fs::read(&key_path).unwrap(), previous);
    std::fs::remove_dir_all(dir).unwrap();
}

#[test]
fn generic_openai_requires_an_explicit_mapping_for_reasoning_metadata() {
    let text = CONFIG.replace(
        "image_input = true",
        "image_input = true\nreasoning=['low','high']",
    );
    assert!(parse(&text).is_err());
    assert!(
        parse(&text.replace(
            "protocol = \"openai\"",
            "protocol = \"openai\"\npreset='openai'"
        ))
        .is_ok()
    );
}

#[test]
fn a_resource_specific_preset_requires_its_own_endpoint() {
    let vendor = rig_harness::rig_core::providers::azure::PROVIDER_NAME;
    let azure = format!(
        "[providers.instances.azure]\nprotocol='openai'\npreset='{vendor}'\ncredential={{source='none'}}\n"
    );
    assert!(parse(&azure).is_err());
    assert!(parse(&format!("{azure}base_url='http://127.0.0.1:9'\n")).is_ok());
}

#[test]
fn reload_observes_credentials_removed_by_another_session() {
    use rig_harness::{harness_protocol::Home, services::RigHome};
    let dir = scratch();
    std::fs::write(dir.join("settings.toml"), CONFIG).unwrap();
    let mut app = App::new();
    app.add_message::<Notice>();
    app.insert_resource(RigHome(Home::at(&dir)));
    app.insert_resource(Settings::from_toml(CONFIG));
    app.add_plugins(ProvidersPlugin);
    vivy_gateway::save_key(app.world_mut(), "company", "fixture-session-key").unwrap();
    let connection = app
        .world()
        .resource::<Models>()
        .connect("company/org/model-1")
        .unwrap();
    let agent = app
        .world_mut()
        .spawn((Agent, ModelChoice("company/org/model-1".into()), connection))
        .id();
    std::fs::remove_file(dir.join("auth/provider-keys/company.json")).unwrap();
    vivy_gateway::reload(app.world_mut()).unwrap();
    assert!(
        app.world()
            .resource::<Models>()
            .connect("company/org/model-1")
            .is_err()
    );
    assert!(app.world().get::<Connection>(agent).is_none());
    std::fs::remove_dir_all(dir).unwrap();
}

#[test]
fn bootstrap_dialect_survives_persistence_without_overriding_managed_settings() {
    let mut config = ProvidersConfig::default();
    assert!(config.bootstrap(
        "https://api.deepseek.com",
        "manual-model",
        Some("deepseek".into())
    ));
    assert_eq!(
        config.instances["gateway"].preset.as_deref(),
        Some("deepseek")
    );
    let saved = toml::to_string(&config).unwrap();
    let mut restored: ProvidersConfig = toml::from_str(&saved).unwrap();
    assert!(!restored.bootstrap("https://other.example/v1", "replacement", None));
    assert_eq!(restored, config);
}
