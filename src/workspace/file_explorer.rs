//! File explorer sidebar component for browsing markdown files.

use gpui::*;
use gpui::prelude::FluentBuilder;
use gpui_component::{ActiveTheme, scroll::ScrollableElement, v_flex, h_flex, button::Button, button::ButtonVariants, Icon, IconName, Sizable, menu::DropdownMenu, menu::PopupMenuItem, menu::ContextMenuExt, tooltip::Tooltip};
use std::path::PathBuf;
use std::collections::{HashMap, HashSet};
use std::rc::Rc;
use crate::state::config::{ExplorerRootMode, ExplorerSortMode};
use crate::services::shell;
use crate::text::ElementExt;

/// Default and minimum width for the file explorer sidebar.
const DEFAULT_WIDTH: f32 = 200.0;
const MIN_WIDTH: f32 = 120.0;
const MAX_WIDTH: f32 = 400.0;
const RESIZE_HANDLE_WIDTH: f32 = 6.0;

/// Truncate a path in the middle, preserving the root and final component.
/// Example: "C:\very\long\path\to\project" -> "C:\...\project"
fn truncate_path_middle(path: &str, max_chars: usize) -> String {
    let path = super::normalize_unc_path(path);
    if path.chars().count() <= max_chars {
        return path;
    }

    // Detect path separator
    let sep = if path.contains('\\') { '\\' } else { '/' };

    // Get the last component (directory/file name)
    let last_component = path.rsplit(sep).next().unwrap_or(&path);

    // Reserve space for "..." (3 chars) + separator (1 char) + last component
    let reserved = 4 + last_component.chars().count();

    if reserved >= max_chars {
        // If last component itself is too long, just show "...name" truncated
        let available = max_chars.saturating_sub(3);
        let truncated: String = last_component.chars().take(available).collect();
        return format!("...{}", truncated);
    }

    // Find the first path component (drive letter or root)
    let prefix_end = path
        .char_indices()
        .find(|(_, c)| *c == sep)
        .map(|(i, _)| i + 1)
        .unwrap_or(0);
    let prefix: String = path.chars().take(prefix_end).collect();
    let prefix_len = prefix.chars().count();

    // Calculate available space for prefix
    let available_for_prefix = max_chars.saturating_sub(reserved);

    if prefix_len <= available_for_prefix && prefix_len > 0 {
        // Format: "C:\...\project"
        format!("{}...{}{}", prefix, sep, last_component)
    } else {
        // Format: "...\project"
        format!("...{}{}", sep, last_component)
    }
}

/// Callback type for when a file is clicked.
pub type OnFileClick = Box<dyn Fn(PathBuf, &mut Window, &mut App) + 'static>;

/// Callback type for when width changes.
pub type OnWidthChange = Box<dyn Fn(f32, &mut App) + 'static>;

/// Callback type for when close button is clicked.
pub type OnExplorerClose = Box<dyn Fn(&mut Window, &mut App) + 'static>;

/// Callback type for when expanded state changes.
pub type OnExpandedChange = Box<dyn Fn(HashSet<PathBuf>, &mut App) + 'static>;

/// Callback type for when root mode changes.
pub type OnRootModeChange = Rc<dyn Fn(ExplorerRootMode, &mut App) + 'static>;

/// Callback type for when sort mode changes.
pub type OnSortModeChange = Rc<dyn Fn(ExplorerSortMode, &mut App) + 'static>;

/// Callback type for editing markers.
pub type OnEditMarkers = Rc<dyn Fn(&mut Window, &mut App) + 'static>;

/// Kind of file entry.
#[derive(Clone)]
pub enum EntryKind {
    Directory { expanded: bool, has_children: bool },
    File,
}

/// File entry in the explorer.
#[derive(Clone)]
pub struct FileEntry {
    pub path: PathBuf,
    pub name: String,
    pub depth: usize,
    pub kind: EntryKind,
    pub modified_time: Option<std::time::SystemTime>,
}

