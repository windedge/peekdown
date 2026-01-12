//! Additional language support for syntax highlighting.
//!
//! This module registers languages that are not included in gpui-component's
//! default set. Languages are registered with the global LanguageRegistry
//! at application startup.

use gpui_component::highlighter::{LanguageConfig, LanguageRegistry};

/// Kotlin syntax highlighting query adapted for gpui-component.
/// Based on nvim-treesitter (Apache license), modified for gpui-component's HIGHLIGHT_NAMES.
const KOTLIN_HIGHLIGHTS_QUERY: &str = r#"
;;; Identifiers

(simple_identifier) @variable

(class_parameter
	(simple_identifier) @property)

(class_body
	(property_declaration
		(variable_declaration
			(simple_identifier) @property)))

(_
	(navigation_suffix
		(simple_identifier) @property))

(enum_entry
	(simple_identifier) @constant)

(type_identifier) @type

(label) @label

;;; Function definitions

(function_declaration
	. (simple_identifier) @function)

(getter
	("get") @function)
(setter
	("set") @function)

(primary_constructor) @constructor
(secondary_constructor
	("constructor") @constructor)

(constructor_invocation
	(user_type
		(type_identifier) @constructor))

(anonymous_initializer
	("init") @constructor)

(parameter
	(simple_identifier) @variable)

(parameter_with_optional_type
	(simple_identifier) @variable)

(lambda_literal
	(lambda_parameters
		(variable_declaration
			(simple_identifier) @variable)))

;;; Function calls

(call_expression
	. (simple_identifier) @function)

(call_expression
	(navigation_expression
		(navigation_suffix
			(simple_identifier) @function) . ))

;;; Literals

[
	(line_comment)
	(multiline_comment)
	(shebang_line)
] @comment

[
	(real_literal)
	(integer_literal)
	(long_literal)
	(hex_literal)
	(bin_literal)
	(unsigned_literal)
] @number

(boolean_literal) @boolean
(null_literal) @constant

(character_literal) @string
(string_literal) @string
(character_escape_seq) @string.escape

;;; Keywords

(type_alias "typealias" @keyword)

[
	(class_modifier)
	(member_modifier)
	(function_modifier)
	(property_modifier)
	(platform_modifier)
	(variance_modifier)
	(parameter_modifier)
	(visibility_modifier)
	(reification_modifier)
	(inheritance_modifier)
] @keyword

[
	"val"
	"var"
	"enum"
	"class"
	"object"
	"interface"
	"fun"
	"if"
	"else"
	"when"
	"for"
	"do"
	"while"
	"try"
	"catch"
	"throw"
	"finally"
	"import"
	"package"
	"return"
	"break"
	"continue"
] @keyword

(jump_expression) @keyword

(annotation
	"@" @attribute (use_site_target)? @attribute)
(annotation
	(user_type
		(type_identifier) @attribute))
(annotation
	(constructor_invocation
		(user_type
			(type_identifier) @attribute)))

(file_annotation
	"@" @attribute "file" @attribute ":" @attribute)
(file_annotation
	(user_type
		(type_identifier) @attribute))
(file_annotation
	(constructor_invocation
		(user_type
			(type_identifier) @attribute)))

;;; Operators & Punctuation

[
	"!" "!=" "!==" "=" "==" "==="
	">" ">=" "<" "<=" "||" "&&"
	"+" "++" "+=" "-" "--" "-="
	"*" "*=" "/" "/="
	"%" "%=" "?." "?:"
	"!!" "is" "!is" "in" "!in"
	"as" "as?" ".." "->"
] @operator

[
	"(" ")" "[" "]" "{" "}"
] @punctuation.bracket

[
	"." "," ";" ":" "::"
] @punctuation.delimiter

(string_literal
	"$" @punctuation.special)
(string_literal
	"${" @punctuation.special
	"}" @punctuation.special)
"#;

/// Register additional languages to the global LanguageRegistry.
///
/// This should be called during application initialization, after
/// `gpui_component::init()` but before any syntax highlighting is used.
pub fn register_languages() {
    let registry = LanguageRegistry::singleton();

    // Register Kotlin with both "kotlin" and "kt" aliases
    let kotlin_config = LanguageConfig::new(
        "kotlin",
        tree_sitter_kotlin::language().into(),
        vec![],
        KOTLIN_HIGHLIGHTS_QUERY,
        "", // injections query
        "", // locals query
    );

    registry.register("kotlin", &kotlin_config);
    registry.register("kt", &kotlin_config);

    tracing::info!("Registered Kotlin language support");
}
