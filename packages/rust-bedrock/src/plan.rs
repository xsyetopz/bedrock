//! The changes adopt makes, which it can print instead of applying.

use std::fs;
use std::io;
use std::os::unix::fs::{PermissionsExt, symlink};
use std::path::Path;

#[derive(Debug, PartialEq, Eq)]
pub(crate) enum Step {
    Write {
        path: String,
        content: String,
        executable: bool,
    },
    Remove {
        path: String,
    },
    Link {
        path: String,
        target: &'static str,
    },
    /// Renames a file, so it needs no backup. `content` replaces its text when the
    /// rename changes what its relative paths mean.
    Move {
        from: String,
        path: String,
        content: Option<String>,
    },
}

impl Step {
    pub(crate) fn write(path: impl Into<String>, content: impl Into<String>) -> Self {
        Self::Write {
            path: path.into(),
            content: content.into(),
            executable: false,
        }
    }

    pub(crate) fn path(&self) -> &str {
        match self {
            Self::Write { path, .. }
            | Self::Remove { path }
            | Self::Link { path, .. }
            | Self::Move { path, .. } => path,
        }
    }

    pub(crate) fn describe(&self, root: &Path) -> String {
        match self {
            Self::Write { path, .. } if root.join(path).exists() => {
                format!("update  {path} (kept as {path}.bak)")
            }
            Self::Write { path, .. } => format!("create  {path}"),
            Self::Remove { path } => format!("remove  {path} (kept as {path}.bak)"),
            Self::Link { path, target } => format!("link    {path} -> {target}"),
            Self::Move { from, path, .. } => format!("move    {from} -> {path}"),
        }
    }

    /// Whether `root` already matches this step.
    pub(crate) fn is_done(&self, root: &Path) -> bool {
        let path = root.join(self.path());
        match self {
            Self::Write {
                content,
                executable,
                ..
            } => {
                fs::read_to_string(&path).is_ok_and(|current| current == *content)
                    && (!executable || is_executable(&path))
            }
            Self::Remove { .. } => fs::symlink_metadata(&path).is_err(),
            Self::Link { target, .. } => {
                fs::read_link(&path).is_ok_and(|current| current == Path::new(target))
            }
            Self::Move { from, .. } => fs::symlink_metadata(root.join(from)).is_err(),
        }
    }

    /// Applies the step, keeping a file it replaces or removes as `<file>.bak`. An
    /// existing backup stays, since it holds the file from before the first adopt.
    pub(crate) fn apply(&self, root: &Path) -> io::Result<()> {
        let path = root.join(self.path());
        if let Self::Move { from, content, .. } = self {
            fs::rename(root.join(from), &path)?;
            return content
                .as_ref()
                .map_or(Ok(()), |text| fs::write(&path, text));
        }
        if let Ok(meta) = fs::symlink_metadata(&path) {
            let backup = root.join(format!("{}.bak", self.path()));
            if fs::symlink_metadata(&backup).is_err() {
                fs::rename(&path, backup)?;
            } else if meta.is_dir() {
                fs::remove_dir_all(&path)?;
            } else {
                fs::remove_file(&path)?;
            }
        }
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent)?;
        }
        match self {
            Self::Write {
                content,
                executable,
                ..
            } => {
                fs::write(&path, content)?;
                if *executable {
                    fs::set_permissions(&path, fs::Permissions::from_mode(0o755))?;
                }
            }
            Self::Remove { .. } | Self::Move { .. } => {}
            Self::Link { target, .. } => symlink(target, &path)?,
        }
        Ok(())
    }
}

fn is_executable(path: &Path) -> bool {
    fs::metadata(path).is_ok_and(|meta| meta.permissions().mode() & 0o111 != 0)
}

/// Drops the steps `root` already matches.
pub(crate) fn pending(root: &Path, steps: Vec<Step>) -> Vec<Step> {
    steps
        .into_iter()
        .filter(|step| !step.is_done(root))
        .collect()
}

#[cfg(test)]
mod tests;