/// File explorer sidebar view showing directory tree.
pub struct FileExplorerView {
    root_path: Option<PathBuf>,
    entries: Vec<FileEntry>,
    expanded_dirs: HashSet<PathBuf>,
    is_loading: bool,
    refresh_seq: u64,
    subtree_refresh_seq: HashMap<PathBuf, u64>,
    root_mode: ExplorerRootMode,
    sort_mode: ExplorerSortMode,
    width: f32,
    is_resizing: bool,
    resize_start_x: f32,
    resize_start_width: f32,
    /// Currently selected file path (highlighted in the tree)
    selected_path: Option<PathBuf>,
    /// Row bounds cache for right-click selection (window coordinates).
    entry_hitboxes: Vec<(PathBuf, Bounds<Pixels>)>,
    /// Scroll handle for programmatic scrolling to selected items.
    scroll_handle: ScrollHandle,
    /// Pending path to scroll to after async refresh.
    pending_scroll_to: Option<PathBuf>,
    on_click: Option<OnFileClick>,
    on_width_change: Option<OnWidthChange>,
    on_close: Option<OnExplorerClose>,
    on_expanded_change: Option<OnExpandedChange>,
    on_root_mode_change: Option<OnRootModeChange>,
    on_sort_mode_change: Option<OnSortModeChange>,
    on_edit_markers: Option<OnEditMarkers>,
}

impl FileExplorerView {
    /// Create a new file explorer view.
    pub fn new() -> Self {
        Self {
            root_path: None,
            entries: Vec::new(),
            expanded_dirs: HashSet::new(),
            is_loading: false,
            refresh_seq: 0,
            subtree_refresh_seq: HashMap::new(),
            root_mode: ExplorerRootMode::CurrentDir,
            sort_mode: ExplorerSortMode::default(),
            width: DEFAULT_WIDTH,
            is_resizing: false,
            resize_start_x: 0.0,
            resize_start_width: DEFAULT_WIDTH,
            selected_path: None,
            entry_hitboxes: Vec::new(),
            scroll_handle: ScrollHandle::new(),
            pending_scroll_to: None,
            on_click: None,
            on_width_change: None,
            on_close: None,
            on_expanded_change: None,
            on_root_mode_change: None,
            on_sort_mode_change: None,
            on_edit_markers: None,
        }
    }

    /// Set file click handler.
    pub fn on_click(mut self, callback: impl Fn(PathBuf, &mut Window, &mut App) + 'static) -> Self {
        self.on_click = Some(Box::new(callback));
        self
    }

    /// Set close handler.
    pub fn on_close(mut self, callback: impl Fn(&mut Window, &mut App) + 'static) -> Self {
        self.on_close = Some(Box::new(callback));
        self
    }

    /// Set width change handler.
    #[allow(dead_code)]
    pub fn on_width_change(mut self, callback: impl Fn(f32, &mut App) + 'static) -> Self {
        self.on_width_change = Some(Box::new(callback));
        self
    }

    /// Set expanded state change handler.
    #[allow(dead_code)]
    pub fn on_expanded_change(mut self, callback: impl Fn(HashSet<PathBuf>, &mut App) + 'static) -> Self {
        self.on_expanded_change = Some(Box::new(callback));
        self
    }

    /// Set root mode change handler.
    pub fn on_root_mode_change(mut self, callback: impl Fn(ExplorerRootMode, &mut App) + 'static) -> Self {
        self.on_root_mode_change = Some(Rc::new(callback));
        self
    }

    /// Set edit markers handler.
    pub fn on_edit_markers(mut self, callback: impl Fn(&mut Window, &mut App) + 'static) -> Self {
        self.on_edit_markers = Some(Rc::new(callback));
        self
    }

    /// Set sort mode change handler.
    pub fn on_sort_mode_change(mut self, callback: impl Fn(ExplorerSortMode, &mut App) + 'static) -> Self {
        self.on_sort_mode_change = Some(Rc::new(callback));
        self
    }

    /// Update root mode (for menu check state).
    pub fn set_root_mode(&mut self, mode: ExplorerRootMode, cx: &mut Context<Self>) {
        self.root_mode = mode;
        cx.notify();
    }

    /// Update sort mode (for menu check state).
    pub fn set_sort_mode(&mut self, mode: ExplorerSortMode, cx: &mut Context<Self>) {
        if self.sort_mode != mode {
            self.sort_mode = mode;
            self.refresh_entries(cx);
        }
    }

    /// Set width.
    #[allow(dead_code)]
    pub fn width(mut self, width: f32) -> Self {
        self.width = width.clamp(MIN_WIDTH, MAX_WIDTH);
        self
    }

    /// Get root path.
    pub fn root_path(&self) -> Option<&PathBuf> {
        self.root_path.as_ref()
    }

    /// Get expanded directories.
    pub fn expanded_dirs(&self) -> &HashSet<PathBuf> {
        &self.expanded_dirs
    }

