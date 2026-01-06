//! UI Views and layout.

use gpui::*;
use std::path::PathBuf;
use crate::state::document::Document;
use gpui_component::ActiveTheme;
use gpui_component::{Icon, IconName, Sizable};
use crate::state::config::{AppConfig, AppThemeMode};
use gpui_component::menu::{PopupMenu, PopupMenuItem};
use gpui_component::popover::Popover;
use gpui_component::button::{Button, ButtonVariants};

mod welcome;
use welcome::WelcomeView;
mod markdown_view;
use markdown_view::MarkdownView;
use smol::channel::Receiver;
use crate::ipc::IpcMessage;

pub fn init(cx: &mut App, initial_files: Vec<PathBuf>, ipc_rx: Option<Receiver<IpcMessage>>, config: Entity<AppConfig>) {
    cx.open_window(
        WindowOptions {
            titlebar: Some(TitlebarOptions {
                title: Some("Peekdown".into()),
                ..Default::default()
            }),
            window_bounds: Some(WindowBounds::Windowed(Bounds::centered(
                None,
                size(1024.0.into(), 768.0.into()),
                cx,
            ))),
            ..Default::default()
        },
        move |_, cx| {
            // Apply theme (F6)
            let theme = config.read(cx).appearance.theme;
            theme.apply(None, cx);

            cx.new(|cx| {
                let mut view = WorkspaceView::new(cx, config.clone());
                for path in initial_files.clone() {
                    view.open_file(path, cx);
                }

                if let Some(rx) = ipc_rx {
                    cx.spawn(|workspace: WeakEntity<WorkspaceView>, cx: &mut AsyncApp| {
                        let mut cx = cx.clone();
                        async move {
                        while let Ok(msg) = rx.recv().await {
                            let mut cx_clone = cx.clone();
                            workspace.update(&mut cx_clone, |workspace, cx| {
                                match msg {
                                    IpcMessage::OpenFiles(paths) => workspace.open_files(paths, cx),
                                    IpcMessage::FocusWindow => {}, // Just activate below
                                }
                                cx.activate(true);
                            }).ok();
                        }
                        }
                    }).detach();
                }

                view
            })
        },
    )
    .unwrap();
}

struct WorkspaceTab {
    path: PathBuf,
    view: AnyView,
    title: String,
}

struct WorkspaceView {
    tabs: Vec<WorkspaceTab>,
    active_tab_index: usize,
    config: Entity<AppConfig>,
}

impl WorkspaceView {
    pub fn new(cx: &mut Context<Self>, config: Entity<AppConfig>) -> Self {
        cx.observe(&config, |_, _, cx| {
            cx.notify();
        }).detach();

        Self {
            tabs: Vec::new(),
            active_tab_index: 0,
            config,
        }
    }

    pub fn open_file(&mut self, path: PathBuf, cx: &mut Context<Self>) {
        self.open_files(vec![path], cx);
    }

    pub fn open_files(&mut self, paths: Vec<PathBuf>, cx: &mut Context<Self>) {
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
                                 continue;
                             }

                             let title = path.file_name()
                                .map(|s| s.to_string_lossy().to_string())
                                .unwrap_or_else(|| "Untitled".to_string());

                             let doc = cx.new(|_cx| Document::new(content, path.clone()));
                             let view = cx.new(|_cx| MarkdownView::new(doc));

                             workspace.tabs.push(WorkspaceTab {
                                 path,
                                 view: view.into(),
                                 title,
                             });
                             workspace.active_tab_index = workspace.tabs.len() - 1;
                         }
                         cx.notify();
                     }).ok();
                }
            }
        }).detach();
    }

    fn update_theme(&mut self, mode: AppThemeMode, window: &mut Window, cx: &mut Context<Self>) {
        self.config.update(cx, |config, _| {
            config.appearance.theme = mode;
            config.save();
        });
        mode.apply(Some(window), cx);
    }

    fn close_tab(&mut self, index: usize, cx: &mut Context<Self>) {
        if index >= self.tabs.len() {
            return;
        }
        self.tabs.remove(index);

        if self.tabs.is_empty() {
            self.active_tab_index = 0;
        } else {
            if self.active_tab_index >= index && self.active_tab_index > 0 {
                self.active_tab_index -= 1;
            }
            if self.active_tab_index >= self.tabs.len() {
                self.active_tab_index = self.tabs.len().saturating_sub(1);
            }
        }
        cx.notify();
    }

    fn activate_tab(&mut self, index: usize, cx: &mut Context<Self>) {
        if index < self.tabs.len() {
            self.active_tab_index = index;
            cx.notify();
        }
    }
}

