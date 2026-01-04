//! UI Views and layout.

use gpui::*;
use std::path::PathBuf;
use crate::state::theme::Theme;
use crate::state::document::Document;

mod welcome;
use welcome::WelcomeView;
mod markdown_view;
use markdown_view::MarkdownView;

pub fn init(cx: &mut App) {
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
        |_, cx| {
            cx.new(|cx| WorkspaceView::new(cx))
        },
    )
    .unwrap();
}

struct WorkspaceView {
    theme: Theme,
    active_view: Option<AnyView>,
}

impl WorkspaceView {
    pub fn new(cx: &mut Context<Self>) -> Self {
        let view = Self {
            theme: Theme::dark(),
            active_view: None,
        };
        
        // Auto-load README.md for verification
        let path = PathBuf::from("README.md");
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

        view
    }
}

impl Render for WorkspaceView {
    fn render(&mut self, _window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let theme = &self.theme;
        
        let body_content = if let Some(view) = &self.active_view {
            div().child(view.clone())
        } else {
            div().child(cx.new(|_cx| WelcomeView::new()))
        };

        div()
            .flex()
            .flex_col()
            .size_full()
            .bg(theme.bg_base)
            .text_color(theme.text_primary)
            .child(
                // Header
                div()
                    .flex()
                    .items_center()
                    .h_10()
                    .px_4()
                    .bg(theme.bg_header)
                    .border_b_1()
                    .border_color(theme.border)
                    .child("Peekdown Header"),
            )
            .child(
                // Body
                div()
                    .flex()
                    .flex_grow()
                    .child(body_content),
            )
            .child(
                // Footer
                div()
                    .flex()
                    .items_center()
                    .h_8()
                    .px_4()
                    .bg(theme.bg_footer)
                    .border_t_1()
                    .border_color(theme.border)
                    .text_xs()
                    .child("Ready"),
            )
    }
}