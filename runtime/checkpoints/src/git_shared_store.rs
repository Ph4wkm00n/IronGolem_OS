//! Git-backed shared content-addressable checkpoint store.
//!
//! `GitSharedStore` snapshots arbitrary project working directories into a
//! single shared **bare** git repository. Content-addressable deduplication
//! comes for free from git's object database: identical file contents across
//! projects (or across snapshots) are stored exactly once in `objects/`.
//!
//! Store layout (all under one configurable root, default
//! `<data_dir>/checkpoints/store`):
//!
//! - `objects/`, `refs/`, `HEAD`, ... — standard bare-repo plumbing
//! - `refs/irongolem/<hash16>` — per-project snapshot ref (linear history)
//! - `indexes/<hash16>` — per-project git index file
//! - `projects/<hash16>.json` — per-project metadata (workdir, created_at, last_touch)
//! - `info/exclude` — shared default excludes (node_modules, target, .env*, ...)
//!
//! `<hash16>` is the first 16 hex chars of `sha256(absolute workdir path)`.
//!
//! This is a Rust port of hermes-agent `tools/checkpoint_manager.py` (v2).
//! It intentionally does **not** implement the crate's `CheckpointStore`
//! trait: that trait is shaped around plan-state blobs keyed by plan UUIDs,
//! while this store snapshots filesystem trees keyed by workdir — the
//! semantics are equivalent (take / list / restore / prune) but the types are
//! not, so forcing the trait would be a lie. See `SqliteCheckpointStore` for
//! the plan-state backend.

use std::collections::HashSet;
use std::ffi::OsString;
use std::fmt::Write as _;
use std::io::Read;
use std::path::{Component, Path, PathBuf};
use std::process::{Command, Stdio};
use std::sync::OnceLock;
use std::time::{Duration, Instant};
use std::{env, fs, thread};

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use tracing::{debug, info};

/// Default per-file size limit; larger files are dropped from snapshots.
pub const DEFAULT_MAX_FILE_BYTES: u64 = 10 * 1024 * 1024;
/// Default workdir file-count limit; larger trees refuse to snapshot.
pub const DEFAULT_MAX_FILE_COUNT: usize = 50_000;
/// Default per-git-operation timeout.
pub const DEFAULT_OP_TIMEOUT: Duration = Duration::from_secs(30);

/// Errors produced by [`GitSharedStore`].
#[derive(Debug, thiserror::Error)]
pub enum GitStoreError {
    /// The `git` binary could not be found or executed.
    #[error("git binary is not available on PATH")]
    GitUnavailable,

    /// The workdir failed a safety or existence check.
    #[error("invalid workdir '{path}': {reason}")]
    InvalidWorkdir { path: String, reason: String },

    /// The workdir contains more files than the configured limit.
    #[error("workdir has more than {limit} files; refusing to snapshot")]
    TooManyFiles { limit: usize },

    /// The per-project ref moved between read and update (compare-and-swap
    /// failure). The caller may re-read the ref and retry; we never retry
    /// silently.
    #[error("concurrent update of {reference}: ref moved since it was read")]
    RefCasConflict { reference: String },

    /// A commit hash argument was not plain hex (or started with `-`).
    #[error("invalid commit hash: {0:?}")]
    InvalidCommit(String),

    /// A restore path was absolute or contained `..` traversal.
    #[error("invalid restore path: {0:?}")]
    InvalidPath(String),

    /// An argument was out of range (e.g. `keep_last == 0`).
    #[error("invalid argument: {0}")]
    InvalidArgument(String),

    /// A git subprocess exceeded the per-operation timeout and was killed.
    #[error("git command timed out after {seconds}s: {command}")]
    Timeout { seconds: u64, command: String },

    /// A git subprocess exited non-zero.
    #[error("git command failed ({command}): {stderr}")]
    GitCommand { command: String, stderr: String },

    /// Filesystem or process I/O failure.
    #[error("io error: {0}")]
    Io(#[from] std::io::Error),

    /// Metadata (de)serialization failure.
    #[error("metadata error: {0}")]
    Metadata(#[from] serde_json::Error),
}

impl From<GitStoreError> for irongolem_core::Error {
    fn from(err: GitStoreError) -> Self {
        irongolem_core::Error::Checkpoint {
            reason: err.to_string(),
        }
    }
}

/// Result alias for [`GitSharedStore`] operations.
pub type GitStoreResult<T> = std::result::Result<T, GitStoreError>;

/// One snapshot in a project's checkpoint history.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct CheckpointEntry {
    /// Full commit hash.
    pub hash: String,
    /// Abbreviated commit hash.
    pub short_hash: String,
    /// Author timestamp (strict ISO 8601, git `%aI`).
    pub timestamp: String,
    /// Snapshot reason (commit subject).
    pub subject: String,
}

/// Aggregate store statistics.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct StoreStatus {
    /// Number of projects with metadata in the store.
    pub project_count: usize,
    /// Approximate total on-disk size of the store, in bytes.
    pub disk_bytes: u64,
}

/// Per-project metadata persisted at `projects/<hash16>.json`.
#[derive(Debug, Clone, Serialize, Deserialize)]
struct ProjectMeta {
    workdir: String,
    created_at: String,
    last_touch: String,
}