impl Render for WorkspaceView {
    fn render(&mut self, _window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let theme = cx.theme().clone();

        let body_content = if self.tabs.is_empty() {
            div().size_full().child(cx.new(|_cx| WelcomeView::new()))
        } else {
            let tab = &self.tabs[self.active_tab_index];
            div().size_full().child(tab.view.clone())
        };

        div()
            .flex()
            .flex_col()
            .size_full()
            .bg(theme.background)
            .text_color(theme.foreground)
            .on_drop(cx.listener(|workspace, event: &ExternalPaths, _, cx| {
                workspace.open_files(event.paths().to_vec(), cx);
            }))
            .child(
                // Header
                div()
                    .flex()
                    .items_center()
                    .h_10()
                    .bg(theme.title_bar)
                    .border_b_1()
                    .border_color(theme.border)
                    .child(
                        div()
                            .id("tab-bar-container")
                            .flex()
                            .flex_row()
                            .flex_grow()
                            .overflow_x_scroll()
                            .children(self.tabs.iter().enumerate().map(|(ix, tab)| {
                                let is_active = ix == self.active_tab_index;

                                div()
                                    .id(("tab", ix))
                                    .flex()
                                    .items_center()
                                    .h_full()
                                    .px_4()
                                    .gap_2()
                                    .border_r_1()
                                    .border_color(theme.border)
                                    .cursor_pointer()
                                    .bg(if is_active {
                                        theme.background
                                    } else {
                                        gpui::transparent_black()
                                    })
                                    .text_color(if is_active {
                                        theme.foreground
                                    } else {
                                        theme.muted_foreground
                                    })
                                    .hover(|s| {
                                        if !is_active {
                                            s.bg(theme.secondary)
                                        } else {
                                            s
                                        }
                                    })
                                    .on_click(cx.listener(move |workspace, _, _window, cx| {
                                        workspace.activate_tab(ix, cx);
                                    }))
                                    .child(
                                        div()
                                            .max_w(px(150.))
                                            .overflow_hidden()
                                            .whitespace_nowrap()
                                            .child(tab.title.clone())
                                    )
                                    .child(
                                        div()
                                            .id(("close_tab", ix))
                                            .flex()
                                            .items_center()
                                            .justify_center()
                                            .w_5()
                                            .h_5()
                                            .rounded_md()
                                            .hover(|style| {
                                                style
                                                    .bg(theme.danger)
                                                    .text_color(theme.danger_foreground)
                                            })
                                            .child(IconName::Close)
                                            .on_click(cx.listener(move |workspace, _, _window, cx| {
                                                cx.stop_propagation();
                                                workspace.close_tab(ix, cx);
                                            }))
                                    )
                            }))
                    )
                    .child(
                        div()
                            .px_2()
                            .child(
                                {
                                    let config = self.config.clone();
                                    let workspace_handle = cx.entity().downgrade();
                                    Popover::new("settings-popover")
                                        .trigger(
                                            Button::new("settings-btn")
                                                .icon(Icon::new(IconName::Settings))
                                                .ghost()
                                                .small()
                                        )
                                        .content(move |_, window, cx| {
                                            let config = config.clone();
                                            let current_theme = config.read(cx).appearance.theme;
                                            let workspace_handle = workspace_handle.clone();

                                            PopupMenu::build(window, cx, move |menu, _, _cx| {
                                                let handle1 = workspace_handle.clone();
                                                let handle2 = workspace_handle.clone();
                                                let handle3 = workspace_handle.clone();

                                                menu
                                                    .item(
                                                        PopupMenuItem::new("Light")
                                                            .checked(current_theme == AppThemeMode::Light)
                                                            .on_click(move |_, window, cx| {
                                                                handle1.update(cx, |workspace, cx| {
                                                                    workspace.update_theme(AppThemeMode::Light, window, cx);
                                                                }).ok();
                                                            })
                                                    )
                                                    .item(
                                                        PopupMenuItem::new("Dark")
                                                            .checked(current_theme == AppThemeMode::Dark)
                                                            .on_click(move |_, window, cx| {
                                                                handle2.update(cx, |workspace, cx| {
                                                                    workspace.update_theme(AppThemeMode::Dark, window, cx);
                                                                }).ok();
                                                            })
                                                    )
                                                    .item(
                                                        PopupMenuItem::new("Auto")
                                                            .checked(current_theme == AppThemeMode::Auto)
                                                            .on_click(move |_, window, cx| {
                                                                handle3.update(cx, |workspace, cx| {
                                                                    workspace.update_theme(AppThemeMode::Auto, window, cx);
                                                                }).ok();
                                                            })
                                                    )
                                            })
                                        })
                                }
                            )
                    ),
            )
            .child(
                // Body
                div()
                    .flex()
                    .flex_grow()
                    .overflow_hidden()
                    .child(body_content),
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
    }
}
