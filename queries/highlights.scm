; Highlights for MinZ

; Comments
(comment) @comment

; Keywords
[
  "import"
  "as"
  "fn"
  "let"
  "mut"
  "const"
  "type"
  "struct"
  "enum"
  "export"
  "return"
  "if"
  "else"
  "while"
  "for"
  "in"
  "loop"
  "break"
  "continue"
  "defer"
  "case"
  "asm"
  "sizeof"
  "alignof"
] @keyword

(visibility) @keyword

; Types
[
  "u8"
  "u16"
  "i8"
  "i16"
  "bool"
  "void"
  "Error"
] @type.builtin

(type_identifier) @type

; Literals
(number_literal) @number
(string_literal) @string
(char_literal) @character
(boolean_literal) @boolean

; Functions
(function_declaration
  name: (identifier) @function)

(call_expression
  function: (identifier) @function.call)

; Fields
(field_declaration
  name: (identifier) @field)

(field_expression
  field: (identifier) @field)

(field_initializer
  name: (identifier) @field)

; Parameters
(parameter
  name: (identifier) @parameter)

; Variables
(identifier) @variable

; Operators
[
  "+"
  "-"
  "*"
  "/"
  "%"
  "="
  "=="
  "!="
  "<"
  ">"
  "<="
  ">="
  "<<"
  ">>"
  "&"
  "|"
  "^"
  "~"
  "!"
  "and"
  "or"
  "?"
  "->"
  "=>"
] @operator

; Punctuation
[
  "("
  ")"
  "["
  "]"
  "{"
  "}"
  ","
  ";"
  ":"
  "."
] @punctuation

; Attributes
(attribute) @attribute

; Special
"_" @variable.builtin