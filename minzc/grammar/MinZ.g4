grammar MinZ;

// ============================================================================
// Parser Rules
// ============================================================================

program
    : (importDecl | declaration)* EOF
    ;

// Import declarations
importDecl
    : 'import' importPath ('as' alias=IDENTIFIER)? ';'
    ;

importPath
    : IDENTIFIER ('.' IDENTIFIER)*
    ;

// Top-level declarations
declaration
    : functionDecl
    | structDecl
    | interfaceDecl
    | enumDecl
    | constDecl
    | globalDecl
    | typeAlias
    | metafunction
    ;

// Function declaration
functionDecl
    : 'pub'? ('fun' | 'fn') name=IDENTIFIER 
      genericParams?
      '(' parameterList? ')' 
      returnType?
      functionBody
    ;

genericParams
    : '<' IDENTIFIER (',' IDENTIFIER)* '>'
    ;

returnType
    : '->' type errorType?
    ;

errorType
    : '?'
    ;

functionBody
    : block
    | ';'  // forward declaration
    ;

parameterList
    : parameter (',' parameter)*
    ;

parameter
    : 'self'                          // self parameter
    | name=IDENTIFIER ':' type       // regular parameter
    ;

// Struct declaration
structDecl
    : 'pub'? 'struct' name=IDENTIFIER '{' structField* '}'
    ;

structField
    : name=IDENTIFIER ':' type ','?
    ;

// Interface declaration
interfaceDecl
    : 'pub'? 'interface' name=IDENTIFIER '{' methodSignature* '}'
    ;

methodSignature
    : ('fun' | 'fn') name=IDENTIFIER '(' parameterList? ')' returnType? ';'?
    ;

// Enum declaration
enumDecl
    : 'pub'? 'enum' name=IDENTIFIER '{' enumVariant (',' enumVariant)* ','? '}'
    ;

enumVariant
    : IDENTIFIER ('=' INTEGER)?
    ;

// Const declaration
constDecl
    : 'pub'? 'const' name=IDENTIFIER ':' type '=' expression ';'
    ;

// Global variable declaration
globalDecl
    : 'pub'? 'global' name=IDENTIFIER ':' type ('=' expression)? ';'
    ;

// Type alias
typeAlias
    : 'pub'? 'type' name=IDENTIFIER '=' type ';'
    ;

// Metafunction calls
metafunction
    : '@' name=IDENTIFIER '(' expressionList? ')' ';'?
    ;

// ============================================================================
// Types
// ============================================================================

type
    : primitiveType
    | namedType
    | arrayType
    | pointerType
    | errorableType
    | mutableType
    | iteratorType
    ;

primitiveType
    : 'u8' | 'u16' | 'u24' | 'u32'
    | 'i8' | 'i16' | 'i24' | 'i32'
    | 'bool' | 'void'
    | 'f8.8' | 'f.8' | 'f16.8' | 'f8.16'
    ;

namedType
    : IDENTIFIER ('.' IDENTIFIER)*
    ;

arrayType
    : type '[' arraySize? ']'
    ;

arraySize
    : INTEGER
    | IDENTIFIER
    ;

pointerType
    : '*' type
    | '*' 'mut' type
    ;

errorableType
    : type '?'
    ;

mutableType
    : 'mut' type
    ;

iteratorType
    : 'Iterator' '<' type '>'
    ;

// ============================================================================
// Statements
// ============================================================================

statement
    : letStatement
    | ifStatement
    | whileStatement
    | forStatement
    | matchStatement
    | returnStatement
    | breakStatement
    | continueStatement
    | deferStatement
    | block
    | assignmentStatement
    | expressionStatement
    | asmBlock
    ;

letStatement
    : 'let' 'mut'? name=IDENTIFIER (':' type)? ('=' initializer=expression)? ';'
    ;

ifStatement
    : 'if' condition=expression thenBlock=block 
      ('else' 'if' elseIfCondition=expression elseIfBlock=block)*
      ('else' elseBlock=block)?
    ;

whileStatement
    : 'while' condition=expression body=block
    ;

forStatement
    : 'for' iterator=IDENTIFIER 'in' iterable=expression body=block
    ;

matchStatement
    : 'match' expression '{' matchArm* '}'
    ;

matchArm
    : pattern '=>' (expression ';' | block) ','?
    ;

pattern
    : '_'                             // wildcard
    | literal                         // literal pattern
    | IDENTIFIER                      // variable binding
    | enumPattern                     // enum variant
    ;

enumPattern
    : IDENTIFIER '::' IDENTIFIER
    ;

returnStatement
    : 'return' expression? ';'
    ;

breakStatement
    : 'break' ';'
    ;

continueStatement
    : 'continue' ';'
    ;

deferStatement
    : 'defer' (expression ';' | block)
    ;

assignmentStatement
    : assignmentTarget assignmentOp expression ';'
    ;

assignmentTarget
    : IDENTIFIER
    | expression '.' IDENTIFIER
    | expression '[' expression ']'
    ;

assignmentOp
    : '=' | '+=' | '-=' | '*=' | '/=' | '%=' 
    | '&=' | '|=' | '^=' | '<<=' | '>>='
    ;

expressionStatement
    : expression ';'
    ;

block
    : '{' statement* '}'
    ;

asmBlock
    : '@asm' '{' ASM_CODE '}'
    ;

// ============================================================================
// Expressions
// ============================================================================

