//! UI Views and layout.

use gpui::*;
use std::path::PathBuf;
use crate::state::document::Document;
use gpui_component::ActiveTheme;

mod welcome;
use welcome::WelcomeView;
mod markdown_view;
use markdown_view::MarkdownView;

pub fn init(cx: &mut App, initial_file: Option<PathBuf>) {
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
                if let Some(path) = initial_file.clone() {
                    view.open_file(path, cx);
                }
                view
            })
        },
    )
    .unwrap();
}

struct WorkspaceView {
    active_view: Option<AnyView>,
}

impl WorkspaceView {
    pub fn new(_cx: &mut Context<Self>) -> Self {
        Self {
            active_view: None,
        }
    }

    pub fn open_file(&mut self, path: PathBuf, cx: &mut Context<Self>) {
        cx.spawn(|workspace: WeakEntity<WorkspaceView>, cx: &mut AsyncApp| {
            let mut cx = cx.clone();
            async move {
                let content = smol::fs::read_to_string(&path).await;
                if let Ok(content) = content {
                     workspace.update(&mut cx, |workspace, cx| {
                         let doc = cx.new(|_cx| Document::new(content, path));
                         let view = cx.new(|_cx| MarkdownView::new(doc));
                         workspace.active_view = Some(view.into());
                         cx.notify();
                     }).ok();
                }
            }
        }).detach();
    }
}

impl Render for WorkspaceView {
    fn render(&mut self, _window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let theme = cx.theme().clone();
        
        let body_content = if let Some(view) = &self.active_view {
            div().size_full().child(view.clone())
        } else {
            div().size_full().child(cx.new(|_cx| WelcomeView::new()))
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
                    .px_4()
                    .bg(theme.title_bar)
                    .border_b_1()
                    .border_color(theme.border)
                    .child("Peekdown Header"),
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
                    .child("Ready"),
            )
    }
}
