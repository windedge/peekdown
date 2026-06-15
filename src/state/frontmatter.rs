//! YAML frontmatter parsing support.
//!
//! Extracts and represents YAML frontmatter from Markdown documents
//! as a structured, order-preserving key-value mapping.

/// The value type for a frontmatter entry.
#[derive(Debug, Clone, PartialEq)]
pub enum Value {
    String(String),
    Number(f64),
    Bool(bool),
    Null,
    List(Vec<Value>),
    Map(Vec<(String, Value)>),
}

/// Convert a serde_yaml key to a String (non-string keys are stringified).
fn yaml_key_to_string(key: &serde_yaml::Value) -> String {
    match key {
        serde_yaml::Value::String(s) => s.clone(),
        other => yaml_value_to_string(other),
    }
}

/// Convert a serde_yaml value to a displayable string.
fn yaml_value_to_string(val: &serde_yaml::Value) -> String {
    match val {
        serde_yaml::Value::Null => "null".to_string(),
        serde_yaml::Value::Bool(b) => b.to_string(),
        serde_yaml::Value::Number(n) => n.to_string(),
        serde_yaml::Value::String(s) => s.clone(),
        serde_yaml::Value::Sequence(seq) => {
            let items: Vec<String> = seq.iter().map(yaml_value_to_string).collect();
            format!("[{}]", items.join(", "))
        }
        serde_yaml::Value::Mapping(map) => {
            let items: Vec<String> = map
                .iter()
                .map(|(k, v)| format!("{}: {}", yaml_value_to_string(k), yaml_value_to_string(v)))
                .collect();
            format!("{{{}}}", items.join(", "))
        }
        serde_yaml::Value::Tagged(tagged) => {
            // Tagged values: just show the value part
            yaml_value_to_string(&tagged.value)
        }
    }
}

impl std::fmt::Display for Value {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Value::String(s) => write!(f, "{}", s),
            Value::Number(n) => {
                if *n == n.floor() && n.is_finite() {
                    write!(f, "{}", *n as i64)
                } else {
                    write!(f, "{}", n)
                }
            }
            Value::Bool(b) => write!(f, "{}", b),
            Value::Null => write!(f, "null"),
            Value::List(items) => {
                let strs: Vec<String> = items.iter().map(|v| v.to_string()).collect();
                write!(f, "[{}]", strs.join(", "))
            }
            Value::Map(entries) => {
                let strs: Vec<String> = entries
                    .iter()
                    .map(|(k, v)| format!("{}: {}", k, v))
                    .collect();
                write!(f, "{{{}}}", strs.join(", "))
            }
        }
    }
}

/// Parsed YAML frontmatter, with order-preserving entries.
#[derive(Debug, Clone, PartialEq)]
pub struct Frontmatter {
    /// Entries in insertion order.
    pub entries: Vec<(String, Value)>,
}

#[allow(dead_code)]
impl Frontmatter {
    /// Returns true if there are no entries.
    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }

    /// Returns the number of entries.
    pub fn len(&self) -> usize {
        self.entries.len()
    }
}

/// Parse a YAML string into a `Frontmatter` value.
///
/// Only handles top-level mapping (key-value pairs).
/// Returns `Err` if the YAML is not a mapping or cannot be parsed.
pub fn parse(source: &str) -> Result<Frontmatter, serde_yaml::Error> {
    let yaml_value: serde_yaml::Value = serde_yaml::from_str(source)?;

    match yaml_value {
        serde_yaml::Value::Mapping(mapping) => {
            let mut entries = Vec::with_capacity(mapping.len());
            for (key, value) in mapping {
                entries.push((yaml_key_to_string(&key), convert_value(value)));
            }
            Ok(Frontmatter { entries })
        }
        // Empty document (just `---\n---`) produces Null
        serde_yaml::Value::Null => Ok(Frontmatter {
            entries: Vec::new(),
        }),
        other => {
            // Non-mapping top-level YAML is an error
            Err(serde::de::Error::custom(format!(
                "expected a mapping at top level, got {}",
                yaml_type_name(&other)
            )))
        }
    }
}