    /// Get mutable reference to expanded directories (for batch operations).
    pub fn expanded_dirs_mut(&mut self) -> &mut HashSet<PathBuf> {
        &mut self.expanded_dirs
    }

    /// Set selected file path (for highlighting).
    pub fn set_selected_path(&mut self, path: Option<PathBuf>, cx: &mut Context<Self>) {
        self.selected_path = path.clone();
        self.pending_scroll_to = path;
        cx.notify();
    }

    /// Scroll to the selected (or pending) file if it's present in current entries.
    pub fn scroll_to_selected(&mut self, _cx: &mut Context<Self>) {
        let target = self
            .pending_scroll_to
            .as_ref()
            .or(self.selected_path.as_ref());

        let Some(target) = target else {
            return;
        };

        if let Some(index) = self.entries.iter().position(|e| &e.path == target) {
            self.scroll_handle.scroll_to_item(index);
            self.pending_scroll_to = None;
        }
    }

    /// Set root directory and scan for files.
    pub fn set_root(&mut self, path: Option<PathBuf>, cx: &mut Context<Self>) {
        self.root_path = path;
        self.refresh_entries(cx);
    }

    /// Set expanded directories.
    pub fn set_expanded_dirs(&mut self, dirs: HashSet<PathBuf>, cx: &mut Context<Self>) {
        self.expanded_dirs = dirs;
        self.refresh_entries(cx);
    }

    /// Update width and notify.
    pub fn set_width(&mut self, width: f32, cx: &mut Context<Self>) {
        self.width = width.clamp(MIN_WIDTH, MAX_WIDTH);
        cx.notify();
    }

    /// Refresh entries from root path asynchronously (lazy load).
    pub fn refresh_entries(&mut self, cx: &mut Context<Self>) {
        let Some(root) = self.root_path.clone() else {
            self.entries.clear();
            cx.notify();
            return;
        };

        // Start loading. Keep current entries visible to avoid layout jitter.
        self.is_loading = true;
        self.refresh_seq = self.refresh_seq.wrapping_add(1);
        let request_id = self.refresh_seq;
        let expanded_dirs = self.expanded_dirs.clone();
        let sort_mode = self.sort_mode;

        cx.spawn(move |this: WeakEntity<Self>, cx: &mut AsyncApp| {
            let mut cx = cx.clone();
            async move {
                // Scan directory tree in background thread (lazy load)
                let entries = smol::unblock(move || {
                    build_tree_lazy(&root, 0, &expanded_dirs, sort_mode)
                }).await;

                // Update entries on main thread
                let _ = this.update(&mut cx, |view, cx| {
                    // Ignore stale results from older refreshes.
                    if view.refresh_seq != request_id {
                        return;
                    }
                    view.entries = entries;
                    view.is_loading = false;
                    // Scroll to pending selected path if any
                    if view.pending_scroll_to.is_some() {
                        view.scroll_to_selected(cx);
                    }
                    cx.notify();
                });
            }
        }).detach();

        cx.notify();
    }

    /// Toggle directory expanded state.
    fn toggle_directory(&mut self, path: PathBuf, cx: &mut Context<Self>) {
        if self.root_path.is_none() {
            return;
        }

        let was_expanded = self.expanded_dirs.contains(&path);
        if was_expanded {
            self.expanded_dirs.remove(&path);
        } else {
            self.expanded_dirs.insert(path.clone());
        }

        // Update the entry's expanded flag for UI rendering
        if let Some(dir_index) = self.entries.iter().position(|e| e.path == path) {
            if let EntryKind::Directory { expanded, .. } = &mut self.entries[dir_index].kind {
                *expanded = !was_expanded;
            }
        }

        if let Some(on_expanded_change) = &self.on_expanded_change {
            on_expanded_change(self.expanded_dirs.clone(), cx);
        }

        if was_expanded {
            // Collapsing: remove children immediately for visual feedback
            if let Some(dir_index) = self.entries.iter().position(|e| e.path == path) {
                let dir_depth = self.entries[dir_index].depth;
                let mut end = dir_index + 1;
                while end < self.entries.len() && self.entries[end].depth > dir_depth {
                    end += 1;
                }
                self.entries.drain(dir_index + 1..end);
                cx.notify();
            }
        } else {
            // Expanding: schedule a subtree refresh
            self.refresh_subtree(path, cx);
        }
    }

