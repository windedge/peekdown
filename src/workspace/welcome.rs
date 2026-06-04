use gpui::*;
use gpui::prelude::FluentBuilder;
use gpui_component::button::{Button, ButtonVariants};
use gpui_component::{ActiveTheme, StyledExt};
use std::path::PathBuf;

use crate::state::config::AppConfig;
use super::WorkspaceView;

/// Render the welcome view when no documents are open
pub fn render_welcome(config: &Entity<AppConfig>, cx: &mut Context<WorkspaceView>) -> impl IntoElement {
    let theme = cx.theme();

    // Read recent files from config
    let recent_files = config.read(cx).recent_files().to_vec();

    div()
        .flex()
        .flex_col()
        .items_center()
        .justify_center()
        .size_full()
        .overflow_hidden()
        .bg(theme.background)
        .text_color(theme.foreground)
        .gap_6()
        .child(
            div()
                .text_xl()
                .font_bold()
                .child("Welcome to Peekdown"),
        )
        .child(
            div()
                .text_sm()
                .text_color(theme.muted_foreground)
                .child("Open a Markdown file to get started"),
        )
        .child(
            Button::new("open-file-btn")
                .label("Open File...")
                .primary()
                .outline()
                .on_click(cx.listener(|workspace, _, _window, cx| {
                    workspace.open_file_dialog(cx);
                })),
        )
        .child(
            div()
                .text_xs()
                .text_color(theme.muted_foreground)
                .mt_2()
                .child(if cfg!(target_os = "macos") {
                    "Or drag and drop files • Cmd+O to open"
                } else {
                    "Or drag and drop files • Ctrl+O to open"
                }),
        )
        .child(render_recent_files_section(recent_files, theme.clone(), cx))
}

/// Render the "Recent Files" section below the welcome greeting
fn render_recent_files_section(
    recent_files: Vec<PathBuf>,
    theme: gpui_component::theme::Theme,
    cx: &mut Context<WorkspaceView>,
) -> impl IntoElement {
    div()
        .flex()
        .flex_col()
        .items_center()
        .w_full()
        .max_w(px(440.))
        .mt_4()
        .child(
            div()
                .flex()
                .items_center()
                .justify_between()
                .w_full()
                .mb_2()
                .child(
                    div()
                        .text_xs()
                        .font_bold()
                        .text_color(theme.muted_foreground)
                        .child("Recent Files"),
                )
                .when(!recent_files.is_empty(), |this| {
                    this.child(
                        div()
                            .text_xs()
                            .text_color(theme.muted_foreground)
                            .hover(|s| s.text_color(theme.foreground))
                            .cursor_pointer()
                            .child("Clear")
                            .on_mouse_down(MouseButton::Left, cx.listener(|workspace, _event: &MouseDownEvent, _window, cx| {
                                workspace.config.update(cx, |cfg, _| {
                                    cfg.clear_recent_files();
                                    cfg.save();
                                });
                                cx.notify();
                            })),
                    )
                }),
        )
        .child(
            if recent_files.is_empty() {
                div()
                    .text_xs()
                    .text_color(theme.muted_foreground)
                    .py_2()
                    .child("No recent files")
            } else {
                let mut items: Vec<gpui::Div> = Vec::new();
                for path in &recent_files {
                    items.push(render_recent_file_item(path.clone(), &theme, cx));
                }
                div().flex().flex_col().w_full().gap_1().children(items)
            },
        )
}

/// Render a single recent file item
fn render_recent_file_item(
    path: PathBuf,
    theme: &gpui_component::theme::Theme,
    cx: &mut Context<WorkspaceView>,
) -> gpui::Div {
    let file_name = path
        .file_name()
        .map(|s| s.to_string_lossy().to_string())
        .unwrap_or_else(|| "Unknown".to_string());
    let parent_path = path
        .parent()
        .map(|p| p.to_string_lossy().to_string())
        .unwrap_or_default();

    div()
        .flex()
        .flex_col()
        .w_full()
        .px_4()
        .py_2()
        .rounded(theme.radius)
        .hover(|s| s.bg(theme.accent))
        .cursor_pointer()
        .on_mouse_down(MouseButton::Left, cx.listener(move |workspace, _event: &MouseDownEvent, _window, cx| {
            workspace.open_file(path.clone(), cx);
        }))
        .child(
            div()
                .text_sm()
                .font_bold()
                .child(file_name),
        )
        .child(
            div()
                .text_xs()
                .text_color(theme.muted_foreground)
                .overflow_hidden()
                .text_ellipsis()
                .child(parent_path),
        )
}
