//! UI Views and layout.

use gpui::*;
use std::path::PathBuf;
use crate::state::document::Document;
use gpui_component::ActiveTheme;
use gpui_component::tab::{Tab, TabBar};
use gpui_component::button::{Button, ButtonVariants};
use gpui_component::{IconName, Sizable};

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
        // Check if already open
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
                        TabBar::new("tab_bar")
                            .children(self.tabs.iter().enumerate().map(|(ix, tab)| {
                                Tab::new()
                                    .label(tab.title.clone())
                                    .suffix(
                                        Button::new(("close_tab", ix))
                                            .icon(IconName::Close)
                                            .ghost()
                                            .xsmall()
                                            .on_click(cx.listener(move |workspace, _, _window, cx| {
                                                cx.stop_propagation();
                                                workspace.close_tab(ix, cx);
                                            }))
                                    )
                            }))
                            .selected_index(self.active_tab_index)
                            .on_click(cx.listener(|workspace, index, _window, cx| {
                                workspace.activate_tab(*index, cx);
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
