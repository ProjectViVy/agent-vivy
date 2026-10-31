//! Prompt templates, as Pi has them: each `.md` file directly in
//! `.rig/prompts` of the project root or in `RIG_HOME/prompts` is a
//! `/<file name>` command that sends the file's text, its arguments
//! substituted, as the user's message. Of two templates with one name the
//! project's wins; a name another command already has is skipped with a
//! warning.
//!
//! ```markdown
//! ---
//! description: Review staged git changes
//! argument-hint: "[focus]"
//! ---
//! Review the staged changes. Focus on ${1:-correctness and error handling}.
//! ```
//!
//! Substitutions: `$1`, `$2`, …; `$@` and `$ARGUMENTS` (all, joined by
//! spaces); `${1:-default}`, `${@:-default}`, `${ARGUMENTS:-default}`;
//! `${@:N}` (from the Nth) and `${@:N:L}` (L from the Nth). Arguments split
//! at spaces outside `"…"` or `'…'`. Values are not substituted again.
//!
//! Unlike Pi, a project's templates load without a trust step, as Rig's
//! project skills do: a template only sends text the user can read, as the
//! user's message.

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use bevy_log::warn;
use rig_harness::prelude::*;
use serde::Deserialize;

/// The longest description shown, in characters.
const MAX_DESCRIPTION: usize = 60;

/// Adds a command per prompt template.
#[derive(Default)]
pub struct PromptTemplatesPlugin;

impl Plugin for PromptTemplatesPlugin {
    fn build(&self, app: &mut App) {
        app.init_resource::<ProjectRoot>();
    }

    // In `finish`, so every other plugin's commands exist and keep their names.
    fn finish(&self, app: &mut App) {
        let world = app.world_mut();
        let home = RigHome::of(world);
        let project = world.resource::<ProjectRoot>().0.clone();
        let roots: Vec<PathBuf> = [project.join(".rig/prompts"), home.root().join("prompts")]
            .into_iter()
            .filter(|root| crate::trust::allowed(world, root))
            .collect();
        let (templates, problems) = discover(&roots);
        for problem in problems {
            warn!("prompt templates: {problem}");
        }
        for template in templates.into_values() {
            let help = match &template.argument_hint {
                Some(hint) => format!("{} {hint}", template.description),
                None => template.description.clone(),
            };
            let name = template.name.clone();
            app.add_command(
                &name,
                &help,
                move |In(args): In<CommandArgs>, mut commands: Commands| {
                    let text = substitute(&template.body, &parse_args(&args.args));
                    let author = Author::user().titled(format!("template {}", template.name));
                    let message = Incoming::new(text, args.mode).with_author(author);
                    commands.spawn((message, ForAgent(args.agent)));
                },
            );
        }
    }
}

/// A template found on disk.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Template {
    /// The command name: the file name without `.md`.
    pub name: String,
    /// The frontmatter's `description`, else the body's first line.
    pub description: String,
    /// The frontmatter's `argument-hint`.
    pub argument_hint: Option<String>,
    /// The text after the frontmatter.
    pub body: String,
}

#[derive(Deserialize, Default)]
#[serde(default, rename_all = "kebab-case")]
struct Frontmatter {
    description: Option<String>,
    argument_hint: Option<String>,
}

/// The templates under `roots` by name, the first root's winning, and what
/// was wrong with the files skipped.
pub fn discover(roots: &[PathBuf]) -> (BTreeMap<String, Template>, Vec<String>) {
    let mut templates = BTreeMap::new();
    let mut problems = Vec::new();
    for root in roots {
        let Ok(entries) = std::fs::read_dir(root) else {
            continue;
        };
        let mut files: Vec<PathBuf> = entries
            .filter_map(|entry| entry.ok().map(|entry| entry.path()))
            .filter(|path| path.extension().is_some_and(|ext| ext == "md") && path.is_file())
            .collect();
        files.sort();
        for file in files {
            match load(&file) {
                Ok(template) => {
                    templates.entry(template.name.clone()).or_insert(template);
                }
                Err(why) => problems.push(format!("{}: {why}", file.display())),
            }
        }
    }
    (templates, problems)
}

/// The template in `file`.
pub fn load(file: &Path) -> Result<Template, String> {
    let name = file
        .file_stem()
        .and_then(|stem| stem.to_str())
        .filter(|name| !name.is_empty() && !name.contains(char::is_whitespace))
        .ok_or("the file name is not a command name")?
        .to_owned();
    let text = std::fs::read_to_string(file).map_err(|error| error.to_string())?;
    parse(name, &text)
}

