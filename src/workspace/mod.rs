//! UI Views and layout.

/// Normalize a Windows UNC path (\\?\ prefix) to a regular path.
fn normalize_unc_path(path: &str) -> String {
    if path.starts_with("\\\\?\\") {
        path[4..].to_string()
    } else if path.starts_with("//?/") {
        path[4..].to_string()
    } else {
        path.to_string()
    }
}

#[cfg(windows)]
pub(super) fn normalize_unc_pathbuf(path: &std::path::Path) -> PathBuf {
    let s = path.to_string_lossy();

    if let Some(rest) = s.strip_prefix("\\\\?\\UNC\\") {
        return PathBuf::from(format!("\\\\{}", rest));
    }

    if let Some(rest) = s.strip_prefix("//?/UNC/") {
        return PathBuf::from(format!("//{}", rest));
    }

    if let Some(rest) = s.strip_prefix("\\\\?\\") {
        return PathBuf::from(rest.to_string());
    }

    if let Some(rest) = s.strip_prefix("//?/") {
        return PathBuf::from(rest.to_string());
    }

    path.to_path_buf()
}

#[cfg(not(windows))]
pub(super) fn normalize_unc_pathbuf(path: &std::path::Path) -> PathBuf {
    path.to_path_buf()
}

use gpui::*;
use gpui::prelude::FluentBuilder;
use std::path::PathBuf;
use std::time::Instant;
use std::collections::{HashMap, HashSet};
use crate::state::document::Document;
use gpui_component::{ActiveTheme, Root, button::Button, button::ButtonVariants, Icon, IconName, Sizable};
use crate::state::config::{AppConfig, ExplorerRootMode, ExplorerSortMode};

mod welcome;
use welcome::render_welcome;
mod markdown_view;
use markdown_view::MarkdownView;
mod settings_dialog;
mod header;
mod outline;
use outline::OutlineView;
mod file_explorer;
use file_explorer::FileExplorerView;
mod search_bar;
use search_bar::{SearchBar, SearchState};
use smol::channel::Receiver;
use crate::ipc::IpcMessage;
use crate::file_watcher::FileWatchManager;
use crate::services::shell;
use serde::Deserialize;

#[cfg(windows)]
use windows_sys::Win32::UI::WindowsAndMessaging::{
    EnumWindows, GetWindowTextW, GetWindowTextLengthW, GetForegroundWindow,
    GetWindowThreadProcessId, SetForegroundWindow, ShowWindow, IsIconic, SW_RESTORE,
};
#[cfg(windows)]
use windows_sys::Win32::UI::Input::KeyboardAndMouse::{
    keybd_event, KEYEVENTF_EXTENDEDKEY, KEYEVENTF_KEYUP, VK_MENU,
};
#[cfg(windows)]
use windows_sys::Win32::System::Threading::{AttachThreadInput, GetCurrentThreadId};
#[cfg(windows)]
use windows_sys::Win32::Foundation::HWND;

gpui::actions!([
    OpenFileDialog,
    OpenSearch,
    CloseSearch,
    // Navigation
    NextTab,
    PrevTab,
    CloseTab,
    // Editing
    SelectAll,
    ScrollToTop,
    ScrollToBottom,
    // View
    RefreshDocument,
    ToggleOutline,
    ToggleExplorer,
]);

#[derive(Action, Clone, Copy, PartialEq, Eq, Deserialize)]
#[action(namespace = workspace, no_json)]
pub struct SetExplorerRootMode(pub ExplorerRootMode);

#[derive(Action, Clone, PartialEq, Eq, Deserialize)]
#[action(namespace = workspace, no_json)]
pub struct RevealInExplorer {
    pub path: PathBuf,
    pub tab_index: usize,
}

#[derive(Action, Clone, Copy, PartialEq, Eq, Deserialize)]
#[action(namespace = workspace, no_json)]
pub struct CloseTabAt(pub usize);

pub fn init(cx: &mut App, initial_files: Vec<PathBuf>, ipc_rx: Option<Receiver<IpcMessage>>, config: Entity<AppConfig>) {
    cx.bind_keys(vec![
        // Open file dialog
        #[cfg(target_os = "macos")]
        KeyBinding::new("cmd-o", OpenFileDialog, Some("Workspace")),
        #[cfg(not(target_os = "macos"))]
        KeyBinding::new("ctrl-o", OpenFileDialog, Some("Workspace")),

        // Search
        #[cfg(target_os = "macos")]
        KeyBinding::new("cmd-f", OpenSearch, Some("Workspace")),
        #[cfg(not(target_os = "macos"))]
        KeyBinding::new("ctrl-f", OpenSearch, Some("Workspace")),
        #[cfg(target_os = "macos")]
        KeyBinding::new("cmd-f", OpenSearch, Some("SearchBar")),
        #[cfg(not(target_os = "macos"))]
        KeyBinding::new("ctrl-f", OpenSearch, Some("SearchBar")),
        KeyBinding::new("escape", CloseSearch, Some("Workspace && search == open")),

        // Tab navigation
        KeyBinding::new("ctrl-tab", NextTab, Some("Workspace")),
        KeyBinding::new("ctrl-shift-tab", PrevTab, Some("Workspace")),
        #[cfg(target_os = "macos")]
        KeyBinding::new("cmd-w", CloseTab, Some("Workspace")),
        #[cfg(not(target_os = "macos"))]
        KeyBinding::new("ctrl-w", CloseTab, Some("Workspace")),
        KeyBinding::new("ctrl-f4", CloseTab, Some("Workspace")),

        // Selection & Scrolling
        #[cfg(target_os = "macos")]
        KeyBinding::new("cmd-a", SelectAll, Some("Workspace")),
        #[cfg(not(target_os = "macos"))]
        KeyBinding::new("ctrl-a", SelectAll, Some("Workspace")),
        #[cfg(target_os = "macos")]
        KeyBinding::new("cmd-home", ScrollToTop, Some("Workspace")),
        #[cfg(not(target_os = "macos"))]
        KeyBinding::new("ctrl-home", ScrollToTop, Some("Workspace")),
        #[cfg(target_os = "macos")]
        KeyBinding::new("cmd-end", ScrollToBottom, Some("Workspace")),
        #[cfg(not(target_os = "macos"))]
        KeyBinding::new("ctrl-end", ScrollToBottom, Some("Workspace")),

        // View
        KeyBinding::new("f5", RefreshDocument, Some("Workspace")),
        #[cfg(target_os = "macos")]
        KeyBinding::new("cmd-b", ToggleOutline, Some("Workspace")),
        #[cfg(not(target_os = "macos"))]
        KeyBinding::new("ctrl-b", ToggleOutline, Some("Workspace")),
        #[cfg(target_os = "macos")]
        KeyBinding::new("cmd-shift-e", ToggleExplorer, Some("Workspace")),
        #[cfg(not(target_os = "macos"))]
        KeyBinding::new("ctrl-shift-e", ToggleExplorer, Some("Workspace")),
    ]);

    // Read window size from config
    let window_size = {
        let cfg = config.read(cx);
        size(
            px(cfg.appearance.window_width),
            px(cfg.appearance.window_height),
        )
    };

    cx.open_window(
        WindowOptions {
            titlebar: Some(TitlebarOptions {
                title: Some("Peekdown".into()),
                ..Default::default()
            }),
            window_bounds: Some(WindowBounds::Windowed(Bounds::centered(
                None,
                window_size,
                cx,
            ))),
            ..Default::default()
        },
        move |window, cx| {
            // Apply theme
            {
                let theme = config.read(cx).appearance.theme;
                theme.apply(None, cx);
            }

            // Apply font settings - clone the config first to avoid borrow conflict
            let appearance = config.read(cx).appearance.clone();
            appearance.apply_font_settings(cx);

            // Create WorkspaceView first
            let workspace_view = cx.new(|cx| {
                let mut view = WorkspaceView::new(cx, config.clone());
                for path in initial_files.clone() {
                    view.open_file(path, cx);
                }
                view
            });

            // Focus the workspace so keyboard shortcuts work immediately
            window.focus(&workspace_view.read(cx).focus_handle);

            // Setup IPC handler
            if let Some(rx) = ipc_rx {
                let workspace_weak = workspace_view.downgrade();
                cx.spawn(|cx: &mut AsyncApp| {
                    let mut cx = cx.clone();
                    async move {
                        while let Ok(msg) = rx.recv().await {
                            tracing::info!("Received IPC message: {:?}", msg);
                            let mut cx_clone = cx.clone();
                            workspace_weak.update(&mut cx_clone, |workspace, cx| {
                                match msg {
                                    IpcMessage::OpenFiles(paths) => workspace.open_files(paths, cx),
                                    IpcMessage::FocusWindow => {},
                                }
                                // Bring window to foreground on Windows
                                #[cfg(windows)]
                                {
                                    tracing::info!("Attempting to bring window to foreground");
                                    bring_window_to_foreground();
                                }
                                cx.activate(true);
                            }).ok();
                        }
                    }
                }).detach();
            }

            // Wrap in Root for dialog support and observe window bounds
            let config_for_bounds = config.clone();
            let config_for_quit = config.clone();
            let workspace_for_quit = workspace_view.downgrade();
            cx.new(|cx| {
                // Observe window bounds changes to save window size
                cx.observe_window_bounds(window, move |_root, window, cx| {
                    if window.is_maximized() {
                        return;
                    }
                    let bounds = window.bounds();
                    config_for_bounds.update(cx, |config, _| {
                        config.appearance.window_width = bounds.size.width.into();
                        config.appearance.window_height = bounds.size.height.into();
                        config.save();
                    });
                }).detach();

                // Save outline and explorer visibility and width on app quit
                cx.on_app_quit({
                    let config = config_for_quit.clone();
                    let workspace = workspace_for_quit.clone();
                    move |_root, cx| {
                        if let Some(ws) = workspace.upgrade() {
                            let ws = ws.read(cx);
                            let outline_visible = ws.outline_visible;
                            let outline_width = ws.outline_width;
                            let explorer_visible = ws.explorer_visible;
                            let explorer_width = ws.explorer_width;
                            config.update(cx, |config, _| {
                                config.appearance.outline_visible = outline_visible;
                                config.appearance.outline_width = outline_width;
                                config.appearance.explorer_visible = explorer_visible;
                                config.appearance.explorer_width = explorer_width;
                                config.save();
                            });
                        }
                        async {}
                    }
                }).detach();

                Root::new(workspace_view, window, cx)
            })
        },
    )
    .unwrap();
}

