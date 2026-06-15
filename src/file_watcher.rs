//! File watcher module for automatic refresh.
//!
//! Design goals:
//! - Prefer event-driven watching (low power) on Windows NTFS using ReadDirectoryChangesW
//!   via notify's recommended watcher.
//! - Coalesce bursts of events and refresh explorers by root, not by individual file.
//! - Respect .gitignore at scan time (Explorer uses ignore::WalkBuilder). For events,
//!   we only need to know which root was affected.

use notify::{Config, Event, EventKind, RecommendedWatcher, RecursiveMode, Watcher};
use smol::channel::{Receiver, Sender};
use std::collections::{HashMap, HashSet};
use std::path::{Path, PathBuf};
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc, Condvar, Mutex,
};
use std::thread;
use std::time::{Duration, Instant};

/// Coalesce filesystem events into a single UI refresh.
/// Zed uses ~100ms; we keep it slightly higher for a viewer.
const FS_WATCH_LATENCY: Duration = Duration::from_millis(200);

#[derive(Debug, Clone)]
pub enum WatchEvent {
    /// Markdown file's content changed (used for tab auto-refresh).
    FileModified(PathBuf),
    /// Worktree structure changed (used for explorer refresh).
    RootChanged(PathBuf),
}

/// Keeps a single OS watcher and multiplexes callbacks.
///
/// We watch explorer roots recursively. When any event arrives, we map it to the
/// deepest matching root and emit `RootChanged(root)` (coalesced).
///
/// We also optionally watch individual opened files for auto-refresh.
pub struct FileWatchManager {
    watcher: Option<RecommendedWatcher>,

    /// Opened file watches (non-recursive)
    file_watch_counts: HashMap<PathBuf, usize>,

    /// Explorer roots watched recursively
    watched_roots: HashSet<PathBuf>,

    /// A snapshot of watched roots used by the notify callback.
    ///
    /// notify requires a 'static callback, so we keep the roots in an Arc and update it
    /// whenever roots are added/removed.
    watched_roots_for_cb: Arc<Mutex<HashSet<PathBuf>>>,

    /// Pending events + condvar to avoid busy looping.
    pending: Arc<(Mutex<Pending>, Condvar)>,

    /// Worker thread shutdown flag.
    shutdown: Arc<AtomicBool>,

    /// Debounce worker thread handle.
    worker: Option<thread::JoinHandle<()>>,

    // Kept to ensure the channel stays alive for cloned senders.
    _event_tx: Sender<WatchEvent>,
    event_rx: Receiver<WatchEvent>,

    /// Auto-refresh opened markdown files.
    auto_refresh_files: Arc<AtomicBool>,
}

impl Drop for FileWatchManager {
    fn drop(&mut self) {
        self.shutdown.store(true, Ordering::Relaxed);
        // Wake the worker if it's sleeping.
        self.pending.1.notify_all();
        if let Some(handle) = self.worker.take() {
            let _ = handle.join();
        }
    }
}

#[derive(Default)]
struct Pending {
    roots: HashMap<PathBuf, Instant>,
    files: HashMap<PathBuf, Instant>,
}

