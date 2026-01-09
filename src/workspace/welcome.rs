use gpui::*;
use gpui_component::button::{Button, ButtonVariants};
use gpui_component::{ActiveTheme, StyledExt};

use super::WorkspaceView;

/// Render the welcome view when no documents are open
pub fn render_welcome(cx: &mut Context<WorkspaceView>) -> impl IntoElement {
    let theme = cx.theme();

    div()
        .flex()
        .flex_col()
        .items_center()
        .justify_center()
        .size_full()
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
}
