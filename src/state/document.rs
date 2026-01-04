use std::path::PathBuf;
use gpui::*;
use anyhow::Result;
use smol::fs;
use pulldown_cmark::{Parser, Options, Event, Tag, TagEnd};

#[derive(Debug, Clone)]
#[allow(dead_code)]
pub enum Block {
    Header(String, u32),
    Paragraph(String),
    List(Vec<Block>, bool), // items, is_ordered
    ListItem(Vec<Block>),
    Code(String, String), // content, language
    Quote(Vec<Block>),
    Image(String, String), // src, alt
    Rule,
}

#[allow(dead_code)]
pub struct Document {
    pub content: String,
    pub path: PathBuf,
    pub blocks: Vec<Block>,
}

#[allow(dead_code)]
impl Document {
    /// Create a new Document instance from content and path.
    pub fn new(content: String, path: PathBuf) -> Self {
        let blocks = Self::parse_markdown(&content);
        Self { content, path, blocks }
    }

    fn parse_markdown(content: &str) -> Vec<Block> {
        let mut options = Options::empty();
        options.insert(Options::ENABLE_STRIKETHROUGH);
        options.insert(Options::ENABLE_TABLES);
        options.insert(Options::ENABLE_TASKLISTS);

        let parser = Parser::new_ext(content, options);
        let mut blocks = Vec::new();
        let mut current_text = String::new();
        let mut current_level = 0;
        let mut current_lang = String::new();
        
        // Simple state to detect if a paragraph is just an image
        let mut potential_image: Option<(String, String)> = None;

        for event in parser {
            match event {
                Event::Start(Tag::Heading { level, .. }) => {
                    current_level = match level {
                        pulldown_cmark::HeadingLevel::H1 => 1,
                        pulldown_cmark::HeadingLevel::H2 => 2,
                        pulldown_cmark::HeadingLevel::H3 => 3,
                        pulldown_cmark::HeadingLevel::H4 => 4,
                        pulldown_cmark::HeadingLevel::H5 => 5,
                        pulldown_cmark::HeadingLevel::H6 => 6,
                    };
                    current_text.clear();
                }
                Event::End(TagEnd::Heading(_)) => {
                    blocks.push(Block::Header(current_text.clone(), current_level));
                    current_text.clear();
                }
                Event::Start(Tag::Paragraph) => {
                    current_text.clear();
                    potential_image = None;
                }
                Event::End(TagEnd::Paragraph) => {
                    if let Some((src, alt)) = potential_image.take() {
                         // If we found an image and text is empty (or just the alt text?),
                         // we treat it as an Image Block.
                         // This is a heuristic.
                         blocks.push(Block::Image(src, alt));
                    } else if !current_text.trim().is_empty() {
                         blocks.push(Block::Paragraph(current_text.clone()));
                    }
                    current_text.clear();
                }
                Event::Start(Tag::Image { dest_url, .. }) => { 
                   // We capture the URL. The alt text will come as Text events?
                   // pulldown-cmark provides dest_url here.
                   // The text content inside Image tag is the Alt text.
                   potential_image = Some((dest_url.to_string(), String::new()));
                }
                 Event::End(TagEnd::Image) => {
                     if let Some((_, ref mut alt)) = potential_image {
                         *alt = current_text.clone();
                     }
                     current_text.clear(); // Clear text so it doesn't become paragraph text
                }
                Event::Start(Tag::CodeBlock(kind)) => {
                   current_lang = match kind {
                       pulldown_cmark::CodeBlockKind::Fenced(l) => l.to_string(),
                       _ => String::new(),
                   };
                   current_text.clear();
                }
                 Event::End(TagEnd::CodeBlock) => {
                    blocks.push(Block::Code(current_text.clone(), current_lang.clone()));
                    current_text.clear();
                }
                // ... (other cases)
                Event::Text(text) => {
                    current_text.push_str(&text);
                }
                Event::Code(text) => {
                    current_text.push('`');
                    current_text.push_str(&text);
                    current_text.push('`');
                }
                Event::SoftBreak | Event::HardBreak => {
                    current_text.push('\n');
                }
                Event::Rule => {
                    blocks.push(Block::Rule);
                }
                _ => {}
            }
        }
        
        blocks
    }

    /// Load a document from a file path asynchronously.
    /// Returns a Task that produces a Result containing the Entity handle.
    pub fn load(path: PathBuf, cx: &mut App) -> Task<Result<Entity<Self>>> {
        let path_clone = path.clone();
        cx.spawn(|cx: &mut AsyncApp| {
            let cx = cx.clone(); 
            async move {
                let content = fs::read_to_string(&path_clone).await?;
                
                // We need to jump back to the main thread (UI thread) to create the Entity
                cx.update(|cx| {
                    cx.new(|_cx| Self::new(content, path_clone))
                })
            }
        })
    }

    /// Update the document content.
    /// This could be used later for live reloading or editing.
    pub fn update_content(&mut self, content: String, cx: &mut Context<Self>) {
        self.content = content;
        cx.notify();
    }
}