struct WorkspaceTab {
    path: PathBuf,
    view: Entity<MarkdownView>,
    title: String,
}

struct WorkspaceView {
    tabs: Vec<WorkspaceTab>,
    active_tab_index: usize,
    config: Entity<AppConfig>,
    outline_visible: bool,
    outline_view: Option<Entity<OutlineView>>,
    search_bar: Option<Entity<SearchBar>>,
    tab_context_menu: Option<TabContextMenuState>,
    tab_tooltip: Option<TabTooltipState>,
    tab_bar_bounds: Option<Bounds<Pixels>>,
    tab_hitboxes: Vec<(usize, Bounds<Pixels>)>,
    outline_width: f32,
    explorer_visible: bool,
    /// Explorer instances per project root (preserves state when switching tabs)
    explorer_views: HashMap<PathBuf, Entity<FileExplorerView>>,
    /// Current active explorer root
    current_explorer_root: Option<PathBuf>,
    explorer_width: f32,
    focus_handle: FocusHandle,
    tab_scroll_handle: ScrollHandle,
    // FPS counter
    fps_counter: FpsCounter,
    // File watcher for auto-refresh
    file_watcher: FileWatchManager,
}

#[derive(Clone)]
struct TabContextMenuState {
    position: Point<Pixels>,
    tab_index: usize,
    tab_path: PathBuf,
}

#[derive(Clone)]
struct TabTooltipState {
    position: Point<Pixels>,
    text: String,
}

#[derive(Clone, Copy, PartialEq, Eq)]
enum ExplorerUpdateMode {
    Default,
    PreserveRootIfContains,
}

/// Simple FPS counter
struct FpsCounter {
    last_frame_time: Instant,
    frame_count: u32,
    current_fps: f32,
}

impl FpsCounter {
    fn new() -> Self {
        Self {
            last_frame_time: Instant::now(),
            frame_count: 0,
            current_fps: 0.0,
        }
    }

    /// Call this every frame, returns true if FPS was updated
    fn tick(&mut self) -> bool {
        self.frame_count += 1;
        let elapsed = self.last_frame_time.elapsed();
        if elapsed.as_secs_f32() >= 0.5 {
            self.current_fps = self.frame_count as f32 / elapsed.as_secs_f32();
            self.frame_count = 0;
            self.last_frame_time = Instant::now();
            true
        } else {
            false
        }
    }

    fn fps(&self) -> f32 {
        self.current_fps
    }
}

impl WorkspaceView {
    pub fn new(cx: &mut Context<Self>, config: Entity<AppConfig>) -> Self {
        let (outline_visible, outline_width, explorer_visible, explorer_width, auto_refresh) = {
            let cfg = config.read(cx);
            (
                cfg.appearance.outline_visible,
                cfg.appearance.outline_width,
                cfg.appearance.explorer_visible,
                cfg.appearance.explorer_width,
                cfg.appearance.auto_refresh,
            )
        };

        // Initialize file watcher
        let mut file_watcher = FileWatchManager::new();
        file_watcher.set_enabled(auto_refresh);

        // Setup event listener for file changes
        let event_rx = file_watcher.event_receiver();
        cx.spawn(|workspace_weak: WeakEntity<WorkspaceView>, cx: &mut AsyncApp| {
            let cx = cx.clone();
            async move {
                while let Ok(event) = event_rx.recv().await {
                    let mut cx_clone = cx.clone();
                    workspace_weak.update(&mut cx_clone, |ws, cx| {
                        ws.handle_file_change(event, cx);
                    }).ok();
                }
            }
        }).detach();


        // Observe config changes for auto_refresh
        let config_clone = config.clone();
        cx.observe(&config, move |this, config, cx| {
            let auto_refresh = config.read(cx).appearance.auto_refresh;
            this.file_watcher.set_enabled(auto_refresh);
            if auto_refresh {
                // Re-register all current tabs
                for tab in &this.tabs {
                    let _ = this.file_watcher.watch(tab.path.clone());
                }
            }
            cx.notify();
        }).detach();

        Self {
            tabs: Vec::new(),
            active_tab_index: 0,
            config: config_clone,
            outline_visible,
            outline_view: None,
            search_bar: None,
            tab_context_menu: None,
            tab_tooltip: None,
            tab_bar_bounds: None,
            tab_hitboxes: Vec::new(),
            outline_width,
            explorer_visible,
            explorer_views: HashMap::new(),
            current_explorer_root: None,
            explorer_width,
            focus_handle: cx.focus_handle(),
            tab_scroll_handle: ScrollHandle::new(),
            fps_counter: FpsCounter::new(),
            file_watcher,
        }
    }

    #[allow(dead_code)]
    fn debug_trigger_explorer_refresh(&self, cx: &mut Context<Self>) {
        if let Some(root) = self.current_explorer_root.clone() {
            tracing::debug!("debug trigger explorer refresh: {:?}", root);
            self.file_watcher.trigger_root_refresh(root);
            cx.notify();
        }
    }

    fn clear_search_highlight_for_tab(&mut self, tab_index: usize, cx: &mut Context<Self>) {
        if tab_index >= self.tabs.len() {
            return;
        }
        let text_view_state = self.tabs[tab_index].view.read(cx).text_view_state().clone();
        text_view_state.update(cx, |state, cx| {
            state.set_search_query("", cx);
        });
    }

    fn open_tab_context_menu(
        &mut self,
        tab_index: usize,
        position: Point<Pixels>,
        cx: &mut Context<Self>,
    ) {
        if tab_index >= self.tabs.len() {
            return;
        }
        let tab_path = self.tabs[tab_index].path.clone();
        self.tab_tooltip = None;
        self.tab_context_menu = Some(TabContextMenuState {
            position,
            tab_index,
            tab_path,
        });
        cx.notify();
    }

    fn close_tab_context_menu(&mut self, cx: &mut Context<Self>) {
        if self.tab_context_menu.is_some() {
            self.tab_context_menu = None;
            cx.notify();
        }
    }

    fn show_tab_tooltip(&mut self, tab_index: usize, cx: &mut Context<Self>) {
        if tab_index >= self.tabs.len() {
            return;
        }
        if self.tab_context_menu.is_some() {
            return;
        }
        let tab_path = self.tabs[tab_index].path.clone();
        let text = normalize_unc_path(&tab_path.display().to_string());
        let position = self
            .tab_hitboxes
            .iter()
            .find(|(ix, _)| *ix == tab_index)
            .map(|(_, bounds)| {
                let mut pos = bounds.origin;
                // Position tooltip below tab with small spacing.
                pos.y += bounds.size.height + px(4.);
                pos
            });

        if let Some(position) = position {
            self.tab_tooltip = Some(TabTooltipState { position, text });
            cx.notify();
        }
    }

    fn clear_tab_tooltip(&mut self, cx: &mut Context<Self>) {
        if self.tab_tooltip.is_some() {
            self.tab_tooltip = None;
            cx.notify();
        }
    }

    fn ensure_tab_visible(&mut self, tab_index: usize) {
        let Some(tab_bar_bounds) = self.tab_bar_bounds else {
            return;
        };
        let Some((_, tab_bounds)) = self.tab_hitboxes.iter().find(|(ix, _)| *ix == tab_index) else {
            return;
        };

        let current_offset = self.tab_scroll_handle.offset();
        let max_offset = self.tab_scroll_handle.max_offset();
        let mut next_offset_x = current_offset.x;

        let visible_left = tab_bar_bounds.left();
        let visible_right = tab_bar_bounds.right();
        let tab_left = tab_bounds.left() + current_offset.x;
        let tab_right = tab_bounds.right() + current_offset.x;

        if tab_left < visible_left {
            next_offset_x = visible_left - tab_bounds.left();
        } else if tab_right > visible_right {
            next_offset_x = visible_right - tab_bounds.right();
        }

        next_offset_x = next_offset_x.clamp(-max_offset.width, px(0.));

        if next_offset_x != current_offset.x {
            self.tab_scroll_handle
                .set_offset(point(next_offset_x, current_offset.y));
        }
    }

