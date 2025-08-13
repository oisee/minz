grammar MinZ;

// Parser Rules
sourceFile
    : (importStatement | declaration | statement)* EOF
    ;

// Import statements
importStatement
    : 'import' importPath ('as' alias=IDENTIFIER)? ';'
    ;

importPath
    : stringLiteral
    | IDENTIFIER ('.' IDENTIFIER)*
    ;

// Declarations
declaration
    : functionDeclaration
    | structDeclaration
    | enumDeclaration
    | typeAliasDeclaration
    | interfaceDeclaration
    | implBlock
    | constDeclaration
    | globalVarDeclaration
    | compileTimeDeclaration
    ;

// Function declarations
functionDeclaration
    : visibility? functionPrefix IDENTIFIER genericParams? '(' parameterList? ')' returnType? errorReturnType? block
    ;

functionPrefix
    : 'fun'
    | 'fn'
    | 'asm' 'fun'
    | 'mir' 'fun'
    ;

visibility
    : 'pub'
    ;

genericParams
    : '<' IDENTIFIER (',' IDENTIFIER)* '>'
    ;

parameterList
    : parameter (',' parameter)*
    ;

parameter
    : IDENTIFIER ':' type
    ;

returnType
    : '->' type
    ;

errorReturnType
    : '?' type?
    ;

// Struct declarations
structDeclaration
    : visibility? 'struct' IDENTIFIER genericParams? '{' fieldList? '}'
    ;

fieldList
    : field (',' field)* ','?
    ;

field
    : IDENTIFIER ':' type
    ;

// Enum declarations
enumDeclaration
    : visibility? 'enum' IDENTIFIER genericParams? '{' enumMemberList? '}'
    ;

enumMemberList
    : enumMember (',' enumMember)* ','?
    ;

enumMember
    : IDENTIFIER ('=' expression)?
    ;

// Type alias
typeAliasDeclaration
    : 'type' IDENTIFIER '=' type ';'
    ;

// Interface declarations
interfaceDeclaration
    : visibility? 'interface' IDENTIFIER genericParams? '{' interfaceMethodList? '}'
    ;

interfaceMethodList
    : interfaceMethod*
    ;

interfaceMethod
    : IDENTIFIER '(' parameterList? ')' returnType? errorReturnType? ';'
    ;

// Impl blocks
implBlock
    : 'impl' type 'for' type '{' functionDeclaration* '}'
    ;

// Constants and globals
constDeclaration
    : visibility? 'const' IDENTIFIER ':' type '=' expression ';'
    ;

globalVarDeclaration
    : visibility? 'global' IDENTIFIER ':' type ('=' expression)? ';'
    ;

// Compile-time declarations
compileTimeDeclaration
    : compileTimeIf
    | compileTimeMinz
    | compileTimeMir
    | targetBlock
    ;

compileTimeIf
    : '@if' '(' expression ')' block ('else' block)?
    ;

compileTimeMinz
    : '@minz' '(' stringLiteral (',' expression)* ')'
    ;

compileTimeMir
    : '@mir' mirBlock
    ;

mirBlock
    : '{' mirStatement* '}'
    ;

mirStatement
    : mirInstruction ';'
    ;

mirInstruction
    : IDENTIFIER mirOperand*
    ;

mirOperand
    : mirRegister
    | mirImmediate
    | mirMemory
    | mirLabel
    ;

mirRegister
    : 'r' NUMBER
    ;

mirImmediate
    : '#' NUMBER
    ;

mirMemory
    : '[' expression ']'
    ;

mirLabel
    : IDENTIFIER ':'
    ;

targetBlock
    : '@target' '(' stringLiteral ')' block
    ;

// Statements
statement
    : letStatement
    | varStatement
    | assignmentStatement
    | expressionStatement
    | returnStatement
    | ifStatement
    | whileStatement
    | forStatement
    | loopStatement
    | caseStatement
    | blockStatement
    | breakStatement
    | continueStatement
    | deferStatement
    | asmStatement
    ;

