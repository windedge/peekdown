//! File watcher module for automatic document refresh.

use notify::{Config, Event, EventKind, RecommendedWatcher, Watcher};
use smol::channel::{Receiver, Sender};
use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

/// Debounce duration for file change events (1 second)
const DEBOUNCE_DURATION: Duration = Duration::from_secs(1);

/// File watcher manager with reference counting and debouncing.
///
/// Manages file watchers for multiple tabs, ensuring:
/// - Same file opened in multiple tabs only creates one watcher
/// - Debouncing prevents rapid successive reloads
/// - Proper cleanup when tabs are closed
pub struct FileWatchManager {
    /// The underlying notify watcher (wrapped in Option for lazy init)
    watcher: Option<RecommendedWatcher>,

    /// Reference count for each watched path.
    /// Allows same file opened in multiple tabs without duplicate watches.
    watch_counts: HashMap<PathBuf, usize>,

    /// Pending debounce state per path (last event time)
    pending_events: Arc<Mutex<HashMap<PathBuf, Instant>>>,

    /// Channel to send debounced events to WorkspaceView
    event_tx: Sender<PathBuf>,

    /// Channel receiver (stored for WorkspaceView to consume)
    event_rx: Receiver<PathBuf>,

    /// Whether auto-refresh is enabled (from config)
    enabled: bool,
}

impl FileWatchManager {
    /// Create a new file watcher manager.
    pub fn new() -> Self {
        let (event_tx, event_rx) = smol::channel::unbounded();
        Self {
            watcher: None,
            watch_counts: HashMap::new(),
            pending_events: Arc::new(Mutex::new(HashMap::new())),
            event_tx,
            event_rx,
            enabled: true,
        }
    }

    /// Get the receiver for file change events.
    pub fn event_receiver(&self) -> Receiver<PathBuf> {
        self.event_rx.clone()
    }

    /// Enable or disable auto-refresh.
    pub fn set_enabled(&mut self, enabled: bool) {
        if self.enabled == enabled {
            return;
        }
        self.enabled = enabled;
        if !enabled {
            self.clear_all_watches();
        }
    }

    /// Start watching a file path (increments reference count).
    pub fn watch(&mut self, path: PathBuf) -> anyhow::Result<()> {
        if !self.enabled {
            return Ok(());
        }

        // Canonicalize path for consistent comparison
        let canonical_path = path.canonicalize().unwrap_or(path);

        // Increment reference count
        let count = self.watch_counts.entry(canonical_path.clone()).or_insert(0);
        *count += 1;

        // Only add watcher if this is the first reference
        if *count == 1 {
            self.ensure_watcher_initialized()?;
            if let Some(watcher) = &mut self.watcher {
                if let Err(e) = watcher.watch(&canonical_path, notify::RecursiveMode::NonRecursive) {
                    tracing::warn!("Failed to watch file {:?}: {}", canonical_path, e);
                    // Remove from counts since watch failed
                    self.watch_counts.remove(&canonical_path);
                    return Err(e.into());
                }
                tracing::debug!("Started watching: {:?}", canonical_path);
            }
        }

        Ok(())
    }

    /// Stop watching a file path (decrements reference count).
    pub fn unwatch(&mut self, path: &PathBuf) -> anyhow::Result<()> {
        let canonical_path = path.canonicalize().unwrap_or_else(|_| path.clone());

        if let Some(count) = self.watch_counts.get_mut(&canonical_path) {
            *count = count.saturating_sub(1);

            // Only remove watcher if no more references
            if *count == 0 {
                self.watch_counts.remove(&canonical_path);
                if let Some(watcher) = &mut self.watcher {
                    if let Err(e) = watcher.unwatch(&canonical_path) {
                        tracing::warn!("Failed to unwatch file {:?}: {}", canonical_path, e);
                    } else {
                        tracing::debug!("Stopped watching: {:?}", canonical_path);
                    }
                }
                // Also remove any pending events for this path
                if let Ok(mut pending) = self.pending_events.lock() {
                    pending.remove(&canonical_path);
                }
            }
        }

        Ok(())
    }

    /// Clear all watches (used when disabling auto-refresh).
    fn clear_all_watches(&mut self) {
        if let Some(watcher) = &mut self.watcher {
            for path in self.watch_counts.keys() {
                let _ = watcher.unwatch(path);
            }
        }
        self.watch_counts.clear();
        if let Ok(mut pending) = self.pending_events.lock() {
            pending.clear();
        }
    }

    /// Initialize the watcher lazily.
    fn ensure_watcher_initialized(&mut self) -> anyhow::Result<()> {
        if self.watcher.is_some() {
            return Ok(());
        }

        let pending_events = self.pending_events.clone();
        let event_tx = self.event_tx.clone();

        let watcher = RecommendedWatcher::new(
            move |result: Result<Event, notify::Error>| {
                if let Ok(event) = result {
                    Self::handle_raw_event(event, &pending_events, &event_tx);
                }
            },
            Config::default(),
        )?;

        self.watcher = Some(watcher);
        tracing::info!("File watcher initialized");
        Ok(())
    }

    /// Handle raw notify event with debouncing.
    fn handle_raw_event(
        event: Event,
        pending_events: &Arc<Mutex<HashMap<PathBuf, Instant>>>,
        event_tx: &Sender<PathBuf>,
    ) {
        // Only handle modify events
        if !matches!(event.kind, EventKind::Modify(_)) {
            return;
        }

        for path in event.paths {
            // Only watch markdown files
            let is_markdown = path
                .extension()
                .and_then(|e| e.to_str())
                .map(|e| e.eq_ignore_ascii_case("md") || e.eq_ignore_ascii_case("markdown"))
                .unwrap_or(false);

            if !is_markdown {
                continue;
            }

            let now = Instant::now();

            // Update pending event time
            if let Ok(mut pending) = pending_events.lock() {
                pending.insert(path.clone(), now);
            }

            // Clone for async task
            let path_clone = path.clone();
            let pending_clone = pending_events.clone();
            let tx_clone = event_tx.clone();

            // Spawn debounce timer task
            smol::spawn(async move {
                smol::Timer::after(DEBOUNCE_DURATION).await;

                // Check if this event is still the latest
                let should_emit = {
                    if let Ok(pending) = pending_clone.lock() {
                        if let Some(last_time) = pending.get(&path_clone) {
                            last_time.elapsed() >= DEBOUNCE_DURATION
                        } else {
                            false
                        }
                    } else {
                        false
                    }
                };

                if should_emit {
                    // Remove from pending and emit
                    if let Ok(mut pending) = pending_clone.lock() {
                        pending.remove(&path_clone);
                    }

                    if let Err(e) = tx_clone.try_send(path_clone.clone()) {
                        tracing::warn!("Failed to send file change event: {}", e);
                    } else {
                        tracing::debug!("File change detected: {:?}", path_clone);
                    }
                }
            })
            .detach();
        }
    }
}

impl Drop for FileWatchManager {
    fn drop(&mut self) {
        self.clear_all_watches();
    }
}