    pub fn open_file(&mut self, path: PathBuf, cx: &mut Context<Self>) {
        self.open_files_with_mode(vec![path], ExplorerUpdateMode::Default, cx);
    }

    pub fn open_files(&mut self, paths: Vec<PathBuf>, cx: &mut Context<Self>) {
        self.open_files_with_mode(paths, ExplorerUpdateMode::Default, cx);
    }

    fn open_file_from_explorer(&mut self, path: PathBuf, cx: &mut Context<Self>) {
        self.open_files_with_mode(vec![path], ExplorerUpdateMode::PreserveRootIfContains, cx);
    }

    fn open_files_with_mode(&mut self, paths: Vec<PathBuf>, mode: ExplorerUpdateMode, cx: &mut Context<Self>) {
        let config = self.config.clone();
        cx.spawn(move |workspace: WeakEntity<WorkspaceView>, cx: &mut AsyncApp| {
            let mut cx = cx.clone();
            async move {
                let mut loaded = Vec::new();
                for path in paths {
                    if let Ok(content) = smol::fs::read_to_string(&path).await {
                        loaded.push((path, content));
                    }
                }

                if !loaded.is_empty() {
                     workspace.update(&mut cx, |workspace, cx| {
                         let workspace_weak = cx.entity().downgrade();  // Get weak reference here
                         for (path, content) in loaded {
                             if let Some(index) = workspace.tabs.iter().position(|t| t.path == path) {
                                 workspace.active_tab_index = index;
                                 workspace.tab_scroll_handle.scroll_to_item(index);
                                 continue;
                             }

                             let title = path.file_name()
                                .map(|s| s.to_string_lossy().to_string())
                                .unwrap_or_else(|| "Untitled".to_string());

                             let doc = cx.new(|_cx| Document::new(content, path.clone()));
                             let view = cx.new(|cx| MarkdownView::new(doc, config.clone(), workspace_weak.clone(), cx));

                             // Observe the MarkdownView's text_view_state for changes
                             let text_view_state = view.read(cx).text_view_state().clone();
                             cx.observe(&text_view_state, |workspace, _, cx| {
                                 // Update outline when text parsing completes
                                 workspace.update_outline(cx);
                             }).detach();

                             workspace.tabs.push(WorkspaceTab {
                                 path: path.clone(),
                                 view,
                                 title,
                             });

                             // Register file watcher for auto-refresh
                             if let Err(e) = workspace.file_watcher.watch(path) {
                                 tracing::warn!("Failed to watch file: {}", e);
                             }

                             workspace.active_tab_index = workspace.tabs.len() - 1;
                             workspace.tab_scroll_handle.scroll_to_item(workspace.active_tab_index);
                         }
                         workspace.update_outline(cx);
                         workspace.update_explorer_with_mode(mode, cx);
                         cx.notify();
                     }).ok();
                }
            }
        }).detach();
    }

    fn close_tab(&mut self, index: usize, cx: &mut Context<Self>) {
        if index >= self.tabs.len() {
            return;
        }

        // Unregister file watcher before removing tab
        let path = self.tabs[index].path.clone();
        if let Err(e) = self.file_watcher.unwatch(&path) {
            tracing::warn!("Failed to unwatch file {:?}: {}", path, e);
        }

        self.tabs.remove(index);

        if self.tabs.is_empty() {
            self.active_tab_index = 0;
            self.outline_view = None;
            self.explorer_views.clear();
            self.current_explorer_root = None;
        } else {
            if self.active_tab_index >= index && self.active_tab_index > 0 {
                self.active_tab_index -= 1;
            }
            if self.active_tab_index >= self.tabs.len() {
                self.active_tab_index = self.tabs.len().saturating_sub(1);
            }
            self.update_outline(cx);
            self.update_explorer(cx);
            self.cleanup_unused_explorers(cx);
        }
        // Ensure the new active tab is scrolled into view
        if !self.tabs.is_empty() {
            self.tab_scroll_handle.scroll_to_item(self.active_tab_index);
        }
        if self.search_bar.is_some() {
            // Clear search bar without restoring focus (tab is being closed)
            self.search_bar = None;
            self.clear_search_highlight_for_tab(self.active_tab_index, cx);
        }
        cx.notify();
    }

    fn close_all_tabs(&mut self, cx: &mut Context<Self>) {
        // Unregister all file watchers
        for tab in &self.tabs {
            if let Err(e) = self.file_watcher.unwatch(&tab.path) {
                tracing::warn!("Failed to unwatch file {:?}: {}", tab.path, e);
            }
        }

        // Unregister all explorer root watchers
        for (root, _) in self.explorer_views.iter() {
            let _ = self.file_watcher.unwatch(root);
        }

        self.tabs.clear();
        self.active_tab_index = 0;
        self.outline_view = None;
        self.explorer_views.clear();
        self.current_explorer_root = None;

        if self.search_bar.is_some() {
            self.search_bar = None;
            self.clear_search_highlight_for_tab(self.active_tab_index, cx);
        }
        cx.notify();
    }

    fn close_other_tabs(&mut self, keep_index: usize, cx: &mut Context<Self>) {
        if keep_index >= self.tabs.len() || self.tabs.len() <= 1 {
            return;
        }

        // Collect indices of tabs to remove (all except keep_index)
        let indices_to_remove: Vec<usize> = (0..self.tabs.len())
            .filter(|&i| i != keep_index)
            .collect();

        // Unregister file watchers for tabs being closed
        for &i in &indices_to_remove {
            let path = self.tabs[i].path.clone();
            if let Err(e) = self.file_watcher.unwatch(&path) {
                tracing::warn!("Failed to unwatch file {:?}: {}", path, e);
            }
        }

        // Keep only the specified tab
        let kept_tab = self.tabs.remove(keep_index);
        self.tabs.clear();
        self.tabs.push(kept_tab);

        self.active_tab_index = 0;
        self.tab_scroll_handle.scroll_to_item(0);

        // Clear search if open
        if self.search_bar.is_some() {
            self.search_bar = None;
            self.clear_search_highlight_for_tab(0, cx);
        }

        self.update_outline(cx);
        self.update_explorer(cx);
        self.cleanup_unused_explorers(cx);
        cx.notify();
    }

    fn activate_tab(&mut self, index: usize, cx: &mut Context<Self>) {
        if index < self.tabs.len() {
            if self.search_bar.is_some() {
                let previous_tab_index = self.active_tab_index;
                self.search_bar = None;
                self.clear_search_highlight_for_tab(previous_tab_index, cx);
            }
            self.active_tab_index = index;
            self.tab_scroll_handle.scroll_to_item(index);
            self.update_outline(cx);
            self.update_explorer(cx);
            cx.notify();
        }
    }

    fn toggle_outline(&mut self, cx: &mut Context<Self>) {
        self.outline_visible = !self.outline_visible;

        // Save to config
        let visible = self.outline_visible;
        self.config.update(cx, |config, _| {
            config.appearance.outline_visible = visible;
            config.save();
        });

        cx.notify();
    }

    fn toggle_explorer(&mut self, cx: &mut Context<Self>) {
        self.explorer_visible = !self.explorer_visible;
        if self.explorer_visible {
            self.update_explorer(cx);
        }
        let visible = self.explorer_visible;
        self.config.update(cx, |config, _| {
            config.appearance.explorer_visible = visible;
            config.save();
        });
        cx.notify();
    }

    fn update_outline(&mut self, cx: &mut Context<Self>) {
        if self.tabs.is_empty() {
            self.outline_view = None;
            return;
        }

        let tab = &self.tabs[self.active_tab_index];
        let headings = tab.view.read(cx).headings(cx);
        let markdown_view = tab.view.clone();
        let outline_width = self.outline_width;
        let workspace = cx.entity().downgrade();

        if let Some(outline_view) = &self.outline_view {
            outline_view.update(cx, |view, cx| {
                view.set_headings(headings);
                view.set_width(outline_width, cx);
                view.set_on_click(move |block_index, _window, cx| {
                    markdown_view.update(cx, |view, cx| {
                        view.scroll_to_heading(block_index, cx);
                    });
                }, cx);
            });
            return;
        }

        let workspace_for_close = workspace.clone();
        let config_for_width = self.config.clone();
        self.outline_view = Some(cx.new(|_| {
            OutlineView::new(headings)
                .width(outline_width)
                .on_width_change(move |width, cx| {
                    if let Some(ws) = workspace.upgrade() {
                        ws.update(cx, |ws, _| {
                            ws.outline_width = width;
                        });
                    }
                    // Save width to config immediately
                    config_for_width.update(cx, |config, _| {
                        config.appearance.outline_width = width;
                        config.save();
                    });
                })
                .on_click(move |block_index, _window, cx| {
                    markdown_view.update(cx, |view, cx| {
                        view.scroll_to_heading(block_index, cx);
                    });
                })
                .on_close(move |_window, cx| {
                    if let Some(ws) = workspace_for_close.upgrade() {
                        ws.update(cx, |ws, cx| {
                            ws.toggle_outline(cx);
                        });
                    }
                })
        }));
    }