/// The template `name` whose file holds `text`.
pub fn parse(name: String, text: &str) -> Result<Template, String> {
    let (frontmatter, body) = split::<Frontmatter>(text)?;
    let description = frontmatter
        .description
        .filter(|description| !description.trim().is_empty())
        .unwrap_or_else(|| {
            let first = body
                .lines()
                .find(|line| !line.trim().is_empty())
                .unwrap_or("");
            let first = first.trim();
            match first.char_indices().nth(MAX_DESCRIPTION) {
                Some((at, _)) => format!("{}...", &first[..at]),
                None => first.to_owned(),
            }
        });
    Ok(Template {
        name,
        description,
        argument_hint: frontmatter.argument_hint,
        body: body.to_owned(),
    })
}

/// The frontmatter, when the text starts with `---`, and the body.
pub(crate) fn split<T: serde::de::DeserializeOwned + Default>(
    text: &str,
) -> Result<(T, &str), String> {
    let Some(rest) = text
        .strip_prefix("---\n")
        .or_else(|| text.strip_prefix("---\r\n"))
    else {
        return Ok((T::default(), text));
    };
    if rest == "---" || rest.starts_with("---\n") || rest.starts_with("---\r\n") {
        return Ok((T::default(), rest[3..].trim_start_matches(['\r', '\n'])));
    }
    let end = rest
        .match_indices("\n---")
        .find(|(at, _)| {
            let after = &rest[at + 4..];
            after.is_empty() || after.starts_with('\n') || after.starts_with("\r\n")
        })
        .map(|(at, _)| at)
        .ok_or("the frontmatter has no closing `---`")?;
    let yaml = &rest[..end];
    let body = rest[end + 4..].trim_start_matches(['\r', '\n']);
    let frontmatter = if yaml.trim().is_empty() {
        T::default()
    } else {
        serde_saphyr::from_str(yaml).map_err(|error| format!("frontmatter: {error}"))?
    };
    Ok((frontmatter, body))
}

/// The arguments in `line`: split at whitespace outside `"…"` and `'…'`,
/// the quotes removed; `""` is an empty argument. As Pi's.
pub fn parse_args(line: &str) -> Vec<String> {
    let mut args = Vec::new();
    let mut current = String::new();
    let mut started = false;
    let mut quote: Option<char> = None;
    for c in line.chars() {
        match quote {
            Some(open) if c == open => quote = None,
            Some(_) => current.push(c),
            None if c == '"' || c == '\'' => {
                quote = Some(c);
                started = true;
            }
            None if c.is_whitespace() => {
                if started {
                    args.push(std::mem::take(&mut current));
                    started = false;
                }
            }
            None => {
                current.push(c);
                started = true;
            }
        }
    }
    if started {
        args.push(current);
    }
    args
}

/// `template` with its placeholders replaced by `args`, as Pi's
/// `substituteArgs`: one pass over the template, values left as they are.
pub fn substitute(template: &str, args: &[String]) -> String {
    let all = args.join(" ");
    let nth = |n: usize| {
        n.checked_sub(1)
            .and_then(|i| args.get(i))
            .map_or("", String::as_str)
    };
    let mut out = String::with_capacity(template.len());
    let mut rest = template;
    while let Some(at) = rest.find('$') {
        out.push_str(&rest[..at]);
        let after = &rest[at + 1..];
        let Some((placeholder, used)) = placeholder(after) else {
            out.push('$');
            rest = after;
            continue;
        };
        match placeholder {
            Placeholder::All => out.push_str(&all),
            Placeholder::Nth(n) => out.push_str(nth(n)),
            Placeholder::Default { of, default } => {
                let value = of.map_or(all.as_str(), nth);
                out.push_str(if value.is_empty() { default } else { value });
            }
            Placeholder::Slice { start, len } => {
                // 1-based, with 0 taken as 1, as in bash.
                let start = start.saturating_sub(1).min(args.len());
                let end = len.map_or(args.len(), |len| start.saturating_add(len).min(args.len()));
                out.push_str(&args[start..end].join(" "));
            }
        }
        rest = &after[used..];
    }
    out.push_str(rest);
    out
}

/// A placeholder: `$@`/`$ARGUMENTS`, `$N`, `${N:-d}`/`${@:-d}` (`of` is
/// `None` for all), or `${@:N}`/`${@:N:L}`.
enum Placeholder<'a> {
    All,
    Nth(usize),
    Default { of: Option<usize>, default: &'a str },
    Slice { start: usize, len: Option<usize> },
}

/// The leading run of ASCII digits of `text`, as a number, and its length.
fn number(text: &str) -> Option<(usize, usize)> {
    let len = text.bytes().take_while(u8::is_ascii_digit).count();
    Some((text[..len].parse().ok()?, len))
}

