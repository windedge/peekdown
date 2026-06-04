use std::collections::HashMap;

use gpui::{
    App, InteractiveElement as _, IntoElement, ListState, ParentElement as _, Pixels, SharedString,
    Styled as _, Window, div, px, relative,
};

use super::node::{BlockNode, NodeContext, Paragraph};

/// A heading item extracted from the document for outline display.
#[derive(Debug, Clone)]
pub struct HeadingItem {
    /// The heading level (1-6)
    pub level: u8,
    /// The text content of the heading
    pub text: String,
    /// The block index in the document (for scrolling)
    pub block_index: usize,
}

/// The parsed document AST.
#[derive(Debug, Clone, PartialEq, Default)]
pub struct ParsedDocument {
    pub(crate) source: SharedString,
    pub(crate) blocks: Vec<BlockNode>,
    /// Map from anchor slug to block index for heading navigation.
    pub(crate) heading_map: HashMap<String, usize>,
}

#[derive(Default, Clone, Copy)]
pub(crate) struct NodeRenderOptions {
    pub(crate) ix: usize,
    pub(crate) in_list: bool,
    pub(crate) todo: bool,
    pub(crate) ordered: bool,
    pub(crate) depth: usize,
    pub(crate) is_last: bool,
}

impl NodeRenderOptions {
    pub(crate) fn is_last(mut self, is_last: bool) -> Self {
        self.is_last = is_last;
        self
    }
}

impl ParsedDocument {
    /// Build the heading_map by walking all blocks and collecting heading IDs.
    pub(crate) fn build_heading_map(&mut self) {
        self.heading_map.clear();
        let mut slug_counts: HashMap<String, usize> = HashMap::new();

        for (index, block) in self.blocks.iter().enumerate() {
            Self::collect_heading_ids(block, index, &mut self.heading_map, &mut slug_counts);
        }
    }

    fn collect_heading_ids(
        block: &BlockNode,
        block_index: usize,
        map: &mut HashMap<String, usize>,
        slug_counts: &mut HashMap<String, usize>,
    ) {
        match block {
            BlockNode::Heading { id, .. } => {
                if let Some(slug) = id {
                    if !slug.is_empty() {
                        let count = slug_counts.entry(slug.to_string()).or_insert(0);
                        let final_slug = if *count > 0 {
                            format!("{}-{}", slug, *count)
                        } else {
                            slug.to_string()
                        };
                        *count += 1;
                        map.insert(final_slug, block_index);
                    }
                }
            }
            BlockNode::Root { children, .. }
            | BlockNode::Blockquote { children, .. }
            | BlockNode::List { children, .. }
            | BlockNode::ListItem { children, .. } => {
                for child in children {
                    Self::collect_heading_ids(child, block_index, map, slug_counts);
                }
            }
            _ => {}
        }
    }

    pub(super) fn selected_text(&self) -> String {
        let mut text = String::new();
        for block in self.blocks.iter() {
            text.push_str(&block.selected_text());
        }
        text
    }

    /// Clear all InlineState selections in the document.
    pub(super) fn clear_all_selections(&self) {
        for block in self.blocks.iter() {
            block.clear_selection();
        }
    }

    /// Extract all headings from the document for outline display.
    pub fn extract_headings(&self) -> Vec<HeadingItem> {
        let mut headings = Vec::new();
        for (index, block) in self.blocks.iter().enumerate() {
            Self::collect_headings_from_block(block, index, &mut headings);
        }
        headings
    }

    fn collect_headings_from_block(block: &BlockNode, block_index: usize, headings: &mut Vec<HeadingItem>) {
        match block {
            BlockNode::Heading { level, children, .. } => {
                let text = Self::extract_text_from_paragraph(children);
                headings.push(HeadingItem {
                    level: *level,
                    text,
                    block_index,
                });
            }
            BlockNode::Root { children, .. }
            | BlockNode::Blockquote { children, .. }
            | BlockNode::List { children, .. }
            | BlockNode::ListItem { children, .. } => {
                for child in children {
                    Self::collect_headings_from_block(child, block_index, headings);
                }
            }
            _ => {}
        }
    }

    fn extract_text_from_paragraph(paragraph: &Paragraph) -> String {
        paragraph.children.iter()
            .map(|node| node.text.to_string())
            .collect::<Vec<_>>()
            .join("")
    }

    /// Get block spans (index, byte range) for search matching.
    pub fn block_spans(&self) -> Vec<(usize, std::ops::Range<usize>)> {
        self.blocks.iter().enumerate().filter_map(|(ix, block)| {
            block.span().map(|span| (ix, span.start..span.end))
        }).collect()
    }

    /// Converts the node to markdown format.
    ///
    /// This is used to generate markdown for test.
    #[allow(dead_code)]
    pub(crate) fn to_markdown(&self) -> String {
        self.blocks
            .iter()
            .map(|child| child.to_markdown())
            .collect::<Vec<_>>()
            .join("\n\n")
    }

    pub(super) fn render_root(
        &self,
        list_state: Option<ListState>,
        node_cx: &NodeContext,
        content_max_width: Option<Pixels>,
        window: &mut Window,
        cx: &mut App,
    ) -> impl IntoElement {
        let options = NodeRenderOptions {
            is_last: true,
            ..Default::default()
        };

        let Some(list_state) = list_state else {
            let document = div()
                .id("document")
                .w_full()
                .min_w(px(0.));

            let document = match content_max_width {
                Some(max_width) => document.max_w(max_width).mx_auto(),
                None => document.max_w(relative(1.)),
            };

            return document.children(self.blocks.iter().enumerate().map(move |(ix, node)| {
                node.render_block(NodeRenderOptions { ix, ..options }, node_cx, window, cx)
            }));
        };

        let blocks = &self.blocks;

        if list_state.item_count() != blocks.len() {
            list_state.reset(blocks.len());
        }

        let document = div()
            .id("document")
            .size_full()
            .min_w(px(0.));

        let document = match content_max_width {
            Some(max_width) => document.max_w(max_width).mx_auto(),
            None => document.max_w(relative(1.)),
        };

        document.child(
            gpui::list(list_state, {
                let node_cx = node_cx.clone();
                let blocks = blocks.clone();
                move |ix, window, cx| {
                    let is_last = ix + 1 == blocks.len();
                    div()
                        .w_full()
                        .min_w(px(0.))
                        .max_w(relative(1.))
                        .child(blocks[ix].render_block(
                            NodeRenderOptions {
                                ix,
                                is_last,
                                ..options
                            },
                            &node_cx,
                            window,
                            cx,
                        ))
                        .into_any_element()
                }
            })
            .size_full(),
        )
    }
}