    fn set_explorer_root_mode(&mut self, mode: ExplorerRootMode, cx: &mut Context<Self>) {
        self.config.update(cx, |config, _| {
            config.appearance.explorer_root_mode = mode;
            config.save();
        });
        self.update_explorer(cx);
        cx.notify();
    }

    fn set_explorer_sort_mode(&mut self, mode: ExplorerSortMode, cx: &mut Context<Self>) {
        self.config.update(cx, |config, _| {
            config.appearance.explorer_sort_mode = mode;
            config.save();
        });

        for explorer_view in self.explorer_views.values() {
            explorer_view.update(cx, |view, cx| {
                view.set_sort_mode(mode, cx);
            });
        }

        cx.notify();
    }

    /// Reveal a file in the explorer sidebar
    fn reveal_in_explorer(&mut self, path: PathBuf, cx: &mut Context<Self>) {
        // Ensure explorer is visible
        if !self.explorer_visible {
            self.explorer_visible = true;
            self.config.update(cx, |config, _| {
                config.appearance.explorer_visible = true;
                config.save();
            });
        }

        // Canonicalize to reduce path-mismatch between explorer entries and selection paths.
        // On Windows, canonicalize may introduce the \\?\ prefix; strip it to match ignore::WalkBuilder paths.
        let path = path.canonicalize().unwrap_or(path);
        let path = normalize_unc_pathbuf(&path);

        // Calculate root based on the file path (NOT the current active tab)
        let current_dir = match path.parent() {
            Some(dir) => dir.to_path_buf(),
            None => return,
        };

        let (root_mode, markers, sort_mode) = {
            let cfg = self.config.read(cx);
            (
                cfg.appearance.explorer_root_mode,
                cfg.appearance.project_root_markers.clone(),
                cfg.appearance.explorer_sort_mode,
            )
        };

        let root = match root_mode {
            ExplorerRootMode::CurrentDir => current_dir.clone(),
            ExplorerRootMode::ProjectRoot => {
                Self::find_project_root(&current_dir, &markers)
                    .unwrap_or_else(|| current_dir.clone())
            }
        };

        // Normalize to reduce path-mismatch between watcher and explorer map keys.
        let root = root.canonicalize().unwrap_or(root);
        let root = normalize_unc_pathbuf(&root);

        // Ensure we have an explorer for this root (create if needed)
        if !self.explorer_views.contains_key(&root) {
            // Register root watcher
            if let Err(e) = self.file_watcher.watch_root(root.clone()) {
                tracing::warn!("Failed to watch explorer root {:?}: {}", root, e);
            }

            // Need to create a new explorer for this root
            let explorer_width = self.explorer_width;

            let config = self.config.clone();
            let workspace = cx.entity().downgrade();
            let workspace_for_width = workspace.clone();
            let workspace_for_click = workspace.clone();
            let workspace_for_close = workspace.clone();
            let workspace_for_root_mode = workspace.clone();
            let workspace_for_sort_mode = workspace.clone();
            let root_for_expanded = root.clone();
            let config_for_markers = config.clone();
            let config_for_width = config.clone();
            let config_for_expanded = config.clone();

            let explorer_view = cx.new(|_| {
                FileExplorerView::new()
                    .on_width_change(move |width, cx| {
                        if let Some(ws) = workspace_for_width.upgrade() {
                            ws.update(cx, |ws, _| ws.explorer_width = width);
                        }
                        config_for_width.update(cx, |config, _| {
                            config.appearance.explorer_width = width;
                            config.save();
                        });
                    })
                    .on_click(move |path, _window, cx| {
                        if let Some(ws) = workspace_for_click.upgrade() {
                            ws.update(cx, |ws, cx| {
                                ws.open_file_from_explorer(path, cx);
                            });
                        }
                    })
                    .on_close(move |_window, cx| {
                        if let Some(ws) = workspace_for_close.upgrade() {
                            ws.update(cx, |ws, cx| {
                                ws.toggle_explorer(cx);
                            });
                        }
                    })
                    .on_expanded_change(move |expanded, cx| {
                        let relative: Vec<String> = expanded.iter()
                            .filter_map(|p| p.strip_prefix(&root_for_expanded).ok())
                            .map(|p| p.to_string_lossy().to_string())
                            .collect();
                        config_for_expanded.update(cx, |config, _| {
                            config.appearance.expanded_dirs = relative;
                            config.save();
                        });
                    })
                    .on_root_mode_change(move |mode, cx| {
                        if let Some(ws) = workspace_for_root_mode.upgrade() {
                            ws.update(cx, |ws, cx| {
                                ws.set_explorer_root_mode(mode, cx);
                            });
                        }
                    })
                    .on_sort_mode_change(move |mode, cx| {
                        if let Some(ws) = workspace_for_sort_mode.upgrade() {
                            ws.update(cx, |ws, cx| {
                                ws.set_explorer_sort_mode(mode, cx);
                            });
                        }
                    })
                    .on_edit_markers(move |window, cx| {
                        settings_dialog::open_settings_dialog(config_for_markers.clone(), window, cx);
                    })
            });

            // Initialize with restored expanded_dirs from config
            let restored_expanded = {
                let cfg = self.config.read(cx);
                let expanded_dirs: HashSet<PathBuf> = cfg.appearance.expanded_dirs.iter()
                    .map(|rel| root.join(rel))
                    .collect();
                expanded_dirs
            };

            // Calculate parent directories that need to be expanded for the target file
            let mut dirs_to_expand_for_new = restored_expanded.clone();
            {
                let mut current = path.parent();
                while let Some(dir) = current {
                    if dir.starts_with(&root) && dir != root {
                        dirs_to_expand_for_new.insert(dir.to_path_buf());
                    }
                    if dir == root || !dir.starts_with(&root) {
                        break;
                    }
                    current = dir.parent();
                }
            }

            explorer_view.update(cx, |view, cx| {
                // Set metadata before triggering root scan
                view.set_width(explorer_width, cx);
                view.set_root_mode(root_mode, cx);
                view.set_sort_mode(sort_mode, cx);
                *view.expanded_dirs_mut() = dirs_to_expand_for_new;

                // set_root triggers async refresh_entries which uses current view state
                view.set_root(Some(root.clone()), cx);
                view.set_selected_path(Some(path.clone()), cx);
            });


            self.explorer_views.insert(root.clone(), explorer_view);
        }

        // Switch to this explorer root
        self.current_explorer_root = Some(root.clone());

        // Ensure root watcher is active
        if let Err(e) = self.file_watcher.watch_root(root.clone()) {
            tracing::warn!("Failed to watch explorer root {:?}: {}", root, e);
        }

        // Expand all parent directories of the file
        if let Some(explorer_view) = self.explorer_views.get(&root) {
            explorer_view.update(cx, |view, cx| {
                // Find all parent directories between root and file
                let mut current = path.parent();
                let mut dirs_to_expand = Vec::new();

                while let Some(dir) = current {
                    if dir.starts_with(&root) && dir != root {
                        dirs_to_expand.push(dir.to_path_buf());
                    }
                    if dir == root || !dir.starts_with(&root) {
                        break;
                    }
                    current = dir.parent();
                }

                // Batch expand: add all directories to expanded_dirs at once
                let mut needs_refresh = false;
                for dir in &dirs_to_expand {
                    if view.expanded_dirs_mut().insert(dir.clone()) {
                        needs_refresh = true;
                    }
                }

                // Set the selected path first so scrolling targets the right item.
                view.set_selected_path(Some(path.clone()), cx);

                // Only refresh once after expanding all directories
                if needs_refresh {
                    view.refresh_entries(cx);
                } else {
                    // Directories already expanded, scroll immediately
                    view.scroll_to_selected(cx);
                }
            });
        }

        cx.notify();
    }

    fn update_explorer(&mut self, cx: &mut Context<Self>) {
        self.update_explorer_with_mode(ExplorerUpdateMode::Default, cx);
    }

