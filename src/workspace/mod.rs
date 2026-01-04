//! UI Views and layout.

use gpui::*;
use std::path::PathBuf;
use crate::state::document::Document;
use gpui_component::ActiveTheme;
use gpui_component::IconName;

mod welcome;
use welcome::WelcomeView;
mod markdown_view;
use markdown_view::MarkdownView;

pub fn init(cx: &mut App, initial_files: Vec<PathBuf>) {
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
            cx.new(|cx| {
                let mut view = WorkspaceView::new(cx);
                for path in initial_files.clone() {
                    view.open_file(path, cx);
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
}

impl WorkspaceView {
    pub fn new(_cx: &mut Context<Self>) -> Self {
        Self {
            tabs: Vec::new(),
            active_tab_index: 0,
        }
    }

    pub fn open_file(&mut self, path: PathBuf, cx: &mut Context<Self>) {
        if let Some(index) = self.tabs.iter().position(|t| t.path == path) {
            self.active_tab_index = index;
            cx.notify();
            return;
        }

        cx.spawn(|workspace: WeakEntity<WorkspaceView>, cx: &mut AsyncApp| {
            let mut cx = cx.clone();
            async move {
                let content = smol::fs::read_to_string(&path).await;
                if let Ok(content) = content {
                     workspace.update(&mut cx, |workspace, cx| {
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
