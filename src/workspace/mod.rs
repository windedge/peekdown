//! UI Views and layout.

use gpui::*;
use gpui::prelude::FluentBuilder;
use std::path::PathBuf;
use crate::state::document::Document;
use gpui_component::{ActiveTheme, Root, button::Button, button::ButtonVariants, Icon, IconName, Sizable};
use crate::state::config::AppConfig;

mod welcome;
use welcome::WelcomeView;
mod markdown_view;
use markdown_view::MarkdownView;
mod settings_dialog;
mod header;
mod outline;
use outline::OutlineView;
mod search_bar;
use search_bar::{SearchBar, SearchState};
use smol::channel::Receiver;
use crate::ipc::IpcMessage;

gpui::actions!([OpenSearch, CloseSearch]);

pub fn init(cx: &mut App, initial_files: Vec<PathBuf>, ipc_rx: Option<Receiver<IpcMessage>>, config: Entity<AppConfig>) {
    cx.bind_keys(vec![
        #[cfg(target_os = "macos")]
        KeyBinding::new("cmd-f", OpenSearch, Some("Workspace")),
        #[cfg(not(target_os = "macos"))]
        KeyBinding::new("ctrl-f", OpenSearch, Some("Workspace")),
        #[cfg(target_os = "macos")]
        KeyBinding::new("cmd-f", OpenSearch, Some("SearchBar")),
        #[cfg(not(target_os = "macos"))]
        KeyBinding::new("ctrl-f", OpenSearch, Some("SearchBar")),
        KeyBinding::new("escape", CloseSearch, Some("Workspace && search == open")),
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

                // Save outline visibility and width on app quit
                cx.on_app_quit({
                    let config = config_for_quit.clone();
                    let workspace = workspace_for_quit.clone();
                    move |_root, cx| {
                        if let Some(ws) = workspace.upgrade() {
                            let ws = ws.read(cx);
                            let outline_visible = ws.outline_visible;
                            let outline_width = ws.outline_width;
                            config.update(cx, |config, _| {
                                config.appearance.outline_visible = outline_visible;
                                config.appearance.outline_width = outline_width;
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
    focus_handle: FocusHandle,
    tab_scroll_handle: ScrollHandle,
}

impl WorkspaceView {
    pub fn new(cx: &mut Context<Self>, config: Entity<AppConfig>) -> Self {
        let (outline_visible, outline_width) = {
            let cfg = config.read(cx);
            (cfg.appearance.outline_visible, cfg.appearance.outline_width)
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
            focus_handle: cx.focus_handle(),
            tab_scroll_handle: ScrollHandle::new(),
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
        } else {
            if self.active_tab_index >= index && self.active_tab_index > 0 {
                self.active_tab_index -= 1;
            }
            if self.active_tab_index >= self.tabs.len() {
                self.active_tab_index = self.tabs.len().saturating_sub(1);
            }
            self.update_outline(cx);
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
            cx.notify();
        }
    }

    fn toggle_outline(&mut self, cx: &mut Context<Self>) {
        self.outline_visible = !self.outline_visible;
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
                    markdown_view.read(cx).scroll_to_heading(block_index, cx);
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
                    markdown_view.read(cx).scroll_to_heading(block_index, cx);
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
                    markdown_view.read(cx).scroll_to_heading(block_index, cx);
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
            tab.view.read(cx).scroll_to_heading(block_index, cx);
        }
    }
}

impl Render for WorkspaceView {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let theme = cx.theme().clone();

        // Check if outline is resizing (read from outline_view if exists)
        let is_resizing = if let Some(outline_view) = &self.outline_view {
            outline_view.read(cx).is_resizing()
        } else {
            false
        };

        // Use cached outline sidebar if visible
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
                .child(cx.new(|_cx| WelcomeView::new()))
                // No outline toggle button when no documents are open
        } else {
            let tab = &self.tabs[self.active_tab_index];
            div()
                .relative()
                .size_full()
                .flex_grow()
                .child(tab.view.clone())
                .when(!self.outline_visible, |this| {
                    this.child(
                        deferred(
                            div()
                                .absolute()
                                .top_2()
                                .left_2()
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
        };

        // Get dialog layer to render on top
        let dialog_layer = Root::render_dialog_layer(window, cx);

        // Search bar overlay
        let search_bar = self.search_bar.clone();

        // Clone outline view for event handlers
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
            .on_action(cx.listener(|workspace, _: &OpenSearch, window, cx| {
                workspace.open_search(window, cx);
            }))
            .on_action(cx.listener(|workspace, _: &CloseSearch, window, cx| {
                if workspace.search_bar.is_some() {
                    workspace.close_search(window, cx);
                }
            }))
            .on_drop(cx.listener(|workspace, event: &ExternalPaths, _, cx| {
                workspace.open_files(event.paths().to_vec(), cx);
            }))
            // Handle global mouse move when resizing
            .when(is_resizing, |this| {
                this.on_mouse_move(move |event: &MouseMoveEvent, _, cx| {
                    if let Some(outline) = &outline_for_move {
                        outline.update(cx, |view, cx| {
                            view.handle_resize_move(f32::from(event.position.x), cx);
                        });
                    }
                })
                .on_mouse_up(MouseButton::Left, move |_, _, cx| {
                    if let Some(outline) = &outline_for_up {
                        outline.update(cx, |view, cx| {
                            view.end_resize(cx);
                        });
                    }
                })
            })
            .child(
                // Header
                header::render_header(self, cx)
            )
            .child(
                // Body with optional outline sidebar and overlays
                div()
                    .relative()
                    .flex()
                    .flex_row()
                    .flex_grow()
                    .overflow_hidden()
                    .children(outline_sidebar)
                    .child(body_content)
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
            .child(
                // Footer
                div()
                    .flex()
                    .items_center()
                    .h_8()
                    .px_4()
                    .bg(theme.tab_bar)
                    .border_t_1()
                    .border_color(theme.border)
                    .text_xs()
                    .child(if self.tabs.is_empty() { "No file" } else { "Ready" }),
            )
            // Render dialogs on top
            .children(dialog_layer)
    }
}
