//! User-owned imports. Source directories are never edited or removed.
use super::{Skill, discover, roots};
use crate::setup::fingerprint;
use std::{
    fs, io,
    path::{Path, PathBuf},
};

pub struct Document {
    pub name: String,
    pub description: String,
    pub text: String,
    pub revision: String,
    pub editable: bool,
}
fn err(text: impl ToString) -> io::Error {
    io::Error::other(text.to_string())
}
fn name(value: &str) -> io::Result<()> {
    if value.is_empty()
        || value.len() > 64
        || !value
            .bytes()
            .all(|c| c.is_ascii_lowercase() || c.is_ascii_digit() || c == b'-')
    {
        return Err(err("invalid skill name"));
    }
    Ok(())
}
fn reparse(path: &Path) -> io::Result<()> {
    let metadata = fs::symlink_metadata(path)?;
    #[cfg(windows)]
    {
        use std::os::windows::fs::MetadataExt;
        if metadata.file_attributes() & 0x400 != 0 {
            return Err(err("skill paths cannot contain reparse points"));
        }
    }
    if metadata.file_type().is_symlink() {
        return Err(err("skill paths cannot contain symlinks"));
    }
    Ok(())
}
fn boundary(path: &Path, base: &Path) -> io::Result<()> {
    if !path.canonicalize()?.starts_with(base.canonicalize()?) {
        return Err(err("skill path escapes its root"));
    }
    let relative = path.strip_prefix(base).map_err(err)?;
    let mut current = base.to_path_buf();
    reparse(&current)?;
    for part in relative.components() {
        current.push(part);
        reparse(&current)?;
    }
    Ok(())
}
fn inventory(root: &Path, home: &Path, trusted: bool) -> Vec<Skill> {
    let places = if trusted {
        roots(root, home)
    } else {
        std::iter::once(home.join("skills"))
            .chain(std::env::home_dir().map(|p| p.join(".agents/skills")))
            .collect()
    };
    discover(&places).0
}
fn internal(home: &Path) -> io::Result<PathBuf> {
    let base = home.join("skills");
    fs::create_dir_all(&base)?;
    reparse(&base)?;
    Ok(base)
}
fn lock(home: &Path) -> io::Result<fs::File> {
    fs::create_dir_all(home)?;
    let file = fs::OpenOptions::new()
        .create(true)
        .truncate(false)
        .write(true)
        .open(home.join("skills.lock"))?;
    file.lock()?;
    Ok(file)
}
pub fn list(root: &Path, home: &Path, trusted: bool) -> io::Result<Vec<Document>> {
    let base = home.join("skills");
    inventory(root, home, trusted)
        .into_iter()
        .map(|s| {
            let text = fs::read_to_string(&s.file)?;
            if text.len() > 256 * 1024 {
                return Err(err("skill Markdown exceeds 256 KiB"));
            }
            let editable = s.file.parent() == Some(base.join(&s.name).as_path())
                && boundary(&s.file, &base).is_ok();
            Ok(Document {
                name: s.name,
                description: s.description,
                revision: fingerprint(text.as_bytes()),
                text,
                editable,
            })
        })
        .collect()
}
fn copy(
    source: &Path,
    destination: &Path,
    base: &Path,
    count: &mut usize,
    bytes: &mut u64,
) -> io::Result<()> {
    boundary(source, base)?;
    let metadata = fs::metadata(source)?;
    *count += 1;
    if *count > 2048 {
        return Err(err("skill import exceeds 2048 files"));
    }
    if metadata.is_dir() {
        fs::create_dir(destination)?;
        for entry in fs::read_dir(source)? {
            let entry = entry?;
            copy(
                &entry.path(),
                &destination.join(entry.file_name()),
                base,
                count,
                bytes,
            )?;
        }
    } else if metadata.is_file() {
        *bytes += metadata.len();
        if *bytes > 16 * 1024 * 1024 {
            return Err(err("skill import exceeds 16 MiB"));
        }
        fs::copy(source, destination)?;
    } else {
        return Err(err("skill import requires regular files and directories"));
    }
    Ok(())
}
pub fn import(
    root: &Path,
    home: &Path,
    trusted: bool,
    source: &Path,
    expected_name: &str,
) -> io::Result<()> {
    name(expected_name)?;
    let _lock = lock(home)?;
    let base = internal(home)?;
    if inventory(root, home, trusted)
        .iter()
        .any(|s| s.name == expected_name)
        || base.join(expected_name).exists()
    {
        return Err(err("skill name already exists"));
    }
    reparse(source)?;
    let source = source.canonicalize()?;
    if base.canonicalize()?.starts_with(&source) || source.starts_with(base.canonicalize()?) {
        return Err(err("import source overlaps internal skills"));
    }
    let recovery = home.join("skill-recovery");
    fs::create_dir_all(&recovery)?;
    reparse(&recovery)?;
    let stamp = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map_err(err)?
        .as_nanos();
    let stage = recovery.join(format!("import-{stamp}-{expected_name}"));
    copy(&source, &stage, &source, &mut 0, &mut 0)?;
    let (skills, issues) = discover(&[stage.clone()]);
    if !issues.is_empty()
        || skills.len() != 1
        || skills[0].name != expected_name
        || skills[0].file != stage.join("SKILL.md")
    {
        return Err(err(format!(
            "import must contain one valid root SKILL.md with the requested name: {}",
            issues.join("; ")
        )));
    }
    fs::rename(stage, base.join(expected_name))
}
pub fn save(home: &Path, skill: &str, revision: &str, text: &str) -> io::Result<()> {
    name(skill)?;
    if text.len() > 256 * 1024 {
        return Err(err("skill Markdown exceeds 256 KiB"));
    }
    let _lock = lock(home)?;
    let base = internal(home)?;
    let file = base.join(skill).join("SKILL.md");
    boundary(&file, &base)?;
    if fingerprint(&fs::read(&file)?) != revision {
        return Err(err("skill changed; reload before saving"));
    }
    // Validate using the same parser before replacing the live Markdown.
    let recovery = home.join("skill-recovery");
    fs::create_dir_all(&recovery)?;
    reparse(&recovery)?;
    let stage = recovery.join(format!("edit-{}", fingerprint(text.as_bytes())));
    fs::create_dir_all(&stage)?;
    fs::write(stage.join("SKILL.md"), text)?;
    let (skills, issues) = discover(&[stage]);
    if !issues.is_empty() || skills.len() != 1 || skills[0].name != skill {
        return Err(err(
            "edited Markdown must preserve the skill name and valid frontmatter",
        ));
    }
    rig_harness::rig_tools::fs::write_atomic(&file, text.as_bytes())
}
pub fn remove(home: &Path, skill: &str, revision: &str) -> io::Result<()> {
    name(skill)?;
    let _lock = lock(home)?;
    let base = internal(home)?;
    let directory = base.join(skill);
    boundary(&directory.join("SKILL.md"), &base)?;
    if fingerprint(&fs::read(directory.join("SKILL.md"))?) != revision {
        return Err(err("skill changed; reload before removing"));
    }
    let recovery = home.join("skill-recovery");
    fs::create_dir_all(&recovery)?;
    reparse(&recovery)?;
    let stamp = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map_err(err)?
        .as_nanos();
    fs::rename(directory, recovery.join(format!("removed-{stamp}-{skill}")))
}