    fn update_explorer_with_mode(&mut self, mode: ExplorerUpdateMode, cx: &mut Context<Self>) {
        if self.tabs.is_empty() {
            self.current_explorer_root = None;
            return;
        }

        // Get current file path and parent directory
        let tab = &self.tabs[self.active_tab_index];
        let current_file = tab.path.clone();
        let current_file = current_file.canonicalize().unwrap_or(current_file);
        let current_file = normalize_unc_pathbuf(&current_file);
        let current_dir = current_file.parent().map(|p| p.to_path_buf());
        let Some(current_dir) = current_dir else { return };

        // Resolve explorer root based on config
        let (root_mode, markers, sort_mode) = {
            let cfg = self.config.read(cx);
            (
                cfg.appearance.explorer_root_mode,
                cfg.appearance.project_root_markers.clone(),
                cfg.appearance.explorer_sort_mode,
            )
        };

        let mut root = match root_mode {
            ExplorerRootMode::CurrentDir => current_dir.clone(),
            ExplorerRootMode::ProjectRoot => {
                Self::find_project_root(&current_dir, &markers)
                    .unwrap_or_else(|| current_dir.clone())
            }
        };

        let mut use_current_root = false;
        if mode == ExplorerUpdateMode::PreserveRootIfContains {
            if let Some(current_root) = self.current_explorer_root.clone() {
                root = current_root;
                use_current_root = true;
            }
        }

        // Normalize to reduce path-mismatch between watcher and explorer map keys.
        let root = if use_current_root {
            root
        } else {
            let root = root.canonicalize().unwrap_or(root);
            normalize_unc_pathbuf(&root)
        };

        let explorer_width = self.explorer_width;

        // Check if we already have an explorer for this root
        if let Some(explorer_view) = self.explorer_views.get(&root) {
            // Ensure root watcher is active (it may have been cleared previously)
            if let Err(e) = self.file_watcher.watch_root(root.clone()) {
                tracing::warn!("Failed to watch explorer root {:?}: {}", root, e);
            }

            // Same root - just update selection and width
            let current_file_for_view = current_file.clone();
            explorer_view.update(cx, |view, cx| {
                view.set_selected_path(Some(current_file_for_view), cx);
                view.set_width(explorer_width, cx);
                view.set_root_mode(root_mode, cx);
                view.set_sort_mode(sort_mode, cx);
            });
            self.current_explorer_root = Some(root);
            return;
        }

        // Create new explorer view for this root
        // Register root watcher
        if let Err(e) = self.file_watcher.watch_root(root.clone()) {
            tracing::warn!("Failed to watch explorer root {:?}: {}", root, e);
        }

        let config = self.config.clone();

        let workspace = cx.entity().downgrade();
        let workspace_for_width = workspace.clone();
        let workspace_for_click = workspace.clone();
        let workspace_for_close = workspace.clone();
        let workspace_for_root_mode = workspace.clone();
        let workspace_for_sort_mode = workspace.clone();
        let root_for_expanded = root.clone();
        let config_for_markers = config.clone();
        let config_for_width = config.clone();
        let config_for_expanded = config.clone();

        let explorer_view = cx.new(|_| {
            FileExplorerView::new()
                .on_width_change(move |width, cx| {
                    if let Some(ws) = workspace_for_width.upgrade() {
                        ws.update(cx, |ws, _| ws.explorer_width = width);
                    }
                    config_for_width.update(cx, |config, _| {
                        config.appearance.explorer_width = width;
                        config.save();
                    });
                })
                .on_click(move |path, _window, cx| {
                    if let Some(ws) = workspace_for_click.upgrade() {
                        ws.update(cx, |ws, cx| {
                            ws.open_file_from_explorer(path, cx);
                        });
                    }
                })
                .on_close(move |_window, cx| {
                    if let Some(ws) = workspace_for_close.upgrade() {
                        ws.update(cx, |ws, cx| {
                            ws.toggle_explorer(cx);
                        });
                    }
                })
                .on_expanded_change(move |expanded, cx| {
                    let relative: Vec<String> = expanded.iter()
                        .filter_map(|p| p.strip_prefix(&root_for_expanded).ok())
                        .map(|p| p.to_string_lossy().to_string())
                        .collect();
                    config_for_expanded.update(cx, |config, _| {
                        config.appearance.expanded_dirs = relative;
                        config.save();
                    });
                })
                .on_root_mode_change(move |mode, cx| {
                    if let Some(ws) = workspace_for_root_mode.upgrade() {
                        ws.update(cx, |ws, cx| {
                            ws.set_explorer_root_mode(mode, cx);
                        });
                    }
                })
                .on_sort_mode_change(move |mode, cx| {
                    if let Some(ws) = workspace_for_sort_mode.upgrade() {
                        ws.update(cx, |ws, cx| {
                            ws.set_explorer_sort_mode(mode, cx);
                        });
                    }
                })
                .on_edit_markers(move |window, cx| {
                    settings_dialog::open_settings_dialog(config_for_markers.clone(), window, cx);
                })
        });

        // Initialize with restored expanded_dirs from config
        let restored_expanded = {
            let cfg = self.config.read(cx);
            let expanded_dirs: HashSet<PathBuf> = cfg.appearance.expanded_dirs.iter()
                .map(|rel| root.join(rel))
                .collect();
            expanded_dirs
        };

        // Calculate parent directories that need to be expanded for the target file
        let mut dirs_to_expand_for_new = restored_expanded.clone();
        {
            let mut current = current_file.parent();
            while let Some(dir) = current {
                if dir.starts_with(&root) && dir != root {
                    dirs_to_expand_for_new.insert(dir.to_path_buf());
                }
                if dir == root || !dir.starts_with(&root) {
                    break;
                }
                current = dir.parent();
            }
        }

        // Initialize the explorer
        let current_file_for_view = current_file.clone();
        explorer_view.update(cx, |view, cx| {
            view.set_width(explorer_width, cx);
            view.set_root_mode(root_mode, cx);
            view.set_sort_mode(sort_mode, cx);
            *view.expanded_dirs_mut() = dirs_to_expand_for_new;
            view.set_root(Some(root.clone()), cx);
            view.set_selected_path(Some(current_file_for_view), cx);
        });


        self.explorer_views.insert(root.clone(), explorer_view);
        self.current_explorer_root = Some(root);
    }

    /// Get the current active explorer view (if any)
    fn current_explorer_view(&self) -> Option<&Entity<FileExplorerView>> {
        self.current_explorer_root.as_ref()
            .and_then(|root| self.explorer_views.get(root))
    }

    /// Clean up explorer instances for roots that no longer have any tabs
    fn cleanup_unused_explorers(&mut self, cx: &mut Context<Self>) {
        // Read config to determine root mode
        let (root_mode, markers, _sort_mode) = {
            let cfg = self.config.read(cx);
            (
                cfg.appearance.explorer_root_mode,
                cfg.appearance.project_root_markers.clone(),
                cfg.appearance.explorer_sort_mode,
            )
        };

        // Collect all roots that have at least one tab
        let active_roots: std::collections::HashSet<PathBuf> = self.tabs.iter()
            .filter_map(|tab| {
                let current_dir = tab.path.parent()?;
                match root_mode {
                    ExplorerRootMode::CurrentDir => Some(current_dir.to_path_buf()),
                    ExplorerRootMode::ProjectRoot => {
                        Self::find_project_root(current_dir, &markers)
                            .or_else(|| Some(current_dir.to_path_buf()))
                    }
                }
            })
            .collect();

        // Remove explorers for roots that have no tabs
        for (root, _) in self.explorer_views.iter() {
            if !active_roots.contains(root) {
                let _ = self.file_watcher.unwatch(root);
            }
        }
        self.explorer_views.retain(|root, _| active_roots.contains(root));


        // Clear current_explorer_root if it was removed
        if let Some(ref current) = self.current_explorer_root {
            if !self.explorer_views.contains_key(current) {
                self.current_explorer_root = None;
            }
        }
    }

    fn find_project_root(start: &std::path::Path, markers: &[String]) -> Option<PathBuf> {
        // Hardcoded VCS markers (not user-configurable)
        const BOTTOM_UP_VCS: &[&str] = &[".git", ".hg", ".pijul", "_darcs", ".bzr", ".jj"];
        const RECURRING_VCS: &[&str] = &[".svn", "CVS"];

        let mut bottom_up_root: Option<PathBuf> = None;
        let mut top_down_root: Option<PathBuf> = None;
        let mut recurring_hits: Vec<PathBuf> = Vec::new();

        let mut dir = Some(start);
        while let Some(current) = dir {
            // Check hardcoded bottom-up VCS markers
            for &vcs in BOTTOM_UP_VCS {
                if current.join(vcs).exists() && bottom_up_root.is_none() {
                    bottom_up_root = Some(current.to_path_buf());
                    break;
                }
            }

            // Check hardcoded recurring VCS markers
            for &vcs in RECURRING_VCS {
                if current.join(vcs).exists() {
                    recurring_hits.push(current.to_path_buf());
                    break;
                }
            }

            // Check user-configurable markers (top-down: find outermost)
            for marker in markers {
                let trimmed = marker.trim();
                if !trimmed.is_empty() && current.join(trimmed).exists() {
                    top_down_root = Some(current.to_path_buf());
                    break;
                }
            }

            dir = current.parent();
        }

        // Process recurring hits: find the bottommost in the topmost sequence
        let recurring_root = if !recurring_hits.is_empty() {
            // Hits are collected bottom-up, so reverse to get top-down order
            recurring_hits.reverse();
            // Find the last consecutive hit from the top
            let mut result = recurring_hits[0].clone();
            for i in 1..recurring_hits.len() {
                if recurring_hits[i].parent() == Some(recurring_hits[i - 1].as_path()) {
                    result = recurring_hits[i].clone();
                } else {
                    break;
                }
            }
            Some(result)
        } else {
            None
        };

        // Priority: bottom-up VCS > recurring VCS > user markers (top-down)
        bottom_up_root.or(recurring_root).or(top_down_root)
    }