/// Resolved per-project context (paths derived from the workdir hash).
struct ProjectCtx {
    workdir: PathBuf,
    refname: String,
    index_file: PathBuf,
    meta_file: PathBuf,
}

/// Captured output of a finished git subprocess.
struct GitOut {
    code: i32,
    stdout: String,
    stderr: String,
}

/// Git-backed shared checkpoint store. See module docs for the layout.
///
/// Construction never touches the filesystem or spawns git; the bare repo is
/// created lazily on first use, and a missing `git` binary surfaces as
/// [`GitStoreError::GitUnavailable`] from operations rather than a panic.
pub struct GitSharedStore {
    root: PathBuf,
    max_file_bytes: u64,
    max_file_count: usize,
    op_timeout: Duration,
    /// When set, overrides the child process `PATH` (used by tests to
    /// simulate a machine without git).
    path_env: Option<OsString>,
    /// Lazily-computed result of the `git --version` probe.
    git_available: OnceLock<bool>,
}

impl GitSharedStore {
    /// Create a store rooted at `root`. The directory (and bare repo inside
    /// it) is created lazily on first operation.
    pub fn new(root: impl Into<PathBuf>) -> Self {
        let root = root.into();
        // GIT_DIR must stay valid after we chdir into workdirs, so pin the
        // root to an absolute path up front.
        let root = if root.is_absolute() {
            root
        } else {
            env::current_dir().map(|d| d.join(&root)).unwrap_or(root)
        };
        Self {
            root,
            max_file_bytes: DEFAULT_MAX_FILE_BYTES,
            max_file_count: DEFAULT_MAX_FILE_COUNT,
            op_timeout: DEFAULT_OP_TIMEOUT,
            path_env: None,
            git_available: OnceLock::new(),
        }
    }

    /// Default store root: `<data_dir>/checkpoints/store`, where `<data_dir>`
    /// is `$IRONGOLEM_DATA_DIR` or `$HOME/.irongolem`.
    pub fn default_root() -> PathBuf {
        let data_dir = env::var_os("IRONGOLEM_DATA_DIR")
            .map(PathBuf::from)
            .or_else(|| env::var_os("HOME").map(|h| PathBuf::from(h).join(".irongolem")))
            .unwrap_or_else(|| PathBuf::from(".irongolem"));
        data_dir.join("checkpoints").join("store")
    }

    /// Override the per-file size limit (bytes) above which files are
    /// dropped from snapshots.
    pub fn with_max_file_bytes(mut self, bytes: u64) -> Self {
        self.max_file_bytes = bytes;
        self
    }

    /// Override the workdir file-count limit above which snapshots refuse.
    pub fn with_max_file_count(mut self, count: usize) -> Self {
        self.max_file_count = count;
        self
    }

    /// Override the per-git-operation timeout.
    pub fn with_op_timeout(mut self, timeout: Duration) -> Self {
        self.op_timeout = timeout;
        self
    }

    /// Override the `PATH` environment passed to git subprocesses. Intended
    /// for tests (pass an empty string to simulate git being absent).
    pub fn with_path_env(mut self, path: impl Into<OsString>) -> Self {
        self.path_env = Some(path.into());
        self
    }

    // ------------------------------------------------------------------
    // Public API
    // ------------------------------------------------------------------

    /// Snapshot `workdir`. Returns `Ok(Some(commit_hash))` for a new
    /// snapshot, or `Ok(None)` when nothing changed since the last one.
    pub fn take(&self, workdir: &Path, reason: &str) -> GitStoreResult<Option<String>> {
        let proj = self.project_ctx(workdir)?;
        self.ensure_git()?;
        self.ensure_initialized()?;
        self.check_file_count(&proj.workdir)?;
        let old = self.read_ref(&proj)?;
        self.take_inner(&proj, reason, old.as_deref())
    }

    /// List the most recent `limit` snapshots for `workdir`, newest first.
    pub fn list(&self, workdir: &Path, limit: usize) -> GitStoreResult<Vec<CheckpointEntry>> {
        let proj = self.project_ctx(workdir)?;
        self.ensure_git()?;
        self.ensure_initialized()?;
        if self.read_ref(&proj)?.is_none() {
            return Ok(Vec::new());
        }
        let out = self.run_ok(
            &[
                "log",
                &proj.refname,
                "--format=%H|%h|%aI|%s",
                "-n",
                &limit.to_string(),
            ],
            None,
            &[],
        )?;
        let mut entries = Vec::new();
        for line in out.stdout.lines() {
            let mut parts = line.splitn(4, '|');
            let (Some(hash), Some(short), Some(ts)) = (parts.next(), parts.next(), parts.next())
            else {
                continue;
            };
            entries.push(CheckpointEntry {
                hash: hash.to_string(),
                short_hash: short.to_string(),
                timestamp: ts.to_string(),
                subject: parts.next().unwrap_or_default().to_string(),
            });
        }
        Ok(entries)
    }