fn convert_value(value: serde_yaml::Value) -> Value {
    match value {
        serde_yaml::Value::String(s) => Value::String(s),
        serde_yaml::Value::Number(n) => Value::Number(n.as_f64().unwrap_or(0.0)),
        serde_yaml::Value::Bool(b) => Value::Bool(b),
        serde_yaml::Value::Null => Value::Null,
        serde_yaml::Value::Sequence(seq) => {
            Value::List(seq.into_iter().map(convert_value).collect())
        }
        serde_yaml::Value::Mapping(map) => {
            let entries: Vec<(String, Value)> = map
                .into_iter()
                .map(|(k, v)| (yaml_key_to_string(&k), convert_value(v)))
                .collect();
            Value::Map(entries)
        }
        // Tagged values — extract the inner value
        serde_yaml::Value::Tagged(tagged) => convert_value(tagged.value.clone()),
    }
}

fn yaml_type_name(value: &serde_yaml::Value) -> &'static str {
    match value {
        serde_yaml::Value::Null => "null",
        serde_yaml::Value::Bool(_) => "bool",
        serde_yaml::Value::Number(_) => "number",
        serde_yaml::Value::String(_) => "string",
        serde_yaml::Value::Sequence(_) => "sequence",
        serde_yaml::Value::Mapping(_) => "mapping",
        serde_yaml::Value::Tagged(_) => "tagged",
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_simple_key_value() {
        let fm = parse("title: Hello\nversion: 1.5").unwrap();
        assert_eq!(fm.entries.len(), 2);
        assert_eq!(fm.entries[0].0, "title");
        assert_eq!(fm.entries[0].1, Value::String("Hello".to_string()));
        assert_eq!(fm.entries[1].0, "version");
        assert_eq!(fm.entries[1].1, Value::Number(1.5));
    }

    #[test]
    fn test_bool_and_null() {
        let fm = parse("draft: true\npublished: no\nextra:").unwrap();
        assert_eq!(fm.entries[0].1, Value::Bool(true));
        // In serde_yaml 0.9, "no" is parsed as a string by default
        assert_eq!(fm.entries[1].1, Value::String("no".to_string()));
        // null field
        assert_eq!(fm.entries[2].1, Value::Null);
    }

    #[test]
    fn test_nested_map() {
        let fm = parse(
            "author:\n  name: Alice\n  email: a@b.com\n",
        )
        .unwrap();
        assert_eq!(fm.entries.len(), 1);
        assert_eq!(fm.entries[0].0, "author");
        match &fm.entries[0].1 {
            Value::Map(entries) => {
                assert_eq!(entries.len(), 2);
                assert_eq!(entries[0].0, "name");
                assert_eq!(entries[0].1, Value::String("Alice".to_string()));
                assert_eq!(entries[1].0, "email");
                assert_eq!(entries[1].1, Value::String("a@b.com".to_string()));
            }
            _ => panic!("expected map"),
        }
    }

    #[test]
    fn test_list() {
        let fm = parse("tags: [rust, markdown, frontmatter]").unwrap();
        assert_eq!(fm.entries.len(), 1);
        assert_eq!(fm.entries[0].0, "tags");
        match &fm.entries[0].1 {
            Value::List(items) => {
                assert_eq!(items.len(), 3);
                assert_eq!(items[0], Value::String("rust".to_string()));
                assert_eq!(items[1], Value::String("markdown".to_string()));
                assert_eq!(items[2], Value::String("frontmatter".to_string()));
            }
            _ => panic!("expected list"),
        }
    }

    #[test]
    fn test_empty_frontmatter() {
        let fm = parse("").unwrap();
        // Empty string -> Null
        assert!(fm.is_empty());
    }

    #[test]
    fn test_not_a_mapping() {
        let result = parse("[a, b, c]");
        assert!(result.is_err());
    }

    #[test]
    fn test_invalid_yaml() {
        let result = parse(": broken yaml [");
        assert!(result.is_err());
    }

    #[test]
    fn test_order_preserved() {
        let fm = parse("z: last\na: first\nm: middle").unwrap();
        assert_eq!(fm.entries[0].0, "z");
        assert_eq!(fm.entries[1].0, "a");
        assert_eq!(fm.entries[2].0, "m");
    }

    #[test]
    fn test_number_integer_display() {
        let fm = parse("count: 42").unwrap();
        assert_eq!(fm.entries[0].1.to_string(), "42");
    }

    #[test]
    fn test_number_decimal_display() {
        let fm = parse("pi: 3.14").unwrap();
        assert_eq!(fm.entries[0].1.to_string(), "3.14");
    }
}