impl FileWatchManager {
    pub fn new() -> Self {
        let (event_tx, event_rx) = smol::channel::unbounded();

        let pending = Arc::new((Mutex::new(Pending::default()), Condvar::new()));
        let shutdown = Arc::new(AtomicBool::new(false));
        let auto_refresh_files = Arc::new(AtomicBool::new(true));

        // Worker: coalesce roots/files and emit events after FS_WATCH_LATENCY.
        let pending_worker = pending.clone();
        let shutdown_worker = shutdown.clone();
        let event_tx_worker = event_tx.clone();
        let worker = thread::spawn(move || {
            let (lock, cv) = &*pending_worker;
            while !shutdown_worker.load(Ordering::Relaxed) {
                let mut guard = match lock.lock() {
                    Ok(g) => g,
                    Err(poisoned) => poisoned.into_inner(),
                };

                // Wait until we have something to process.
                while guard.roots.is_empty() && guard.files.is_empty() {
                    if shutdown_worker.load(Ordering::Relaxed) {
                        return;
                    }
                    guard = match cv.wait(guard) {
                        Ok(g) => g,
                        Err(poisoned) => poisoned.into_inner(),
                    };
                }

                let now = Instant::now();
                let mut due_root_keys = Vec::new();
                let mut due_file_keys = Vec::new();
                let mut next_deadline: Option<Instant> = None;

                for (root, t) in guard.roots.iter() {
                    let deadline = *t + FS_WATCH_LATENCY;
                    if now >= deadline {
                        due_root_keys.push(root.clone());
                    } else {
                        next_deadline = Some(next_deadline.map_or(deadline, |d| d.min(deadline)));
                    }
                }

                for (path, t) in guard.files.iter() {
                    let deadline = *t + FS_WATCH_LATENCY;
                    if now >= deadline {
                        due_file_keys.push(path.clone());
                    } else {
                        next_deadline = Some(next_deadline.map_or(deadline, |d| d.min(deadline)));
                    }
                }

                // If nothing is due yet, sleep until next deadline or until a new event arrives.
                if due_root_keys.is_empty() && due_file_keys.is_empty() {
                    if shutdown_worker.load(Ordering::Relaxed) {
                        return;
                    }

                    if let Some(deadline) = next_deadline {
                        let wait_dur = deadline.saturating_duration_since(now);
                        let (g, _) = match cv.wait_timeout(guard, wait_dur) {
                            Ok(v) => v,
                            Err(poisoned) => poisoned.into_inner(),
                        };
                        drop(g);
                        continue;
                    }

                    // Fallback: should not happen, but avoid tight loop.
                    let (g, _) = match cv.wait_timeout(guard, FS_WATCH_LATENCY) {
                        Ok(v) => v,
                        Err(poisoned) => poisoned.into_inner(),
                    };
                    drop(g);
                    continue;
                }

                for k in &due_root_keys {
                    guard.roots.remove(k);
                }
                for k in &due_file_keys {
                    guard.files.remove(k);
                }
                drop(guard);

                for root in due_root_keys {
                    let _ = event_tx_worker.try_send(WatchEvent::RootChanged(root));
                }
                for path in due_file_keys {
                    let _ = event_tx_worker.try_send(WatchEvent::FileModified(path));
                }
            }
        });

        Self {
            watcher: None,
            file_watch_counts: HashMap::new(),
            watched_roots: HashSet::new(),
            watched_roots_for_cb: Arc::new(Mutex::new(HashSet::new())),
            pending,
            shutdown,
            worker: Some(worker),
            _event_tx: event_tx,
            event_rx,
            auto_refresh_files,
        }
    }

    pub fn event_receiver(&self) -> Receiver<WatchEvent> {
        self.event_rx.clone()
    }

    pub fn set_enabled(&mut self, enabled: bool) {
        self.auto_refresh_files.store(enabled, Ordering::Relaxed);
    }

    pub fn watch_root(&mut self, root: PathBuf) -> anyhow::Result<()> {
        let root = root.canonicalize().unwrap_or(root);
        if self.watched_roots.contains(&root) {
            return Ok(());
        }

        self.ensure_watcher_initialized()?;
        if let Some(watcher) = &mut self.watcher {
            watcher.watch(&root, RecursiveMode::Recursive)?;
        }
        self.watched_roots.insert(root);

        if let Ok(mut roots) = self.watched_roots_for_cb.lock() {
            *roots = self.watched_roots.clone();
        }

        tracing::debug!("watch_root registered: {:?}", self.watched_roots);
        Ok(())
    }

    /// Force a manual explorer refresh event for a root (debug/diagnostics).
    pub fn trigger_root_refresh(&self, root: PathBuf) {
        queue_root_refresh(root, &self.pending);
    }

    pub fn watch(&mut self, path: PathBuf) -> anyhow::Result<()> {
        if !self.auto_refresh_files.load(Ordering::Relaxed) {
            return Ok(());
        }

        let path = path.canonicalize().unwrap_or(path);
        let count = self.file_watch_counts.entry(path.clone()).or_insert(0);
        *count += 1;
        if *count == 1 {
            self.ensure_watcher_initialized()?;
            if let Some(watcher) = &mut self.watcher {
                watcher.watch(&path, RecursiveMode::NonRecursive)?;
            }
        }
        Ok(())
    }

    pub fn unwatch(&mut self, path: &Path) -> anyhow::Result<()> {
        let path = path.canonicalize().unwrap_or_else(|_| path.to_path_buf());

        if self.watched_roots.remove(&path) {
            if let Some(watcher) = &mut self.watcher {
                let _ = watcher.unwatch(&path);
            }

            if let Ok(mut roots) = self.watched_roots_for_cb.lock() {
                *roots = self.watched_roots.clone();
            }

            if let Ok(mut pending) = self.pending.0.lock() {
                pending.roots.remove(&path);
            }
            return Ok(());
        }

        if let Some(count) = self.file_watch_counts.get_mut(&path) {
            *count = count.saturating_sub(1);
            if *count == 0 {
                self.file_watch_counts.remove(&path);
                if let Some(watcher) = &mut self.watcher {
                    let _ = watcher.unwatch(&path);
                }
                if let Ok(mut pending) = self.pending.0.lock() {
                    pending.files.remove(&path);
                }
            }
        }

        Ok(())
    }