    /// Restore `workdir` (or a single relative `path` inside it) to the
    /// state captured by `commit`. A pre-restore snapshot is taken first so
    /// the restore itself can be undone.
    pub fn restore(&self, workdir: &Path, commit: &str, path: Option<&str>) -> GitStoreResult<()> {
        validate_commit(commit)?;
        if let Some(p) = path {
            validate_relative_path(p)?;
        }
        let proj = self.project_ctx(workdir)?;
        self.ensure_git()?;
        self.ensure_initialized()?;

        // "Undo the undo": snapshot current state before overwriting it.
        self.check_file_count(&proj.workdir)?;
        let old = self.read_ref(&proj)?;
        self.take_inner(&proj, "pre-restore snapshot", old.as_deref())?;

        let target = path.unwrap_or(".");
        // `--` prevents the commit or path from ever being parsed as a flag.
        self.run_ok(&["checkout", commit, "--", target], Some(&proj), &[])?;
        info!(commit, target, workdir = %proj.workdir.display(), "checkpoint restored");
        Ok(())
    }

    /// Keep only the most recent `keep_last` snapshots for `workdir`,
    /// rebuilding a linear chain of the kept trees and garbage-collecting
    /// unreachable objects.
    pub fn prune(&self, workdir: &Path, keep_last: usize) -> GitStoreResult<()> {
        if keep_last == 0 {
            return Err(GitStoreError::InvalidArgument(
                "keep_last must be at least 1".to_string(),
            ));
        }
        let proj = self.project_ctx(workdir)?;
        self.ensure_git()?;
        self.ensure_initialized()?;
        let Some(tip) = self.read_ref(&proj)? else {
            return Ok(()); // Nothing to prune.
        };
        let count: usize = self
            .run_ok(&["rev-list", "--count", &proj.refname], None, &[])?
            .stdout
            .trim()
            .parse()
            .unwrap_or(0);
        if count <= keep_last {
            return Ok(());
        }

        // Collect the kept commits (newest first), then rebuild the chain
        // oldest -> newest so the new tip has exactly `keep_last` ancestors.
        let out = self.run_ok(
            &[
                "log",
                &proj.refname,
                "--format=%H|%T|%aI|%s",
                "-n",
                &keep_last.to_string(),
            ],
            None,
            &[],
        )?;
        let mut kept: Vec<(String, String, String)> = Vec::new(); // (tree, date, subject)
        for line in out.stdout.lines() {
            let mut parts = line.splitn(4, '|');
            let (Some(_hash), Some(tree), Some(date)) = (parts.next(), parts.next(), parts.next())
            else {
                continue;
            };
            kept.push((
                tree.to_string(),
                date.to_string(),
                parts.next().unwrap_or("checkpoint").to_string(),
            ));
        }
        kept.reverse();

        let mut parent: Option<String> = None;
        for (tree, date, subject) in &kept {
            let subject = if subject.is_empty() {
                "checkpoint"
            } else {
                subject.as_str()
            };
            let mut args: Vec<String> = vec![
                "commit-tree".to_string(),
                "-m".to_string(),
                subject.to_string(),
                "--no-gpg-sign".to_string(),
            ];
            if let Some(p) = &parent {
                args.push("-p".to_string());
                args.push(p.clone());
            }
            args.push(tree.clone());
            let arg_refs: Vec<&str> = args.iter().map(String::as_str).collect();
            // Preserve the original timestamps on the rebuilt commits.
            let envs = [
                ("GIT_AUTHOR_DATE", date.as_str()),
                ("GIT_COMMITTER_DATE", date.as_str()),
            ];
            let out = self.run_ok(&arg_refs, None, &envs)?;
            parent = Some(out.stdout.trim().to_string());
        }
        let Some(new_tip) = parent else {
            return Ok(());
        };
        self.update_ref(&proj, &new_tip, Some(&tip))?;

        // Drop reflog entries and unreachable objects from the old chain.
        self.run_ok(&["reflog", "expire", "--expire=now", "--all"], None, &[])?;
        self.run_ok(&["gc", "--prune=now", "--quiet"], None, &[])?;
        // git 2.34+ treats a missing refs/heads or branches dir as "not a
        // git repository"; gc may have removed them from the bare store.
        fs::create_dir_all(self.root.join("refs").join("heads"))?;
        fs::create_dir_all(self.root.join("branches"))?;
        info!(keep_last, workdir = %proj.workdir.display(), "checkpoints pruned");
        Ok(())
    }

    /// Report the number of registered projects and the approximate on-disk
    /// size of the store.
    pub fn status(&self) -> GitStoreResult<StoreStatus> {
        let projects_dir = self.root.join("projects");
        let mut project_count = 0;
        if projects_dir.is_dir() {
            for entry in fs::read_dir(&projects_dir)? {
                let entry = entry?;
                if entry.path().extension().is_some_and(|e| e == "json") {
                    project_count += 1;
                }
            }
        }
        let disk_bytes = if self.root.is_dir() {
            dir_size(&self.root)?
        } else {
            0
        };
        Ok(StoreStatus {
            project_count,
            disk_bytes,
        })
    }

    // ------------------------------------------------------------------
    // Snapshot internals
    // ------------------------------------------------------------------

