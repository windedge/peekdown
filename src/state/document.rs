use gpui::*;
use std::path::PathBuf;

pub struct Document {
    pub content: SharedString,
    #[allow(dead_code)]
    pub path: PathBuf,
}

impl Document {
    pub fn new(content: String, path: PathBuf) -> Self {
        Self {
            content: content.into(),
            path,
        }
    }
}