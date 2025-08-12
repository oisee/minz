# ANTLR Parser Migration Research for MinZ

## Executive Summary
Research into migrating MinZ from tree-sitter to ANTLR4 for better embedding and reliability.

## Current Issues with Tree-Sitter

### Problems
1. **External Dependency** - Requires tree-sitter CLI installed separately
2. **Grammar Files** - Need grammar.js and generated C files accessible
3. **Platform Issues** - Different behavior across OS platforms
4. **Distribution** - Can't embed grammar in binary easily
5. **User Experience** - Installation failures for new users

### Current Flow
```
MinZ Source → tree-sitter CLI → S-expression → Go AST → Compilation
```

## ANTLR4 Alternative

### Advantages
1. **Pure Go Runtime** - ANTLR4 has native Go target
2. **Embedded Grammar** - Grammar compiles to Go code
3. **No External Dependencies** - Everything in the binary
4. **Better Error Messages** - Built-in error recovery and reporting
5. **IDE Support** - Better tooling and debugging
6. **Cross-Platform** - Consistent behavior everywhere

### Proposed Flow
```
MinZ Source → ANTLR4 Go Parser → Go AST → Compilation
```

## Implementation Plan

### Phase 1: Grammar Translation
Convert MinZ grammar from tree-sitter format to ANTLR4:

```antlr
// MinZ.g4
grammar MinZ;

// Parser Rules
program: (importDecl | declaration)* EOF;

importDecl: 'import' importPath ('as' IDENTIFIER)? ';';
importPath: IDENTIFIER ('.' IDENTIFIER)*;

declaration
    : functionDecl
    | structDecl
    | interfaceDecl
    | enumDecl
    | constDecl
    | globalDecl
    | typeAlias
    ;

functionDecl
    : 'pub'? ('fun' | 'fn') IDENTIFIER 
      '(' parameterList? ')' 
      ('->' type)?
      block
    ;

parameterList: parameter (',' parameter)*;
parameter: IDENTIFIER ':' type;

type
    : primitiveType
    | IDENTIFIER
    | type '[' expression? ']'  // array
    | type '?'                   // error type
    | 'mut' type                 // mutable
    ;

primitiveType
    : 'u8' | 'u16' | 'u24' 
    | 'i8' | 'i16' | 'i24'
    | 'bool' | 'void'
    | 'f8.8' | 'f16.8' | 'f8.16'
    ;

// Statements
statement
    : letStatement
    | ifStatement
    | whileStatement
    | forStatement
    | returnStatement
    | breakStatement
    | continueStatement
    | expressionStatement
    | assignmentStatement
    ;

letStatement: 'let' 'mut'? IDENTIFIER (':' type)? ('=' expression)? ';';
returnStatement: 'return' expression? ';';
breakStatement: 'break' ';';
continueStatement: 'continue' ';';

// Expressions
expression
    : primaryExpression
    | expression '(' argumentList? ')'        // call
    | expression '.' IDENTIFIER               // member
    | expression '[' expression ']'           // index
    | expression '?'                          // error check
    | expression '??' expression              // error default
    | expression binaryOp expression          // binary
    | unaryOp expression                      // unary
    | lambdaExpression
    ;

lambdaExpression
    : '|' parameterList? '|' ('=>' type)? block
    ;

primaryExpression
    : INTEGER
    | STRING
    | CHAR
    | 'true' | 'false'
    | IDENTIFIER
    | '(' expression ')'
    ;

// Lexer Rules
IDENTIFIER: [a-zA-Z_][a-zA-Z0-9_]*;
INTEGER: [0-9]+;
STRING: '"' (~["\r\n\\] | '\\' .)* '"';
CHAR: '\'' (~['\r\n\\] | '\\' .) '\'';

COMMENT: '//' ~[\r\n]* -> skip;
BLOCK_COMMENT: '/*' .*? '*/' -> skip;
WS: [ \t\r\n]+ -> skip;
```

### Phase 2: Go Integration

```go
// pkg/parser/antlr_parser.go
package parser

import (
    "github.com/antlr/antlr4/runtime/Go/antlr"
    "github.com/minz/minzc/pkg/ast"
    "github.com/minz/minzc/pkg/parser/generated"
)

type AntlrParser struct {
    source string
}

func NewAntlrParser(source string) *AntlrParser {
    return &AntlrParser{source: source}
}

func (p *AntlrParser) Parse() (*ast.File, error) {
    // Create input stream
    input := antlr.NewInputStream(p.source)
    
    // Create lexer
    lexer := generated.NewMinZLexer(input)
    stream := antlr.NewCommonTokenStream(lexer, 0)
    
    // Create parser
    parser := generated.NewMinZParser(stream)
    
    // Add error listener
    errorListener := &MinZErrorListener{}
    parser.AddErrorListener(errorListener)
    
    // Parse
    tree := parser.Program()
    
    // Check for errors
    if errorListener.HasErrors() {
        return nil, errorListener.GetError()
    }
    
    // Convert to AST
    visitor := &ASTBuilder{}
    ast := visitor.Visit(tree).(*ast.File)
    
    return ast, nil
}
```