    /// Core snapshot algorithm, plumbing-only (never needs HEAD or a
    /// branch). `expected_old` is the ref value observed by the caller; it
    /// is passed to `update-ref` as the compare-and-swap guard.
    fn take_inner(
        &self,
        proj: &ProjectCtx,
        reason: &str,
        expected_old: Option<&str>,
    ) -> GitStoreResult<Option<String>> {
        // 1. Seed the per-project index from the previous snapshot so
        //    `add -A` computes a delta rather than rehashing history.
        match expected_old {
            Some(old) => self.run_ok(&["read-tree", old], Some(proj), &[])?,
            None => self.run_ok(&["read-tree", "--empty"], Some(proj), &[])?,
        };

        // 2. Stage everything (honors the shared info/exclude defaults).
        self.run_ok(&["add", "-A"], Some(proj), &[])?;

        // 3. Drop oversize files from the index.
        self.drop_oversize_files(proj)?;

        // 4. Skip no-change snapshots. `diff-index --cached --quiet` exits
        //    0 when the index matches the old tree, 1 when it differs.
        //    With no previous ref we always treat the tree as changed.
        if let Some(old) = expected_old {
            let out = self.run(&["diff-index", "--cached", "--quiet", old], Some(proj), &[])?;
            match out.code {
                0 => {
                    self.touch_meta(proj)?;
                    return Ok(None);
                }
                1 => {}
                _ => {
                    return Err(GitStoreError::GitCommand {
                        command: "diff-index".to_string(),
                        stderr: out.stderr.trim().to_string(),
                    });
                }
            }
        }

        // 5. Persist the tree and wrap it in a commit.
        let tree = self
            .run_ok(&["write-tree"], Some(proj), &[])?
            .stdout
            .trim()
            .to_string();
        let reason = if reason.is_empty() {
            "checkpoint"
        } else {
            reason
        };
        let mut args: Vec<&str> = vec!["commit-tree", "-m", reason, "--no-gpg-sign"];
        if let Some(old) = expected_old {
            args.push("-p");
            args.push(old);
        }
        args.push(&tree);
        let commit = self
            .run_ok(&args, Some(proj), &[])?
            .stdout
            .trim()
            .to_string();

        // 6. Advance the ref with compare-and-swap semantics.
        self.update_ref(proj, &commit, expected_old)?;
        self.touch_meta(proj)?;
        debug!(commit = %commit, workdir = %proj.workdir.display(), "checkpoint taken");
        Ok(Some(commit))
    }

    /// Remove index entries whose on-disk size exceeds `max_file_bytes`.
    fn drop_oversize_files(&self, proj: &ProjectCtx) -> GitStoreResult<()> {
        let out = self.run_ok(&["ls-files", "--cached", "-z"], Some(proj), &[])?;
        let oversize: Vec<&str> = out
            .stdout
            .split('\0')
            .filter(|p| !p.is_empty())
            .filter(|p| {
                // symlink_metadata: judge the entry itself, never its target.
                fs::symlink_metadata(proj.workdir.join(p))
                    .map(|m| m.is_file() && m.len() > self.max_file_bytes)
                    .unwrap_or(false)
            })
            .collect();
        // Batch to keep the argv comfortably under OS limits.
        for chunk in oversize.chunks(100) {
            let mut args: Vec<&str> = vec!["rm", "--cached", "--quiet", "--"];
            args.extend_from_slice(chunk);
            self.run_ok(&args, Some(proj), &[])?;
        }
        Ok(())
    }

    /// Read the current per-project ref, if any.
    fn read_ref(&self, proj: &ProjectCtx) -> GitStoreResult<Option<String>> {
        let target = format!("{}^{{commit}}", proj.refname);
        let out = self.run(&["rev-parse", "--verify", "--quiet", &target], None, &[])?;
        if out.code == 0 {
            Ok(Some(out.stdout.trim().to_string()))
        } else {
            Ok(None)
        }
    }

    /// Compare-and-swap update of the per-project ref. `expected_old` of
    /// `None` requires that the ref does not exist yet (git treats an empty
    /// old-value as "must not exist").
    fn update_ref(
        &self,
        proj: &ProjectCtx,
        new: &str,
        expected_old: Option<&str>,
    ) -> GitStoreResult<()> {
        let old = expected_old.unwrap_or("");
        let out = self.run(&["update-ref", &proj.refname, new, old], None, &[])?;
        if out.code == 0 {
            return Ok(());
        }
        let stderr = out.stderr.to_ascii_lowercase();
        if stderr.contains("cannot lock ref")
            || stderr.contains("expected")
            || stderr.contains("unable to update")
        {
            Err(GitStoreError::RefCasConflict {
                reference: proj.refname.clone(),
            })
        } else {
            Err(GitStoreError::GitCommand {
                command: "update-ref".to_string(),
                stderr: out.stderr.trim().to_string(),
            })
        }
    }

    /// Create or refresh `projects/<hash16>.json`.
    fn touch_meta(&self, proj: &ProjectCtx) -> GitStoreResult<()> {
        let now = chrono::Utc::now().to_rfc3339();
        let created_at = fs::read_to_string(&proj.meta_file)
            .ok()
            .and_then(|s| serde_json::from_str::<ProjectMeta>(&s).ok())
            .map(|m| m.created_at)
            .unwrap_or_else(|| now.clone());
        let meta = ProjectMeta {
            workdir: proj.workdir.display().to_string(),
            created_at,
            last_touch: now,
        };
        fs::write(&proj.meta_file, serde_json::to_vec_pretty(&meta)?)?;
        Ok(())
    }

    // ------------------------------------------------------------------
    // Store bootstrap and guards
    // ------------------------------------------------------------------

