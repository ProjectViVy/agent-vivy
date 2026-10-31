//! User-owned provider instances. Protocol implementations stay in Rig.
use rig_harness::rig_core::{
    catalog::{ModelSpec, Pricing, ReasoningSupport},
    completion::Effort,
    providers::registry::{Format, ProviderId},
};
use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, BTreeSet};

#[derive(Clone, Copy, Debug, Deserialize, Serialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum Protocol {
    Openai,
    Anthropic,
}

impl Protocol {
    pub fn name(self) -> &'static str {
        match self {
            Self::Openai => "openai",
            Self::Anthropic => "anthropic",
        }
    }
    pub fn format(self) -> Format {
        match self {
            Self::Openai => Format::OpenAi,
            Self::Anthropic => Format::Anthropic,
        }
    }
    pub fn compiled(self) -> bool {
        match self {
            Self::Openai => cfg!(feature = "openai"),
            Self::Anthropic => cfg!(feature = "anthropic"),
        }
    }
}

#[derive(Clone, Debug, Default, Deserialize, Serialize, PartialEq, Eq)]
#[serde(tag = "source", rename_all = "lowercase", deny_unknown_fields)]
pub enum Credential {
    #[default]
    Stored,
    Env {
        name: String,
    },
    None,
}

#[derive(Clone, Debug, Deserialize, Serialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct Instance {
    pub protocol: Protocol,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub preset: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub base_url: Option<String>,
    #[serde(default)]
    pub credential: Credential,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub models: Vec<Model>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub disabled_models: Vec<String>,
}

impl Instance {
    pub fn generic(protocol: Protocol) -> Self {
        Self {
            protocol,
            name: None,
            preset: None,
            base_url: None,
            credential: Credential::Stored,
            models: vec![],
            disabled_models: vec![],
        }
    }
    pub fn preset_id(&self) -> Result<Option<ProviderId>, String> {
        self.preset
            .as_deref()
            .map(|name| {
                presets()
                    .into_iter()
                    .find(|p| p.vendor() == name && p.format() == Some(self.protocol.format()))
                    .ok_or_else(|| {
                        format!(
                            "preset `{name}` is not a compiled {} API-key preset",
                            self.protocol.name()
                        )
                    })
            })
            .transpose()
    }
}

#[derive(Clone, Debug, Default, Deserialize, Serialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct Model {
    pub id: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub context_window: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_output_tokens: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tools: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub image_input: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reasoning: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub input_price: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub output_price: Option<f64>,
}

impl Model {
    pub fn new(id: impl Into<String>) -> Self {
        Self {
            id: id.into(),
            ..Self::default()
        }
    }
    pub fn apply(&self, spec: &mut ModelSpec) {
        if let Some(v) = &self.name {
            spec.display_name = v.clone();
        }
        if let Some(v) = self.context_window {
            spec.context_window = Some(v);
        }
        if let Some(v) = self.max_output_tokens {
            spec.max_output_tokens = Some(v);
        }
        if let Some(v) = self.tools {
            spec.tools = v;
        }
        if let Some(v) = self.image_input {
            spec.input.image = v;
        }
        if let Some(levels) = &self.reasoning {
            spec.reasoning = ReasoningSupport::Listed {
                levels: levels.iter().filter_map(|s| effort(s)).collect(),
                budget: None,
                can_disable: levels.iter().any(|s| s == "off"),
                default: None,
            };
        }
        if let (Some(input), Some(output)) = (self.input_price, self.output_price) {
            spec.pricing = Some(Pricing::new(input, output));
        }
    }
}

pub fn effort(name: &str) -> Option<Effort> {
    [
        Effort::Minimal,
        Effort::Low,
        Effort::Medium,
        Effort::High,
        Effort::XHigh,
        Effort::Max,
    ]
    .into_iter()
    .find(|v| v.as_str() == name)
}

