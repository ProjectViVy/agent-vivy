//! Provider and model editing on Rig's shared picker, with explicit drafts.
use super::{
    config::{self, Credential, Instance, Model, Protocol},
    providers::{self, ProvidersState},
};
use rig_harness::pick::{Picked, Picker};
use rig_harness::prelude::*;

#[derive(Clone)]
struct Draft {
    id: String,
    instance: Instance,
    original: Option<Instance>,
}

#[derive(Clone)]
enum Action {
    Root,
    Templates,
    New(Protocol, Option<String>),
    Detail(String),
    Edit(Draft),
    ProviderField(Draft, &'static str),
    Protocol(Draft),
    Preset(Draft),
    SetPreset(Draft, Option<String>),
    SetProtocol(Draft, Protocol),
    Credential(Draft),
    SetCredential(Draft, Credential),
    Models(Draft),
    RestoreModels(Draft),
    Model(Draft, Model, bool),
    ModelField(Draft, Model, bool, &'static str),
    SaveModel(Draft, Model, bool),
    DeleteModel(Draft, String),
    Save(Draft),
    SaveAndTest(Draft),
    Test(String),
    Delete(String),
    ConfirmDelete(String),
    PickModels(String),
    Use(String),
    Login(String),
    Logout(String),
    Default,
}

#[derive(Clone)]
enum Input {
    Provider(Draft, &'static str),
    Model(Draft, Model, bool, &'static str),
    Key(String),
}

pub(super) fn add(app: &mut App) {
    app.add_command(
        "provider",
        "Manage providers, credentials and models",
        provider,
    )
    .add_command("login", "Save an API key: /login [instance]", login)
    .add_command(
        "logout",
        "Remove a saved API key: /logout [instance]",
        logout,
    )
    .add_command(
        "model-default",
        "Save the current model and thinking level as startup defaults",
        default_model,
    );
    use rig_harness::rig_ecs::{stage::AgentLoop, tasks::PollTasks};
    app.add_systems(Update, refresh_models.after(PollTasks).before(AgentLoop));
}

fn refresh_models(world: &mut World) {
    use rig_harness::rig_ecs::{
        inbox::Incoming,
        stage::{Outcome, Settled, Started},
    };
    let requested = world
        .query_filtered::<&Incoming, (Without<Started>, Without<Outcome>, Without<Settled>)>()
        .iter(world)
        .any(|m| {
            m.command()
                .is_some_and(|line| line.split_whitespace().next() == Some("model"))
        });
    if requested {
        if let Err(why) = super::reload(world) {
            // The existing model picker remains usable with the last valid catalog.
            world
                .resource_mut::<Messages<Notice>>()
                .write(Notice::error(
                    None,
                    format!("Configuration unchanged: {why}"),
                ));
        }
    }
}

fn provider(In(args): In<CommandArgs>, mut commands: Commands) {
    commands.queue(move |world: &mut World| {
        if let Err(why) = super::reload(world) {
            notice(
                world,
                args.agent,
                &format!("Configuration unchanged: {why}"),
                true,
            );
        }
        run(
            world,
            args.agent,
            if args.args.is_empty() {
                Action::Root
            } else {
                Action::Detail(args.args.trim().into())
            },
        );
    });
}
fn login(In(args): In<CommandArgs>, mut commands: Commands) {
    commands.queue(move |world: &mut World| {
        credential_command(world, args.agent, args.args.trim(), true)
    });
}
fn logout(In(args): In<CommandArgs>, mut commands: Commands) {
    commands.queue(move |world: &mut World| {
        credential_command(world, args.agent, args.args.trim(), false)
    });
}
fn default_model(In(args): In<CommandArgs>, mut commands: Commands) {
    commands.queue(move |world: &mut World| run(world, args.agent, Action::Default));
}
fn credential_command(world: &mut World, agent: Entity, id: &str, login: bool) {
    if let Err(why) = super::reload(world) {
        notice(world, agent, &why, true);
    }
    if !id.is_empty() {
        run(
            world,
            agent,
            if login {
                Action::Login(id.into())
            } else {
                Action::Logout(id.into())
            },
        );
        return;
    }
    let items = world
        .resource::<ProvidersState>()
        .config
        .instances
        .keys()
        .map(|id| {
            (
                id.clone(),
                if login {
                    Action::Login(id.clone())
                } else {
                    Action::Logout(id.clone())
                },
            )
        })
        .collect();
    menu(
        world,
        agent,
        if login {
            "Save API key"
        } else {
            "Remove API key"
        },
        "Choose a configured provider. /provider adds one.",
        items,
    );
}

fn notice(world: &mut World, agent: Entity, text: &str, error: bool) {
    world.resource_mut::<Messages<Notice>>().write(if error {
        Notice::error(agent, text.to_owned())
    } else {
        Notice::info(agent, text.to_owned())
    });
}

fn menu(world: &mut World, agent: Entity, title: &str, body: &str, items: Vec<(String, Action)>) {
    let mut picker = Picker::new(title).for_agent(agent).body(body);
    for (label, _) in &items {
        picker = picker.item(label, "");
    }
    world
        .spawn(picker)
        .observe(move |picked: On<Picked>, mut commands: Commands| {
            let Some(index) = picked
                .chosen
                .as_ref()
                .and_then(|c| c.items.first())
                .copied()
            else {
                return;
            };
            let Some((_, action)) = items.get(index) else {
                return;
            };
            let action = action.clone();
            commands.queue(move |world: &mut World| run(world, agent, action));
        });
}

fn input(
    world: &mut World,
    agent: Entity,
    title: &str,
    current: &str,
    target: Input,
    secret: bool,
    issue: Option<&str>,
) {
    let body = if secret {
        "The API key is hidden and saved only in your user authentication directory.".to_owned()
    } else {
        format!(
            "Current: {}\nType a value; Enter keeps it. Use - to clear an optional field.{}",
            if current.is_empty() {
                "(empty)"
            } else {
                current
            },
            issue.map(|s| format!("\n{s}")).unwrap_or_default()
        )
    };
    let picker = Picker::new(title).for_agent(agent).body(body);
    let picker = if secret {
        picker.secret()
    } else {
        picker.free_text().item("Keep current value", "")
    };
    let current = current.to_owned();
    let title = title.to_owned();
    world
        .spawn(picker)
        .observe(move |picked: On<Picked>, mut commands: Commands| {
            let Some(chosen) = &picked.chosen else {
                return;
            };
            let value = chosen
                .text
                .as_ref()
                .filter(|s| !s.trim().is_empty())
                .cloned()
                .unwrap_or_else(|| current.clone());
            let target = target.clone();
            let title = title.clone();
            commands.queue(move |world: &mut World| {
                if let Err(why) = accept(world, agent, target.clone(), &value) {
                    if secret {
                        notice(world, agent, &why, true);
                    }
                    input(
                        world,
                        agent,
                        &title,
                        if secret { "" } else { &value },
                        target,
                        secret,
                        Some(&why),
                    );
                }
            });
        });
}

fn draft(world: &World, id: &str) -> Option<Draft> {
    world
        .resource::<ProvidersState>()
        .config
        .instances
        .get(id)
        .cloned()
        .map(|instance| Draft {
            id: id.into(),
            original: Some(instance.clone()),
            instance,
        })
}
fn effective_url(instance: &Instance) -> String {
    instance
        .base_url
        .clone()
        .or_else(|| {
            instance
                .preset_id()
                .ok()
                .flatten()
                .and_then(|id| id.config(""))
                .map(|c| c.base_url().to_owned())
        })
        .unwrap_or_default()
}
fn key_state(world: &World, id: &str, instance: &Instance) -> String {
    match &instance.credential {
        Credential::None => "no key".into(),
        Credential::Env { name } => {
            if std::env::var_os(name).is_some_and(|v| !v.is_empty()) {
                format!("environment: {name}")
            } else {
                format!("missing: {name}")
            }
        }
        Credential::Stored => {
            let path = rig_harness::services::RigHome::of(world)
                .auth()
                .join("provider-keys")
                .join(format!("{id}.json"));
            if path.is_file() {
                "saved key".into()
            } else {
                "key required".into()
            }
        }
    }
}

fn run(world: &mut World, agent: Entity, action: Action) {
    let test_after_save = matches!(&action, Action::SaveAndTest(_));
    match action {
        Action::Root => {
            let state = world.resource::<ProvidersState>();
            let mut items = vec![("Add provider".into(), Action::Templates)];
            for (id, instance) in &state.config.instances {
                items.push((
                    format!(
                        "{} · {} · {} models · {}",
                        instance.name.as_deref().unwrap_or(id),
                        instance.protocol.name(),
                        providers::rows(id, instance).len(),
                        key_state(world, id, instance)
                    ),
                    Action::Detail(id.clone()),
                ));
            }
            items.push((
                "Save current model and thinking as default".into(),
                Action::Default,
            ));
            let body = state.issue.clone().unwrap_or_else(|| {
                "Configured providers. Add one from a built-in preset or compatible endpoint."
                    .into()
            });
            menu(world, agent, "Providers", &body, items);
        }
        Action::Templates => {
            let mut items = vec![];
            for protocol in [Protocol::Openai, Protocol::Anthropic]
                .into_iter()
                .filter(|p| p.compiled())
            {
                items.push((
                    format!(
                        "Custom {}",
                        if protocol == Protocol::Openai {
                            "OpenAI"
                        } else {
                            "Anthropic"
                        }
                    ),
                    Action::New(protocol, None),
                ));
            }
            for preset in config::presets() {
                let protocol = if preset.format()
                    == Some(rig_harness::rig_core::providers::registry::Format::OpenAi)
                {
                    Protocol::Openai
                } else {
                    Protocol::Anthropic
                };
                items.push((
                    format!("{} · {}", preset.vendor(), protocol.name()),
                    Action::New(protocol, Some(preset.vendor().into())),
                ));
            }
            menu(
                world,
                agent,
                "Add provider",
                "Choose a preset or a custom compatible endpoint.",
                items,
            );
        }
        Action::New(protocol, preset) => {
            let mut instance = Instance::generic(protocol);
            instance.preset = preset.clone();
            if let Ok(Some(id)) = instance.preset_id() {
                instance.credential = if id.requires_credential() {
                    id.api_key_envs()
                        .first()
                        .map(|name| Credential::Env {
                            name: (*name).into(),
                        })
                        .unwrap_or_default()
                } else {
                    Credential::None
                };
            }
            let stem = preset.unwrap_or_else(|| format!("custom-{}", protocol.name()));
            let stem: String = stem
                .chars()
                .map(|c| {
                    if c.is_ascii_lowercase() || c.is_ascii_digit() || c == '-' || c == '_' {
                        c
                    } else {
                        '-'
                    }
                })
                .collect();
            let stem = if stem.starts_with(|c: char| c.is_ascii_lowercase()) {
                stem
            } else {
                format!("provider-{stem}")
            };
            let mut id = stem.clone();
            let mut n = 2;
            while world
                .resource::<ProvidersState>()
                .config
                .instances
                .contains_key(&id)
            {
                id = format!("{stem}-{n}");
                n += 1;
            }
            run(
                world,
                agent,
                Action::Edit(Draft {
                    id,
                    instance,
                    original: None,
                }),
            );
        }
        Action::Detail(id) => {
            let Some(d) = draft(world, &id) else {
                notice(
                    world,
                    agent,
                    &format!("Unknown provider `{id}`. Use /provider to add it."),
                    true,
                );
                return;
            };
            let body = format!(
                "{}\n{}\n{}",
                id,
                effective_url(&d.instance),
                key_state(world, &id, &d.instance)
            );
            menu(
                world,
                agent,
                "Provider",
                &body,
                vec![
                    ("Choose model".into(), Action::PickModels(id.clone())),
                    ("Edit provider and models".into(), Action::Edit(d)),
                    ("Save API key".into(), Action::Login(id.clone())),
                    ("Remove saved API key".into(), Action::Logout(id.clone())),
                    ("Delete provider".into(), Action::Delete(id)),
                    ("Back".into(), Action::Root),
                ],
            );
        }
        Action::Edit(d) => {
            let mut items = vec![];
            if d.original.is_none() {
                items.push((
                    format!("Instance ID: {}", d.id),
                    Action::ProviderField(d.clone(), "id"),
                ));
            }
            items.extend([
                (
                    format!("Name: {}", d.instance.name.as_deref().unwrap_or(&d.id)),
                    Action::ProviderField(d.clone(), "name"),
                ),
                (
                    format!("Protocol: {}", d.instance.protocol.name()),
                    Action::Protocol(d.clone()),
                ),
                (
                    format!(
                        "Compatibility preset: {}",
                        d.instance.preset.as_deref().unwrap_or("generic")
                    ),
                    Action::Preset(d.clone()),
                ),
                (
                    format!("Endpoint: {}", effective_url(&d.instance)),
                    Action::ProviderField(d.clone(), "base_url"),
                ),
                (
                    format!(
                        "Credential: {}",
                        match &d.instance.credential {
                            Credential::Stored => "saved key".into(),
                            Credential::None => "no key".into(),
                            Credential::Env { name } => format!("environment {name}"),
                        }
                    ),
                    Action::Credential(d.clone()),
                ),
                (
                    format!("Models: {}", providers::rows(&d.id, &d.instance).len()),
                    Action::Models(d.clone()),
                ),
                ("Save only (unverified)".into(), Action::Save(d.clone())),
                ("Save and test".into(), Action::SaveAndTest(d)),
                ("Cancel changes".into(), Action::Root),
            ]);
            menu(
                world,
                agent,
                "Edit provider",
                "Changes remain a draft until Save provider. Instance IDs stay fixed after creation.",
                items,
            );
        }
        Action::ProviderField(d, field) => {
            let value = match field {
                "id" => d.id.clone(),
                "name" => d.instance.name.clone().unwrap_or_default(),
                "base_url" => effective_url(&d.instance),
                "env" => match &d.instance.credential {
                    Credential::Env { name } => name.clone(),
                    _ => d
                        .instance
                        .preset_id()
                        .ok()
                        .flatten()
                        .and_then(|p| p.api_key_envs().first().copied())
                        .unwrap_or("PROVIDER_API_KEY")
                        .into(),
                },
                _ => String::new(),
            };
            input(
                world,
                agent,
                field,
                &value,
                Input::Provider(d, field),
                false,
                None,
            );
        }
        Action::Preset(d) => {
            let mut items = vec![(
                "Generic compatible endpoint".into(),
                Action::SetPreset(d.clone(), None),
            )];
            items.extend(
                config::presets()
                    .into_iter()
                    .filter(|p| p.format() == Some(d.instance.protocol.format()))
                    .map(|p| {
                        (
                            p.vendor().into(),
                            Action::SetPreset(d.clone(), Some(p.vendor().into())),
                        )
                    }),
            );
            menu(
                world,
                agent,
                "Compatibility preset",
                "Choose the endpoint's option mapping. The address and model IDs stay as configured.",
                items,
            );
        }
        Action::SetPreset(mut d, preset) => {
            d.instance.preset = preset;
            run(world, agent, Action::Edit(d));
        }
        Action::Protocol(d) => {
            let items = [Protocol::Openai, Protocol::Anthropic]
                .into_iter()
                .filter(|p| p.compiled())
                .map(|p| (p.name().into(), Action::SetProtocol(d.clone(), p)))
                .collect();
            menu(
                world,
                agent,
                "Protocol",
                "Changing protocol clears the previous preset. Review endpoint and model facts before saving.",
                items,
            );
        }
        Action::SetProtocol(mut d, p) => {
            if d.instance.protocol != p {
                d.instance.protocol = p;
                d.instance.preset = None;
            }
            run(world, agent, Action::Edit(d));
        }
        Action::Credential(d) => menu(
            world,
            agent,
            "Credential source",
            "Choose exactly one source. Missing credentials never fall back to another account.",
            vec![
                (
                    "Saved API key".into(),
                    Action::SetCredential(d.clone(), Credential::Stored),
                ),
                (
                    "Environment variable".into(),
                    Action::ProviderField(d.clone(), "env"),
                ),
                (
                    "No key required".into(),
                    Action::SetCredential(d.clone(), Credential::None),
                ),
                ("Back".into(), Action::Edit(d)),
            ],
        ),
        Action::SetCredential(mut d, credential) => {
            d.instance.credential = credential;
            run(world, agent, Action::Edit(d));
        }
        Action::Models(d) => {
            let mut items = vec![(
                "Add model".into(),
                Action::Model(d.clone(), Model::default(), true),
            )];
            for row in providers::rows(&d.id, &d.instance) {
                let model = d
                    .instance
                    .models
                    .iter()
                    .find(|m| m.id == row.id)
                    .cloned()
                    .unwrap_or_else(|| Model::new(&row.id));
                items.push((
                    format!("{} · {}", row.id, row.display_name),
                    Action::Model(d.clone(), model, false),
                ));
            }
            if !d.instance.disabled_models.is_empty() {
                items.push((
                    "Restore removed preset models".into(),
                    Action::RestoreModels(d.clone()),
                ));
            }
            items.push(("Back to provider".into(), Action::Edit(d)));
            menu(
                world,
                agent,
                "Provider models",
                "Model IDs are sent unchanged to the endpoint. Changes are saved with the provider.",
                items,
            );
        }
        Action::RestoreModels(mut d) => {
            d.instance.disabled_models.clear();
            run(world, agent, Action::Models(d));
        }
        Action::Model(d, m, new) => {
            let mut items = vec![];
            for field in [
                "id",
                "name",
                "context_window",
                "max_output_tokens",
                "tools",
                "image_input",
                "reasoning",
                "input_price",
                "output_price",
            ] {
                if field == "id" && !new {
                    continue;
                }
                items.push((
                    format!("{field}: {}", model_value(&m, field)),
                    Action::ModelField(d.clone(), m.clone(), new, field),
                ));
            }
            items.push((
                "Save model to draft".into(),
                Action::SaveModel(d.clone(), m.clone(), new),
            ));
            if !new {
                items.push((
                    "Remove model".into(),
                    Action::DeleteModel(d.clone(), m.id.clone()),
                ));
            }
            items.push(("Cancel model changes".into(), Action::Models(d)));
            menu(
                world,
                agent,
                "Edit model",
                &format!(
                    "{}\nUnspecified facts inherit the preset, or remain unknown. tools/image_input: true or false; reasoning: comma-separated levels.",
                    m.id
                ),
                items,
            );
        }
        Action::ModelField(d, m, new, field) => input(
            world,
            agent,
            field,
            &model_value(&m, field),
            Input::Model(d, m, new, field),
            false,
            None,
        ),
        Action::SaveModel(mut d, m, new) => {
            if m.id.trim().is_empty()
                || (new
                    && providers::rows(&d.id, &d.instance)
                        .iter()
                        .any(|r| r.id == m.id))
            {
                notice(world, agent, "Model ID is empty or already exists.", true);
                run(world, agent, Action::Model(d, m, new));
                return;
            }
            d.instance.models.retain(|row| row.id != m.id);
            d.instance.disabled_models.retain(|id| id != &m.id);
            d.instance.models.push(m);
            run(world, agent, Action::Models(d));
        }
        Action::DeleteModel(mut d, id) => {
            d.instance.models.retain(|m| m.id != id);
            if d.instance.preset.is_some() && !d.instance.disabled_models.contains(&id) {
                d.instance.disabled_models.push(id);
            }
            run(world, agent, Action::Models(d));
        }
        Action::Save(d) | Action::SaveAndTest(d) => {
            let mut config = world.resource::<ProvidersState>().config.clone();
            if config.instances.get(&d.id) != d.original.as_ref() {
                notice(
                    world,
                    agent,
                    "Provider changed or the ID is already used; reopen the editor.",
                    true,
                );
                run(world, agent, Action::Edit(d));
                return;
            }
            config.instances.insert(d.id.clone(), d.instance.clone());
            match super::save_config(world, config) {
                Ok(()) => {
                    notice(world, agent, "Provider saved.", false);
                    if test_after_save {
                        let items = providers::rows(&d.id, &d.instance)
                            .into_iter()
                            .map(|row| {
                                (row.id.clone(), Action::Test(format!("{}/{}", d.id, row.id)))
                            })
                            .collect();
                        menu(
                            world,
                            agent,
                            "Choose model to test",
                            "Uses the saved provider configuration. No session content is sent.",
                            items,
                        );
                    } else {
                        run(world, agent, Action::Detail(d.id));
                    }
                }
                Err(why) => {
                    notice(world, agent, &why, true);
                    run(world, agent, Action::Edit(d));
                }
            }
        }
        Action::Test(model) => super::setup::test_reference(world, agent, &model),
        Action::Delete(id) => menu(
            world,
            agent,
            "Delete provider",
            &format!(
                "Delete {id} and its saved API key? Recorded sessions keep their model reference and will need repair."
            ),
            vec![
                (
                    "Delete provider and saved key".into(),
                    Action::ConfirmDelete(id.clone()),
                ),
                ("Cancel".into(), Action::Detail(id)),
            ],
        ),
        Action::ConfirmDelete(id) => match providers::remove_provider(world, &id) {
            Ok(()) => {
                notice(
                    world,
                    agent,
                    "Provider deleted. Select another model if needed.",
                    false,
                );
                run(world, agent, Action::Root);
            }
            Err(why) => {
                notice(world, agent, &why, true);
                run(world, agent, Action::Detail(id));
            }
        },
        Action::PickModels(id) => {
            let Some(d) = draft(world, &id) else {
                return;
            };
            let items = providers::rows(&id, &d.instance)
                .into_iter()
                .map(|m| {
                    let reference = m.reference();
                    let status = world.resource::<Models>().0.status(&m);
                    (
                        format!("{} · {} · {:?}", m.id, m.display_name, status),
                        Action::Use(reference),
                    )
                })
                .collect();
            menu(world, agent, "Choose model", &id, items);
        }
        Action::Use(model) => world.trigger(SetModel {
            entity: agent,
            model,
        }),
        Action::Login(id) => {
            if draft(world, &id).is_none() {
                notice(
                    world,
                    agent,
                    "Unknown provider. Add it with /provider first.",
                    true,
                );
                return;
            }
            input(
                world,
                agent,
                &format!("API key for {id}"),
                "",
                Input::Key(id),
                true,
                None,
            );
        }
        Action::Logout(id) => match super::remove_key(world, &id) {
            Ok(()) => {
                notice(
                    world,
                    agent,
                    "Saved API key removed. Environment variables are unchanged.",
                    false,
                );
                run(world, agent, Action::Detail(id));
            }
            Err(why) => notice(world, agent, &why, true),
        },
        Action::Default => {
            let Some(model) = world.get::<ModelChoice>(agent).map(|m| m.0.clone()) else {
                notice(world, agent, "Choose a model first.", true);
                return;
            };
            if world.resource::<Models>().connect(&model).is_err() {
                notice(
                    world,
                    agent,
                    "The current model is unavailable; repair its provider first.",
                    true,
                );
                return;
            }
            let selected = world
                .get::<ReasoningSetting>(agent)
                .copied()
                .unwrap_or_default();
            let thinking = world
                .resource::<Models>()
                .0
                .catalog()
                .resolve(&model)
                .ok()
                .and_then(|row| {
                    row.spec
                        .reasoning
                        .choices()
                        .into_iter()
                        .find(|choice| choice.reasoning == selected.0)
                })
                .filter(|choice| choice.name != "default")
                .map(|choice| choice.name.to_owned());
            if selected.0.is_some() && thinking.is_none() {
                notice(
                    world,
                    agent,
                    "Choose a supported thinking level before saving the startup default.",
                    true,
                );
                return;
            }
            let mut config = world.resource::<ProvidersState>().config.clone();
            config.default_model = Some(model);
            config.default_thinking = thinking;
            match super::save_config(world, config) {
                Ok(()) => notice(
                    world,
                    agent,
                    "Startup model and thinking default saved. Existing sessions keep their choices.",
                    false,
                ),
                Err(why) => notice(world, agent, &why, true),
            }
        }
    }
}

fn accept(world: &mut World, agent: Entity, target: Input, text: &str) -> Result<(), String> {
    let value = if text.trim() == "-" { "" } else { text.trim() };
    match target {
        Input::Provider(mut d, field) => {
            match field {
                "id" => {
                    if !config::valid_id(value) {
                        return Err(
                            "Use lowercase letters, numbers, - or _; start with a letter.".into(),
                        );
                    }
                    d.id = value.into();
                }
                "name" => d.instance.name = (!value.is_empty()).then(|| value.into()),
                "base_url" => d.instance.base_url = (!value.is_empty()).then(|| value.into()),
                "env" => {
                    if value.is_empty()
                        || value.contains('=')
                        || value.chars().any(char::is_whitespace)
                    {
                        return Err("Enter an environment variable name.".into());
                    }
                    d.instance.credential = Credential::Env { name: value.into() };
                }
                _ => return Err("Unknown provider field".into()),
            }
            run(world, agent, Action::Edit(d));
        }
        Input::Model(d, mut m, new, field) => {
            match field {
                "id" => m.id = value.into(),
                "name" => m.name = (!value.is_empty()).then(|| value.into()),
                "context_window" => m.context_window = number(value)?,
                "max_output_tokens" => m.max_output_tokens = number(value)?,
                "tools" => m.tools = boolean(value)?,
                "image_input" => m.image_input = boolean(value)?,
                "reasoning" => {
                    let list: Vec<String> = value
                        .split(',')
                        .map(str::trim)
                        .filter(|s| !s.is_empty())
                        .map(str::to_owned)
                        .collect();
                    if list
                        .iter()
                        .any(|s| s != "off" && config::effort(s).is_none())
                    {
                        return Err("Use off, minimal, low, medium, high, xhigh or max.".into());
                    }
                    m.reasoning = (!value.is_empty()).then_some(list);
                }
                "input_price" => m.input_price = number(value)?,
                "output_price" => m.output_price = number(value)?,
                _ => return Err("Unknown model field".into()),
            }
            run(world, agent, Action::Model(d, m, new));
        }
        Input::Key(id) => {
            super::save_key(world, &id, text)?;
            notice(
                world,
                agent,
                "API key saved. This instance now uses its saved key.",
                false,
            );
            run(world, agent, Action::Detail(id));
        }
    }
    Ok(())
}
fn number<T: std::str::FromStr>(value: &str) -> Result<Option<T>, String> {
    if value.is_empty() {
        Ok(None)
    } else {
        value
            .parse()
            .map(Some)
            .map_err(|_| "Enter a number, or - to clear.".into())
    }
}
fn boolean(value: &str) -> Result<Option<bool>, String> {
    number(value).map_err(|_| "Enter true, false, or - to inherit.".into())
}
fn model_value(m: &Model, field: &str) -> String {
    match field {
        "id" => m.id.clone(),
        "name" => m.name.clone().unwrap_or_default(),
        "context_window" => m.context_window.map(|v| v.to_string()).unwrap_or_default(),
        "max_output_tokens" => m
            .max_output_tokens
            .map(|v| v.to_string())
            .unwrap_or_default(),
        "tools" => m.tools.map(|v| v.to_string()).unwrap_or_default(),
        "image_input" => m.image_input.map(|v| v.to_string()).unwrap_or_default(),
        "reasoning" => m
            .reasoning
            .as_ref()
            .map(|v| v.join(","))
            .unwrap_or_default(),
        "input_price" => m.input_price.map(|v| v.to_string()).unwrap_or_default(),
        "output_price" => m.output_price.map(|v| v.to_string()).unwrap_or_default(),
        _ => String::new(),
    }
}
