//! `vivy`: builds a Rig agent from a YAML recipe.
//!
//! ```text
//! vivy generate <recipe.yaml>   write .generated/<name>/
//! vivy build    <recipe.yaml>   generate, then cargo build; prints the binary
//! ```
//!
//! The agent binary is then run directly; its runtime settings, such as the
//! gateway, are in `RIG_HOME/settings.toml`.

mod generate;
mod launch;
mod recipe;
mod rig_source;

use std::error::Error;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, ExitCode};

use recipe::Recipe;

type Result<T> = std::result::Result<T, Box<dyn Error>>;

const USAGE: &str = "usage: vivy [--resume <id>] [--model <model>] [-p prompt | --json]\n       vivy generate|build <recipe.yaml> [--rig-path <checkout>]\n       vivy run <recipe.yaml> [--rig-path <checkout>] -- [agent arguments]";

struct RecipeCommand {
    verb: String,
    path: PathBuf,
    rig: Option<PathBuf>,
    args: Vec<String>,
}

fn parse_recipe_command(args: &[String]) -> Result<Option<RecipeCommand>> {
    let Some(verb) = args
        .first()
        .filter(|v| matches!(v.as_str(), "generate" | "build" | "run"))
    else {
        return Ok(None);
    };
    let path = args.get(1).filter(|p| !p.starts_with('-')).ok_or(USAGE)?;
    let mut command = RecipeCommand {
        verb: verb.clone(),
        path: path.into(),
        rig: None,
        args: Vec::new(),
    };
    let mut rest = args.iter().skip(2);
    while let Some(arg) = rest.next() {
        match arg.as_str() {
            "--rig-path" if command.rig.is_none() => {
                command.rig = Some(rest.next().ok_or("--rig-path needs a checkout")?.into())
            }
            "--" if verb == "run" => {
                command.args.extend(rest.cloned());
                break;
            }
            _ => return Err(USAGE.into()),
        }
    }
    Ok(Some(command))
}

fn recipe_command(command: RecipeCommand) -> Result<u8> {
    let mut agent = Agent::load(&command.path)?;
    if let Some(path) = command.rig {
        agent.rig = Some(rig_source::DevRig::discover(&path)?);
        agent.isolate_development();
    }
    if command.verb == "run" {
        if let Some(rig) = &agent.rig {
            let recorded=fs::read_to_string(agent.package.join("development-source.json"))
                .map_err(|_|"local Rig build metadata is missing; build this recipe with --rig-path first")?;
            let identity: serde_json::Value = serde_json::from_str(&recorded)?;
            if identity["rig_root"] != serde_json::json!(rig.root) {
                return Err("the development binary was built from a different Rig checkout; rebuild it first".into());
            }
        }
        return launch::run(&agent.binary(), &command.args);
    }
    agent.generate()?;
    let output = if command.verb == "build" {
        agent.build()?
    } else {
        agent.package
    };
    println!("{}", output.display());
    Ok(0)
}

fn main() -> ExitCode {
    let args: Vec<String> = std::env::args().skip(1).collect();
    match parse_recipe_command(&args) {
        Ok(Some(command)) => {
            return match recipe_command(command) {
                Ok(code) => ExitCode::from(code),
                Err(error) => {
                    eprintln!("vivy: {error}");
                    ExitCode::FAILURE
                }
            };
        }
        Err(error) => {
            eprintln!("vivy: {error}");
            return ExitCode::FAILURE;
        }
        Ok(None) => {}
    }
    if matches!(args.as_slice(), [flag] if flag == "--help" || flag == "-h") {
        println!("{USAGE}");
        return ExitCode::SUCCESS;
    }
    match Agent::load(&repository_root().join("recipes/coding-agent.yaml"))
        .and_then(|agent| launch::run(&agent.binary(), &args))
    {
        Ok(code) => ExitCode::from(code),
        Err(error) => {
            eprintln!("vivy: {error}");
            ExitCode::FAILURE
        }
    }
}

/// One recipe and where its generated package, lock and binary live.
struct Agent {
    rig: Option<rig_source::DevRig>,
    recipe: Recipe,
    origin: String,
    directory: PathBuf,
    /// The lock kept next to the recipe, so a build is reproducible.
    lock: PathBuf,
    package: PathBuf,
    target: PathBuf,
}

impl Agent {
    fn load(path: &Path) -> Result<Self> {
        let text = fs::read_to_string(path)
            .map_err(|error| format!("cannot read recipe {}: {error}", path.display()))?;
        let recipe =
            Recipe::parse(&text).map_err(|error| format!("recipe {}: {error}", path.display()))?;
        let absolute = std::path::absolute(path)?;
        // Generated packages share one target directory, so a changed
        // selection rebuilds only what changed.
        let generated = repository_root().join(".generated");
        Ok(Self {
            rig: None,
            package: generated.join(&recipe.name),
            target: target_directory(std::env::var_os("CARGO_TARGET_DIR"))?,
            lock: path.with_extension("Cargo.lock"),
            origin: path.display().to_string(),
            directory: absolute.parent().unwrap_or(Path::new("/")).to_path_buf(),
            recipe,
        })
    }

    fn generate(&self) -> Result<()> {
        generate::write(&self.package, &self.recipe, &self.origin, &self.directory)?;
        if let Some(rig) = &self.rig {
            let mut manifest = generate::manifest(&self.recipe, &self.origin, &self.directory);
            manifest.push_str(&rig.patches(&self.recipe.rig.git));
            generate::write_if_changed(&self.package.join("Cargo.toml"), &manifest)?;
            let original = format!("git {}?rev={}", self.recipe.rig.git, self.recipe.rig.rev);
            let local = format!("local Rig {}", rig.root.display());
            let main = generate::main_rs(&self.recipe, &self.origin)
                .replace(&format!("{original:?}"), &format!("{local:?}"));
            generate::write_if_changed(&self.package.join("src/main.rs"), &main)?;
        }
        if self.lock.is_file() {
            let lock = fs::read_to_string(&self.lock)?;
            generate::write_if_changed(&self.package.join("Cargo.lock"), &lock)?;
        }
        Ok(())
    }

