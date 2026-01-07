use gpui::*;
use super::{WorkspaceView, settings_dialog};
use gpui_component::{Icon, IconName, button::Button, ActiveTheme, button::ButtonVariants, Sizable};

pub fn render_header(workspace: &mut WorkspaceView, cx: &mut Context<WorkspaceView>) -> impl IntoElement {
    let theme = cx.theme().clone();
    let config = workspace.config.clone();

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
                .children(workspace.tabs.iter().enumerate().map(|(ix, tab)| {
                    let is_active = ix == workspace.active_tab_index;

                    div()
                        .id(("tab", ix))
                        .flex()
                        .items_center()
                        .h_full()
                        .px_3()
                        .gap_2()
                        .text_sm()
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
                        .border_t_2()
                        .border_color(if is_active {
                            theme.foreground
                        } else {
                            gpui::transparent_black()
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
                                .font_weight(if is_active { FontWeight::MEDIUM } else { FontWeight::NORMAL })
                                .child(tab.title.clone())
                        )
                        .child(
                            div()
                                .id(("close_tab", ix))
                                .flex()
                                .items_center()
                                .justify_center()
                                .w_4()
                                .h_4()
                                .rounded_sm()
                                .text_color(theme.muted_foreground)
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
                    Button::new("settings-btn")
                        .icon(Icon::new(IconName::Settings))
                        .ghost()
                        .small()
                        .on_click(move |_, window, cx| {
                            settings_dialog::open_settings_dialog(config.clone(), window, cx);
                        })
                )
        )
}
