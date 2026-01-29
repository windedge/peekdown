use gpui::*;
use gpui::prelude::FluentBuilder;
use super::{WorkspaceView, settings_dialog};
use gpui_component::{Icon, IconName, button::Button, ActiveTheme, button::ButtonVariants, Sizable, tooltip::Tooltip, menu::{ContextMenuExt, PopupMenuItem}};
use crate::services::shell;

pub fn render_header(workspace: &mut WorkspaceView, cx: &mut Context<WorkspaceView>) -> impl IntoElement {
    let theme = cx.theme().clone();
    let config = workspace.config.clone();
    let has_tabs = !workspace.tabs.is_empty();
    let tab_scroll_handle = workspace.tab_scroll_handle.clone();
    let workspace_entity = cx.entity().downgrade();  // Get weak reference to workspace

    // Tab bar design:
    // - No bottom border line on header
    // - Active tab has same background as content area (seamless)
    // - Inactive tabs have distinct background for separation
    // - Fixed height prevents layout shifts when outline toggles
    div()
        .flex()
        .items_end()
        .h_10()
        .flex_shrink_0()
        .bg(theme.title_bar)
        .child(
            div()
                .id("tab-bar-container")
                .flex()
                .flex_row()
                .flex_grow()
                .items_end()
                .overflow_x_scroll()
                .track_scroll(&tab_scroll_handle)
                .pl_2()
                .children(workspace.tabs.iter().enumerate().map(|(ix, tab)| {
                    let is_active = ix == workspace.active_tab_index;
                    let is_last = ix == workspace.tabs.len() - 1;
                    let tab_path_str = super::normalize_unc_path(&tab.path.display().to_string());
                    let tab_path_tooltip = tab_path_str.clone();
                    let tab_path_buf = tab.path.clone();

                    div()
                        .id(("tab", ix))
                        .flex()
                        .items_center()
                        .h(px(34.))
                        .px_3()
                        .gap_2()
                        .text_sm()
                        .cursor_pointer()
                        .tooltip(move |_, cx| cx.new(|_| Tooltip::new(tab_path_tooltip.clone())).into())
                        .when(is_active, |this| {
                            this
                                .bg(theme.background)
                                .text_color(theme.foreground)
                        })
                        .when(!is_active, |this| {
                            this
                                .bg(theme.secondary)
                                .text_color(theme.muted_foreground)
                                .hover(|s| s.bg(theme.accent).text_color(theme.foreground))
                        })
                        .when(!is_last, |this| {
                            this
                                .border_r_1()
                                .border_color(theme.border)
                        })
                        .on_click(cx.listener(move |workspace, _, _window, cx| {
                            workspace.activate_tab(ix, cx);
                        }))
                        .context_menu({
                            let tab_path_buf = tab_path_buf.clone();
                            let tab_path_str = tab_path_str.clone();
                            let tab_index = ix;
                            let ws_for_reveal = workspace_entity.clone();
                            let ws_for_close = workspace_entity.clone();
                            move |menu, _window, _cx| {
                                menu.item(
                                    PopupMenuItem::new("Reveal in Sidebar")
                                        .on_click({
                                            let ws_for_reveal = ws_for_reveal.clone();
                                            let tab_path_buf = tab_path_buf.clone();
                                            move |_, _window, cx| {
                                                if let Some(ws) = ws_for_reveal.upgrade() {
                                                    let path = tab_path_buf.clone();
                                                    ws.update(cx, move |workspace, cx| {
                                                        workspace.reveal_in_explorer(path, cx);
                                                    });
                                                }
                                            }
                                        })
                                )
                                .item(
                                    PopupMenuItem::new("Open in File Manager")
                                        .on_click({
                                            let tab_path_buf = tab_path_buf.clone();
                                            move |_, _window, _cx| shell::open_in_explorer(&tab_path_buf)
                                        })
                                )
                                .item(
                                    PopupMenuItem::new("Copy File Path")
                                        .on_click({
                                            let tab_path_str = tab_path_str.clone();
                                            move |_, _window, cx| {
                                                cx.write_to_clipboard(ClipboardItem::new_string(tab_path_str.clone()));
                                            }
                                        })
                                )
                                .separator()
                                .item(
                                    PopupMenuItem::new("Close Tab")
                                        .on_click({
                                            let ws_for_close = ws_for_close.clone();
                                            move |_, _window, cx| {
                                                if let Some(ws) = ws_for_close.upgrade() {
                                                    ws.update(cx, |workspace, cx| {
                                                        workspace.close_tab(tab_index, cx);
                                                    });
                                                }
                                            }
                                        })
                                )
                            }
                        })
                        .child(
                            // Use relative positioning with two layers to prevent width jumping
                            // when font weight changes between active/inactive states
                            div()
                                .relative()
                                .max_w(px(150.))
                                .overflow_hidden()
                                .whitespace_nowrap()
                                // Invisible bold text layer for width calculation
                                .child(
                                    div()
                                        .font_weight(FontWeight::MEDIUM)
                                        .invisible()
                                        .child(tab.title.clone())
                                )
                                // Visible text layer with actual style
                                .child(
                                    div()
                                        .absolute()
                                        .inset_0()
                                        .when(is_active, |this| this.font_weight(FontWeight::MEDIUM))
                                        .child(tab.title.clone())
                                )
                        )
                        .child(
                            div()
                                .id(("close_tab", ix))
                                .flex()
                                .items_center()
                                .justify_center()
                                .w_5()
                                .h_5()
                                .rounded_sm()
                                .text_color(if is_active { theme.foreground } else { theme.muted_foreground })
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
                .mb(px(4.))
                .flex()
                .flex_row()
                .gap_1()
                // Only show search button when there are open documents
                .when(has_tabs, |this| {
                    this.child(
                        Button::new("search-btn")
                            .icon(Icon::new(IconName::Search))
                            .ghost()
                            .small()
                            .on_click(cx.listener(|workspace, _, window, cx| {
                                workspace.open_search(window, cx);
                            }))
                    )
                })
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