    fn open_search(&mut self, window: &mut Window, cx: &mut Context<Self>) {
        if self.tabs.is_empty() {
            return;
        }

        if self.search_bar.is_some() {
            // Already open, just focus
            if let Some(search_bar) = &self.search_bar {
                let handle = search_bar
                    .read(cx)
                    .input_state()
                    .read(cx)
                    .focus_handle(cx)
                    .clone();
                window.focus(&handle);
            }
            return;
        }

        let workspace = cx.entity().downgrade();
        let markdown_view = self.tabs[self.active_tab_index].view.clone();

        let search_bar = cx.new(|cx| {
            SearchBar::new(window, cx)
                .on_navigate(move |block_index, _window, cx| {
                    markdown_view.update(cx, |view, cx| {
                        view.scroll_to_heading(block_index, cx);
                    });
                })
                .on_close({
                    let workspace = workspace.clone();
                    move |window, cx| {
                        if let Some(ws) = workspace.upgrade() {
                            ws.update(cx, |ws, cx| {
                                ws.close_search(window, cx);
                            });
                        }
                    }
                })
                .on_change({
                    let workspace = workspace.clone();
                    move |query, cx| {
                        if let Some(ws) = workspace.upgrade() {
                            ws.update(cx, |ws, cx| {
                                ws.update_search(query, cx);
                            });
                        }
                    }
                })
        });

        // Focus the input inside search bar
        let input_handle = search_bar.read(cx).input_state().read(cx).focus_handle(cx).clone();
        window.focus(&input_handle);

        self.search_bar = Some(search_bar);
        cx.notify();
    }

    fn close_search(&mut self, window: &mut Window, cx: &mut Context<Self>) {
        self.search_bar = None;
        self.clear_search_highlight_for_tab(self.active_tab_index, cx);
        // Restore focus to workspace so Ctrl+F can work again
        window.focus(&self.focus_handle);
        cx.notify();
    }

    fn update_search(&mut self, query: &str, cx: &mut Context<Self>) {
        if self.tabs.is_empty() {
            return;
        }

        let tab = &self.tabs[self.active_tab_index];
        let source = tab.view.read(cx).source_text(cx);
        let block_spans = tab.view.read(cx).block_spans(cx);

        let mut state = SearchState::default();
        state.search(source.as_ref(), query, &block_spans);
        let query_text = query.to_string();
        let text_view_state = tab.view.read(cx).text_view_state().clone();
        text_view_state.update(cx, |state, cx| {
            state.set_search_query(&query_text, cx);
        });

        // Navigate to first match if any
        let first_block = state.current().map(|m| m.block_index);

        if let Some(search_bar) = self.search_bar.clone() {
            cx.defer(move |cx| {
                search_bar
                    .update(cx, |bar, cx| {
                        bar.update_state(state, cx);
                    });
            });
        }

        // Scroll to first match
        if let Some(block_index) = first_block {
            tab.view.update(cx, |view, cx| {
                view.scroll_to_heading(block_index, cx);
            });
        }
    }

    fn next_tab(&mut self, cx: &mut Context<Self>) {
        if self.tabs.is_empty() {
            return;
        }
        let next = (self.active_tab_index + 1) % self.tabs.len();
        self.activate_tab(next, cx);
    }

    fn prev_tab(&mut self, cx: &mut Context<Self>) {
        if self.tabs.is_empty() {
            return;
        }
        let prev = if self.active_tab_index == 0 {
            self.tabs.len() - 1
        } else {
            self.active_tab_index - 1
        };
        self.activate_tab(prev, cx);
    }

    fn close_current_tab(&mut self, cx: &mut Context<Self>) {
        if !self.tabs.is_empty() {
            self.close_tab(self.active_tab_index, cx);
        }
    }

    fn refresh_document(&mut self, cx: &mut Context<Self>) {
        if self.tabs.is_empty() {
            return;
        }
        let tab = &self.tabs[self.active_tab_index];
        let path = tab.path.clone();
        let text_view_state = tab.view.read(cx).text_view_state().clone();

        cx.spawn(|_workspace: WeakEntity<WorkspaceView>, cx: &mut AsyncApp| {
            let cx = cx.clone();
            async move {
                if let Ok(content) = smol::fs::read_to_string(&path).await {
                    _ = cx.update(|cx| {
                        text_view_state.update(cx, |state, cx| {
                            state.set_text(&content, cx);
                        });
                    });
                }
            }
        })
        .detach();
    }

    /// Handle file change event from file watcher.
    fn handle_file_change(&mut self, event: crate::file_watcher::WatchEvent, cx: &mut Context<Self>) {
        match event {
            crate::file_watcher::WatchEvent::FileModified(path) => {
                // Find all tabs with this path and refresh them
                let canonical_path = path.canonicalize().unwrap_or_else(|_| path.clone());

                for index in 0..self.tabs.len() {
                    let tab_path = self.tabs[index]
                        .path
                        .canonicalize()
                        .unwrap_or_else(|_| self.tabs[index].path.clone());

                    if tab_path == canonical_path {
                        tracing::info!("Auto-refreshing tab {}: {:?}", index, path);
                        self.refresh_tab_at(index, cx);
                    }
                }
            }
            crate::file_watcher::WatchEvent::RootChanged(root) => {
                // Normalize watcher root to match `explorer_views` keys.
                let root = root.canonicalize().unwrap_or_else(|_| root.clone());
                let root = normalize_unc_pathbuf(&root);

                if let Some(view) = self.explorer_views.get(&root) {
                    tracing::debug!("Refreshing explorer for root: {:?}", root);
                    view.update(cx, |view, cx| {
                        view.refresh_entries(cx);
                    });
                } else {
                    // Fallback: avoid missing updates due to path normalization mismatches.
                    tracing::debug!("Refreshing all explorers (unmatched root): {:?}", root);
                    for view in self.explorer_views.values() {
                        view.update(cx, |view, cx| {
                            view.refresh_entries(cx);
                        });
                    }
                }
            }
        }
    }

    /// Refresh a specific tab by index.

    fn refresh_tab_at(&mut self, index: usize, cx: &mut Context<Self>) {
        if index >= self.tabs.len() {
            return;
        }

        let tab = &self.tabs[index];
        let path = tab.path.clone();
        let text_view_state = tab.view.read(cx).text_view_state().clone();

        cx.spawn(|_workspace: WeakEntity<WorkspaceView>, cx: &mut AsyncApp| {
            let cx = cx.clone();
            async move {
                if let Ok(content) = smol::fs::read_to_string(&path).await {
                    _ = cx.update(|cx| {
                        text_view_state.update(cx, |state, cx| {
                            state.set_text(&content, cx);
                        });
                    });
                }
            }
        })
        .detach();
    }

    fn scroll_to_top(&mut self, cx: &mut Context<Self>) {
        if self.tabs.is_empty() {
            return;
        }
        let text_view_state = self.tabs[self.active_tab_index].view.read(cx).text_view_state().clone();
        text_view_state.update(cx, |state, _| {
            state.scroll_to_block(0);
        });
    }

    fn scroll_to_bottom(&mut self, cx: &mut Context<Self>) {
        if self.tabs.is_empty() {
            return;
        }
        let text_view_state = self.tabs[self.active_tab_index].view.read(cx).text_view_state().clone();
        let block_count = text_view_state.read(cx).block_count();
        if block_count > 0 {
            text_view_state.update(cx, |state, _| {
                state.scroll_to_block(block_count.saturating_sub(1));
            });
        }
    }

    fn select_all(&mut self, cx: &mut Context<Self>) {
        if self.tabs.is_empty() {
            return;
        }
        let tab = &self.tabs[self.active_tab_index];
        let text_view_state = tab.view.read(cx).text_view_state().clone();
        text_view_state.update(cx, |state, cx| {
            state.select_all(cx);
        });
    }

    fn open_file_dialog(&mut self, cx: &mut Context<Self>) {
        let paths_rx = cx.prompt_for_paths(PathPromptOptions {
            files: true,
            directories: false,
            multiple: true,
            prompt: Some("Select Markdown files".into()),
        });

        cx.spawn(|workspace: WeakEntity<WorkspaceView>, cx: &mut AsyncApp| {
            let mut cx = cx.clone();
            async move {
                if let Ok(Ok(Some(paths))) = paths_rx.await {
                    // Filter markdown files only
                    let md_paths: Vec<_> = paths
                        .into_iter()
                        .filter(|p| {
                            p.extension()
                                .and_then(|e| e.to_str())
                                .map(|e| e.eq_ignore_ascii_case("md") || e.eq_ignore_ascii_case("markdown"))
                                .unwrap_or(false)
                        })
                        .collect();

                    if !md_paths.is_empty() {
                        workspace
                            .update(&mut cx, |ws, cx| {
                                ws.open_files(md_paths, cx);
                            })
                            .ok();
                    }
                }
            }
        })
        .detach();
    }
}