    /// Lazy `git --version` probe. The result is cached; a missing binary
    /// makes every operation fail with `GitUnavailable` instead of panicking.
    fn ensure_git(&self) -> GitStoreResult<()> {
        let available = *self.git_available.get_or_init(|| {
            let mut cmd = self.bare_git_command();
            cmd.arg("--version");
            matches!(self.wait_with_timeout(cmd, "git --version"), Ok(out) if out.code == 0)
        });
        if available {
            Ok(())
        } else {
            Err(GitStoreError::GitUnavailable)
        }
    }

    /// Create the bare repo, shared excludes, and layout dirs on first use.
    fn ensure_initialized(&self) -> GitStoreResult<()> {
        if !self.root.join("HEAD").exists() {
            fs::create_dir_all(&self.root)?;
            self.run_ok(&["init", "--bare", "--quiet"], None, &[])?;
            info!(root = %self.root.display(), "initialized shared checkpoint store");
        }
        let exclude = self.root.join("info").join("exclude");
        if !exclude.exists() {
            if let Some(parent) = exclude.parent() {
                fs::create_dir_all(parent)?;
            }
            fs::write(&exclude, DEFAULT_EXCLUDES)?;
        }
        fs::create_dir_all(self.root.join("indexes"))?;
        fs::create_dir_all(self.root.join("projects"))?;
        Ok(())
    }

    /// Validate the workdir and derive per-project paths from its hash.
    fn project_ctx(&self, workdir: &Path) -> GitStoreResult<ProjectCtx> {
        let invalid = |reason: &str| GitStoreError::InvalidWorkdir {
            path: workdir.display().to_string(),
            reason: reason.to_string(),
        };
        let meta = fs::metadata(workdir).map_err(|_| invalid("does not exist"))?;
        if !meta.is_dir() {
            return Err(invalid("not a directory"));
        }
        let canon = fs::canonicalize(workdir).map_err(GitStoreError::Io)?;
        if canon == Path::new("/") {
            return Err(invalid("refusing to snapshot the filesystem root"));
        }
        if let Some(home) = env::var_os("HOME") {
            let home = PathBuf::from(home);
            let home = fs::canonicalize(&home).unwrap_or(home);
            if canon == home {
                return Err(invalid("refusing to snapshot the home directory"));
            }
        }
        let hash = hash16(&canon);
        Ok(ProjectCtx {
            refname: format!("refs/irongolem/{hash}"),
            index_file: self.root.join("indexes").join(&hash),
            meta_file: self.root.join("projects").join(format!("{hash}.json")),
            workdir: canon,
        })
    }

    /// Refuse to snapshot pathological trees; walks with early exit.
    fn check_file_count(&self, workdir: &Path) -> GitStoreResult<()> {
        let mut count = 0usize;
        let mut stack = vec![workdir.to_path_buf()];
        while let Some(dir) = stack.pop() {
            let entries = match fs::read_dir(&dir) {
                Ok(e) => e,
                Err(_) => continue, // Unreadable dirs are git's problem, not a fatal guard.
            };
            for entry in entries.flatten() {
                let Ok(ft) = entry.file_type() else { continue };
                if ft.is_dir() {
                    stack.push(entry.path()); // Never follows symlinked dirs.
                } else {
                    count += 1;
                    if count > self.max_file_count {
                        return Err(GitStoreError::TooManyFiles {
                            limit: self.max_file_count,
                        });
                    }
                }
            }
        }
        Ok(())
    }

    // ------------------------------------------------------------------
    // Git subprocess plumbing
    // ------------------------------------------------------------------

    /// Base git command with full environment isolation.
    ///
    /// GIT_CONFIG_GLOBAL / GIT_CONFIG_SYSTEM are pointed at the OS null
    /// device and GIT_CONFIG_NOSYSTEM is set so the user's gitconfig can
    /// never leak in: settings like `commit.gpgsign` or credential helpers
    /// would otherwise hang snapshots on interactive prompts. Identity is
    /// injected via env because no config file exists to provide it.
    fn bare_git_command(&self) -> Command {
        let devnull = if cfg!(windows) { "NUL" } else { "/dev/null" };
        let mut cmd = Command::new("git");
        cmd.env("GIT_CONFIG_GLOBAL", devnull)
            .env("GIT_CONFIG_SYSTEM", devnull)
            .env("GIT_CONFIG_NOSYSTEM", "1")
            .env("GIT_TERMINAL_PROMPT", "0")
            .env("GIT_AUTHOR_NAME", "IronGolem Checkpoints")
            .env("GIT_AUTHOR_EMAIL", "checkpoints@irongolem.local")
            .env("GIT_COMMITTER_NAME", "IronGolem Checkpoints")
            .env("GIT_COMMITTER_EMAIL", "checkpoints@irongolem.local");
        if let Some(path) = &self.path_env {
            cmd.env("PATH", path);
        }
        cmd
    }

    /// Run git against the store; `proj` adds the per-project index and
    /// worktree environment (and runs with the workdir as cwd).
    fn run(
        &self,
        args: &[&str],
        proj: Option<&ProjectCtx>,
        extra_env: &[(&str, &str)],
    ) -> GitStoreResult<GitOut> {
        let mut cmd = self.bare_git_command();
        cmd.env("GIT_DIR", &self.root);
        if let Some(proj) = proj {
            cmd.env("GIT_INDEX_FILE", &proj.index_file);
            cmd.env("GIT_WORK_TREE", &proj.workdir);
            cmd.current_dir(&proj.workdir);
        }
        for (k, v) in extra_env {
            cmd.env(k, v);
        }
        cmd.args(args);
        let desc = format!("git {}", args.first().unwrap_or(&""));
        self.wait_with_timeout(cmd, &desc)
    }

