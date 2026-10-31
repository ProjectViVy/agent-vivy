//! The recipe: which Rig plugin crates and plugin types one agent is built
//! from. Parsing checks what Cargo and rustc would only report later or less
//! clearly; rustc still checks that each plugin type exists and is a plugin.

use std::collections::{BTreeMap, BTreeSet};
use std::fmt;

use serde::Deserialize;

/// A parsed and checked recipe.
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Recipe {
    /// The generated package's name, and so the agent binary's.
    pub name: String,
    /// Where every Rig crate comes from.
    pub rig: RigSource,
    /// The selected components, in the order their plugins are added.
    pub components: Vec<Component>,
}

/// One pinned Rig revision.
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RigSource {
    pub git: String,
    pub rev: String,
}

/// A named group of plugin types from one crate: a crate of the Rig
/// revision, or one of this repository's own by `path`.
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Component {
    pub name: String,
    #[serde(rename = "crate")]
    pub krate: String,
    /// The crate's directory, relative to the recipe; none for a Rig crate.
    /// It must build on the recipe's Rig revision.
    pub path: Option<String>,
    /// Plugin types (`Plugin + Default`), each added with `rig_harness::load`.
    pub plugins: Vec<String>,
    /// Cargo features this component needs, by crate.
    #[serde(default)]
    pub features: BTreeMap<String, Vec<String>>,
}

/// One selected crate.
#[derive(Debug, Default, PartialEq)]
pub struct Selected<'a> {
    pub path: Option<&'a str>,
    pub features: BTreeSet<&'a str>,
}

/// A recipe that cannot be built, with what to change.
#[derive(Debug)]
pub struct RecipeError(pub String);

impl fmt::Display for RecipeError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl std::error::Error for RecipeError {}

fn fail<T>(message: impl Into<String>) -> Result<T, RecipeError> {
    Err(RecipeError(message.into()))
}

impl Recipe {
    /// Parses and checks the recipe text.
    pub fn parse(text: &str) -> Result<Self, RecipeError> {
        let recipe: Recipe =
            serde_saphyr::from_str(text).map_err(|error| RecipeError(error.to_string()))?;
        recipe.check()?;
        Ok(recipe)
    }

    /// The selected crates, each once, with its `path` (none for a Rig
    /// crate) and the union of the features selected components ask of it.
    pub fn crates(&self) -> BTreeMap<&str, Selected<'_>> {
        let mut crates: BTreeMap<&str, Selected<'_>> = BTreeMap::new();
        for component in &self.components {
            let selected = crates.entry(&component.krate).or_default();
            selected.path = selected.path.or(component.path.as_deref());
        }
        for component in &self.components {
            for (krate, features) in &component.features {
                if let Some(selected) = crates.get_mut(krate.as_str()) {
                    selected
                        .features
                        .extend(features.iter().map(String::as_str));
                }
            }
        }
        crates
    }

    fn check(&self) -> Result<(), RecipeError> {
        if !is_package_name(&self.name) {
            return fail(format!(
                "name `{}` must be a Cargo package name: lowercase letters, digits, `-` or `_`, starting with a letter",
                self.name
            ));
        }
        if self.rig.git.trim().is_empty() || self.rig.rev.trim().is_empty() {
            return fail("rig needs both `git` and `rev`");
        }
        if self.components.is_empty() {
            return fail("components is empty: select at least one component");
        }
        let mut names = BTreeSet::new();
        let mut paths: BTreeMap<&str, Option<&str>> = BTreeMap::new();
        let mut plugins = BTreeSet::new();
        let selected: BTreeSet<&str> = self.components.iter().map(|c| c.krate.as_str()).collect();
        for component in &self.components {
            let name = &component.name;
            if component.krate == "vivy-process" {
                return fail(
                    "vivy-process is reserved for generated session ownership, not a selectable plugin crate",
                );
            }
            if !names.insert(name.as_str()) {
                return fail(format!("component `{name}` is listed twice"));
            }
            if !is_package_name(name) {
                return fail(format!(
                    "component name `{name}` must use lowercase letters, digits, `-` or `_`, starting with a letter"
                ));
            }
            if !is_package_name(&component.krate) {
                return fail(format!(
                    "component `{name}`: crate `{}` is not a Cargo package name",
                    component.krate
                ));
            }
            let path = component.path.as_deref();
            if path.is_some_and(|path| path.trim().is_empty()) {
                return fail(format!("component `{name}`: `path` is empty"));
            }
            if let Some(earlier) = paths.insert(&component.krate, path)
                && earlier != path
            {
                return fail(format!(
                    "crate `{}` comes from two sources; give every component of it the same `path`",
                    component.krate
                ));
            }
            if component.plugins.is_empty() {
                return fail(format!("component `{name}` lists no plugins"));
            }
            let module = component.krate.replace('-', "_");
            for plugin in &component.plugins {
                if !is_type_path(plugin) {
                    return fail(format!(
                        "component `{name}`: `{plugin}` is not a Rust type path such as `{module}::SomePlugin`"
                    ));
                }
                if plugin.split("::").next() != Some(module.as_str()) {
                    return fail(format!(
                        "component `{name}`: plugin `{plugin}` must be in its crate `{}`, so start with `{module}::`",
                        component.krate
                    ));
                }
                if !plugins.insert(plugin.as_str()) {
                    return fail(format!(
                        "plugin `{plugin}` is selected twice (again in component `{name}`)"
                    ));
                }
            }
            for (krate, features) in &component.features {
                if !selected.contains(krate.as_str()) {
                    return fail(format!(
                        "component `{name}` sets features of crate `{krate}`, which no component selects"
                    ));
                }
                if let Some(bad) = features.iter().find(|feature| !is_feature(feature)) {
                    return fail(format!("component `{name}`: `{bad}` is not a feature name"));
                }
            }
        }
        Ok(())
    }
}