    fn refresh_subtree(&mut self, dir: PathBuf, cx: &mut Context<Self>) {
        let seq = self.subtree_refresh_seq.entry(dir.clone()).or_insert(0);
        *seq = seq.wrapping_add(1);
        let request_id = *seq;

        let expanded_dirs = self.expanded_dirs.clone();
        let sort_mode = self.sort_mode;
        let dir_for_update = dir.clone();

        cx.spawn(move |this: WeakEntity<Self>, cx: &mut AsyncApp| {
            let mut cx = cx.clone();
            async move {
                let subtree_entries = smol::unblock(move || {
                    build_tree_lazy(&dir, 0, &expanded_dirs, sort_mode)
                }).await;

                let _ = this.update(&mut cx, |view, cx| {
                    let Some(current_seq) = view.subtree_refresh_seq.get(&dir_for_update).copied() else {
                        return;
                    };
                    if current_seq != request_id {
                        return;
                    }

                    let Some(dir_index) = view.entries.iter().position(|e| e.path == dir_for_update) else {
                        return;
                    };
                    let dir_depth = view.entries[dir_index].depth;

                    // Compute current range of children.
                    let mut end = dir_index + 1;
                    while end < view.entries.len() && view.entries[end].depth > dir_depth {
                        end += 1;
                    }

                    // If the dir is currently collapsed, keep it collapsed.
                    if !view.expanded_dirs.contains(&dir_for_update) {
                        if end > dir_index + 1 {
                            view.entries.drain(dir_index + 1..end);
                        }
                        cx.notify();
                        return;
                    }

                    // Transform subtree_entries into entries with correct depth.
                    // build_tree_lazy returns children with depth relative to the dir (starting at 0).
                    // We need to adjust them to be relative to the global tree.
                    let mut normalized = Vec::new();
                    for mut e in subtree_entries {
                        e.depth = dir_depth + e.depth + 1;
                        normalized.push(e);
                    }

                    view.entries.splice(dir_index + 1..end, normalized);
                    cx.notify();
                });
            }
        }).detach();
    }

    /// Expand a directory (without toggling).
    pub fn expand_directory(&mut self, path: PathBuf, cx: &mut Context<Self>) {
        if !self.expanded_dirs.contains(&path) {
            self.expanded_dirs.insert(path);

            if let Some(on_expanded_change) = &self.on_expanded_change {
                on_expanded_change(self.expanded_dirs.clone(), cx);
            }

            self.refresh_entries(cx);
        }
    }

    /// Check if currently resizing.
    pub fn is_resizing(&self) -> bool {
        self.is_resizing
    }

    fn entry_path_at_position(&self, position: Point<Pixels>) -> Option<&PathBuf> {
        self.entry_hitboxes
            .iter()
            .find_map(|(path, bounds)| bounds.contains(&position).then_some(path))
    }

    /// Handle mouse move during resize (called from workspace).
    pub fn handle_resize_move(&mut self, mouse_x: f32, cx: &mut Context<Self>) {
        if self.is_resizing {
            let delta = mouse_x - self.resize_start_x;
            let new_width = (self.resize_start_width + delta).clamp(MIN_WIDTH, MAX_WIDTH);
            self.width = new_width;
            if let Some(on_width_change) = &self.on_width_change {
                on_width_change(new_width, cx);
            }
            cx.notify();
        }
    }

    /// End resize operation.
    pub fn end_resize(&mut self, cx: &mut Context<Self>) {
        self.is_resizing = false;
        cx.notify();
    }
}

fn select_entry_under_cursor(
    view: &mut FileExplorerView,
    position: Point<Pixels>,
    cx: &mut Context<FileExplorerView>,
) {
    let Some(path) = view.entry_path_at_position(position).cloned() else {
        return;
    };

    if view.selected_path.as_ref() == Some(&path) {
        return;
    }

    view.selected_path = Some(path);
    cx.notify();
}

fn right_click_selection_overlay(view: Entity<FileExplorerView>) -> impl IntoElement {
    canvas(
        |_bounds, _window, _cx| (),
        move |bounds, (), window, _cx| {
            let view = view.clone();
            window.on_mouse_event(move |event: &MouseDownEvent, phase, _window, cx| {
                if !phase.capture()
                    || event.button != MouseButton::Right
                    || !bounds.contains(&event.position)
                {
                    return;
                }

                let position = event.position;
                view.update(cx, |view, cx| select_entry_under_cursor(view, position, cx));
            });
        },
    )
    .absolute()
    .size_full()
}

