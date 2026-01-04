use std::path::PathBuf;
use gpui::*;
use anyhow::Result;
use smol::fs;

pub struct Document {
    pub content: String,
    pub path: PathBuf,
}

impl Document {
    /// Create a new Document instance from content and path.
    pub fn new(content: String, path: PathBuf) -> Self {
        Self { content, path }
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