    fn ensure_watcher_initialized(&mut self) -> anyhow::Result<()> {
        if self.watcher.is_some() {
            return Ok(());
        }

        let pending = self.pending.clone();
        let auto_refresh_files = self.auto_refresh_files.clone();

        let watched_roots_for_cb = self.watched_roots_for_cb.clone();

        let watcher = RecommendedWatcher::new(
            move |result: Result<Event, notify::Error>| {
                let Ok(event) = result else {
                    return;
                };

                tracing::debug!("fs event: kind={:?} paths={:?}", event.kind, event.paths);

                // Ignore access-only events; they are noisy and not useful for refresh.
                if matches!(event.kind, EventKind::Access(_)) {
                    return;
                }

                // Snapshot roots once per event.
                let roots_snapshot: Vec<PathBuf> = {
                    let roots = watched_roots_for_cb.lock().unwrap();
                    roots.iter().cloned().collect()
                };

                if roots_snapshot.is_empty() {
                    return;
                }

                // Some backends may emit events with no paths. In that case, refresh all roots.
                if event.paths.is_empty() {
                    for root in roots_snapshot {
                        queue_root_refresh(root, &pending);
                    }
                    return;
                }

                // Map event paths to roots/files.
                let mut any_unmatched = false;
                for p in &event.paths {
                    // Root refresh: pick deepest matching root.
                    let root = deepest_matching_root(p, roots_snapshot.iter());

                    if let Some(root) = root {
                        tracing::debug!("fs event mapped to root: {:?}", root);
                        queue_root_refresh(root, &pending);
                    } else {
                        any_unmatched = true;
                    }

                    // File refresh (only for markdown)
                    if auto_refresh_files.load(Ordering::Relaxed)
                        && matches!(event.kind, EventKind::Modify(_))
                        && is_markdown(p)
                    {
                        queue_file_refresh(p.clone(), &pending);
                    }
                }

                // If we couldn't map at least one path to a root, fall back to refreshing all roots.
                // This avoids missing updates due to Windows short paths / canonicalization mismatches.
                if any_unmatched {
                    for root in roots_snapshot {
                        queue_root_refresh(root, &pending);
                    }
                }
            },
            Config::default(),
        )?;

        self.watcher = Some(watcher);

        if let Ok(mut roots) = self.watched_roots_for_cb.lock() {
            *roots = self.watched_roots.clone();
        }

        Ok(())
    }
}

fn queue_root_refresh(root: PathBuf, pending: &Arc<(Mutex<Pending>, Condvar)>) {
    let now = Instant::now();
    let (lock, cv) = &**pending;
    if let Ok(mut guard) = lock.lock() {
        guard.roots.insert(root, now);
        cv.notify_one();
    }
}

fn queue_file_refresh(path: PathBuf, pending: &Arc<(Mutex<Pending>, Condvar)>) {
    let now = Instant::now();
    let (lock, cv) = &**pending;
    if let Ok(mut guard) = lock.lock() {
        guard.files.insert(path, now);
        cv.notify_one();
    }
}

fn deepest_matching_root<'a>(
    path: &Path,
    roots: impl Iterator<Item = &'a PathBuf>,
) -> Option<PathBuf> {
    let mut best: Option<(&'a PathBuf, usize)> = None;
    for root in roots {
        if path_starts_with_ci(path, root) {
            let depth = root.components().count();
            if best.is_none_or(|(_, best_depth)| depth > best_depth) {
                best = Some((root, depth));
            }
        }
    }
    best.map(|(r, _)| r.clone())
}

fn is_markdown(path: &Path) -> bool {
    path.extension()
        .and_then(|e| e.to_str())
        .map(|e| e.eq_ignore_ascii_case("md") || e.eq_ignore_ascii_case("markdown"))
        .unwrap_or(false)
}

#[cfg(windows)]
fn path_starts_with_ci(path: &Path, root: &Path) -> bool {
    let mut p = path.components();
    let mut r = root.components();
    loop {
        let Some(rc) = r.next() else {
            return true;
        };
        let Some(pc) = p.next() else {
            return false;
        };
        let r_str = rc.as_os_str().to_string_lossy();
        let p_str = pc.as_os_str().to_string_lossy();
        if !p_str.eq_ignore_ascii_case(&r_str) {
            return false;
        }
    }
}

#[cfg(not(windows))]
fn path_starts_with_ci(path: &Path, root: &Path) -> bool {
    path.starts_with(root)
}