fn is_package_name(name: &str) -> bool {
    name.starts_with(|c: char| c.is_ascii_lowercase())
        && name
            .chars()
            .all(|c| c.is_ascii_lowercase() || c.is_ascii_digit() || c == '-' || c == '_')
}

fn is_identifier(segment: &str) -> bool {
    segment.starts_with(|c: char| c.is_ascii_alphabetic() || c == '_')
        && segment
            .chars()
            .all(|c| c.is_ascii_alphanumeric() || c == '_')
}

fn is_type_path(path: &str) -> bool {
    let segments: Vec<&str> = path.split("::").collect();
    segments.len() >= 2 && segments.iter().all(|segment| is_identifier(segment))
}

fn is_feature(feature: &str) -> bool {
    !feature.is_empty()
        && feature
            .chars()
            .all(|c| c.is_ascii_alphanumeric() || matches!(c, '-' | '_' | '/' | '+' | '.'))
}

#[cfg(test)]
mod tests {
    #[test]
    fn baseline_selects_the_two_provider_families_and_manager() {
        let recipe =
            super::Recipe::parse(include_str!("../../../recipes/coding-agent.yaml")).unwrap();
        let crates = recipe.crates();
        let gateway = &crates["vivy-gateway"];
        assert!(
            gateway.features.contains("openai")
                && gateway.features.contains("anthropic")
                && gateway.features.contains("tui")
        );
        assert!(recipe.components.iter().any(|c| {
            c.plugins
                .iter()
                .any(|p| p == "vivy_gateway::ProvidersPlugin")
        }));
    }
    use super::*;

    const BASE: &str = "
name: agent
rig: { git: https://example.invalid/rig, rev: abc }
components:
  - name: tools
    crate: rig-basics
    plugins: [rig_basics::coding_tools::ReadTool]
  - name: terminal
    crate: rig-tui
    plugins: [rig_tui::TuiPlugin]
    features: { rig-basics: [tui] }
";

    fn error(text: &str) -> String {
        Recipe::parse(text).unwrap_err().0
    }

    #[test]
    fn parses_in_order_with_features() {
        let recipe = Recipe::parse(BASE).unwrap();
        let names: Vec<_> = recipe.components.iter().map(|c| c.name.as_str()).collect();
        assert_eq!(names, ["tools", "terminal"]);
        let crates = recipe.crates();
        assert_eq!(crates["rig-basics"].features, BTreeSet::from(["tui"]));
        assert_eq!(crates["rig-tui"], Selected::default());
    }

    #[test]
    fn rejects_unknown_fields() {
        let text = BASE.replace("rev: abc", "rev: abc, branch: main");
        assert!(error(&text).contains("branch"), "{}", error(&text));
    }

    #[test]
    fn rejects_duplicate_component() {
        let text = BASE.replace("name: terminal", "name: tools");
        assert_eq!(error(&text), "component `tools` is listed twice");
    }

    #[test]
    fn rejects_duplicate_plugin() {
        let text = format!(
            "{BASE}  - name: again\n    crate: rig-basics\n    plugins: [rig_basics::coding_tools::ReadTool]\n"
        );
        assert!(error(&text).contains("selected twice"));
    }

    #[test]
    fn rejects_empty_plugins_and_components() {
        let text = BASE.replace("plugins: [rig_tui::TuiPlugin]", "plugins: []");
        assert_eq!(error(&text), "component `terminal` lists no plugins");
        let text = "name: a\nrig: { git: g, rev: r }\ncomponents: []\n";
        assert!(error(text).contains("components is empty"));
    }

    #[test]
    fn rejects_bad_type_paths() {
        let text = BASE.replace("rig_tui::TuiPlugin", "TuiPlugin");
        assert!(error(&text).contains("not a Rust type path"));
        let text = BASE.replace("rig_tui::TuiPlugin", "rig_basics::TuiPlugin");
        assert!(error(&text).contains("must be in its crate `rig-tui`"));
    }

    #[test]
    fn rejects_features_of_unselected_crate() {
        let text = BASE.replace("rig-basics: [tui]", "rig-steel: [x]");
        assert!(error(&text).contains("no component selects"));
    }

    #[test]
    fn keeps_one_source_per_crate() {
        let text = BASE.replace("crate: rig-tui\n", "crate: rig-tui\n    path: ../tui\n");
        assert_eq!(
            Recipe::parse(&text).unwrap().crates()["rig-tui"].path,
            Some("../tui")
        );
        let text =
            format!("{BASE}  - name: more\n    crate: rig-tui\n    plugins: [rig_tui::Other]\n");
        assert!(Recipe::parse(&text).is_ok());
        let text = text.replacen("crate: rig-tui\n", "crate: rig-tui\n    path: ../tui\n", 1);
        assert!(error(&text).contains("two sources"));
    }

    #[test]
    fn rejects_bad_names() {
        assert!(error(&BASE.replace("name: agent", "name: Agent")).contains("Cargo package name"));
        assert!(
            error(&BASE.replace("crate: rig-tui", "crate: rig tui")).contains("crate `rig tui`")
        );
    }
}