impl Render for FileExplorerView {
    fn render(&mut self, _window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let theme = cx.theme().clone();
        let width = self.width;
        let is_resizing = self.is_resizing;
        let current_root_mode = self.root_mode;
        let current_sort_mode = self.sort_mode;
        let on_root_mode_change = self.on_root_mode_change.clone();
        let on_sort_mode_change = self.on_sort_mode_change.clone();
        let on_edit_markers = self.on_edit_markers.clone();

        div()
            .id("explorer-container")
            .flex()
            .flex_row()
            .flex_shrink_0()
            .h_full()
            .child(
                // Main explorer content
                v_flex()
                    .id("explorer-view")
                    .w(px(width))
                    .h_full()
                    .flex_shrink_0()
                    .bg(theme.sidebar)
                    .child(
                        // Header - fixed at top
                        div()
                            .flex_shrink_0()
                            .px_3()
                            .py_2()
                            .flex()
                            .flex_row()
                            .items_center()
                            .justify_between()
                            .child({
                                // Estimate max characters based on available width
                                // Account for padding (24px), buttons (~50px), and font size (~7px/char)
                                let max_chars = ((width - 74.0) / 7.0).max(10.0) as usize;
                                let (title, full_path) = if let Some(root) = &self.root_path {
                                    let full = root.to_string_lossy().to_string();
                                    let truncated = truncate_path_middle(&full, max_chars);
                                    let needs_tooltip = truncated != full;
                                    (truncated, if needs_tooltip { Some(full) } else { None })
                                } else {
                                    ("EXPLORER".to_string(), None)
                                };

                                div()
                                    .id("explorer-title")
                                    .overflow_hidden()
                                    .text_ellipsis()
                                    .whitespace_nowrap()
                                    .flex_shrink()
                                    .min_w_0()
                                    .text_xs()
                                    .text_color(theme.muted_foreground)
                                    .font_weight(FontWeight::SEMIBOLD)
                                    .child(title)
                                    .when_some(full_path, |this, path| {
                                        this.tooltip(move |_window, cx| {
                                            Tooltip::new(path.clone()).build(_window, cx)
                                        })
                                    })
                            })
                            .child(
                                h_flex()
                                    .gap_1()
                                    .items_center()
                                    .child(
                                        Button::new("explorer-sort-menu")
                                            .icon(Icon::new(match current_sort_mode {
                                                ExplorerSortMode::NameAsc => IconName::SortAscending,
                                                ExplorerSortMode::NameDesc => IconName::SortDescending,
                                                ExplorerSortMode::TimeDesc => IconName::ArrowDown,
                                                ExplorerSortMode::TimeAsc => IconName::ArrowUp,
                                            }))
                                            .ghost()
                                            .xsmall()
                                            .tooltip(match current_sort_mode {
                                                ExplorerSortMode::NameAsc => "Sort: Name (A-Z)",
                                                ExplorerSortMode::NameDesc => "Sort: Name (Z-A)",
                                                ExplorerSortMode::TimeDesc => "Sort: Time (Newest)",
                                                ExplorerSortMode::TimeAsc => "Sort: Time (Oldest)",
                                            })
                                            .dropdown_menu(move |menu, _window, _cx| {
                                                let on_sort_name_asc = on_sort_mode_change.clone();
                                                let on_sort_name_desc = on_sort_mode_change.clone();
                                                let on_sort_time_desc = on_sort_mode_change.clone();
                                                let on_sort_time_asc = on_sort_mode_change.clone();

                                                menu
                                                    .label("Sort By")
                                                    .item(
                                                        PopupMenuItem::new("Name (A-Z)")
                                                            .checked(current_sort_mode == ExplorerSortMode::NameAsc)
                                                            .on_click(move |_, _, cx| {
                                                                if let Some(handler) = &on_sort_name_asc {
                                                                    handler(ExplorerSortMode::NameAsc, cx);
                                                                }
                                                            })
                                                    )
                                                    .item(
                                                        PopupMenuItem::new("Name (Z-A)")
                                                            .checked(current_sort_mode == ExplorerSortMode::NameDesc)
                                                            .on_click(move |_, _, cx| {
                                                                if let Some(handler) = &on_sort_name_desc {
                                                                    handler(ExplorerSortMode::NameDesc, cx);
                                                                }
                                                            })
                                                    )
                                                    .separator()
                                                    .item(
                                                        PopupMenuItem::new("Time (Newest First)")
                                                            .checked(current_sort_mode == ExplorerSortMode::TimeDesc)
                                                            .on_click(move |_, _, cx| {
                                                                if let Some(handler) = &on_sort_time_desc {
                                                                    handler(ExplorerSortMode::TimeDesc, cx);
                                                                }
                                                            })
                                                    )
                                                    .item(
                                                        PopupMenuItem::new("Time (Oldest First)")
                                                            .checked(current_sort_mode == ExplorerSortMode::TimeAsc)
                                                            .on_click(move |_, _, cx| {
                                                                if let Some(handler) = &on_sort_time_asc {
                                                                    handler(ExplorerSortMode::TimeAsc, cx);
                                                                }
                                                            })
                                                    )
                                            })
                                    )
                                    .child(
                                        Button::new("explorer-root-menu")
                                            .icon(Icon::new(IconName::EllipsisVertical))
                                            .ghost()
                                            .xsmall()
                                            .dropdown_menu(move |menu, _window, _cx| {
                                                let on_root_mode_change_current = on_root_mode_change.clone();
                                                let on_root_mode_change_project = on_root_mode_change.clone();
                                                let on_edit_markers = on_edit_markers.clone();
                                                menu
                                                    .label("Explorer Root")
                                                    .item(
                                                        PopupMenuItem::new("Current Directory")
                                                            .checked(current_root_mode == ExplorerRootMode::CurrentDir)
                                                            .on_click(move |_, _, cx| {
                                                                if let Some(handler) = &on_root_mode_change_current {
                                                                    handler(ExplorerRootMode::CurrentDir, cx);
                                                                }
                                                            })
                                                    )
                                                    .item(
                                                        PopupMenuItem::new("Project Directory")
                                                            .checked(current_root_mode == ExplorerRootMode::ProjectRoot)
                                                            .on_click(move |_, _, cx| {
                                                                if let Some(handler) = &on_root_mode_change_project {
                                                                    handler(ExplorerRootMode::ProjectRoot, cx);
                                                                }
                                                            })
                                                    )
                                                    .separator()
                                                    .item(
                                                        PopupMenuItem::new("Edit Markers...")
                                                            .on_click(move |_, window, cx| {
                                                                if let Some(handler) = &on_edit_markers {
                                                                    handler(window, cx);
                                                                }
                                                            })
                                                    )
                                            })
                                    )
                                    .child(
                                        Button::new("explorer-close-btn")
                                            .icon(Icon::new(IconName::Close))
                                            .ghost()
                                            .xsmall()
                                            .on_click(cx.listener(|this, _, window, cx| {
                                                if let Some(on_close) = &this.on_close {
                                                    on_close(window, cx);
                                                }
                                            }))
                                    )
                            )
                    )
                    .child(
                        // Scrollable content area
                        div()
                            .id("explorer-scroll")
                            .relative()
                            .flex_grow()
                            .min_h_0() // Critical: allow flex item to shrink below content size
                            .overflow_hidden()
                            .child(
                                div()
                                    .id("explorer-items")
                                    .relative()
                                    .size_full()
                                    .child({
                                        let view = cx.entity().clone();
                                        right_click_selection_overlay(view)
                                    })
                                    .on_prepaint({
                                        let view = cx.entity().clone();
                                        move |_bounds, _window, cx| view.update(cx, |view, _cx| view.entry_hitboxes.clear())
                                    })
                                    .when(!self.is_loading && self.entries.is_empty(), |this| {
                                        this.child(
                                            div()
                                                .px_3()
                                                .py_2()
                                                .text_sm()
                                                .text_color(theme.muted_foreground)
                                                .child("No markdown files")
                                        )
                                    })
                                    .children(self.entries.iter().map(|entry| {
                                        let indent = (entry.depth as f32) * 12.0;
                                        let path = entry.path.clone();
                                        let is_dir = matches!(entry.kind, EntryKind::Directory { .. });
                                        let (is_expanded, has_children) = match &entry.kind {
                                            EntryKind::Directory { expanded, has_children } => (*expanded, *has_children),
                                            EntryKind::File => (false, false),
                                        };
                                        let is_selected = self.selected_path.as_ref() == Some(&path);

                                        div()
                                            .id(SharedString::from(path.to_string_lossy().to_string()))
                                            .on_prepaint({
                                                let view = cx.entity().clone();
                                                let path_for_hitbox = path.clone();
                                                move |bounds, _window, cx| {
                                                    view.update(cx, |view, _cx| {
                                                        view.entry_hitboxes.push((path_for_hitbox.clone(), bounds));
                                                    });
                                                }
                                            })
                                            .w_full()
                                            .px_3()
                                            .py_1()
                                            .pl(px(12.0 + indent))
                                            .text_sm()
                                            .cursor_pointer()
                                            .text_color(theme.foreground)
                                            .when(is_selected, |this| this.bg(theme.list_active))
                                            .when(!is_selected, |this| this.hover(|s| s.bg(theme.accent)))
                                            .flex()
                                            .flex_row()
                                            .items_center()
                                            .gap_1()
                                            .capture_any_mouse_down(cx.listener({
                                                let path = path.clone();
                                                move |this, event: &MouseDownEvent, _window, cx| {
                                                    if event.button == MouseButton::Right {
                                                        this.selected_path = Some(path.clone());
                                                        cx.notify();
                                                    }
                                                }
                                            }))
                                            .on_click(cx.listener({
                                                let path = path.clone();
                                                move |this, _, window, cx| {
                                                    if is_dir {
                                                        this.toggle_directory(path.clone(), cx);
                                                    } else if let Some(on_click) = &this.on_click {
                                                        on_click(path.clone(), window, cx);
                                                    }
                                                    this.selected_path = Some(path.clone());
                                                    cx.notify();
                                                }
                                            }))
                                            .child(
                                                h_flex()
                                                    .gap_1()
                                                    .items_center()
                                                    .when(is_dir && has_children, |this| {
                                                        this.child(
                                                            Icon::new(if is_expanded {
                                                                IconName::ChevronDown
                                                            } else {
                                                                IconName::ChevronRight
                                                            })
                                                            .text_color(theme.muted_foreground)
                                                            .xsmall()
                                                        )
                                                        .child(
                                                            Icon::new(if is_expanded {
                                                                IconName::FolderOpen
                                                            } else {
                                                                IconName::FolderClosed
                                                            })
                                                            .text_color(theme.muted_foreground)
                                                            .small()
                                                        )
                                                    })
                                                    .when(is_dir && !has_children, |this| {
                                                        this.child(div().w(px(16.0)))
                                                        .child(
                                                            Icon::new(IconName::FolderClosed)
                                                                .text_color(theme.muted_foreground)
                                                                .small()
                                                        )
                                                    })
                                                    .when(!is_dir, |this| {
                                                        this.child(div().w(px(16.0)))
                                                        .child(
                                                            Icon::new(IconName::File)
                                                                .text_color(theme.muted_foreground)
                                                                .xsmall()
                                                        )
                                                    })
                                            )
                                            .child(
                                                div()
                                                    .overflow_hidden()
                                                    .text_ellipsis()
                                                    .child(entry.name.clone())
                                            )
                                    }))
                                    .track_scroll(&self.scroll_handle)
                                    .overflow_y_scrollbar()
                                    .context_menu({
                                        let view = cx.entity().clone();
                                        move |menu, _window, cx| {
                                            let Some(selected_path) = view.read(cx).selected_path.clone() else {
                                                return menu;
                                            };

                                            menu.item(
                                                PopupMenuItem::new("Open in File Manager")
                                                    .on_click({
                                                        let selected_path = selected_path.clone();
                                                        move |_, _window, _cx| shell::open_in_explorer(&selected_path)
                                                    }),
                                            )
                                            .item(
                                                PopupMenuItem::new("Copy File Path")
                                                    .on_click(move |_, _window, cx| {
                                                        let path_str = super::normalize_unc_path(
                                                            selected_path.to_string_lossy().as_ref(),
                                                        );
                                                        cx.write_to_clipboard(ClipboardItem::new_string(path_str));
                                                    }),
                                            )
                                        }
                                    })
                            )
                    )
            )
            .child(
                // Resize handle
                div()
                    .id("explorer-resize-handle")
                    .w(px(RESIZE_HANDLE_WIDTH))
                    .h_full()
                    .cursor_col_resize()
                    .bg(theme.border)
                    .hover(|s| s.bg(theme.primary))
                    .when(is_resizing, |this| this.bg(theme.primary))
                    .on_mouse_down(MouseButton::Left, cx.listener(|this, event: &MouseDownEvent, _, cx| {
                        this.is_resizing = true;
                        this.resize_start_x = f32::from(event.position.x);
                        this.resize_start_width = this.width;
                        cx.notify();
                    }))
            )
    }
}

