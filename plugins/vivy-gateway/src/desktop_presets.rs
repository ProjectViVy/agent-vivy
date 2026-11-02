//! Curated desktop setup data; never writes user settings on read.
use crate::config::{Instance, Model, Protocol};
use serde::{Deserialize, Serialize};
#[derive(Clone, Debug, Serialize, Deserialize)]
#[cfg_attr(feature = "protocol-types", derive(ts_rs::TS))]
pub struct DesktopPreset {
    pub id: String,
    pub name: String,
    pub variants: Vec<DesktopVariant>,
}
#[derive(Clone, Debug, Serialize, Deserialize)]
#[cfg_attr(feature = "protocol-types", derive(ts_rs::TS))]
pub struct DesktopVariant {
    pub id: String,
    pub name: String,
    pub instance: Instance,
    pub default_model: String,
}
fn variant(
    id: &str,
    name: &str,
    protocol: Protocol,
    preset: Option<&str>,
    url: &str,
    models: &[&str],
) -> DesktopVariant {
    let mut instance = Instance::generic(protocol);
    instance.base_url = Some(url.into());
    instance.preset = preset.map(str::to_owned);
    instance.models = models
        .iter()
        .map(|id| Model {
            id: (*id).into(),
            context_window: Some(1_000_000),
            max_output_tokens: Some(8192),
            tools: Some(true),
            ..Model::default()
        })
        .collect();
    DesktopVariant {
        id: id.into(),
        name: name.into(),
        instance,
        default_model: models[0].into(),
    }
}
pub fn presets() -> Vec<DesktopPreset> {
    let mut rows = vec![
        DesktopPreset {
            id: "deepseek".into(),
            name: "DeepSeek".into(),
            variants: vec![variant(
                "standard",
                "Standard API",
                Protocol::Openai,
                Some("deepseek"),
                "https://api.deepseek.com",
                &["deepseek-flash", "deepseek-v4-pro"],
            )],
        },
        DesktopPreset {
            id: "minimax".into(),
            name: "MiniMax".into(),
            variants: vec![variant(
                "china",
                "China API",
                Protocol::Anthropic,
                Some("minimax"),
                "https://api.minimax.cn/anthropic",
                &["MiniMax-M3.1-Flash", "MiniMax-M3"],
            )],
        },
        DesktopPreset {
            id: "bailian".into(),
            name: "阿里云百炼".into(),
            variants: vec![
                variant(
                    "china",
                    "国内普通 API",
                    Protocol::Openai,
                    None,
                    "https://dashscope.aliyuncs.com/compatible-mode/v1",
                    &["qwen3.8-flash", "qwen3.7-plus"],
                ),
                variant(
                    "international",
                    "国际普通 API",
                    Protocol::Openai,
                    None,
                    "https://dashscope-intl.aliyuncs.com/compatible-mode/v1",
                    &["qwen3.8-flash", "qwen3.7-plus"],
                ),
                variant(
                    "coding",
                    "国内 Coding Plan",
                    Protocol::Openai,
                    None,
                    "https://coding.dashscope.aliyuncs.com/v1",
                    &["qwen3.7-plus"],
                ),
            ],
        },
    ];
    rows.retain(|p| p.variants[0].instance.protocol.compiled());
    rows
}
#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::ProvidersConfig;
    #[test]
    fn all_variants_validate_and_preserve_exact_ids() {
        for p in presets() {
            for v in p.variants {
                let c = ProvidersConfig {
                    default_model: Some(format!("{}/{}", p.id, v.default_model)),
                    instances: std::collections::BTreeMap::from([(p.id.clone(), v.instance)]),
                    ..Default::default()
                };
                c.validate().unwrap();
            }
        }
    }
}