    /// Like [`run`](Self::run), but any non-zero exit is an error.
    fn run_ok(
        &self,
        args: &[&str],
        proj: Option<&ProjectCtx>,
        extra_env: &[(&str, &str)],
    ) -> GitStoreResult<GitOut> {
        let out = self.run(args, proj, extra_env)?;
        if out.code == 0 {
            Ok(out)
        } else {
            Err(GitStoreError::GitCommand {
                command: format!("git {}", args.join(" ")),
                stderr: out.stderr.trim().to_string(),
            })
        }
    }

    /// Spawn, drain pipes on background threads (avoids pipe-buffer
    /// deadlock), and poll for exit with a deadline. std has no built-in
    /// wait-with-timeout, so this is a simple kill-on-deadline poll loop.
    fn wait_with_timeout(&self, mut cmd: Command, desc: &str) -> GitStoreResult<GitOut> {
        cmd.stdin(Stdio::null())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());
        let mut child = match cmd.spawn() {
            Ok(child) => child,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => {
                return Err(GitStoreError::GitUnavailable);
            }
            Err(e) => return Err(GitStoreError::Io(e)),
        };
        let stdout_thread = child.stdout.take().map(|mut pipe| {
            thread::spawn(move || {
                let mut buf = Vec::new();
                let _ = pipe.read_to_end(&mut buf);
                buf
            })
        });
        let stderr_thread = child.stderr.take().map(|mut pipe| {
            thread::spawn(move || {
                let mut buf = Vec::new();
                let _ = pipe.read_to_end(&mut buf);
                buf
            })
        });
        let deadline = Instant::now() + self.op_timeout;
        let status = loop {
            match child.try_wait() {
                Ok(Some(status)) => break status,
                Ok(None) => {
                    if Instant::now() >= deadline {
                        let _ = child.kill();
                        let _ = child.wait();
                        return Err(GitStoreError::Timeout {
                            seconds: self.op_timeout.as_secs(),
                            command: desc.to_string(),
                        });
                    }
                    thread::sleep(Duration::from_millis(15));
                }
                Err(e) => return Err(GitStoreError::Io(e)),
            }
        };
        let collect = |handle: Option<thread::JoinHandle<Vec<u8>>>| {
            handle
                .and_then(|h| h.join().ok())
                .map(|b| String::from_utf8_lossy(&b).into_owned())
                .unwrap_or_default()
        };
        Ok(GitOut {
            code: status.code().unwrap_or(-1),
            stdout: collect(stdout_thread),
            stderr: collect(stderr_thread),
        })
    }
}

/// First 16 hex chars of sha256 over the absolute workdir path.
fn hash16(path: &Path) -> String {
    let digest = Sha256::digest(path.as_os_str().as_encoded_bytes());
    let mut out = String::with_capacity(16);
    for byte in digest.iter().take(8) {
        let _ = write!(out, "{byte:02x}");
    }
    out
}

/// Commit hashes must be plain hex; a leading `-` (impossible for hex, but
/// checked explicitly) would otherwise be parsed by git as a flag.
fn validate_commit(commit: &str) -> GitStoreResult<()> {
    if commit.is_empty()
        || commit.starts_with('-')
        || !commit.chars().all(|c| c.is_ascii_hexdigit())
    {
        return Err(GitStoreError::InvalidCommit(commit.to_string()));
    }
    Ok(())
}

/// Restore paths must stay inside the workdir: relative, no `..` components.
fn validate_relative_path(path: &str) -> GitStoreResult<()> {
    let p = Path::new(path);
    if path.is_empty() || p.is_absolute() {
        return Err(GitStoreError::InvalidPath(path.to_string()));
    }
    for component in p.components() {
        match component {
            Component::ParentDir | Component::RootDir | Component::Prefix(_) => {
                return Err(GitStoreError::InvalidPath(path.to_string()));
            }
            Component::Normal(_) | Component::CurDir => {}
        }
    }
    Ok(())
}

/// Approximate recursive size of a directory in bytes (does not follow
/// symlinks).
fn dir_size(root: &Path) -> std::io::Result<u64> {
    let mut total = 0u64;
    let mut seen_dirs: HashSet<PathBuf> = HashSet::new();
    let mut stack = vec![root.to_path_buf()];
    while let Some(dir) = stack.pop() {
        if !seen_dirs.insert(dir.clone()) {
            continue;
        }
        let entries = match fs::read_dir(&dir) {
            Ok(e) => e,
            Err(_) => continue,
        };
        for entry in entries.flatten() {
            let Ok(meta) = entry.metadata() else { continue };
            if meta.is_dir() {
                stack.push(entry.path());
            } else {
                total = total.saturating_add(meta.len());
            }
        }
    }
    Ok(total)
}

/// Shared default excludes written to `<store>/info/exclude`; applies to
/// every project snapshotted through the store.
const DEFAULT_EXCLUDES: &str = "\
# IronGolem shared checkpoint store default excludes
node_modules/
target/
dist/
build/
.next/
.cache/
.env
.env.*
.venv/
venv/
__pycache__/
*.pyc
*.log
logs/
*.mp4
*.mov
*.avi
*.mkv
*.webm
*.mp3
*.wav
*.flac
*.iso
*.dmg
.DS_Store
";