/// The placeholder `text` (what follows a `$`) starts with, and how many
/// bytes of `text` it takes; `None` leaves the `$` as it is.
fn placeholder(text: &str) -> Option<(Placeholder<'_>, usize)> {
    let Some(inner) = text.strip_prefix('{') else {
        if text.starts_with("ARGUMENTS") {
            return Some((Placeholder::All, "ARGUMENTS".len()));
        }
        if text.starts_with('@') {
            return Some((Placeholder::All, 1));
        }
        let (n, len) = number(text)?;
        return Some((Placeholder::Nth(n), len));
    };
    let body = &inner[..inner.find('}')?];
    let used = body.len() + 2;
    for all in ["@", "ARGUMENTS"] {
        if let Some(default) = body.strip_prefix(all).and_then(|b| b.strip_prefix(":-")) {
            return Some((Placeholder::Default { of: None, default }, used));
        }
    }
    if let Some((n, len)) = number(body)
        && let Some(default) = body[len..].strip_prefix(":-")
    {
        return Some((
            Placeholder::Default {
                of: Some(n),
                default,
            },
            used,
        ));
    }
    let slice = body.strip_prefix("@:")?;
    let (start, len) = number(slice)?;
    let len = match &slice[len..] {
        "" => None,
        rest => {
            let rest = rest.strip_prefix(':')?;
            let (count, digits) = number(rest)?;
            (digits == rest.len()).then_some(Some(count))?
        }
    };
    Some((Placeholder::Slice { start, len }, used))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn s(template: &str, args: &[&str]) -> String {
        let args: Vec<String> = args.iter().map(|a| a.to_string()).collect();
        substitute(template, &args)
    }

    #[test]
    fn splits_arguments_like_pi() {
        assert_eq!(
            parse_args(r#"a "b c" 'd e' "" f"#),
            ["a", "b c", "d e", "", "f"]
        );
        assert_eq!(parse_args("  "), Vec::<String>::new());
        assert_eq!(parse_args(r#"x"y z"w"#), ["xy zw"]);
    }

    #[test]
    fn substitutes_positional_and_all() {
        assert_eq!(s("$1-$2-$3", &["a", "b"]), "a-b-");
        assert_eq!(s("$@|$ARGUMENTS", &["a", "b"]), "a b|a b");
        assert_eq!(s("$0 $10", &["a"]), " ");
        assert_eq!(s("cost $ 5 and $x", &[]), "cost $ 5 and $x");
    }

    #[test]
    fn substitutes_defaults() {
        assert_eq!(s("${1:-none}", &[]), "none");
        assert_eq!(s("${1:-none}", &[""]), "none");
        assert_eq!(s("${2:-none}", &["a", "b"]), "b");
        assert_eq!(s("${@:-all}|${ARGUMENTS:-all}", &[]), "all|all");
        assert_eq!(s("${@:-all}", &["x", "y"]), "x y");
    }

    #[test]
    fn substitutes_slices() {
        let args = ["a", "b", "c", "d"];
        assert_eq!(s("${@:2}", &args), "b c d");
        assert_eq!(s("${@:2:2}", &args), "b c");
        assert_eq!(s("${@:0}", &args), "a b c d");
        assert_eq!(s("${@:9}", &args), "");
        assert_eq!(s("${@:3:9}", &args), "c d");
    }

    #[test]
    fn does_not_substitute_values_again() {
        assert_eq!(s("$1 $2", &["$2", "x"]), "$2 x");
    }

    #[test]
    fn leaves_unknown_braces() {
        assert_eq!(s("${HOME} ${@:x}", &["a"]), "${HOME} ${@:x}");
    }

    #[test]
    fn reads_frontmatter_and_description() {
        let text = "---\ndescription: Review changes\nargument-hint: \"[focus]\"\n---\nReview ${1:-all}.\n";
        let template = parse("review".into(), text).unwrap();
        assert_eq!(template.description, "Review changes");
        assert_eq!(template.argument_hint.as_deref(), Some("[focus]"));
        assert_eq!(template.body, "Review ${1:-all}.\n");

        let template = parse("plain".into(), "\n  First line here\nmore").unwrap();
        assert_eq!(template.description, "First line here");
        assert!(parse("bad".into(), "---\ndescription: x\n").is_err());
    }

    #[test]
    fn reads_empty_frontmatter() {
        for text in ["---\n---\nBody", "---\r\n---\r\nBody"] {
            assert_eq!(parse("empty".into(), text).unwrap().body, "Body");
        }
    }

    #[test]
    fn discovers_with_the_first_root_winning() {
        let dir = std::env::temp_dir().join(format!("vivy-templates-{}", std::process::id()));
        let (project, user) = (dir.join("project"), dir.join("user"));
        std::fs::create_dir_all(&project).unwrap();
        std::fs::create_dir_all(&user).unwrap();
        std::fs::write(project.join("review.md"), "Project review").unwrap();
        std::fs::write(user.join("review.md"), "User review").unwrap();
        std::fs::write(user.join("fix.md"), "Fix $1").unwrap();
        std::fs::write(user.join("notes.txt"), "not a template").unwrap();
        let (found, problems) = discover(&[project, user, dir.join("missing")]);
        std::fs::remove_dir_all(&dir).unwrap();
        assert!(problems.is_empty(), "{problems:?}");
        assert_eq!(found.keys().collect::<Vec<_>>(), ["fix", "review"]);
        assert_eq!(found["review"].body, "Project review");
    }
}