expression
    : primaryExpression                                    # Primary
    | expression '(' argumentList? ')'                     # Call
    | expression '.' member=IDENTIFIER                     # MemberAccess
    | expression '[' index=expression ']'                  # IndexAccess
    | expression '?'                                        # ErrorCheck
    | expression '??' defaultValue=expression              # ErrorDefault
    | expression 'as' targetType=type                      # Cast
    | op=('!' | '-' | '~') expression                      # Unary
    | expression op=('*' | '/' | '%') expression           # Multiplicative
    | expression op=('+' | '-') expression                 # Additive
    | expression op=('<<' | '>>') expression               # Shift
    | expression op=('<' | '<=' | '>' | '>=') expression   # Relational
    | expression op=('==' | '!=') expression               # Equality
    | expression op='&' expression                         # BitwiseAnd
    | expression op='^' expression                         # BitwiseXor
    | expression op='|' expression                         # BitwiseOr
    | expression op='&&' expression                        # LogicalAnd
    | expression op='||' expression                        # LogicalOr
    | lambdaExpression                                     # Lambda
    | metafunctionExpr                                     # MetafunctionCall
    ;

primaryExpression
    : literal
    | IDENTIFIER
    | 'self'
    | '(' expression ')'
    | arrayLiteral
    | structLiteral
    ;

literal
    : INTEGER
    | HEX_INTEGER
    | BINARY_INTEGER
    | FLOAT
    | STRING
    | CHAR
    | 'true'
    | 'false'
    | 'null'
    ;

arrayLiteral
    : '[' expressionList? ']'
    ;

structLiteral
    : IDENTIFIER '{' fieldInitializer (',' fieldInitializer)* ','? '}'
    ;

fieldInitializer
    : IDENTIFIER ':' expression
    ;

lambdaExpression
    : '|' lambdaParams? '|' ('=>' type)? (expression | block)
    ;

lambdaParams
    : lambdaParam (',' lambdaParam)*
    ;

lambdaParam
    : IDENTIFIER (':' type)?
    ;

metafunctionExpr
    : '@' name=IDENTIFIER '(' expressionList? ')'
    ;

argumentList
    : expression (',' expression)*
    ;

expressionList
    : expression (',' expression)*
    ;

// ============================================================================
// Lexer Rules
// ============================================================================

// Keywords
IMPORT: 'import';
AS: 'as';
FUN: 'fun';
FN: 'fn';
STRUCT: 'struct';
INTERFACE: 'interface';
ENUM: 'enum';
CONST: 'const';
GLOBAL: 'global';
TYPE: 'type';
LET: 'let';
MUT: 'mut';
IF: 'if';
ELSE: 'else';
WHILE: 'while';
FOR: 'for';
IN: 'in';
MATCH: 'match';
RETURN: 'return';
BREAK: 'break';
CONTINUE: 'continue';
DEFER: 'defer';
PUB: 'pub';
SELF: 'self';
TRUE: 'true';
FALSE: 'false';
NULL: 'null';

// Primitive types
U8: 'u8';
U16: 'u16';
U24: 'u24';
U32: 'u32';
I8: 'i8';
I16: 'i16';
I24: 'i24';
I32: 'i32';
BOOL: 'bool';
VOID: 'void';

// Identifiers and literals
IDENTIFIER
    : [a-zA-Z_][a-zA-Z0-9_]*
    ;

INTEGER
    : [0-9]+
    ;

HEX_INTEGER
    : '0x' [0-9a-fA-F]+
    ;

BINARY_INTEGER
    : '0b' [01]+
    ;

FLOAT
    : [0-9]+ '.' [0-9]+
    ;

STRING
    : '"' (~["\r\n\\] | '\\' .)* '"'
    ;

CHAR
    : '\'' (~['\r\n\\] | '\\' .) '\''
    ;

// Assembly code block (captures everything between @asm { ... })
ASM_CODE
    : (~[{}] | '{' ASM_CODE '}')*
    ;

// Comments and whitespace
LINE_COMMENT
    : '//' ~[\r\n]* -> skip
    ;

BLOCK_COMMENT
    : '/*' .*? '*/' -> skip
    ;

WS
    : [ \t\r\n]+ -> skip
    ;

// Operators and punctuation
ARROW: '->';
DOUBLE_ARROW: '=>';
QUESTION: '?';
DOUBLE_QUESTION: '??';
DOT: '.';
COMMA: ',';
SEMICOLON: ';';
COLON: ':';
DOUBLE_COLON: '::';
LPAREN: '(';
RPAREN: ')';
LBRACE: '{';
RBRACE: '}';
LBRACKET: '[';
RBRACKET: ']';
LT: '<';
GT: '>';
LE: '<=';
GE: '>=';
EQ: '==';
NE: '!=';
ASSIGN: '=';
PLUS_ASSIGN: '+=';
MINUS_ASSIGN: '-=';
STAR_ASSIGN: '*=';
DIV_ASSIGN: '/=';
MOD_ASSIGN: '%=';
AND_ASSIGN: '&=';
OR_ASSIGN: '|=';
XOR_ASSIGN: '^=';
SHL_ASSIGN: '<<=';
SHR_ASSIGN: '>>=';
PLUS: '+';
MINUS: '-';
STAR: '*';
DIV: '/';
MOD: '%';
AND: '&';
OR: '|';
XOR: '^';
NOT: '!';
TILDE: '~';
LOGICAL_AND: '&&';
LOGICAL_OR: '||';
SHL: '<<';
SHR: '>>';
AT: '@';
UNDERSCORE: '_';