#[derive(Clone, Debug, Default, Deserialize, Serialize, PartialEq, bevy_ecs::prelude::Resource)]
#[serde(default, deny_unknown_fields)]
pub struct ProvidersConfig {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub default_model: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub default_thinking: Option<String>,
    #[serde(skip_serializing_if = "BTreeMap::is_empty")]
    pub instances: BTreeMap<String, Instance>,
}

/// Only protocol families compiled into this component, excluding subscription adapters.
pub fn presets() -> Vec<ProviderId> {
    ProviderId::all()
        .filter(|p| !matches!(p.vendor(), "chatgpt" | "copilot"))
        .filter(|p| match p.format() {
            Some(Format::OpenAi) => cfg!(feature = "openai"),
            Some(Format::Anthropic) => cfg!(feature = "anthropic"),
            _ => false,
        })
        .collect()
}

pub fn valid_id(id: &str) -> bool {
    !id.is_empty()
        && id.len() <= 64
        && id.as_bytes()[0].is_ascii_lowercase()
        && id
            .bytes()
            .all(|c| c.is_ascii_lowercase() || c.is_ascii_digit() || c == b'-' || c == b'_')
}

impl ProvidersConfig {
    /// The native launcher supplies only a first-run gateway hint; managed
    /// instances always win and the credential remains an environment reference.
    pub fn bootstrap(&mut self, url: &str, model: &str, dialect: Option<String>) -> bool {
        if self.instances.contains_key("gateway") || url.is_empty() || model.is_empty() {
            return false;
        }
        let legacy = super::GatewaySettings {
            base_url: url.into(),
            model: model.into(),
            dialect,
            ..Default::default()
        };
        self.add_legacy(&legacy);
        true
    }
    pub fn validate(&self) -> Result<(), String> {
        if let Some(v) = &self.default_model {
            if !v
                .split_once('/')
                .is_some_and(|(a, b)| valid_id(a) && !b.trim().is_empty())
            {
                return Err("default_model must be instance/model-id".into());
            }
        }
        if self
            .default_thinking
            .as_deref()
            .is_some_and(|v| v != "off" && effort(v).is_none())
        {
            return Err("default_thinking is not a supported thinking level".into());
        }
        for (id, instance) in &self.instances {
            if !valid_id(id) {
                return Err(format!(
                    "invalid provider instance ID `{id}`; use lowercase letters, numbers, - or _"
                ));
            }
            if !instance.protocol.compiled() {
                return Err(format!(
                    "{} support was not compiled into this recipe",
                    instance.protocol.name()
                ));
            }
            let preset = instance.preset_id()?;
            if instance.protocol == Protocol::Openai
                && preset.is_none()
                && instance
                    .models
                    .iter()
                    .any(|m| m.reasoning.as_ref().is_some_and(|r| !r.is_empty()))
            {
                return Err(format!(
                    "provider `{id}` needs a compatible preset before declaring OpenAI reasoning levels"
                ));
            }
            let preset_url = preset
                .and_then(|p| p.config(""))
                .map(|c| c.base_url().to_owned());
            let effective_url = instance
                .base_url
                .as_deref()
                .or(preset_url.as_deref())
                .unwrap_or("");
            if effective_url.trim().is_empty() {
                return Err(format!("provider `{id}` needs base_url"));
            }
            {
                let url = effective_url;
                let url = url.trim();
                let parsed = url::Url::parse(url).ok();
                if parsed.as_ref().is_none_or(|u| {
                    !matches!(u.scheme(), "http" | "https")
                        || u.host_str().is_none()
                        || !u.username().is_empty()
                        || u.password().is_some()
                        || u.fragment().is_some()
                }) || url.chars().any(char::is_whitespace)
                {
                    return Err(format!(
                        "provider `{id}` needs an HTTP(S) URL without embedded credentials"
                    ));
                }
            }
            if let Credential::Env { name } = &instance.credential {
                if name.is_empty()
                    || name.contains('=')
                    || name.contains('\0')
                    || name.chars().any(char::is_whitespace)
                {
                    return Err(format!(
                        "provider `{id}` has an invalid credential environment variable name"
                    ));
                }
            }
            if instance.models.is_empty() && preset.is_none() {
                return Err(format!("provider `{id}` needs at least one model"));
            }
            let mut seen = BTreeSet::new();
            for model in &instance.models {
                if model.id.trim().is_empty()
                    || model.id.trim() != model.id
                    || model.id.chars().any(char::is_control)
                    || !seen.insert(&model.id)
                {
                    return Err(format!(
                        "provider `{id}` has an empty, invalid or duplicate model ID"
                    ));
                }
                if model.context_window == Some(0) || model.max_output_tokens == Some(0) {
                    return Err(format!(
                        "model `{}` token limits must be positive",
                        model.id
                    ));
                }
                if model
                    .context_window
                    .zip(model.max_output_tokens)
                    .is_some_and(|(a, b)| b > a)
                {
                    return Err(format!(
                        "model `{}` output limit exceeds its context window",
                        model.id
                    ));
                }
                if model
                    .reasoning
                    .as_ref()
                    .is_some_and(|list| list.iter().any(|v| v != "off" && effort(v).is_none()))
                {
                    return Err(format!(
                        "model `{}` has an unknown reasoning level",
                        model.id
                    ));
                }
                match (model.input_price, model.output_price) {
                    (None, None) => {}
                    (Some(a), Some(b))
                        if a.is_finite() && b.is_finite() && a >= 0.0 && b >= 0.0 => {}
                    _ => {
                        return Err(format!(
                            "model `{}` needs both nonnegative finite prices, or neither",
                            model.id
                        ));
                    }
                }
            }
        }
        Ok(())
    }