#[cfg(test)]
mod tests {
    use super::*;
    use std::process::Command;
    use std::sync::atomic::{AtomicU64, Ordering};

    /// Minimal tempdir helper (avoids adding a tempfile dependency).
    struct TestDir(PathBuf);

    impl TestDir {
        fn new(tag: &str) -> Self {
            static COUNTER: AtomicU64 = AtomicU64::new(0);
            let path = env::temp_dir().join(format!(
                "igck-{tag}-{}-{}",
                std::process::id(),
                COUNTER.fetch_add(1, Ordering::SeqCst)
            ));
            fs::create_dir_all(&path).unwrap();
            TestDir(fs::canonicalize(&path).unwrap())
        }

        fn path(&self) -> &Path {
            &self.0
        }
    }

    impl Drop for TestDir {
        fn drop(&mut self) {
            let _ = fs::remove_dir_all(&self.0);
        }
    }

    fn store_and_root() -> (GitSharedStore, TestDir) {
        let root = TestDir::new("store");
        let store = GitSharedStore::new(root.path().join("bare"));
        (store, root)
    }

    /// Deterministic incompressible-ish bytes so dedup assertions are
    /// meaningful (zlib would crush repeated bytes to nothing).
    fn pseudo_random_bytes(len: usize, seed: u64) -> Vec<u8> {
        let mut state = seed;
        (0..len)
            .map(|_| {
                state = state
                    .wrapping_mul(6364136223846793005)
                    .wrapping_add(1442695040888963407);
                (state >> 33) as u8
            })
            .collect()
    }

    /// Run raw git against the store for test assertions.
    fn raw_git(store_root: &Path, args: &[&str]) -> String {
        let out = Command::new("git")
            .env("GIT_DIR", store_root)
            .env("GIT_CONFIG_NOSYSTEM", "1")
            .args(args)
            .output()
            .unwrap();
        assert!(
            out.status.success(),
            "git {args:?} failed: {}",
            String::from_utf8_lossy(&out.stderr)
        );
        String::from_utf8_lossy(&out.stdout).into_owned()
    }

    fn loose_object_kib(store_root: &Path) -> u64 {
        let out = raw_git(store_root, &["count-objects", "-v"]);
        for line in out.lines() {
            if let Some(rest) = line.strip_prefix("size: ") {
                return rest.trim().parse().unwrap();
            }
        }
        panic!("no size line in count-objects output: {out}");
    }

    #[test]
    fn dedup_identical_content_across_projects() {
        let (store, root) = store_and_root();
        let proj_a = TestDir::new("proj-a");
        let proj_b = TestDir::new("proj-b");
        let payload = pseudo_random_bytes(1024 * 1024, 42);
        fs::write(proj_a.path().join("data.bin"), &payload).unwrap();
        fs::write(proj_b.path().join("data.bin"), &payload).unwrap();

        store.take(proj_a.path(), "first project").unwrap().unwrap();
        let size_after_a = loose_object_kib(&store.root);
        store
            .take(proj_b.path(), "second project")
            .unwrap()
            .unwrap();
        let size_after_b = loose_object_kib(&store.root);

        // The ~1MB blob must be stored once; project B only adds a tree and
        // a commit object (a few KiB at most).
        assert!(
            size_after_a > 500,
            "expected ~1MB blob, got {size_after_a} KiB"
        );
        assert!(
            size_after_b < size_after_a + 100,
            "object store nearly doubled: {size_after_a} -> {size_after_b} KiB"
        );
        let _ = root;
    }

    #[test]
    fn take_returns_none_when_nothing_changed() {
        let (store, _root) = store_and_root();
        let proj = TestDir::new("proj");
        fs::write(proj.path().join("a.txt"), "hello").unwrap();

        let first = store.take(proj.path(), "initial").unwrap();
        assert!(first.is_some());
        let second = store.take(proj.path(), "no changes").unwrap();
        assert_eq!(second, None);
    }

    #[test]
    fn contended_cas_surfaces_typed_conflict() {
        let (store, _root) = store_and_root();
        let proj = TestDir::new("proj");
        fs::write(proj.path().join("a.txt"), "v1").unwrap();
        let c1 = store.take(proj.path(), "v1").unwrap().unwrap();

        // Simulate contention: after "reading" c1, another writer moves the
        // ref (this second take plays that role)...
        fs::write(proj.path().join("a.txt"), "v2").unwrap();
        store
            .take(proj.path(), "v2 (concurrent writer)")
            .unwrap()
            .unwrap();

        // ...then our stale writer attempts CAS against c1 and must get a
        // typed conflict, not a silent retry.
        fs::write(proj.path().join("a.txt"), "v3").unwrap();
        let ctx = store.project_ctx(proj.path()).unwrap();
        let err = store
            .take_inner(&ctx, "stale writer", Some(&c1))
            .unwrap_err();
        assert!(
            matches!(err, GitStoreError::RefCasConflict { .. }),
            "expected RefCasConflict, got: {err:?}"
        );
    }