letStatement
    : 'let' 'mut'? IDENTIFIER (':' type)? '=' expression ';'
    ;

varStatement
    : 'var' IDENTIFIER (':' type)? '=' expression ';'
    ;

assignmentStatement
    : expression '=' expression ';'
    ;

expressionStatement
    : expression ';'
    ;

returnStatement
    : 'return' expression? ';'
    ;

ifStatement
    : 'if' expression block ('else' (ifStatement | block))?
    ;

whileStatement
    : 'while' expression block
    ;

forStatement
    : 'for' IDENTIFIER 'in' expression block
    ;

loopStatement
    : 'loop' block
    ;

caseStatement
    : 'case' expression '{' caseArm* '}'
    ;

caseArm
    : pattern '=>' (block | expression ','?)
    ;

blockStatement
    : block
    ;

block
    : '{' statement* '}'
    ;

breakStatement
    : 'break' ';'
    ;

continueStatement
    : 'continue' ';'
    ;

deferStatement
    : 'defer' (block | expression ';')
    ;

asmStatement
    : 'asm' asmBlock
    ;

asmBlock
    : '{' (~'}')* '}'
    ;

// Patterns (for pattern matching)
pattern
    : literalPattern
    | identifierPattern
    | wildcardPattern
    | tuplePattern
    | structPattern
    ;

literalPattern
    : literal
    ;

identifierPattern
    : IDENTIFIER
    ;

wildcardPattern
    : '_'
    ;

tuplePattern
    : '(' pattern (',' pattern)* ')'
    ;

structPattern
    : IDENTIFIER '{' fieldPattern (',' fieldPattern)* '}'
    ;

fieldPattern
    : IDENTIFIER ':' pattern
    ;

// Expressions - fixed precedence
expression
    : lambdaExpression
    | conditionalExpression
    ;

lambdaExpression
    : '|' lambdaParams? '|' ('=>' type)? block
    | conditionalExpression
    ;

lambdaParams
    : lambdaParam (',' lambdaParam)*
    ;

lambdaParam
    : IDENTIFIER (':' type)?
    ;

conditionalExpression
    : whenExpression
    | logicalOrExpression ('?' expression ':' expression)?  // Ternary
    | 'if' logicalOrExpression 'then' expression 'else' expression  // If expression
    ;

whenExpression
    : 'when' expression '{' whenArm* '}'
    ;

whenArm
    : pattern ('if' expression)? '=>' expression ','?
    ;

logicalOrExpression
    : logicalAndExpression (('||' | 'or') logicalAndExpression)*
    ;

logicalAndExpression
    : equalityExpression (('&&' | 'and') equalityExpression)*
    ;

equalityExpression
    : relationalExpression (('==' | '!=') relationalExpression)*
    ;

relationalExpression
    : additiveExpression (('<' | '>' | '<=' | '>=') additiveExpression)*
    ;

additiveExpression
    : multiplicativeExpression (('+' | '-') multiplicativeExpression)*
    ;

multiplicativeExpression
    : castExpression (('*' | '/' | '%') castExpression)*
    ;

castExpression
    : unaryExpression ('as' type)?
    ;

unaryExpression
    : ('!' | '-' | '~' | '&' | '*') unaryExpression
    | postfixExpression
    ;

postfixExpression
    : primaryExpression postfixOperator*
    ;

postfixOperator
    : '[' expression ']'                    // Array index
    | '.' IDENTIFIER                        // Field access
    | '(' argumentList? ')'                 // Function call
    | '?'                                    // Try operator
    | '??'                                   // Nil coalescing
    | '.iter()'                             // Iterator
    | '.map' '(' lambdaExpression ')'      // Map
    | '.filter' '(' lambdaExpression ')'   // Filter
    | '.forEach' '(' lambdaExpression ')'  // ForEach
    ;

argumentList
    : expression (',' expression)*
    ;

primaryExpression
    : literal
    | qualifiedIdentifier
    | IDENTIFIER
    | '(' expression ')'
    | arrayLiteral
    | structLiteral
    | metafunction
    | inlineAssembly
    ;