/// Build lazy-loaded directory tree (only scan expanded directories).
fn build_tree_lazy(
    dir: &PathBuf,
    depth: usize,
    expanded_dirs: &HashSet<PathBuf>,
    sort_mode: ExplorerSortMode,
) -> Vec<FileEntry> {
    let mut entries = Vec::new();

    // Use ignore crate to respect .gitignore
    let mut dir_entries = Vec::new();
    let walker = ignore::WalkBuilder::new(dir)
        .max_depth(Some(1))
        .standard_filters(true)
        .hidden(false) // We handle hidden files ourselves below
        .build();

    for result in walker {
        let Ok(entry) = result else { continue };
        if entry.depth() == 0 { continue; } // Skip root

        let path = entry.path().to_path_buf();
        let metadata = entry.metadata().ok();
        let modified_time = metadata.as_ref().and_then(|m| m.modified().ok());
        dir_entries.push((entry, path, modified_time));
    }

    // Sort: directories first, then by selected mode within same type
    dir_entries.sort_by(|(a, _, a_time), (b, _, b_time)| {
        let a_is_dir = a.file_type().map(|t| t.is_dir()).unwrap_or(false);
        let b_is_dir = b.file_type().map(|t| t.is_dir()).unwrap_or(false);

        match (a_is_dir, b_is_dir) {
            (true, false) => return std::cmp::Ordering::Less,
            (false, true) => return std::cmp::Ordering::Greater,
            _ => {}
        }

        match sort_mode {
            ExplorerSortMode::NameAsc => {
                let a_name = a.file_name().to_string_lossy().to_lowercase();
                let b_name = b.file_name().to_string_lossy().to_lowercase();
                a_name.cmp(&b_name)
            }
            ExplorerSortMode::NameDesc => {
                let a_name = a.file_name().to_string_lossy().to_lowercase();
                let b_name = b.file_name().to_string_lossy().to_lowercase();
                b_name.cmp(&a_name)
            }
            ExplorerSortMode::TimeDesc => match (b_time, a_time) {
                (Some(bt), Some(at)) => bt.cmp(&at),
                (Some(_), None) => std::cmp::Ordering::Less,
                (None, Some(_)) => std::cmp::Ordering::Greater,
                (None, None) => std::cmp::Ordering::Equal,
            },
            ExplorerSortMode::TimeAsc => match (a_time, b_time) {
                (Some(at), Some(bt)) => at.cmp(&bt),
                (Some(_), None) => std::cmp::Ordering::Less,
                (None, Some(_)) => std::cmp::Ordering::Greater,
                (None, None) => std::cmp::Ordering::Equal,
            },
        }
    });

    for (entry, path, modified_time) in dir_entries {
        let name = entry.file_name().to_string_lossy().to_string();

        // Skip hidden files/directories
        if name.starts_with('.') {
            continue;
        }

        if let Some(file_type) = entry.file_type() {
            if file_type.is_dir() {
                // Check if this directory has subdirectories or .md files
                let has_children = has_relevant_children(&path);

                let expanded = expanded_dirs.contains(&path);
                entries.push(FileEntry {
                    path: path.clone(),
                    name,
                    depth,
                    kind: EntryKind::Directory { expanded, has_children },
                    modified_time,
                });

                // Only scan children if expanded (lazy load)
                if expanded {
                    let child_entries = build_tree_lazy(&path, depth + 1, expanded_dirs, sort_mode);
                    entries.extend(child_entries);
                }
            } else if file_type.is_file() {
                // Only include .md files
                if path.extension().and_then(|s| s.to_str()) == Some("md") {
                    entries.push(FileEntry {
                        path,
                        name,
                        depth,
                        kind: EntryKind::File,
                        modified_time,
                    });
                }
            }
        }
    }

    entries
}

/// Check if directory has relevant children (subdirectories or .md files).
fn has_relevant_children(dir: &PathBuf) -> bool {
    let walker = ignore::WalkBuilder::new(dir)
        .max_depth(Some(1))
        .standard_filters(true)
        .hidden(false)
        .build();

    for result in walker {
        let Ok(entry) = result else { continue };
        if entry.depth() == 0 { continue; }

        let name = entry.file_name().to_string_lossy().to_string();
        if name.starts_with('.') {
            continue;
        }

        if let Some(file_type) = entry.file_type() {
            if file_type.is_file() {
                if entry.path().extension().and_then(|s| s.to_str()) == Some("md") {
                    return true;
                }
            } else if file_type.is_dir() {
                return true;
            }
        }
    }
    false
}