    pub fn add_legacy(&mut self, legacy: &super::GatewaySettings) {
        if legacy.missing().is_some() || self.instances.contains_key("gateway") {
            return;
        }
        let mut instance = Instance::generic(Protocol::Openai);
        instance.base_url = Some(legacy.base_url.clone());
        instance.preset = legacy.dialect.clone();
        instance.credential = Credential::Env {
            name: "RIG_SECRET_GATEWAY".into(),
        };
        let mut ids = vec![legacy.model.trim().to_owned()];
        for id in legacy
            .models
            .iter()
            .map(|s| s.trim())
            .filter(|s| !s.is_empty())
        {
            if !ids.iter().any(|s| s == id) {
                ids.push(id.into());
            }
        }
        instance.models = ids
            .into_iter()
            .map(|id| Model {
                id,
                context_window: legacy.context_window,
                max_output_tokens: legacy.max_output_tokens,
                image_input: Some(legacy.image_input),
                tools: Some(true),
                reasoning: if legacy.dialect.is_some() {
                    Some(
                        legacy
                            .reasoning
                            .iter()
                            .filter(|v| v.as_str() == "off" || effort(v).is_some())
                            .cloned()
                            .collect(),
                    )
                } else {
                    None
                },
                input_price: legacy.input_price.zip(legacy.output_price).map(|p| p.0),
                output_price: legacy.input_price.zip(legacy.output_price).map(|p| p.1),
                ..Model::default()
            })
            .collect();
        self.default_model
            .get_or_insert_with(|| format!("gateway/{}", legacy.model.trim()));
        self.instances.insert("gateway".into(), instance);
    }
}

pub fn parse(text: &str) -> Result<ProvidersConfig, String> {
    let table: toml::Table = toml::from_str(text).map_err(|e| format!("settings.toml: {e}"))?;
    let mut config: ProvidersConfig = table
        .get("providers")
        .cloned()
        .map(|v| v.try_into())
        .transpose()
        .map_err(|e| format!("[providers]: {e}"))?
        .unwrap_or_default();
    if let Some(v) = table.get("gateway") {
        let legacy: super::GatewaySettings = v
            .clone()
            .try_into()
            .map_err(|e| format!("[gateway]: {e}"))?;
        config.add_legacy(&legacy);
    }
    config.validate()?;
    Ok(config)
}