impl Render for WorkspaceView {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let theme = cx.theme().clone();
        let show_fps = self.config.read(cx).appearance.show_fps;

        // Update window title based on active tab
        if let Some(tab) = self.tabs.get(self.active_tab_index) {
            let path_str = normalize_unc_path(&tab.path.display().to_string());
            let title = format!("{} - Peekdown", path_str);
            window.set_window_title(&title);
        } else {
            window.set_window_title("Peekdown");
        }

        // Update FPS counter only if showing
        let fps_text = if show_fps {
            self.fps_counter.tick();
            Some(format!("{:.0} FPS", self.fps_counter.fps()))
        } else {
            None
        };

        // Check if explorer is resizing
        let is_explorer_resizing = if let Some(explorer_view) = self.current_explorer_view() {
            explorer_view.read(cx).is_resizing()
        } else {
            false
        };

        // Check if outline is resizing
        let is_outline_resizing = if let Some(outline_view) = &self.outline_view {
            outline_view.read(cx).is_resizing()
        } else {
            false
        };

        let is_resizing = is_explorer_resizing || is_outline_resizing;

        // Use cached sidebars if visible
        let explorer_sidebar = if self.explorer_visible {
            self.current_explorer_view().cloned()
        } else {
            None
        };

        let outline_sidebar = if self.outline_visible {
            self.outline_view.clone()
        } else {
            None
        };

        let body_content = if self.tabs.is_empty() {
            div()
                .relative()
                .size_full()
                .flex_grow()
                .child(render_welcome(cx))
                // No outline toggle button when no documents are open
        } else {
            let tab = &self.tabs[self.active_tab_index];
            div()
                .relative()
                .size_full()
                .flex_grow()
                .child(tab.view.clone())
        };

        // Get dialog layer to render on top
        let dialog_layer = Root::render_dialog_layer(window, cx);

        // Search bar overlay
        let search_bar = self.search_bar.clone();
        let tab_context_menu = self.tab_context_menu.clone();
        let tab_tooltip = self.tab_tooltip.clone();

        // Clone views for event handlers
        let explorer_for_move = self.current_explorer_view().cloned();
        let explorer_for_up = self.current_explorer_view().cloned();
        let outline_for_move = self.outline_view.clone();
        let outline_for_up = self.outline_view.clone();

        div()
            .flex()
            .flex_col()
            .size_full()
            .relative()
            .track_focus(&self.focus_handle)
            .key_context(if self.search_bar.is_some() {
                "Workspace search = open"
            } else {
                "Workspace"
            })
            .bg(theme.background)
            .text_color(theme.foreground)
            .on_action(cx.listener(|workspace, _: &OpenFileDialog, _window, cx| {
                workspace.open_file_dialog(cx);
            }))
            .on_action(cx.listener(|workspace, _: &OpenSearch, window, cx| {
                workspace.open_search(window, cx);
            }))
            .on_action(cx.listener(|workspace, _: &CloseSearch, window, cx| {
                if workspace.search_bar.is_some() {
                    workspace.close_search(window, cx);
                }
            }))
            .on_action(cx.listener(|workspace, _: &NextTab, _window, cx| {
                workspace.next_tab(cx);
            }))
            .on_action(cx.listener(|workspace, _: &PrevTab, _window, cx| {
                workspace.prev_tab(cx);
            }))
            .on_action(cx.listener(|workspace, _: &CloseTab, _window, cx| {
                workspace.close_current_tab(cx);
            }))
            .on_action(cx.listener(|workspace, _: &RefreshDocument, _window, cx| {
                workspace.refresh_document(cx);
            }))
            .on_action(cx.listener(|workspace, _: &ToggleOutline, _window, cx| {
                workspace.toggle_outline(cx);
            }))
            .on_action(cx.listener(|workspace, _: &ToggleExplorer, _window, cx| {
                workspace.toggle_explorer(cx);
            }))
            .on_action(cx.listener(|workspace, _: &SelectAll, _window, cx| {
                workspace.select_all(cx);
            }))
            .on_action(cx.listener(|workspace, _: &ScrollToTop, _window, cx| {
                workspace.scroll_to_top(cx);
            }))
            .on_action(cx.listener(|workspace, _: &ScrollToBottom, _window, cx| {
                workspace.scroll_to_bottom(cx);
            }))
            .on_action(cx.listener(|workspace, action: &SetExplorerRootMode, _window, cx| {
                workspace.set_explorer_root_mode(action.0, cx);
            }))
            .on_action(cx.listener(|workspace, action: &RevealInExplorer, _window, cx| {
                workspace.reveal_in_explorer(action.path.clone(), cx);
            }))
            .on_action(cx.listener(|workspace, action: &CloseTabAt, _window, cx| {
                workspace.close_tab(action.0, cx);
            }))
            .on_drop(cx.listener(|workspace, event: &ExternalPaths, _, cx| {
                workspace.open_files(event.paths().to_vec(), cx);
            }))
            // Handle global mouse move when resizing
            .when(is_resizing, |this| {
                this.on_mouse_move(move |event: &MouseMoveEvent, _, cx| {
                    if let Some(explorer) = &explorer_for_move {
                        explorer.update(cx, |view, cx| {
                            view.handle_resize_move(f32::from(event.position.x), cx);
                        });
                    }
                    if let Some(outline) = &outline_for_move {
                        outline.update(cx, |view, cx| {
                            view.handle_resize_move(f32::from(event.position.x), cx);
                        });
                    }
                })
                .on_mouse_up(MouseButton::Left, move |_, _, cx| {
                    if let Some(explorer) = &explorer_for_up {
                        explorer.update(cx, |view, cx| {
                            view.end_resize(cx);
                        });
                    }
                    if let Some(outline) = &outline_for_up {
                        outline.update(cx, |view, cx| {
                            view.end_resize(cx);
                        });
                    }
                })
            })
            .child(
                // Main area: Explorer + Right content
                div()
                    .relative()
                    .flex()
                    .flex_row()
                    .flex_grow()
                    .overflow_hidden()
                    .children(explorer_sidebar)  // Explorer on left, full height
                    .child(
                        // Right side content
                        div()
                            .flex()
                            .flex_col()
                            .flex_grow()
                            .overflow_hidden()
                            .child(header::render_header(self, cx))  // Tab bar on right
                            .child(
                                // Body content area
                                div()
                                    .relative()
                                    .flex()
                                    .flex_row()
                                    .flex_grow()
                                    .overflow_hidden()
                                    .child(body_content)
                                    .children(outline_sidebar)
                                    // Explorer toggle button at top-left (when explorer hidden)
                                    .when(!self.explorer_visible && !self.tabs.is_empty(), |this| {
                                        this.child(
                                            deferred(
                                                div()
                                                    .absolute()
                                                    .top_2()
                                                    .left_2()
                                                    .child(
                                                        Button::new("explorer-toggle-btn")
                                                            .icon(Icon::new(IconName::Folder))
                                                            .ghost()
                                                            .small()
                                                            .on_click(cx.listener(|workspace, _, _window, cx| {
                                                                workspace.toggle_explorer(cx);
                                                            })),
                                                    ),
                                            )
                                            .priority(10),
                                        )
                                    })
                                    // Outline toggle button at top-right (when outline hidden and has tabs)
                                    .when(!self.outline_visible && !self.tabs.is_empty(), |this| {
                                        this.child(
                                            deferred(
                                                div()
                                                    .absolute()
                                                    .top_2()
                                                    .right_2()
                                                    .child(
                                                        Button::new("outline-toggle-btn")
                                                            .icon(Icon::new(IconName::Menu))
                                                            .ghost()
                                                            .small()
                                                            .on_click(cx.listener(|workspace, _, _window, cx| {
                                                                workspace.toggle_outline(cx);
                                                            })),
                                                    ),
                                            )
                                            .priority(10),
                                        )
                                    })
                                    // Search bar overlay at top-right
                                    .when_some(search_bar, |this, search_bar| {
                                        this.child(
                                            div()
                                                .absolute()
                                                .top_2()
                                                .right_4()
                                                .child(search_bar)
                                        )
                                    }),
                            )
                    )
            )
            .when_some(tab_tooltip, |this, tooltip| {
                let theme = theme.clone();
                let position = tooltip.position;
                let text = tooltip.text.clone();
                this.child(
                    deferred(
                        div()
                            .absolute()
                            .inset_0()
                            .child(
                                anchored()
                                    .position(position)
                                    .snap_to_window_with_margin(px(8.))
                                    .anchor(Corner::TopLeft)
                                    .child(
                                        div()
                                            .bg(theme.popover)
                                            .text_color(theme.popover_foreground)
                                            .border_1()
                                            .border_color(theme.border)
                                            .rounded(px(6.))
                                            .shadow_md()
                                            .py_0p5()
                                            .px_2()
                                            .text_sm()
                                            .child(text),
                                    ),
                            )
                    )
                    .priority(15),
                )
            })
            .when_some(tab_context_menu, |this, menu_state| {
                let theme = theme.clone();
                let position = menu_state.position;
                let tab_index = menu_state.tab_index;
                let tab_path = menu_state.tab_path.clone();
                this.child(
                    deferred(
                        div()
                            .absolute()
                            .inset_0()
                            .occlude()
                            .on_mouse_down(MouseButton::Left, cx.listener(|workspace, _event: &MouseDownEvent, _window, cx| {
                                cx.stop_propagation();
                                workspace.close_tab_context_menu(cx);
                            }))
                            .on_mouse_down(MouseButton::Right, cx.listener(|workspace, _event: &MouseDownEvent, _window, cx| {
                                cx.stop_propagation();
                                workspace.close_tab_context_menu(cx);
                            }))
                            .child(
                                anchored()
                                    .position(position)
                                    .snap_to_window_with_margin(px(8.))
                                    .anchor(Corner::TopLeft)
                                    .child(
                                        div()
                                            .bg(theme.popover)
                                            .text_color(theme.popover_foreground)
                                            .border_1()
                                            .border_color(theme.border)
                                            .rounded(theme.radius)
                                            .py_1()
                                            .min_w(px(180.))
                                            .child(
                                                div()
                                                    .px_3()
                                                    .py_2()
                                                    .text_sm()
                                                    .cursor_pointer()
                                                    .hover(|s| s.bg(theme.accent))
                                                    .child("Reveal in Sidebar")
                                                    .on_mouse_down(MouseButton::Left, cx.listener({
                                                        let tab_path = tab_path.clone();
                                                        move |workspace, _event: &MouseDownEvent, _window, cx| {
                                                            cx.stop_propagation();
                                                            workspace.close_tab_context_menu(cx);
                                                            workspace.reveal_in_explorer(tab_path.clone(), cx);
                                                        }
                                                    }))
                                            )
                                            .child(
                                                div()
                                                    .px_3()
                                                    .py_2()
                                                    .text_sm()
                                                    .cursor_pointer()
                                                    .hover(|s| s.bg(theme.accent))
                                                    .child("Open in File Manager")
                                                    .on_mouse_down(MouseButton::Left, cx.listener({
                                                        let tab_path = tab_path.clone();
                                                        move |workspace, _event: &MouseDownEvent, _window, cx| {
                                                            cx.stop_propagation();
                                                            workspace.close_tab_context_menu(cx);
                                                            shell::open_in_explorer(&tab_path);
                                                        }
                                                    }))
                                            )
                                            .child(
                                                div()
                                                    .px_3()
                                                    .py_2()
                                                    .text_sm()
                                                    .cursor_pointer()
                                                    .hover(|s| s.bg(theme.accent))
                                                    .child("Copy File Path")
                                                    .on_mouse_down(MouseButton::Left, cx.listener({
                                                        let tab_path = tab_path.clone();
                                                        move |workspace, _event: &MouseDownEvent, _window, cx| {
                                                            cx.stop_propagation();
                                                            workspace.close_tab_context_menu(cx);
                                                            let path_str = normalize_unc_path(&tab_path.to_string_lossy());
                                                            cx.write_to_clipboard(ClipboardItem::new_string(path_str));
                                                        }
                                                    }))
                                            )
                                            .child(
                                                div()
                                                    .h(px(1.))
                                                    .mx_2()
                                                    .my_1()
                                                    .bg(theme.border)
                                            )
                                            .child(
                                                div()
                                                    .px_3()
                                                    .py_2()
                                                    .text_sm()
                                                    .cursor_pointer()
                                                    .hover(|s| s.bg(theme.accent))
                                                    .child("Close Tab")
                                                    .on_mouse_down(MouseButton::Left, cx.listener({
                                                        let tab_path = tab_path.clone();
                                                        move |workspace, _event: &MouseDownEvent, _window, cx| {
                                                            cx.stop_propagation();
                                                            workspace.close_tab_context_menu(cx);
                                                            let index = workspace
                                                                .tabs
                                                                .iter()
                                                                .position(|t| t.path == tab_path)
                                                                .unwrap_or(tab_index);
                                                            workspace.close_tab(index, cx);
                                                        }
                                                    }))
                                            )
                                            .child(
                                                div()
                                                    .px_3()
                                                    .py_2()
                                                    .text_sm()
                                                    .cursor_pointer()
                                                    .hover(|s| s.bg(theme.accent))
                                                    .child("Close Other Tabs")
                                                    .on_mouse_down(MouseButton::Left, cx.listener({
                                                        let tab_path = tab_path.clone();
                                                        move |workspace, _event: &MouseDownEvent, _window, cx| {
                                                            cx.stop_propagation();
                                                            workspace.close_tab_context_menu(cx);
                                                            let index = workspace
                                                                .tabs
                                                                .iter()
                                                                .position(|t| t.path == tab_path)
                                                                .unwrap_or(tab_index);
                                                            workspace.close_other_tabs(index, cx);
                                                        }
                                                    }))
                                            )
                                            .child(
                                                div()
                                                    .px_3()
                                                    .py_2()
                                                    .text_sm()
                                                    .cursor_pointer()
                                                    .hover(|s| s.bg(theme.accent))
                                                    .child("Close All Tabs")
                                                    .on_mouse_down(MouseButton::Left, cx.listener({
                                                        move |workspace, _event: &MouseDownEvent, _window, cx| {
                                                            cx.stop_propagation();
                                                            workspace.close_tab_context_menu(cx);
                                                            workspace.close_all_tabs(cx);
                                                        }
                                                    }))
                                            )
                                    ),
                            )
                    )
                    .priority(20),
                )
            })
            .child(
                // Footer
                div()
                    .flex()
                    .items_center()
                    .justify_between()
                    .h_8()
                    .px_4()
                    .bg(theme.tab_bar)
                    .border_t_1()
                    .border_color(theme.border)
                    .text_xs()
                    .child(if self.tabs.is_empty() { "No file" } else { "Ready" })
                    .when_some(fps_text, |this, fps| this.child(fps)),
            )
            // Render dialogs on top
            .children(dialog_layer)
    }
}

