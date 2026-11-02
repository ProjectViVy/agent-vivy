//! Rig's skills, gated by project trust: the same discovery, listing and
//! `/skill:<name>` commands as `rig_basics::skills::SkillsPlugin`, which it
//! replaces in the recipe, without the project's skill directories until
//! the project is trusted ([`crate::trust`]).

use bevy_log::warn;
pub use rig_basics::skills::Skill;
use rig_basics::skills::{discover, message, roots};
use rig_harness::prelude::*;
pub mod local;

/// Skills from the trusted places only.
#[derive(Default)]
pub struct TrustedSkillsPlugin;

/// One trusted discovery result, shared with optional capability consumers.
#[derive(Resource, Default, Clone)]
pub struct TrustedSkillInventory(pub Vec<Skill>);
/// Derived from a validated workbench publication, never an independent catalog.
#[derive(Resource, Default, Clone)]
pub struct PublishedSkills(pub Vec<Skill>);
#[derive(Resource, Default)]
pub struct SkillCollisions(pub Vec<String>);
#[derive(Component)]
struct SkillCommand;

/// Refresh through the same discovery and trust policy used by the skills prompt.
pub fn refresh_inventory(world: &mut World) {
    let home = RigHome::of(world).root().to_path_buf();
    let root = world.resource::<ProjectRoot>().0.clone();
    let places: Vec<_> = roots(&root, &home)
        .into_iter()
        .filter(|place| crate::trust::allowed(world, place))
        .collect();
    let (mut skills, problems) = discover(&places);
    for problem in problems {
        warn!("{problem}");
    }
    let mut collisions = vec![];
    if let Some(published) = world.get_resource::<PublishedSkills>() {
        for skill in &published.0 {
            if skills
                .iter()
                .any(|s| s.name == skill.name && s.file != skill.file)
            {
                warn!(
                    "Published Skill {} collides with ordinary discovery; it is unavailable",
                    skill.name
                );
                collisions.push(skill.name.clone());
                skills.retain(|s| s.name != skill.name);
            } else if !skills.iter().any(|s| s.file == skill.file) {
                skills.push(skill.clone());
            }
        }
    }
    let text = listing(&skills);
    for (name, mut section) in world.query::<(&Name, &mut PromptSection)>().iter_mut(world) {
        if name.as_str() == "prompt:skills" {
            section.text = text.clone();
        }
    }
    world.insert_resource(TrustedSkillInventory(skills));
    world.insert_resource(SkillCollisions(collisions));
    refresh_commands(world);
}

fn refresh_commands(world: &mut World) {
    let old: Vec<_> = world
        .query_filtered::<Entity, With<SkillCommand>>()
        .iter(world)
        .collect();
    for entity in old {
        world.despawn(entity);
    }
    let skills = world.resource::<TrustedSkillInventory>().0.clone();
    for skill in skills {
        let name = skill.name.clone();
        let id = world.register_system(move |In(args): In<CommandArgs>, world: &mut World| {
            let result = world
                .resource::<TrustedSkillInventory>()
                .0
                .iter()
                .find(|s| s.name == name)
                .ok_or_else(|| format!("Skill {name} is no longer available"))
                .and_then(|skill| message(skill, &args.args));
            match result {
                Ok(text) => {
                    let author = Author::user().titled(format!("skill {name}"));
                    world.spawn((
                        Incoming::new(text, args.mode).with_author(author),
                        ForAgent(args.agent),
                    ));
                }
                Err(why) => {
                    world.write_message(Notice::error(args.agent, why));
                }
            }
        });
        world.entity_mut(id.entity()).insert((
            Name::new(format!("/skill:{}", skill.name)),
            rig_harness::commands::SlashCommand {
                help: format!("Skill: {}", first_line(&skill.description)),
            },
            SkillCommand,
        ));
    }
}

impl Plugin for TrustedSkillsPlugin {
    fn build(&self, app: &mut App) {
        app.init_resource::<ProjectRoot>()
            .init_resource::<TrustedSkillInventory>();
        if !app.is_plugin_added::<rig_harness::commands::CommandPlugin>() {
            app.add_plugins(rig_harness::commands::CommandPlugin);
        }
    }

    fn finish(&self, app: &mut App) {
        let world = app.world_mut();
        refresh_inventory(world);
        let skills = world.resource::<TrustedSkillInventory>().0.clone();
        world.spawn((
            Name::new("prompt:skills"),
            PromptSection::new(
                PromptSection::ORDER_PROJECT + 10,
                "skills",
                listing(&skills),
            ),
        ));
    }
}

/// The prompt section listing the skills the model may pick, as Rig's.
pub fn listing(skills: &[Skill]) -> String {
    let entries: Vec<String> = skills
        .iter()
        .filter(|skill| !skill.manual)
        .map(|skill| {
            format!(
                "  <skill>\n    <name>{}</name>\n    <description>{}</description>\n    \
                 <location>{}</location>\n  </skill>",
                escape(&skill.name),
                escape(&skill.description),
                escape(&skill.file.display().to_string()),
            )
        })
        .collect();
    if entries.is_empty() {
        return String::new();
    }
    format!(
        "Skills give instructions for specific tasks. When a task matches a skill's \
         description, read the skill's file with the read tool before you start, and follow \
         it. A relative path in a skill is relative to the skill's directory, the directory \
         of its file.\n\n<available_skills>\n{}\n</available_skills>",
        entries.join("\n")
    )
}

fn escape(text: &str) -> String {
    text.replace('&', "&amp;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
        .replace('"', "&quot;")
}

fn first_line(text: &str) -> String {
    let line = text.lines().next().unwrap_or_default();
    match line.char_indices().nth(80) {
        Some((at, _)) => format!("{}…", &line[..at]),
        None => line.to_owned(),
    }
}

#[cfg(test)]
mod tests;
