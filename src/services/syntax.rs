use syntect::parsing::SyntaxSet;
use syntect::highlighting::{ThemeSet, Style as SyntectStyle, FontStyle};
use syntect::easy::HighlightLines;
use syntect::util::LinesWithEndings;
use std::sync::OnceLock;
use std::ops::Range;
use gpui::{HighlightStyle, Rgba, FontWeight, FontStyle as GpuiFontStyle, UnderlineStyle};

pub struct SyntaxService {
    pub syntaxes: SyntaxSet,
    pub themes: ThemeSet,
}

impl SyntaxService {
    pub fn global() -> &'static Self {
        static SERVICE: OnceLock<SyntaxService> = OnceLock::new();
        SERVICE.get_or_init(|| Self {
            syntaxes: SyntaxSet::load_defaults_newlines(),
            themes: ThemeSet::load_defaults(),
        })
    }

    pub fn highlight(&self, text: &str, language: &str) -> Vec<(Range<usize>, HighlightStyle)> {
        let syntax = self.syntaxes.find_syntax_by_token(language)
            .unwrap_or_else(|| self.syntaxes.find_syntax_plain_text());
        
        // Use a default theme - ensuring it exists, otherwise fallback?
        // default themes include "base16-ocean.dark"
        let theme = &self.themes.themes.get("base16-ocean.dark")
            .or_else(|| self.themes.themes.values().next())
            .expect("No themes loaded");
            
        let mut highlighter = HighlightLines::new(syntax, theme);
        
        let mut highlights = Vec::new();
        let mut offset = 0;
        
        for line in LinesWithEndings::from(text) {
            let ranges: Vec<(SyntectStyle, &str)> = highlighter.highlight_line(line, &self.syntaxes).unwrap_or_default();
            for (style, chunk) in ranges {
                let len = chunk.len();
                let end = offset + len;
                if len > 0 {
                    let highlight = map_style(style);
                    // Optimization: only push if it changes style from default? 
                    // But HighlightStyle is override.
                    highlights.push((offset..end, highlight));
                }
                offset = end;
            }
        }
        
        highlights
    }
}

fn map_style(style: SyntectStyle) -> HighlightStyle {
    let mut highlight = HighlightStyle::default();
    
    // Syntect color is ARGB (u8)
    // GPUI color usually expects Rgba or Hsla
    // Rgba::new(r, g, b, a) -> where components are f32 0.0-1.0
    
    let color = Rgba {
        r: style.foreground.r as f32 / 255.0,
        g: style.foreground.g as f32 / 255.0,
        b: style.foreground.b as f32 / 255.0,
        a: style.foreground.a as f32 / 255.0,
    };
    highlight.color = Some(color.into());

    if style.font_style.contains(FontStyle::BOLD) {
        highlight.font_weight = Some(FontWeight::BOLD);
    }
    if style.font_style.contains(FontStyle::ITALIC) {
        highlight.font_style = Some(GpuiFontStyle::Italic);
    }
    if style.font_style.contains(FontStyle::UNDERLINE) {
        highlight.underline = Some(UnderlineStyle {
             color: None,
             thickness: 1.0.into(),
             wavy: false,
        });
    }
    
    highlight
}