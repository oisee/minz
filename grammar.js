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
    [$.array_initializer, $.block],
    [$.array_initializer, $.struct_literal],
    [$.return_type],
    [$.statement, $.block],
    [$.if_expression, $.if_statement],
    [$.ternary_expression, $.if_expression],
    [$.pattern, $.primary_expression],
    [$.literal_pattern, $.primary_expression],
    [$.pattern, $.postfix_expression],
    [$.primary_expression, $.compile_time_if_declaration],
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
      // Decimal with optional fractional part
      /\d+(\.\d+)?/,
      // Hexadecimal
      /0x[0-9a-fA-F]+/,
      // Binary
      /0b[01]+/,
    ),

    // Strings
    string_literal: $ => token(seq(
      optional(/[lL]/),  // Optional l or L prefix for LString
      '"',
      repeat(choice(
        /[^"\\]+/,
        /\\./,
      )),
      '"',
    )),

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
      $.bit_struct_type,
      $.type_identifier,
      $.error_type,
    ),

    primitive_type: $ => choice(
      'u8', 'u16', 'u24', 'i8', 'i16', 'i24', 'bool', 'void',
      'f8.8', 'f.8', 'f.16', 'f16.8', 'f8.16',
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
      choice('fun', 'fn'),  // Consistency!
      '(',
      optional($.parameter_list),
      ')',
      $.return_type,
    ),

    return_type: $ => choice(
      seq('->', $.type, optional(seq('?', $.type_identifier))),  // -> type ? ErrorEnum
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

    bit_struct_type: $ => seq(
      choice('bits', 'bits_8', 'bits_16'),
      '{',
      optional(seq(
        repeat(seq($.bit_field, ',')),
        $.bit_field,
        optional(',')
      )),
      '}',
    ),

    bit_field: $ => seq(
      $.identifier,
      ':',
      $.number_literal,
    ),

    type_identifier: $ => $.identifier,

    error_type: $ => 'Error',

    // Visibility modifiers
    visibility: $ => 'pub',

    // Declarations
    declaration: $ => choice(
      $.function_declaration,
      $.asm_function,
      $.mir_function,
      $.variable_declaration,
      $.constant_declaration,
      $.type_alias,
      $.struct_declaration,
      $.enum_declaration,
      $.interface_declaration,
      $.impl_block,
      $.attributed_declaration,
      $.lua_block,
      $.compile_time_if_declaration,
      $.minz_metafunction_declaration,
      $.minz_block,
      $.mir_block_declaration,
      $.define_template,
      $.meta_execution_block,
    ),

    function_declaration: $ => seq(
      optional($.ctie_directive),  // NEW: CTIE directives for functions
      optional($.visibility),
      optional('export'),
      choice('fun', 'fn'),  // Both work - developer happiness!
      $.identifier,
      optional('?'),  // Optional ? for error-throwing functions
      optional($.generic_parameters),
      '(',
      optional($.parameter_list),
      ')',
      $.return_type,
      $.block,
    ),

    asm_function: $ => seq(
      optional($.visibility),
      optional('export'),
      'asm',
      choice('fun', 'fn'),
      $.identifier,
      '(',
      optional($.parameter_list),
      ')',
      $.return_type,
      $.asm_body,
    ),

    mir_function: $ => seq(
      optional($.visibility),
      optional('export'),
      'mir',
      choice('fun', 'fn'),
      $.identifier,
      '(',
      optional($.parameter_list),
      ')',
      $.return_type,
      $.mir_body,
    ),

    parameter_list: $ => commaSep1($.parameter),

    parameter: $ => choice(
      'self',  // self parameter for methods
      seq(
        $.identifier,
        ':',
        $.type,
      )
    ),

    variable_declaration: $ => seq(
      optional($.visibility),
      choice('let', 'var', 'global'),  // 'global' as developer-friendly synonym
      optional('mut'),
      $.identifier,
      optional(seq(':', $.type)),
      optional(seq('=', $.expression)),
      ';',
    ),

    constant_declaration: $ => seq(
      optional($.visibility),
      'const',
      $.identifier,
      optional(seq(':', $.type)),
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
      optional($.ctie_directive),  // NEW: @derive for structs
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

    enum_variant: $ => seq(
      $.identifier,
      optional(seq('=', $.number_literal))
    ),

    visibility: $ => 'pub',

    interface_declaration: $ => seq(
      optional($.ctie_directive),  // NEW: CTIE directives like @proof
      optional($.visibility),
      'interface',
      $.identifier,
      optional($.generic_parameters),
      '{',
      repeat(choice(
        $.interface_method,
        $.cast_interface_block,
      )),
      '}',
    ),

    interface_method: $ => seq(
      optional($.ctie_directive),  // NEW: CTIE directives like @execute
      choice('fun', 'fn'),  // Flexibility in interfaces too!
      $.identifier,
      '(',
      optional($.parameter_list),
      ')',
      $.return_type,
      ';',
    ),

    // NEW: Cast interface block - simplified first implementation
    cast_interface_block: $ => seq(
      'cast',
      '<',
      $.identifier,  // Simplified: just identifier for now
      '>',
      '{',
      repeat(seq(
        $.identifier,  // From type
        '->',
        '{',
        '}',         // Empty transform for now
      )),
      '}',
    ),

    impl_block: $ => seq(
      'impl',
      $.identifier,  // interface name
      'for',
      $.type,        // implementing type
      '{',
      repeat($.function_declaration),
      '}',
    ),

    generic_parameters: $ => seq(
      '<',
      commaSep1($.generic_parameter),
      '>',
    ),

    generic_parameter: $ => seq(
      $.identifier,
      optional(seq(':', sep1($.identifier, '+')))  // trait bounds
    ),

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
      $.constant_declaration,  // Allow const declarations in functions
      $.function_declaration,  // Allow nested functions!
      $.defer_statement,
      $.case_statement,
      $.asm_block,
      $.compile_time_asm,
      $.mir_block,
      $.minz_block,
      $.target_block,
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
      optional(seq('if', $.expression)),  // Pattern guard
      '=>',
      choice(
        $.expression,
        $.block,
      ),
      optional(','),
    ),

    pattern: $ => choice(
      $.field_expression,
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
      ...['or', 'and', '||', '&&'].map(operator => prec.left(2, seq(
        field('left', $.expression),
        field('operator', operator),
        field('right', $.expression),
      ))),
      // Swift-style nil coalescing operator
      prec.left(2, seq(
        field('left', $.expression),
        field('operator', '??'),
        field('right', $.expression),
      )),
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
      $.enum_access,  // Add support for State::IDLE
      $.number_literal,
      $.string_literal,
      $.char_literal,
      $.boolean_literal,
      $.array_literal,
      $.array_initializer,
      $.struct_literal,
      $.tuple_literal,
      $.parenthesized_expression,
      $.block,
      $.inline_assembly,
      $.sizeof_expression,
      $.alignof_expression,
      $.metaprogramming_expression,
      $.error_literal,
      $.lambda_expression,
      $.if_expression,
      $.ternary_expression,
      $.when_expression,
    ),

    // Enum variant access: State::IDLE
    enum_access: $ => seq(
      field('enum', $.identifier),
      '::',
      field('variant', $.identifier),
    ),

    array_literal: $ => seq(
      '[',
      optional(commaSep1($.expression)),
      ']',
    ),

    array_initializer: $ => seq(
      '{',
      optional(commaSep1($.expression)),
      '}',
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
      repeat(choice(
        $.statement,
        $.function_declaration,  // Allow nested function definitions!
      )),
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
      $.compile_time_error,
      $.attribute,
      $.lua_expression,
      $.lua_eval,
      $.compile_time_minz,
      $.compile_time_mir,
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
      $.string_literal,  // Only accepts a single string with { } interpolation
      ')',
    ),

    compile_time_assert: $ => seq(
      '@assert',
      '(',
      $.expression,  // Flexible - can be string, expression, multiple params
      repeat(seq(',', $.expression)),  // Optional additional parameters
      ')',
    ),

    compile_time_error: $ => prec.right(seq(
      '@error',
      optional(seq('(', optional($.expression), ')')),
    )),

    compile_time_asm: $ => seq(
      '@asm',
      '{',
      optional($.asm_content),
      '}',
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
      '[[[',
      $.lua_code_block,
      ']]]',
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

    lua_code: $ => /[^)]+/,
    
    // Lua code block that can contain anything including [[ ]]
    lua_code_block: $ => /([^\]]+|\][^\]]+|\]\][^\]]+)*/,

    // MinZ metaprogramming
    compile_time_minz: $ => seq(
      '@minz',
      '[[[',
      $.minz_code_block,
      ']]]',
      '(',
      optional($.argument_list),
      ')',
    ),

    // MinZ code block that can contain anything including [[ ]]
    minz_code_block: $ => /([^\]]+|\][^\]]+|\]\][^\]]+)*/,

    // MIR metaprogramming (MIR = MIR Intermediate Representation 😄)
    compile_time_mir: $ => seq(
      '@mir',
      '[[[',
      $.mir_code_block,
      ']]]',
    ),

    // MIR code block that can contain anything including [[ ]]
    mir_code_block: $ => prec(2, alias(/([^\]]+|\][^\]]+|\]\][^\]]+)*/, 'mir_code_content')),

    // CTIE (Compile-Time Interface Execution) directives
    ctie_directive: $ => choice(
      $.execute_directive,
      $.specialize_directive,
      $.proof_directive,
      $.derive_directive,
      $.analyze_usage_directive,
      $.compile_time_vtable_directive,
    ),

    execute_directive: $ => seq(
      '@execute',
      optional(seq('when', $.execute_condition)),
    ),

    execute_condition: $ => choice(
      'const',  // Execute when inputs are const
      $.expression,  // Custom condition
    ),

    specialize_directive: $ => seq(
      '@specialize',
      optional(seq(
        'for',
        '[',
        commaSep1($.string_literal),  // Type names
        ']',
        optional(seq('threshold:', $.number_literal)),
      )),
    ),

    proof_directive: $ => seq(
      '@proof',
      '{',
      repeat1($.proof_invariant),
      '}',
    ),

    proof_invariant: $ => seq(
      $.identifier,
      ':',
      $.expression,
      optional(','),
    ),

    derive_directive: $ => seq(
      '@derive',
      '(',
      $.identifier,  // Interface name
      ')',
      optional(seq(
        'for',
        $.identifier,  // Type name
        optional($.derive_options),
      )),
    ),

    derive_options: $ => seq(
      '{',
      repeat1($.derive_option),
      '}',
    ),

    derive_option: $ => seq(
      $.identifier,
      ':',
      choice(
        $.identifier,
        $.string_literal,
        '[',
        commaSep1($.string_literal),
        ']',
      ),
      optional(','),
    ),

    analyze_usage_directive: $ => seq(
      '@analyze_usage',
      '{',
      repeat1($.usage_rule),
      '}',
    ),

    usage_rule: $ => seq(
      'if',
      $.expression,
      '->',
      $.ctie_directive,
      optional(','),
    ),

    compile_time_vtable_directive: $ => seq(
      '@compile_time_vtable',
      '{',
      repeat1($.vtable_rule),
      '}',
    ),

    vtable_rule: $ => seq(
      'when',
      $.expression,
      '->',
      $.identifier,  // Strategy name
      optional(','),
    ),

    // Import statements
    import_statement: $ => seq(
      'import',
      $.import_path,
      optional(seq('as', $.identifier)),
      ';',
    ),

    import_path: $ => sep1($.identifier, '.'),

    // Lambda expressions  
    lambda_expression: $ => prec.right(1, choice(
      // Traditional pipe syntax: |x| expr
      seq(
        '|',
        optional($.lambda_parameter_list),
        '|',
        choice(
          $.expression,                    // |x| x + 1
          seq('=>', $.type, $.block),     // |x| => u8 { x + 1 }
          $.block,                         // |x| { x + 1 }
        ),
      ),
    )),

    lambda_parameter_list: $ => commaSep1($.lambda_parameter),

    lambda_parameter: $ => seq(
      $.identifier,
      optional(seq(':', $.type)),
    ),

    // If expression (returns a value)
    if_expression: $ => prec.right(seq(
      'if',
      field('condition', $.expression),
      field('then_branch', $.block),
      'else',
      field('else_branch', choice($.block, $.if_expression)),
    )),

    // Python-style conditional expression (value_if_true if condition else value_if_false)
    ternary_expression: $ => prec.right(3, seq(
      field('true_expr', $.expression),
      'if',
      field('condition', $.expression),
      'else',
      field('false_expr', $.expression),
    )),

    // When expression (pattern matching with guards)
    when_expression: $ => seq(
      'when',
      optional(field('value', $.expression)),
      '{',
      repeat1($.when_arm),
      '}',
    ),

    when_arm: $ => seq(
      choice(
        field('pattern', $.expression),  // Pattern or condition
        'else',
      ),
      optional(seq('if', field('guard', $.expression))),  // Guard condition
      '=>',
      field('body', $.expression),
      optional(','),
    ),

    // Compile-time if declaration (top-level @if)
    compile_time_if_declaration: $ => seq(
      '@if',
      '(',
      field('condition', $.expression),
      ',',
      field('then_code', $.string_literal),
      optional(seq(',', field('else_code', $.string_literal))),
      ')',
    ),

    // MinZ metafunction declaration (top-level @minz)
    minz_metafunction_declaration: $ => seq(
      '@minz',
      '(',
      field('template', $.string_literal),
      repeat(seq(',', field('argument', $.expression))),
      ')',
    ),

    // MinZ compile-time execution block
    minz_block: $ => seq(
      '@minz',
      '[[[',
      field('code', $.minz_raw_block),
      ']]]',
    ),

    // MIR block declaration (top-level @mir)
    mir_block_declaration: $ => seq(
      '@mir',
      '[[[',
      field('code', $.mir_block_content),
      ']]]',
    ),

    minz_block_content: $ => repeat1(choice(
      $.minz_emit,
      $.statement,
      $.expression,
    )),

    minz_emit: $ => prec(30, seq(
      '@emit',
      '(',
      $.expression,
      ')',
    )),

    mir_block_content: $ => prec(1, alias(/([^\]]+|\][^\]]+|\]\][^\]]+)*/, 'mir_block_text')),  // MIR code block content
    
    // Raw MinZ code block that can contain anything including [[ ]]
    minz_raw_block: $ => prec(2, /([^\]]+|\][^\]]+|\]\][^\]]+)*/),

    // @define template system
    define_template: $ => choice(
      // Template definition
      seq(
        '@define',
        '(',
        field('parameters', $.identifier_list),
        ')',
        '[[[',
        field('body', $.template_body),
        ']]]',
      ),
      // Template invocation
      seq(
        '@define',
        '(',
        field('arguments', $.expression_list),
        ')',
      ),
    ),

    // @lang[[[]]] execution blocks
    meta_execution_block: $ => choice(
      $.lua_execution_block,
      $.minz_execution_block,
      $.mir_execution_block,
    ),

    lua_execution_block: $ => seq(
      '@lua',
      '[[[',
      field('code', $.raw_block_content),
      ']]]',
    ),

    minz_execution_block: $ => seq(
      '@minz',
      '[[[',
      field('code', $.raw_block_content),
      ']]]',
    ),

    mir_execution_block: $ => seq(
      '@mir',
      '[[[',
      field('code', $.raw_block_content),
      ']]]',
    ),

    template_body: $ => /([^\]]|\][^\]]|\]\][^\]])+/,
    raw_block_content: $ => /([^\]]|\][^\]]|\]\][^\]])+/,

    identifier_list: $ => prec(2, seq(
      $.identifier,
      repeat(seq(',', $.identifier)),
    )),

    expression_list: $ => prec(1, seq(
      $.expression,
      repeat(seq(',', $.expression)),
    )),

    // Assembly and MIR blocks
    asm_block: $ => seq(
      'asm',
      '{',
      optional($.asm_content),
      '}',
    ),

    mir_block: $ => seq(
      'mir',
      '{',
      optional($.mir_content),
      '}',
    ),

    asm_body: $ => seq(
      '{',
      $.asm_raw_content,
      '}',
    ),

    mir_body: $ => seq(
      '{',
      $.mir_raw_content,
      '}',
    ),

    asm_raw_content: $ => /[^{}]*/,
    
    mir_raw_content: $ => /[^{}]*/,

    asm_content: $ => repeat1(choice(
      $.asm_label,
      $.asm_instruction,
      $.comment,
    )),

    mir_content: $ => repeat1(choice(
      $.mir_instruction,
      $.mir_label,
      $.comment,
    )),

    asm_label: $ => seq(
      $.identifier,
      ':',
    ),

    asm_instruction: $ => /[^\n{}]+/,

    mir_instruction: $ => prec.left(seq(
      choice(
        'load', 'store', 'move',
        'add', 'sub', 'mul', 'div', 'mod',
        'and', 'or', 'xor', 'not',
        'shl', 'shr', 'rol', 'ror',
        'jump', 'call', 'return',
        'push', 'pop',
        'inc', 'dec', 'neg',
        'nop', 'halt', 'syscall',
        /load\.[ui](8|16)/,
        /store\.[ui](8|16)/,
        /cast\.[ui](8|16)/,
        /jump\.(z|nz|eq|ne|lt|gt|le|ge)/,
        'smc.patch', 'phi',
        'push.all', 'pop.all',
      ),
      optional($.mir_operands),
    )),

    mir_label: $ => seq(
      $.identifier,
      ':',
    ),

    mir_operands: $ => commaSep1($.mir_operand),

    mir_operand: $ => choice(
      $.mir_register,
      $.mir_memory,
      $.mir_immediate,
      $.identifier,  // Label or function name
    ),

    mir_register: $ => /r[0-9]+|v[0-9]+/,

    mir_memory: $ => seq(
      '[',
      choice(
        $.mir_register,
        $.identifier,
        $.mir_immediate,
      ),
      ']',
    ),

    mir_immediate: $ => seq('#', $.number_literal),
    
    target_block: $ => seq(
      '@target',
      '(',
      $.string_literal,  // Target name: "z80", "6502", "wasm", etc.
      ')',
      $.block,
    ),
  }
});

// Helper functions
function commaSep(rule) {
  return optional(commaSep1(rule));
}

function commaSep1(rule) {
  return sep1(rule, ',');
}

function commaSep2(rule) {
  return seq(rule, ',', commaSep1(rule));
}

function sep1(rule, separator) {
  return seq(rule, repeat(seq(separator, rule)));
}