/// Bring the Peekdown window to foreground on Windows.
/// This must be called from the target process (the one that owns the window).
#[cfg(windows)]
fn bring_window_to_foreground() {
    unsafe {
        // Find window by enumerating all windows and checking title suffix
        let hwnd = find_peekdown_window();

        if hwnd.is_null() {
            tracing::warn!("Could not find Peekdown window");
            return;
        }

        tracing::info!("Found Peekdown window: {:?}", hwnd);

        // Restore if minimized
        if IsIconic(hwnd) != 0 {
            tracing::info!("Window is minimized, restoring");
            ShowWindow(hwnd, SW_RESTORE);
        }

        // Get thread IDs
        let foreground_hwnd = GetForegroundWindow();
        let foreground_thread = GetWindowThreadProcessId(foreground_hwnd, std::ptr::null_mut());
        let current_thread = GetCurrentThreadId();

        tracing::info!("Foreground thread: {}, Current thread: {}", foreground_thread, current_thread);

        // Attach to foreground thread to gain foreground permission
        if foreground_thread != current_thread {
            AttachThreadInput(current_thread, foreground_thread, 1);
        }

        // Simulate Alt key press to bypass Windows foreground restrictions
        keybd_event(VK_MENU as u8, 0, KEYEVENTF_EXTENDEDKEY, 0);
        keybd_event(VK_MENU as u8, 0, KEYEVENTF_EXTENDEDKEY | KEYEVENTF_KEYUP, 0);

        // Now set foreground window
        let result = SetForegroundWindow(hwnd);
        tracing::info!("SetForegroundWindow result: {}", result);

        // Detach from foreground thread
        if foreground_thread != current_thread {
            AttachThreadInput(current_thread, foreground_thread, 0);
        }
    }
}

/// Find the Peekdown window by enumerating windows and checking title.
/// Window title can be "Peekdown" or "{filename} - Peekdown".
#[cfg(windows)]
fn find_peekdown_window() -> HWND {
    use std::sync::atomic::{AtomicIsize, Ordering};

    static FOUND_HWND: AtomicIsize = AtomicIsize::new(0);
    FOUND_HWND.store(0, Ordering::SeqCst);

    unsafe extern "system" fn enum_callback(hwnd: HWND, _: isize) -> i32 {
        unsafe {
            let len = GetWindowTextLengthW(hwnd);
            if len == 0 {
                return 1; // Continue enumeration
            }

            let mut buffer: Vec<u16> = vec![0; (len + 1) as usize];
            GetWindowTextW(hwnd, buffer.as_mut_ptr(), len + 1);

            // Convert to string and check if it ends with "Peekdown" or equals "Peekdown"
            let title = String::from_utf16_lossy(&buffer[..len as usize]);
            if title == "Peekdown" || title.ends_with(" - Peekdown") {
                FOUND_HWND.store(hwnd as isize, Ordering::SeqCst);
                return 0; // Stop enumeration
            }

            1 // Continue enumeration
        }
    }

    unsafe {
        EnumWindows(Some(enum_callback), 0);
        FOUND_HWND.load(Ordering::SeqCst) as HWND
    }
}
