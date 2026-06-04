use std::cell::Cell;
use std::rc::Rc;
use std::path::PathBuf;
use std::time::Duration;

use gpui::prelude::FluentBuilder;
use gpui::*;
use crate::text::{TextView, TextViewState, TextViewStyle};
use crate::text::document::HeadingItem;
use crate::text::ElementExt;
use gpui_component::{
    ActiveTheme, IconName, Sizable,
    button::{Button, ButtonVariants},
    menu::ContextMenuExt,
};
use crate::state::document::Document;
use crate::state::frontmatter::Frontmatter;
use crate::state::config::{AppConfig, LayoutMode};
use crate::services::shell;
use super::{OpenSearch, RefreshDocument, SelectAll, WorkspaceView};

pub struct MarkdownView {
    #[allow(dead_code)] // Reserved for future file reload functionality
    document: Entity<Document>,
    config: Entity<AppConfig>,
    text_view_state: Entity<TextViewState>,
    workspace: WeakEntity<WorkspaceView>,  // Add workspace reference
}

impl MarkdownView {
    pub fn new(
        document: Entity<Document>,
        config: Entity<AppConfig>,
        workspace: WeakEntity<WorkspaceView>,  // Add workspace parameter
        cx: &mut Context<Self>,
    ) -> Self {
        // Observe config changes to re-render when layout mode changes
        cx.observe(&config, |this, _, cx| {
            // Update scroll settings in TextViewState when config changes
            let appearance = this.config.read(cx).appearance.clone();
            this.text_view_state.update(cx, |state, cx| {
                state.set_scroll_speed(appearance.scroll_speed, cx);
                state.set_inertia_enabled(appearance.inertia_scroll, cx);
            });
            cx.notify();
        }).detach();

        // Create TextViewState once - content is parsed only at initialization
        let content = document.read(cx).content.clone();
        let doc_path = document.read(cx).path.clone();
        let scroll_speed = config.read(cx).appearance.scroll_speed;
        let inertia_enabled = config.read(cx).appearance.inertia_scroll;
        let text_view_state = cx.new(|cx| {
            TextViewState::markdown(content.as_ref(), Some(&doc_path), cx)
                .scroll_speed(scroll_speed)
                .inertia_enabled(inertia_enabled)
        });

        // Observe text_view_state changes to re-render when scroll state updates
        // (e.g., floating button visibility must react to scrolling)
        cx.observe(&text_view_state, |_, _, cx| {
            cx.notify();
        }).detach();

        Self {
            document,
            config,
            text_view_state,
            workspace,  // Store workspace reference
        }
    }

    /// Get the headings from the document for outline display.
    pub fn headings(&self, cx: &App) -> Vec<HeadingItem> {
        self.text_view_state.read(cx).headings()
    }

    /// Scroll to a specific heading by block index.
    pub fn scroll_to_heading(&self, block_index: usize, cx: &mut App) {
        self.text_view_state.update(cx, |state, _| {
            state.scroll_to_block(block_index);
        });
    }

    /// Get a reference to the text view state entity.
    pub fn text_view_state(&self) -> &Entity<TextViewState> {
        &self.text_view_state
    }

    /// Get the source text for search.
    pub fn source_text(&self, cx: &App) -> gpui::SharedString {
        self.text_view_state.read(cx).source_text()
    }

    /// Get block spans for search matching.
    pub fn block_spans(&self, cx: &App) -> Vec<(usize, std::ops::Range<usize>)> {
        self.text_view_state.read(cx).block_spans()
    }

    /// Get the file path of the document.
    pub fn file_path(&self, cx: &App) -> PathBuf {
        self.document.read(cx).path.clone()
    }

    /// Get the parsed frontmatter of the document, if any.
    #[allow(dead_code)]
    pub fn frontmatter(&self, cx: &App) -> Option<Frontmatter> {
        self.document.read(cx).frontmatter.clone()
    }
}