    #[test]
    fn restore_round_trip_with_pre_restore_snapshot() {
        let (store, _root) = store_and_root();
        let proj = TestDir::new("proj");
        let file = proj.path().join("a.txt");
        fs::write(&file, "original").unwrap();
        let c1 = store.take(proj.path(), "original").unwrap().unwrap();

        fs::write(&file, "mutated").unwrap();
        store.restore(proj.path(), &c1, None).unwrap();

        assert_eq!(fs::read_to_string(&file).unwrap(), "original");
        // History: original snapshot + the automatic pre-restore snapshot.
        let entries = store.list(proj.path(), 10).unwrap();
        assert_eq!(entries.len(), 2);
        assert_eq!(entries[0].subject, "pre-restore snapshot");
        assert_eq!(entries[1].hash, c1);
    }

    #[test]
    fn prune_keeps_last_n() {
        let (store, _root) = store_and_root();
        let proj = TestDir::new("proj");
        let file = proj.path().join("a.txt");
        for i in 0..4 {
            fs::write(&file, format!("version {i}")).unwrap();
            store
                .take(proj.path(), &format!("take {i}"))
                .unwrap()
                .unwrap();
        }
        assert_eq!(store.list(proj.path(), 10).unwrap().len(), 4);

        store.prune(proj.path(), 2).unwrap();

        let entries = store.list(proj.path(), 10).unwrap();
        assert_eq!(entries.len(), 2);
        assert_eq!(entries[0].subject, "take 3");
        assert_eq!(entries[1].subject, "take 2");
        // gc must not have left the store looking like "not a git repo".
        assert!(store.root.join("refs").join("heads").is_dir());
        assert!(store.root.join("branches").is_dir());
        // Restoring the surviving tip still works after gc.
        fs::write(&file, "scratch").unwrap();
        store.restore(proj.path(), &entries[0].hash, None).unwrap();
        assert_eq!(fs::read_to_string(&file).unwrap(), "version 3");
    }

    #[test]
    fn oversize_files_excluded_and_inputs_validated() {
        let (store, _root) = store_and_root();
        let proj = TestDir::new("proj");
        fs::write(proj.path().join("small.txt"), "small").unwrap();
        fs::write(
            proj.path().join("big.bin"),
            pseudo_random_bytes(11 * 1024 * 1024, 7),
        )
        .unwrap();

        let commit = store.take(proj.path(), "with big file").unwrap().unwrap();
        let tree = raw_git(&store.root, &["ls-tree", "-r", "--name-only", &commit]);
        assert!(tree.contains("small.txt"));
        assert!(
            !tree.contains("big.bin"),
            "oversize file leaked into snapshot"
        );

        // Flag-injection and traversal guards.
        let err = store.restore(proj.path(), "-abcdef", None).unwrap_err();
        assert!(matches!(err, GitStoreError::InvalidCommit(_)));
        let err = store
            .restore(proj.path(), &commit, Some("../escape.txt"))
            .unwrap_err();
        assert!(matches!(err, GitStoreError::InvalidPath(_)));
    }

    #[test]
    fn git_absent_returns_typed_error_without_panic() {
        let root = TestDir::new("store");
        let proj = TestDir::new("proj");
        fs::write(proj.path().join("a.txt"), "hello").unwrap();
        // Construction must succeed even when git cannot be found.
        let store = GitSharedStore::new(root.path().join("bare")).with_path_env("");

        let err = store.take(proj.path(), "no git").unwrap_err();
        assert!(matches!(err, GitStoreError::GitUnavailable), "got: {err:?}");
        let err = store.list(proj.path(), 5).unwrap_err();
        assert!(matches!(err, GitStoreError::GitUnavailable), "got: {err:?}");
    }

    #[test]
    fn guards_reject_root_home_and_missing_workdirs() {
        let (store, _root) = store_and_root();
        assert!(matches!(
            store.take(Path::new("/"), "nope").unwrap_err(),
            GitStoreError::InvalidWorkdir { .. }
        ));
        if let Some(home) = env::var_os("HOME") {
            assert!(matches!(
                store.take(Path::new(&home), "nope").unwrap_err(),
                GitStoreError::InvalidWorkdir { .. }
            ));
        }
        assert!(matches!(
            store
                .take(Path::new("/definitely/not/a/real/dir"), "nope")
                .unwrap_err(),
            GitStoreError::InvalidWorkdir { .. }
        ));
    }

    #[test]
    fn status_reports_projects_and_disk_usage() {
        let (store, _root) = store_and_root();
        let proj_a = TestDir::new("proj-a");
        let proj_b = TestDir::new("proj-b");
        fs::write(proj_a.path().join("a.txt"), "a").unwrap();
        fs::write(proj_b.path().join("b.txt"), "b").unwrap();
        store.take(proj_a.path(), "a").unwrap();
        store.take(proj_b.path(), "b").unwrap();

        let status = store.status().unwrap();
        assert_eq!(status.project_count, 2);
        assert!(status.disk_bytes > 0);
    }

    #[test]
    fn file_count_guard_trips() {
        let (store, _root) = store_and_root();
        let store = store.with_max_file_count(3);
        let proj = TestDir::new("proj");
        for i in 0..5 {
            fs::write(proj.path().join(format!("f{i}.txt")), "x").unwrap();
        }
        let err = store.take(proj.path(), "too many").unwrap_err();
        assert!(matches!(err, GitStoreError::TooManyFiles { .. }));
    }
}
