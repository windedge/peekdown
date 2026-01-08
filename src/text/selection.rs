//! Text selection utilities for word and line boundary detection.

/// Find word boundaries around a byte index.
/// Returns (start, end) as byte offsets.
pub fn word_boundaries(text: &str, byte_index: usize) -> (usize, usize) {
    if text.is_empty() || byte_index >= text.len() {
        return (byte_index, byte_index);
    }

    // Find the character at byte_index
    let char_at_index = text[byte_index..].chars().next();
    let char_type = char_at_index.map(char_type).unwrap_or(CharType::Other);

    // Find start - scan backwards
    let mut start = byte_index;
    for (i, c) in text[..byte_index].char_indices().rev() {
        if char_type.is_connectable(c) {
            start = i;
        } else {
            break;
        }
    }

    // Find end - scan forwards
    let mut end = byte_index;
    for (i, c) in text[byte_index..].char_indices() {
        let actual_index = byte_index + i;
        if char_type.is_connectable(c) {
            end = actual_index + c.len_utf8();
        } else if i > 0 {
            // Already moved past the first character
            break;
        } else {
            // First character matches or doesn't match
            end = actual_index + c.len_utf8();
            if !char_type.is_connectable(c) {
                break;
            }
        }
    }

    (start, end)
}

/// Find line boundaries around a byte index.
/// Returns (start, end) as byte offsets, not including the newline character.
pub fn line_boundaries(text: &str, byte_index: usize) -> (usize, usize) {
    if text.is_empty() {
        return (0, 0);
    }

    let byte_index = byte_index.min(text.len().saturating_sub(1));

    // Find start - look for newline before byte_index
    let start = text[..byte_index]
        .rfind('\n')
        .map(|i| i + 1)
        .unwrap_or(0);

    // Find end - look for newline after byte_index
    let end = text[byte_index..]
        .find('\n')
        .map(|i| byte_index + i)
        .unwrap_or(text.len());

    (start, end)
}

#[derive(Clone, Copy, PartialEq)]
enum CharType {
    Word,       // a-z, A-Z, 0-9, _
    Whitespace, // spaces, tabs
    Newline,    // \n, \r
    Other,      // punctuation, CJK characters
}

fn char_type(c: char) -> CharType {
    if c.is_alphanumeric() || c == '_' {
        CharType::Word
    } else if c == '\n' || c == '\r' {
        CharType::Newline
    } else if c.is_whitespace() {
        CharType::Whitespace
    } else {
        CharType::Other
    }
}

impl CharType {
    fn is_connectable(&self, c: char) -> bool {
        let other_type = char_type(c);
        match (self, other_type) {
            (CharType::Word, CharType::Word) => true,
            (CharType::Whitespace, CharType::Whitespace) => true,
            _ => false,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_word_boundaries() {
        let text = "hello world";
        assert_eq!(word_boundaries(text, 0), (0, 5)); // h -> hello
        assert_eq!(word_boundaries(text, 2), (0, 5)); // l -> hello
        assert_eq!(word_boundaries(text, 5), (5, 6)); // space
        assert_eq!(word_boundaries(text, 6), (6, 11)); // w -> world
    }

    #[test]
    fn test_word_boundaries_with_underscore() {
        let text = "hello_world test";
        assert_eq!(word_boundaries(text, 0), (0, 11)); // hello_world
        assert_eq!(word_boundaries(text, 6), (0, 11)); // _
    }

    #[test]
    fn test_line_boundaries() {
        let text = "line1\nline2\nline3";
        assert_eq!(line_boundaries(text, 0), (0, 5)); // line1
        assert_eq!(line_boundaries(text, 3), (0, 5)); // still line1
        assert_eq!(line_boundaries(text, 6), (6, 11)); // line2
        assert_eq!(line_boundaries(text, 12), (12, 17)); // line3
    }

    #[test]
    fn test_line_boundaries_single_line() {
        let text = "single line";
        assert_eq!(line_boundaries(text, 0), (0, 11));
        assert_eq!(line_boundaries(text, 5), (0, 11));
    }

    #[test]
    fn test_empty_text() {
        assert_eq!(word_boundaries("", 0), (0, 0));
        assert_eq!(line_boundaries("", 0), (0, 0));
    }
}
