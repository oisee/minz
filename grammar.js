module.exports = grammar({
  name: 'minz',

  extras: $ => [
    /\s/,
    $.comment,
  ],

  conflicts: $ => [
    [$.primary_expression, $.type],
    [$.function_declaration, $.function_type],
    [$.struct_type, $.enum_type],
    [$.declaration, $.statement],
    [$.block_statement, $.primary_expression],
    [$.type_identifier, $.primary_expression],
    [$.case_arm, $.primary_expression],
    [$.array_type, $.array_literal],
  ],

  word: $ => $.identifier,

  rules: {
    source_file: $ => repeat(choice(
      $.import_statement,
      $.declaration,
      $.statement,
    )),

    // Comments
    comment: $ => token(choice(
      seq('//', /.*/),
      seq('/*', /[^*]*\*+([^/*][^*]*\*+)*/, '/'),
    )),

    // Identifiers and literals
    identifier: $ => /[a-zA-Z_][a-zA-Z0-9_]*/,

    // Numbers
    number_literal: $ => choice(
      // Decimal
      /\d+/,
      // Hexadecimal
      /0x[0-9a-fA-F]+/,
      // Binary
      /0b[01]+/,
    ),

    // Strings
    string_literal: $ => seq(
      '"',
      repeat(choice(
        /[^"\\]+/,
        /\\./,
      )),
      '"',
    ),

    // Character literals
    char_literal: $ => seq(
      "'",
      choice(
        /[^'\\]/,
        /\\./,
      ),
      "'",
    ),

    // Boolean literals
    boolean_literal: $ => choice('true', 'false'),

    // Types
    type: $ => choice(
      $.primitive_type,
      $.array_type,
      $.pointer_type,
      $.function_type,
      $.struct_type,
      $.enum_type,
      $.type_identifier,
      $.error_type,
      $.union_type,
    ),

    primitive_type: $ => choice(
      'u8', 'u16', 'i8', 'i16', 'bool', 'void',
    ),

    array_type: $ => choice(
      // Old syntax: [size]type
      seq(
        '[',
        $.expression,
        ']',
        $.type,
      ),
      // New syntax: [type; size]
      seq(
        '[',
        $.type,
        ';',
        $.expression,
        ']',
      ),
    ),

    pointer_type: $ => seq(
      '*',
      optional(choice('mut', 'const')),
      $.type,
    ),

    function_type: $ => seq(
      'fun',
      '(',
      optional($.parameter_list),
      ')',
      $.return_type,
    ),

    return_type: $ => choice(
      seq('->', $.type),
      seq('->', '(', commaSep1($.type), ')'),
    ),

    struct_type: $ => seq(
      'struct',
      '{',
      optional(seq(
        repeat(seq($.field_declaration, ',')),
        $.field_declaration,
        optional(',')
      )),
      '}',
    ),

    enum_type: $ => seq(
      'enum',
      '{',
      seq(
        repeat(seq($.enum_variant, ',')),
        $.enum_variant,
        optional(',')
      ),
      '}',
    ),

    enum_variant: $ => $.identifier,

    type_identifier: $ => $.identifier,

    error_type: $ => 'Error',

    union_type: $ => prec.left(1, seq(
      $.type,
      '|',
      $.type,
    )),

    // Declarations
    declaration: $ => choice(
      $.function_declaration,
      $.variable_declaration,
      $.constant_declaration,
      $.type_alias,
      $.struct_declaration,
      $.enum_declaration,
      $.attributed_declaration,
    ),

    function_declaration: $ => seq(
      optional($.visibility),
      optional('export'),
      'fun',
      $.identifier,
      '(',
      optional($.parameter_list),
      ')',
      $.return_type,
      $.block,
    ),

    parameter_list: $ => commaSep1($.parameter),

    parameter: $ => seq(
      $.identifier,
      ':',
      $.type,
    ),

    variable_declaration: $ => seq(
      choice('let', 'var'),
      optional('mut'),
      $.identifier,
      optional(seq(':', $.type)),
      optional(seq('=', $.expression)),
      ';',
    ),

    constant_declaration: $ => seq(
      'const',
      $.identifier,
      ':',
      $.type,
      '=',
      $.expression,
      ';',
    ),

    type_alias: $ => seq(
      'type',
      $.identifier,
      '=',
      $.type,
      ';',
    ),

    struct_declaration: $ => seq(
      optional($.visibility),
      'struct',
      $.identifier,
      '{',
      optional(seq(
        repeat(seq($.field_declaration, ',')),
        $.field_declaration,
        optional(',')
      )),
      '}',
    ),

    field_declaration: $ => seq(
      optional($.visibility),
      $.identifier,
      ':',
      $.type,
    ),

    enum_declaration: $ => seq(
      optional($.visibility),
      'enum',
      $.identifier,
      '{',
      seq(
        repeat(seq($.enum_variant, ',')),
        $.enum_variant,
        optional(',')
      ),
      '}',
    ),

    visibility: $ => 'pub',

    attributed_declaration: $ => seq(
      $.attribute,
      $.declaration,
    ),

    // Statements
    statement: $ => choice(
      $.expression_statement,
      $.return_statement,
      $.if_statement,
      $.while_statement,
      $.for_statement,
      $.loop_statement,
      $.break_statement,
      $.continue_statement,
      $.block_statement,
      $.variable_declaration,
      $.defer_statement,
      $.case_statement,
    ),

    expression_statement: $ => seq(
      $.expression,
      ';',
    ),

    return_statement: $ => seq(
      'return',
      optional($.expression),
      ';',
    ),

    if_statement: $ => prec.right(seq(
      'if',
      $.expression,
      $.block,
      optional(seq('else', choice($.block, $.if_statement))),
    )),

    while_statement: $ => seq(
      'while',
      $.expression,
      $.block,
    ),

    for_statement: $ => seq(
      'for',
      $.identifier,
      'in',
      $.expression,
      $.block,
    ),

    loop_statement: $ => seq(
      'loop',
      $.block,
    ),

    break_statement: $ => seq(
      'break',
      optional($.expression),
      ';',
    ),

    continue_statement: $ => seq(
      'continue',
      ';',
    ),

    block_statement: $ => $.block,

    defer_statement: $ => seq(
      'defer',
      $.statement,
    ),

    case_statement: $ => seq(
      'case',
      $.expression,
      '{',
      repeat($.case_arm),
      '}',
    ),

    case_arm: $ => seq(
      $.pattern,
      '=>',
      choice(
        $.expression,
        $.block,
      ),
      optional(','),
    ),

    pattern: $ => choice(
      $.identifier,
      $.literal_pattern,
      '_',
    ),

    literal_pattern: $ => choice(
      $.number_literal,
      $.string_literal,
      $.char_literal,
      $.boolean_literal,
    ),

    // Expressions
    expression: $ => choice(
      $.binary_expression,
      $.unary_expression,
      $.postfix_expression,
    ),

    binary_expression: $ => choice(
      // Assignment has lowest precedence and is right-associative
      prec.right(1, seq(
        field('left', $.expression),
        field('operator', '='),
        field('right', $.expression),
      )),
      // Compound assignment operators
      ...['+=', '-=', '*=', '/=', '%='].map(operator => prec.right(1, seq(
        field('left', $.expression),
        field('operator', operator),
        field('right', $.expression),
      ))),
      ...['or', 'and'].map(operator => prec.left(2, seq(
        field('left', $.expression),
        field('operator', operator),
        field('right', $.expression),
      ))),
      ...['==', '!=', '<', '>', '<=', '>='].map(operator => prec.left(3, seq(
        field('left', $.expression),
        field('operator', operator),
        field('right', $.expression),
      ))),
      ...['|', '^', '&'].map(operator => prec.left(4, seq(
        field('left', $.expression),
        field('operator', operator),
        field('right', $.expression),
      ))),
      ...['<<', '>>'].map(operator => prec.left(5, seq(
        field('left', $.expression),
        field('operator', operator),
        field('right', $.expression),
      ))),
      prec.left(5, seq(
        field('left', $.expression),
        field('operator', '..'),
        field('right', $.expression),
      )),
      ...['+', '-'].map(operator => prec.left(6, seq(
        field('left', $.expression),
        field('operator', operator),
        field('right', $.expression),
      ))),
      ...['*', '/', '%'].map(operator => prec.left(7, seq(
        field('left', $.expression),
        field('operator', operator),
        field('right', $.expression),
      ))),
    ),

    unary_expression: $ => prec(8, choice(
      seq('!', $.expression),
      seq('-', $.expression),
      seq('~', $.expression),
      seq('&', optional('mut'), $.expression),
      seq('*', $.expression),
    )),

    postfix_expression: $ => choice(
      $.call_expression,
      $.index_expression,
      $.field_expression,
      $.try_expression,
      $.cast_expression,
      $.primary_expression,
    ),

    call_expression: $ => prec(10, seq(
      field('function', $.postfix_expression),
      '(',
      optional($.argument_list),
      ')',
    )),

    argument_list: $ => commaSep1($.expression),

    index_expression: $ => prec(10, seq(
      field('object', $.postfix_expression),
      '[',
      field('index', $.expression),
      ']',
    )),

    field_expression: $ => prec(10, seq(
      field('object', $.postfix_expression),
      '.',
      field('field', $.identifier),
    )),

    try_expression: $ => prec(10, seq(
      field('expression', $.postfix_expression),
      '?',
    )),

    cast_expression: $ => prec.left(9, seq(
      field('expression', $.expression),
      'as',
      field('type', $.type),
    )),

    primary_expression: $ => choice(
      $.identifier,
      $.number_literal,
      $.string_literal,
      $.char_literal,
      $.boolean_literal,
      $.array_literal,
      $.struct_literal,
      $.tuple_literal,
      $.parenthesized_expression,
      $.block,
      $.inline_assembly,
      $.sizeof_expression,
      $.alignof_expression,
      $.metaprogramming_expression,
      $.error_literal,
    ),

    array_literal: $ => seq(
      '[',
      optional(commaSep1($.expression)),
      ']',
    ),

    struct_literal: $ => seq(
      $.type_identifier,
      '{',
      optional(seq(
        repeat(seq($.field_initializer, ',')),
        $.field_initializer,
        optional(',')
      )),
      '}',
    ),

    field_initializer: $ => seq(
      $.identifier,
      ':',
      $.expression,
    ),

    tuple_literal: $ => seq(
      '(',
      commaSep2($.expression),
      ')',
    ),

    parenthesized_expression: $ => seq(
      '(',
      $.expression,
      ')',
    ),

    block: $ => seq(
      '{',
      repeat($.statement),
      optional($.expression),
      '}',
    ),

    inline_assembly: $ => seq(
      'asm',
      '(',
      $.string_literal,
      optional(seq(
        ':',
        optional($.asm_output_list),
        optional(seq(
          ':',
          optional($.asm_input_list),
          optional(seq(
            ':',
            optional($.asm_clobber_list),
          )),
        )),
      )),
      ')',
    ),

    asm_output_list: $ => commaSep1($.asm_output),
    asm_input_list: $ => commaSep1($.asm_input),
    asm_clobber_list: $ => commaSep1($.string_literal),

    asm_output: $ => seq(
      $.string_literal,
      '(',
      $.identifier,
      ')',
    ),

    asm_input: $ => seq(
      $.string_literal,
      '(',
      $.expression,
      ')',
    ),

    sizeof_expression: $ => seq(
      'sizeof',
      '(',
      $.type,
      ')',
    ),

    alignof_expression: $ => seq(
      'alignof',
      '(',
      $.type,
      ')',
    ),

    error_literal: $ => seq(
      'Error',
      '.',
      $.identifier,
    ),

    // Metaprogramming
    metaprogramming_expression: $ => choice(
      $.compile_time_if,
      $.compile_time_print,
      $.compile_time_assert,
      $.attribute,
      $.lua_block,
      $.lua_expression,
      $.lua_eval,
    ),

    compile_time_if: $ => seq(
      '@if',
      '(',
      $.expression,
      ',',
      $.expression,
      optional(seq(',', $.expression)),
      ')',
    ),

    compile_time_print: $ => seq(
      '@print',
      '(',
      $.string_literal,
      ')',
    ),

    compile_time_assert: $ => seq(
      '@assert',
      '(',
      $.expression,
      optional(seq(',', $.string_literal)),
      ')',
    ),

    attribute: $ => prec.right(seq(
      '@',
      $.identifier,
      optional(seq(
        '(',
        optional($.argument_list),
        ')',
      )),
    )),

    // Lua metaprogramming
    lua_block: $ => seq(
      '@lua',
      '[[',
      $.lua_code,
      ']]',
    ),

    lua_expression: $ => seq(
      '@lua',
      '(',
      $.lua_code,
      ')',
    ),

    lua_eval: $ => seq(
      '@lua_eval',
      '(',
      $.lua_code,
      ')',
    ),

    lua_code: $ => /[^(\]\])]+/,

    // Import statements
    import_statement: $ => seq(
      'import',
      $.import_path,
      optional(seq('as', $.identifier)),
      ';',
    ),

    import_path: $ => sep1($.identifier, '.'),
  }
});

// Helper functions
function commaSep1(rule) {
  return sep1(rule, ',');
}

function commaSep2(rule) {
  return seq(rule, ',', commaSep1(rule));
}

function sep1(rule, separator) {
  return seq(rule, repeat(seq(separator, rule)));
}