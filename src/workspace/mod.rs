//! UI Views and layout.

use gpui::*;
use gpui::prelude::FluentBuilder;
use std::path::PathBuf;
use std::time::Instant;
use crate::state::document::Document;
use gpui_component::{ActiveTheme, Root, button::Button, button::ButtonVariants, Icon, IconName, Sizable};
use crate::state::config::{AppConfig, ExplorerRootMode};

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
use serde::Deserialize;

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
                            let mut cx_clone = cx.clone();
                            workspace_weak.update(&mut cx_clone, |workspace, cx| {
                                match msg {
                                    IpcMessage::OpenFiles(paths) => workspace.open_files(paths, cx),
                                    IpcMessage::FocusWindow => {},
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
    outline_width: f32,
    explorer_visible: bool,
    explorer_view: Option<Entity<FileExplorerView>>,
    explorer_width: f32,
    focus_handle: FocusHandle,
    tab_scroll_handle: ScrollHandle,
    // FPS counter
    fps_counter: FpsCounter,
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
        let (outline_visible, outline_width, explorer_visible, explorer_width) = {
            let cfg = config.read(cx);
            (
                cfg.appearance.outline_visible,
                cfg.appearance.outline_width,
                cfg.appearance.explorer_visible,
                cfg.appearance.explorer_width,
            )
        };

        cx.observe(&config, |_, _, cx| {
            cx.notify();
        }).detach();

        Self {
            tabs: Vec::new(),
            active_tab_index: 0,
            config,
            outline_visible,
            outline_view: None,
            search_bar: None,
            outline_width,
            explorer_visible,
            explorer_view: None,
            explorer_width,
            focus_handle: cx.focus_handle(),
            tab_scroll_handle: ScrollHandle::new(),
            fps_counter: FpsCounter::new(),
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

    pub fn open_file(&mut self, path: PathBuf, cx: &mut Context<Self>) {
        self.open_files(vec![path], cx);
    }

    pub fn open_files(&mut self, paths: Vec<PathBuf>, cx: &mut Context<Self>) {
        let config = self.config.clone();
        cx.spawn(|workspace: WeakEntity<WorkspaceView>, cx: &mut AsyncApp| {
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
                             let view = cx.new(|cx| MarkdownView::new(doc, config.clone(), cx));

                             // Observe the MarkdownView's text_view_state for changes
                             let text_view_state = view.read(cx).text_view_state().clone();
                             cx.observe(&text_view_state, |workspace, _, cx| {
                                 // Update outline when text parsing completes
                                 workspace.update_outline(cx);
                             }).detach();

                             workspace.tabs.push(WorkspaceTab {
                                 path,
                                 view,
                                 title,
                             });
                             workspace.active_tab_index = workspace.tabs.len() - 1;
                             workspace.tab_scroll_handle.scroll_to_item(workspace.active_tab_index);
                         }
                         workspace.update_outline(cx);
                         workspace.update_explorer(cx);
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
        self.tabs.remove(index);

        if self.tabs.is_empty() {
            self.active_tab_index = 0;
            self.outline_view = None;
            self.explorer_view = None;
        } else {
            if self.active_tab_index >= index && self.active_tab_index > 0 {
                self.active_tab_index -= 1;
            }
            if self.active_tab_index >= self.tabs.len() {
                self.active_tab_index = self.tabs.len().saturating_sub(1);
            }
            self.update_outline(cx);
            self.update_explorer(cx);
        }
        if self.search_bar.is_some() {
            // Clear search bar without restoring focus (tab is being closed)
            self.search_bar = None;
            self.clear_search_highlight_for_tab(self.active_tab_index, cx);
        }
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

    fn update_explorer(&mut self, cx: &mut Context<Self>) {
        if self.tabs.is_empty() {
            self.explorer_view = None;
            return;
        }

        // Get current file's parent directory
        let tab = &self.tabs[self.active_tab_index];
        let current_dir = tab.path.parent().map(|p| p.to_path_buf());
        let Some(current_dir) = current_dir else { return };

        // Resolve explorer root based on config
        let (root_mode, markers) = {
            let cfg = self.config.read(cx);
            (
                cfg.appearance.explorer_root_mode,
                cfg.appearance.project_root_markers.clone(),
            )
        };

        let root = match root_mode {
            ExplorerRootMode::CurrentDir => current_dir.clone(),
            ExplorerRootMode::ProjectRoot => {
                Self::find_project_root(&current_dir, &markers)
                    .unwrap_or_else(|| current_dir.clone())
            }
        };

        // Read expanded state from config
        let expanded_dirs: std::collections::HashSet<std::path::PathBuf> = {
            let cfg = self.config.read(cx);
            cfg.appearance.expanded_dirs.iter()
                .map(|s| root.join(s))
                .collect()
        };

        let explorer_width = self.explorer_width;
        let config = self.config.clone();
        let workspace = cx.entity().downgrade();
        let workspace_for_width = workspace.clone();
        let workspace_for_click = workspace.clone();
        let workspace_for_close = workspace.clone();
        let workspace_for_root_mode = workspace.clone();
        let root_for_expanded = root.clone();
        let config_for_markers = config.clone();

        if let Some(explorer_view) = &self.explorer_view {
            explorer_view.update(cx, |view, cx| {
                // Only refresh when root actually changes
                if view.root_path() != Some(&root) {
                    view.set_root(Some(root.clone()), cx);
                }
                // Only refresh when expanded_dirs actually changes
                if view.expanded_dirs() != &expanded_dirs {
                    view.set_expanded_dirs(expanded_dirs, cx);
                }
                view.set_width(explorer_width, cx);
                view.set_root_mode(root_mode, cx);
            });
            return;
        }

        // Create new explorer view
        let config_for_width = config.clone();
        let config_for_expanded = config.clone();

        self.explorer_view = Some(cx.new(|_| {
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
                            ws.open_file(path, cx);
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
                .on_edit_markers(move |window, cx| {
                    settings_dialog::open_settings_dialog(config_for_markers.clone(), window, cx);
                })
        }));

        // Set root and expanded state
        if let Some(explorer_view) = &self.explorer_view {
            explorer_view.update(cx, |view, cx| {
                view.set_root(Some(root), cx);
                view.set_expanded_dirs(expanded_dirs, cx);
                view.set_width(explorer_width, cx);
                view.set_root_mode(root_mode, cx);
            });
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
            let title = format!("{} - Peekdown", tab.path.display());
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
        let is_explorer_resizing = if let Some(explorer_view) = &self.explorer_view {
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
            self.explorer_view.clone()
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

        // Clone views for event handlers
        let explorer_for_move = self.explorer_view.clone();
        let explorer_for_up = self.explorer_view.clone();
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