qualifiedIdentifier
    : IDENTIFIER '::' IDENTIFIER
    ;

literal
    : numberLiteral
    | stringLiteral
    | charLiteral
    | booleanLiteral
    ;

numberLiteral
    : NUMBER
    | HEX_NUMBER
    | BINARY_NUMBER
    ;

stringLiteral
    : STRING
    | LSTRING
    ;

charLiteral
    : CHAR
    ;

booleanLiteral
    : 'true'
    | 'false'
    ;

arrayLiteral
    : '[' (expression (',' expression)*)? ']'
    ;

structLiteral
    : IDENTIFIER '{' (fieldInit (',' fieldInit)*)? '}'
    ;

fieldInit
    : IDENTIFIER ':' expression
    ;

// Metafunctions
metafunction
    : '@print' '(' expression (',' expression)* ')'
    | '@assert' '(' expression (',' expression)? ')'
    | '@error' '(' stringLiteral ')'
    | '@abi' '(' stringLiteral ')'
    | '@lua' luaBlock
    | '@lua_eval' '(' expression ')'
    | '@define' '(' IDENTIFIER ',' expression ')'
    | '@include' '(' stringLiteral ')'
    | '@log' '.' logLevel '(' expression (',' expression)* ')'  // Direct pattern
    | '@log' '(' expression (',' expression)* ')'                // Default @log
    ;

logLevel
    : 'out'
    | 'debug'
    | 'info'
    | 'warn'
    | 'error'
    | 'trace'
    ;

luaBlock
    : LUA_BLOCK
    ;

// Inline assembly
inlineAssembly
    : 'asm' '(' stringLiteral (',' asmOperand)* ')'
    ;

asmOperand
    : ':' '"' asmConstraint '"' '(' expression ')'
    ;

asmConstraint
    : IDENTIFIER
    ;

// Types
type
    : primitiveType
    | arrayType
    | pointerType
    | functionType
    | structType
    | enumType
    | bitStructType
    | typeIdentifier
    | errorType
    ;

primitiveType
    : 'u8' | 'u16' | 'u24' | 'i8' | 'i16' | 'i24' | 'bool' | 'void'
    | 'f8.8' | 'f.8' | 'f.16' | 'f16.8' | 'f8.16'
    ;

arrayType
    : '[' type ';' expression ']'
    | '[' type ']'
    ;

pointerType
    : '*' 'const'? type
    | '*' 'mut' type
    ;

functionType
    : 'fn' '(' typeList? ')' returnType?
    ;

typeList
    : type (',' type)*
    ;

structType
    : 'struct' '{' fieldList? '}'
    ;

enumType
    : 'enum' '{' enumMemberList? '}'
    ;

bitStructType
    : 'bitstruct' '{' bitFieldList? '}'
    ;

bitFieldList
    : bitField (',' bitField)* ','?
    ;

bitField
    : IDENTIFIER ':' NUMBER
    ;

typeIdentifier
    : IDENTIFIER ('::' IDENTIFIER)*
    ;

errorType
    : primitiveType '?'
    | arrayType '?'
    | pointerType '?'
    | functionType '?'
    | structType '?'
    | enumType '?'
    | bitStructType '?'
    | typeIdentifier '?'
    ;

// Lexer Rules
IDENTIFIER
    : [a-zA-Z_][a-zA-Z0-9_]*
    ;

NUMBER
    : [0-9]+ ('.' [0-9]+)?
    ;

HEX_NUMBER
    : '0x' [0-9a-fA-F]+
    ;

BINARY_NUMBER
    : '0b' [01]+
    ;

STRING
    : '"' (~["\\\r\n] | '\\' .)* '"'
    ;

LSTRING
    : [lL] '"' (~["\\\r\n] | '\\' .)* '"'
    ;

CHAR
    : '\'' (~['\\\r\n] | '\\' .) '\''
    ;

LUA_BLOCK
    : '[[[' .*? ']]]'
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