impl Render for MarkdownView {
    fn render(&mut self, _window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let theme = cx.theme().clone();
        let layout_mode = self.config.read(cx).appearance.layout;
        let scroll_speed = self.config.read(cx).appearance.scroll_speed;
        let file_path = self.document.read(cx).path.clone();
        let has_selection = self.text_view_state.read(cx).has_selection();

        let content_max_width = px(900.);
        let min_padding = px(32.);
        let text_style = match layout_mode {
            LayoutMode::Centered => TextViewStyle::default().content_max_width(content_max_width),
            LayoutMode::FullWidth => TextViewStyle::default(),
        };

        // Use Rc<Cell> to share container bounds between on_prepaint and on_scroll_wheel
        let container_width = Rc::new(Cell::new(px(0.)));
        let container_origin_x = Rc::new(Cell::new(px(0.)));
        let container_width_for_prepaint = container_width.clone();
        let container_width_for_scroll = container_width.clone();
        let container_origin_x_for_prepaint = container_origin_x.clone();
        let container_origin_x_for_scroll = container_origin_x.clone();

        let text_state_for_scroll = self.text_view_state.clone();
        let text_state_for_floating = self.text_view_state.clone();
        let text_state_for_menu = self.text_view_state.clone();
        // Read scroll state for floating action buttons
        let scroll_offset = self.text_view_state.read(cx).scroll_offset_y();
        let max_scroll = self.text_view_state.read(cx).max_scroll_y();
        let is_at_top = scroll_offset <= px(0.);
        let is_at_bottom = scroll_offset >= max_scroll;

        // Calculate reading progress (0.0 ~ 1.0)
        let progress = if max_scroll > px(0.) {
            (scroll_offset / max_scroll).clamp(0.0, 1.0)
        } else {
            0.0
        };

        div()
            .id("markdown-container")
            .relative()
            .size_full()
            .flex()
            .flex_col()
            .bg(theme.background)
            // Use on_prepaint to get actual container bounds
            .on_prepaint(move |bounds, _, _cx| {
                container_width_for_prepaint.set(bounds.size.width);
                container_origin_x_for_prepaint.set(bounds.origin.x);
            })
            .on_scroll_wheel(move |event, _window, cx| {
                // Get current container width
                let width = container_width_for_scroll.get();
                let origin_x = container_origin_x_for_scroll.get();

                // ScrollWheelEvent position is in window coordinates
                let local_x = event.position.x - origin_x;
                if local_x < px(0.) || local_x > width {
                    return;
                }

                let side_padding = match layout_mode {
                    LayoutMode::Centered => {
                        if width > content_max_width + min_padding * 2.0 {
                            (width - content_max_width) / 2.0
                        } else {
                            min_padding
                        }
                    }
                    LayoutMode::FullWidth => min_padding,
                };

                // Check if cursor is in the padding area (left or right)
                let in_padding = local_x < side_padding
                    || local_x > (width - side_padding);
                if !in_padding {
                    return;
                }

                // Scroll padding area with same logic as content area
                let delta = event.delta.pixel_delta(px(20.)).y;
                text_state_for_scroll.update(cx, |state, cx| {
                    if state.is_inertia_enabled() {
                        state.add_scroll_impulse(f32::from(delta));
                    } else {
                        let scroll_distance = delta * state.get_scroll_speed();
                        state.scroll_by_direct(-scroll_distance);
                    }
                    cx.notify();
                });
                cx.stop_propagation();
            })
            .child(
                div()
                    .flex_1()
                    .min_h_0()
                    .child(
                TextView::new(&self.text_view_state)
                    .style(text_style)
                    .scrollable(true)
                    .scroll_speed(scroll_speed)
                    .selectable(true)
                    .pb_8()
                    .px(min_padding)
                    .text_size(px(self.config.read(cx).appearance.font_size))
                    .code_block_actions(|code_block, window, cx| {
                        let code = code_block.code();
                        let key: SharedString = match code_block.span {
                            Some(s) => format!("code-copy-{}-{}", s.start, s.end).into(),
                            None => format!("code-copy-{}", code.len()).into(),
                        };
                        let state: Entity<bool> = window.use_keyed_state(
                            gpui::ElementId::Name(key.clone()),
                            cx,
                            |_, _| false,
                        );
                        let copied = *state.read(cx);

                        Button::new(gpui::ElementId::Name(format!("copy-btn-{key}").into()))
                            .icon(if copied { IconName::Check } else { IconName::Copy })
                            .ghost()
                            .xsmall()
                            .tooltip("Copy code")
                            .when(!copied, |this| {
                                this.on_click({
                                    let code = code.clone();
                                    move |_, _window, cx| {
                                        cx.stop_propagation();
                                        cx.write_to_clipboard(
                                            ClipboardItem::new_string(code.to_string()),
                                        );
                                        state.update(cx, |copied, cx| {
                                            *copied = true;
                                            cx.notify();
                                        });

                                        let s = state.clone();
                                        cx.spawn(async move |cx| {
                                            cx.background_executor()
                                                .timer(Duration::from_secs(2))
                                                .await;
                                            _ = s.update(cx, |copied, cx| {
                                                *copied = false;
                                                cx.notify();
                                            });
                                        })
                                        .detach();
                                    }
                                })
                            })
                            .into_any_element()
                        })
                    )
            )
            // Reading progress bar at the top
            .when(max_scroll > px(0.), |this| {
                this.child(
                    div()
                        .absolute()
                        .top_0()
                        .left_0()
                        .h(px(2.))
                        .w(relative(progress))
                        .bg(theme.progress_bar),
                )
            })
            .child(
                // Floating scroll-to-top/bottom buttons
                div()
                    .absolute()
                    .bottom(px(24.))
                    .right(px(24.))
                    .flex()
                    .flex_col()
                    .gap_2()
                    .when(!is_at_top, |this| {
                        this.child(
                            Button::new("scroll-top-btn")
                                .icon(IconName::ChevronUp)
                                .ghost()
                                .xsmall()
                                .tooltip("Scroll to top")
                                .on_click({
                                    let text_state = text_state_for_floating.clone();
                                    move |_, _, cx| {
                                        text_state.update(cx, |state, _| {
                                            state.scroll_to_block(0);
                                        });
                                    }
                                }),
                        )
                    })
                    .when(!is_at_bottom, |this| {
                        this.child(
                            Button::new("scroll-bottom-btn")
                                .icon(IconName::ChevronDown)
                                .ghost()
                                .xsmall()
                                .tooltip("Scroll to bottom")
                                .on_click({
                                    let text_state = text_state_for_floating.clone();
                                    move |_, _, cx| {
                                        let last =
                                            text_state.read(cx).block_count().saturating_sub(1);
                                        text_state.update(cx, |state, _| {
                                            state.scroll_to_block(last);
                                        });
                                    }
                                }),
                        )
                    }),
            )
            .context_menu({
                let file_path = file_path.clone();
                let workspace_for_reveal = self.workspace.clone();
                move |menu, _window, _cx| {
                    let path_for_reveal = file_path.clone();
                    let path_for_explorer = file_path.clone();
                    let path_for_copy = file_path.clone();
                    let text_state_for_copy = text_state_for_menu.clone();
                    let ws = workspace_for_reveal.clone();

                    menu.item(
                            gpui_component::menu::PopupMenuItem::new("Copy")
                                .disabled(!has_selection)
                                .on_click(move |_, _window, cx| {
                                    let selected_text = text_state_for_copy.read(cx).selected_text();
                                    let selected_text = selected_text.trim();
                                    if !selected_text.is_empty() {
                                        cx.write_to_clipboard(ClipboardItem::new_string(selected_text.to_string()));
                                    }
                                }),
                        )
                        .menu("Select All", Box::new(SelectAll))
                        .separator()
                        .menu("Search", Box::new(OpenSearch))
                        .menu("Refresh", Box::new(RefreshDocument))
                        .separator()
                        .item(
                            gpui_component::menu::PopupMenuItem::new("Reveal in Sidebar")
                                .on_click(move |_, _window, cx| {
                                    if let Some(workspace) = ws.upgrade() {
                                        workspace.update(cx, |ws, cx| {
                                            ws.reveal_in_explorer(path_for_reveal.clone(), cx);
                                        });
                                    }
                                }),
                        )
                        .item(
                            gpui_component::menu::PopupMenuItem::new("Open in File Manager")
                                .on_click(move |_, _window, _cx| {
                                    shell::open_in_explorer(&path_for_explorer);
                                }),
                        )
                        .item(
                            gpui_component::menu::PopupMenuItem::new("Copy File Path")
                                .on_click(move |_, _window, cx| {
                                    let path_str = super::normalize_unc_path(&path_for_copy.to_string_lossy());
                                    cx.write_to_clipboard(ClipboardItem::new_string(path_str));
                                }),
                        )
                }
            })
    }
}