### Phase 3: AST Visitor

```go
// pkg/parser/ast_builder.go
type ASTBuilder struct {
    *generated.BaseMinZVisitor
}

func (v *ASTBuilder) VisitProgram(ctx *generated.ProgramContext) interface{} {
    file := &ast.File{
        Imports:      []*ast.ImportStmt{},
        Declarations: []ast.Declaration{},
    }
    
    for _, child := range ctx.AllImportDecl() {
        imp := v.Visit(child).(*ast.ImportStmt)
        file.Imports = append(file.Imports, imp)
    }
    
    for _, child := range ctx.AllDeclaration() {
        decl := v.Visit(child).(ast.Declaration)
        file.Declarations = append(file.Declarations, decl)
    }
    
    return file
}

func (v *ASTBuilder) VisitFunctionDecl(ctx *generated.FunctionDeclContext) interface{} {
    return &ast.FunctionDecl{
        Name:       ctx.IDENTIFIER().GetText(),
        Params:     v.visitParameters(ctx.ParameterList()),
        ReturnType: v.visitType(ctx.Type()),
        Body:       v.Visit(ctx.Block()).(*ast.BlockStmt),
        IsPublic:   ctx.GetChild(0).(antlr.TerminalNode).GetText() == "pub",
    }
}
```

## Migration Strategy

### Step 1: Parallel Implementation (2-3 weeks)
1. Implement ANTLR grammar alongside tree-sitter
2. Add flag to switch between parsers
3. Test both parsers on all examples

### Step 2: Feature Parity (1-2 weeks)
1. Ensure ANTLR handles all MinZ features
2. Compare AST output between parsers
3. Fix any discrepancies

### Step 3: Performance Testing (1 week)
1. Benchmark parsing speed
2. Measure binary size impact
3. Test memory usage

### Step 4: Gradual Migration (2 weeks)
1. Make ANTLR default with tree-sitter fallback
2. Update documentation
3. Remove tree-sitter dependency

## Technical Considerations

### Pros of ANTLR
- **Zero Dependencies** - Everything embedded in binary
- **Better Errors** - Built-in error recovery
- **Visitor Pattern** - Clean AST construction
- **Grammar Validation** - Compile-time checking
- **Debugging** - Better tooling support

### Cons of ANTLR
- **Binary Size** - ANTLR runtime adds ~500KB
- **Learning Curve** - Different grammar syntax
- **Migration Work** - Need to rewrite grammar
- **Testing** - Need comprehensive test suite

## Alternatives Considered

### 1. Hand-Written Recursive Descent Parser
- **Pros**: Full control, smallest binary, fastest
- **Cons**: High maintenance, error-prone, time-consuming

### 2. Yacc/Bison with Go
- **Pros**: Standard tools, good performance
- **Cons**: Still needs external tools, C integration

### 3. PEG Parser (pointlander/peg)
- **Pros**: Pure Go, simple grammar
- **Cons**: Less mature, fewer features

### 4. Participle (alecthomas/participle)
- **Pros**: Pure Go, struct tags for grammar
- **Cons**: Limited grammar expressiveness

## Recommendation

**Implement ANTLR4 as the primary parser** for MinZ because:

1. **Solves Distribution Problem** - No external dependencies
2. **Professional Quality** - Used by many production languages
3. **Maintainable** - Grammar is declarative and versioned
4. **Cross-Platform** - Consistent behavior everywhere
5. **Future-Proof** - Active development and community

## Next Steps

1. Create `MinZ.g4` grammar file
2. Set up ANTLR4 Go runtime
3. Implement AST visitor
4. Create test harness
5. Run parallel testing
6. Benchmark performance
7. Plan migration timeline

## Example Implementation Timeline

- **Week 1**: Grammar development and testing
- **Week 2**: AST visitor implementation
- **Week 3**: Integration and testing
- **Week 4**: Performance optimization
- **Week 5**: Documentation and migration
- **Week 6**: Release v0.14.0 with ANTLR parser

This migration would eliminate the tree-sitter dependency issue permanently and provide a more reliable parsing solution for MinZ.