    /// Builds the generated package and returns the agent binary.
    fn build(&self) -> Result<PathBuf> {
        if self.rig.is_none()
            && self.recipe.components.iter().any(|c| {
                c.features
                    .get("vivy-durable")
                    .is_some_and(|f| f.iter().any(|f| f == "runtime" || f == "checkpoint-probe"))
            })
        {
            return Err("this experimental checkpoint recipe requires --rig-path pointing to the local Rig fork".into());
        }
        if let Some(rig) = &self.rig {
            rig.verify(&self.package.join("Cargo.toml"))?;
        }
        let mut command = cargo_command(
            std::env::var_os("CARGO").unwrap_or_else(|| "cargo".into()),
            &self.package,
            &self.target,
            std::env::vars_os().map(|(key, _)| key),
        );
        let names =
            vivy_process::variables_from_file(&rig_harness_protocol::Home::from_env().settings());
        vivy_process::filter_credentials(&mut command, &names);
        let status = command
            .status()
            .map_err(|error| format!("cannot start cargo: {error}"))?;
        if !status.success() {
            return Err(format!(
                "cargo build of {} failed ({status}); its output above names the cause",
                self.package.display()
            )
            .into());
        }
        // Keep the resolved versions with the recipe.
        let lock = fs::read_to_string(self.package.join("Cargo.lock"))?;
        generate::write_if_changed(&self.lock, &lock)?;
        if let Some(rig) = &self.rig {
            fs::write(
                self.package.join("development-source.json"),
                serde_json::to_vec_pretty(
                    &serde_json::json!({"rig_root":rig.root,"recipe":self.origin}),
                )?,
            )?;
        }
        Ok(self.binary())
    }

    fn binary(&self) -> PathBuf {
        self.target.join("debug").join(format!(
            "{}{}",
            self.recipe.name,
            std::env::consts::EXE_SUFFIX
        ))
    }

    fn isolate_development(&mut self) {
        self.package = self
            .package
            .with_file_name(format!("{}-rig-dev", self.recipe.name));
        self.lock = self.package.join("Cargo.lock");
        self.target = self.target.join("rig-dev");
    }
}

fn target_directory(override_path: Option<std::ffi::OsString>) -> std::io::Result<PathBuf> {
    override_path.map_or_else(
        || Ok(repository_root().join(".generated/target")),
        std::path::absolute,
    )
}

fn cargo_command(
    cargo: impl AsRef<std::ffi::OsStr>,
    package: &Path,
    target: &Path,
    variables: impl IntoIterator<Item = impl AsRef<std::ffi::OsStr>>,
) -> Command {
    let mut command = Command::new(cargo);
    command
        .arg("build")
        .current_dir(package)
        .env("CARGO_TARGET_DIR", target);
    for name in variables {
        if name
            .as_ref()
            .to_string_lossy()
            .to_ascii_uppercase()
            .starts_with("RIG_SECRET_")
        {
            command.env_remove(name);
        }
    }
    command
}

/// This repository's root: `vivy` is run from its own checkout.
fn repository_root() -> PathBuf {
    let manifest = Path::new(env!("CARGO_MANIFEST_DIR"));
    manifest
        .ancestors()
        .nth(2)
        .unwrap_or(manifest)
        .to_path_buf()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn recipe_commands_keep_program_arguments_and_development_source_separate() {
        let args = [
            "run",
            "agent.yaml",
            "--rig-path",
            "local-rig",
            "--",
            "--resume",
            "a",
        ]
        .map(str::to_owned);
        let command = parse_recipe_command(&args).unwrap().unwrap();
        assert_eq!(command.rig, Some(PathBuf::from("local-rig")));
        assert_eq!(command.args, ["--resume", "a"]);
        assert!(
            parse_recipe_command(&["build".into(), "a.yaml".into(), "--unknown".into()]).is_err()
        );
        assert!(
            parse_recipe_command(&["--resume".into(), "a".into()])
                .unwrap()
                .is_none()
        );
    }

    #[test]
    fn development_artifacts_do_not_replace_the_recipe_lock_or_default_binary() {
        let recipe = repository_root().join("recipes/coding-agent.yaml");
        let mut agent = Agent::load(&recipe).unwrap();
        let lock = agent.lock.clone();
        let binary = agent.binary();
        agent.isolate_development();
        assert_ne!(agent.lock, lock);
        assert_ne!(agent.binary(), binary);
        assert!(agent.package.ends_with("coding-agent-rig-dev"));
    }

    #[test]
    fn build_command_removes_secrets() {
        let command = cargo_command(
            "cargo",
            Path::new("."),
            Path::new("target"),
            ["RIG_SECRET_GATEWAY", "PATH"],
        );
        assert!(
            command
                .get_envs()
                .any(|(key, value)| key == "RIG_SECRET_GATEWAY" && value.is_none())
        );
        assert!(!command.get_envs().any(|(key, _)| key == "PATH"));
    }

    #[test]
    fn target_override_is_resolved_before_changing_directory() {
        let cwd = std::env::current_dir().unwrap();
        assert_eq!(
            target_directory(Some("short-target".into())).unwrap(),
            cwd.join("short-target")
        );
    }
}
