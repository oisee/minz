# Native Parser Implementation Plan

**Goal:** Replace tree-sitter with a native Go parser to eliminate OOM issues on large files.

**Target:** O(n) memory usage, no external dependencies, 90%+ compatibility with current AST.

---

## Executive Summary

### Why Replace Tree-sitter?

| Metric | Tree-sitter | Target |
|--------|-------------|--------|
| Memory (1KB file) | ~60KB | ~2KB |
| Memory (1MB file) | 60MB+ | ~2MB |
| Memory (10MB file) | **OOM** | ~20MB |
| Dependencies | CGO + tree-sitter-cli | Pure Go |
| Grammar bugs | 74 files affected | 0 |

### Chosen Approach: Hand-Written Recursive Descent

After analysis, we're implementing a **hand-written recursive descent parser** because:

1. **Zero dependencies** - Pure Go, no external tools
2. **Full control** - Custom error messages, recovery strategies
3. **Proven pattern** - Used by Go's own `go/parser`
4. **Already have example** - `minzc/pkg/z80asm/parser.go`

---

## Test Corpus

### Regression Test Suite

**Location:** `minzc/tests/parser_corpus/`

| Category | Count | Description |
|----------|-------|-------------|
| Primary tests | 146 | Clean tree-sitter parses that compile |
| Grammar bug tests | 74 | Tree-sitter bugs, but code compiles |
| **Total regression** | **220** | Files new parser must handle |
| Excluded | 128 | Semantic failures, aspirational code |

### Test Files

- `corpus.json` - All 348 test cases with metadata
- `regression.json` - 220 regression-relevant tests
- `analysis.json` - Categorization and statistics
- `<test>/input.minz` - Source file
- `<test>/expected.sexp` - Expected S-expression AST
- `<test>/metadata.json` - Hash, node counts, error info

### Running Tests

```bash
# Generate corpus (if needed)
./ast-gen tests/parser_corpus ../stdlib ../examples ../tests

# Validate corpus structure
./ast-compare tests/parser_corpus

# Test new parser against corpus
./ast-compare -parser './rdp-parser' -skip-errors tests/parser_corpus

# Show diffs for failures
./ast-compare -parser './rdp-parser' -diff -max-fail 20 tests/parser_corpus
```

---

## Implementation Plan

### Phase 1: Lexer (Week 1, Days 1-2)

**File:** `minzc/pkg/parser/rdp/lexer.go`

#### Token Types

```go
type TokenType int

const (
    TokenEOF TokenType = iota
    TokenError

    // Literals
    TokenIdent      // foo, bar, MyType
    TokenNumber     // 42, 0xFF, 0b1010
    TokenString     // "hello", 'c'
    TokenRawString  // `raw string`

    // Keywords
    TokenFun        // fun, fn
    TokenLet        // let
    TokenConst      // const
    TokenGlobal     // global
    TokenReturn     // return
    TokenIf         // if
    TokenElse       // else
    TokenWhile      // while
    TokenFor        // for
    TokenIn         // in
    TokenLoop       // loop
    TokenBreak      // break
    TokenContinue   // continue
    TokenStruct     // struct
    TokenEnum       // enum
    TokenImpl       // impl
    TokenImport     // import
    TokenAs         // as
    TokenTrue       // true
    TokenFalse      // false

    // Types
    TokenU8, TokenU16, TokenI8, TokenI16
    TokenBool, TokenVoid, TokenStr

    // Operators
    TokenPlus       // +
    TokenMinus      // -
    TokenStar       // *
    TokenSlash      // /
    TokenPercent    // %
    TokenAmp        // &
    TokenPipe       // |
    TokenCaret      // ^
    TokenTilde      // ~
    TokenBang       // !
    TokenLt         // <
    TokenGt         // >
    TokenEq         // =
    TokenDot        // .
    TokenComma      // ,
    TokenColon      // :
    TokenSemi       // ;
    TokenQuestion   // ?
    TokenAt         // @
    TokenHash       // #

    // Multi-char operators
    TokenEqEq       // ==
    TokenNotEq      // !=
    TokenLtEq       // <=
    TokenGtEq       // >=
    TokenAndAnd     // &&
    TokenOrOr       // ||
    TokenLtLt       // <<
    TokenGtGt       // >>
    TokenArrow      // ->
    TokenFatArrow   // =>
    TokenDotDot     // ..
    TokenColonColon // ::
    TokenQQ         // ??

    // Brackets
    TokenLParen     // (
    TokenRParen     // )
    TokenLBrace     // {
    TokenRBrace     // }
    TokenLBracket   // [
    TokenRBracket   // ]

    // Special
    TokenComment    // // or /* */
    TokenNewline    // for significant newlines
)
```

#### Lexer Structure

```go
type Lexer struct {
    input    string
    pos      int
    line     int
    col      int
    start    int  // start of current token
    tokens   []Token
}

type Token struct {
    Type    TokenType
    Value   string
    Pos     Position
}

type Position struct {
    Offset int
    Line   int
    Column int
}
```

#### Key Methods

```go
func NewLexer(input string) *Lexer
func (l *Lexer) NextToken() Token
func (l *Lexer) Tokenize() ([]Token, error)

// Internal
func (l *Lexer) scanIdent() Token
func (l *Lexer) scanNumber() Token
func (l *Lexer) scanString() Token
func (l *Lexer) scanOperator() Token
func (l *Lexer) skipWhitespace()
func (l *Lexer) skipComment()
```

### Phase 2: Expression Parser (Week 1, Days 3-5)

**File:** `minzc/pkg/parser/rdp/expr.go`

#### Operator Precedence (Pratt Parser)

```go
var precedence = map[TokenType]int{
    TokenOrOr:     1,  // ||
    TokenAndAnd:   2,  // &&
    TokenPipe:     3,  // |
    TokenCaret:    4,  // ^
    TokenAmp:      5,  // &
    TokenEqEq:     6,  // == !=
    TokenNotEq:    6,
    TokenLt:       7,  // < > <= >=
    TokenGt:       7,
    TokenLtEq:     7,
    TokenGtEq:     7,
    TokenLtLt:     8,  // << >>
    TokenGtGt:     8,
    TokenPlus:     9,  // + -
    TokenMinus:    9,
    TokenStar:     10, // * / %
    TokenSlash:    10,
    TokenPercent:  10,
}
```

#### Core Expression Parser

```go
func (p *Parser) parseExpression() ast.Expression {
    return p.parseBinaryExpr(0)
}

func (p *Parser) parseBinaryExpr(minPrec int) ast.Expression {
    left := p.parseUnaryExpr()

    for {
        op := p.current()
        prec, ok := precedence[op.Type]
        if !ok || prec < minPrec {
            break
        }

        p.advance()
        right := p.parseBinaryExpr(prec + 1)
        left = &ast.BinaryExpr{
            Left:  left,
            Op:    op,
            Right: right,
        }
    }

    return left
}

func (p *Parser) parseUnaryExpr() ast.Expression {
    if p.match(TokenMinus, TokenBang, TokenTilde) {
        op := p.previous()
        operand := p.parseUnaryExpr()
        return &ast.UnaryExpr{Op: op, Operand: operand}
    }
    return p.parsePostfixExpr()
}

func (p *Parser) parsePostfixExpr() ast.Expression {
    expr := p.parsePrimaryExpr()

    for {
        switch {
        case p.match(TokenLParen):
            // Function call
            args := p.parseArguments()
            expr = &ast.CallExpr{Func: expr, Args: args}

        case p.match(TokenLBracket):
            // Array index
            index := p.parseExpression()
            p.expect(TokenRBracket)
            expr = &ast.IndexExpr{Array: expr, Index: index}

        case p.match(TokenDot):
            // Field access
            field := p.expect(TokenIdent)
            expr = &ast.FieldExpr{Object: expr, Field: field.Value}

        default:
            return expr
        }
    }
}

func (p *Parser) parsePrimaryExpr() ast.Expression {
    switch {
    case p.match(TokenNumber):
        return &ast.NumberLiteral{Value: p.parseNumber(p.previous())}

    case p.match(TokenString):
        return &ast.StringLiteral{Value: p.parseString(p.previous())}

    case p.match(TokenTrue):
        return &ast.BooleanLiteral{Value: true}

    case p.match(TokenFalse):
        return &ast.BooleanLiteral{Value: false}

    case p.match(TokenIdent):
        return &ast.Identifier{Name: p.previous().Value}

    case p.match(TokenLParen):
        expr := p.parseExpression()
        p.expect(TokenRParen)
        return expr

    case p.match(TokenLBracket):
        return p.parseArrayLiteral()

    case p.match(TokenPipe):
        return p.parseLambda()

    case p.match(TokenAt):
        return p.parseMetafunction()

    default:
        p.error("unexpected token: %v", p.current())
        return nil
    }
}
```

### Phase 3: Statement Parser (Week 2, Days 1-2)

**File:** `minzc/pkg/parser/rdp/stmt.go`

```go
func (p *Parser) parseStatement() ast.Statement {
    switch {
    case p.match(TokenLet):
        return p.parseLetStmt()

    case p.match(TokenConst):
        return p.parseConstStmt()

    case p.match(TokenReturn):
        return p.parseReturnStmt()

    case p.match(TokenIf):
        return p.parseIfStmt()

    case p.match(TokenWhile):
        return p.parseWhileStmt()

    case p.match(TokenFor):
        return p.parseForStmt()

    case p.match(TokenLoop):
        return p.parseLoopStmt()

    case p.check(TokenIdent) && p.checkNext(TokenEq):
        return p.parseAssignStmt()

    default:
        return p.parseExpressionStmt()
    }
}

func (p *Parser) parseBlock() *ast.BlockStmt {
    p.expect(TokenLBrace)

    var stmts []ast.Statement
    for !p.check(TokenRBrace) && !p.isAtEnd() {
        stmt := p.parseStatement()
        if stmt != nil {
            stmts = append(stmts, stmt)
        }
    }

    p.expect(TokenRBrace)
    return &ast.BlockStmt{Statements: stmts}
}
```

### Phase 4: Declaration Parser (Week 2, Days 3-4)

**File:** `minzc/pkg/parser/rdp/decl.go`

```go
func (p *Parser) parseDeclaration() ast.Declaration {
    switch {
    case p.match(TokenFun):
        return p.parseFunctionDecl()

    case p.match(TokenStruct):
        return p.parseStructDecl()

    case p.match(TokenEnum):
        return p.parseEnumDecl()

    case p.match(TokenImpl):
        return p.parseImplDecl()

    case p.match(TokenConst):
        return p.parseConstDecl()

    case p.match(TokenGlobal):
        return p.parseGlobalDecl()

    case p.match(TokenImport):
        return p.parseImportDecl()

    default:
        return nil
    }
}

func (p *Parser) parseFunctionDecl() *ast.FunctionDecl {
    name := p.expect(TokenIdent).Value

    // Generic parameters <T: Constraint>
    var typeParams []*ast.TypeParam
    if p.match(TokenLt) {
        typeParams = p.parseTypeParams()
    }

    // Parameters
    p.expect(TokenLParen)
    params := p.parseParams()
    p.expect(TokenRParen)

    // Return type
    var returnType ast.Type
    if p.match(TokenArrow) {
        returnType = p.parseType()
    }

    // Body
    body := p.parseBlock()

    return &ast.FunctionDecl{
        Name:       name,
        TypeParams: typeParams,
        Params:     params,
        ReturnType: returnType,
        Body:       body,
    }
}
```

### Phase 5: Special Features (Week 2, Day 5)

**File:** `minzc/pkg/parser/rdp/special.go`

#### Metafunctions

```go
func (p *Parser) parseMetafunction() ast.Expression {
    name := p.expect(TokenIdent).Value

    switch name {
    case "print":
        return p.parsePrintMeta()
    case "if":
        return p.parseIfMeta()
    case "define":
        return p.parseDefineMeta()
    case "extern":
        return p.parseExternMeta()
    case "abi":
        return p.parseAbiMeta()
    // ... more metafunctions
    }
}
```

#### Inline Assembly

```go
func (p *Parser) parseAsmBlock() *ast.AsmBlockStmt {
    p.expect(TokenLBrace)

    var content strings.Builder
    braceDepth := 1

    for braceDepth > 0 && !p.isAtEnd() {
        if p.check(TokenLBrace) {
            braceDepth++
        } else if p.check(TokenRBrace) {
            braceDepth--
            if braceDepth == 0 {
                break
            }
        }
        content.WriteString(p.current().Value)
        p.advance()
    }

    p.expect(TokenRBrace)
    return &ast.AsmBlockStmt{Content: content.String()}
}
```

#### String Interpolation

```go
func (p *Parser) parseInterpolatedString() ast.Expression {
    // "Hello {name}, you have {count} messages"
    // Becomes: concat("Hello ", name, ", you have ", count, " messages")

    str := p.previous().Value
    parts := p.parseStringParts(str)

    if len(parts) == 1 && parts[0].IsLiteral {
        return &ast.StringLiteral{Value: parts[0].Value}
    }

    // Build concatenation expression
    return p.buildInterpolation(parts)
}
```

---

## Integration Plan

### Step 1: Parallel Implementation

Keep tree-sitter as fallback while developing:

```go
func NewParser(backend string) Parser {
    switch backend {
    case "rdp":
        return NewRDPParser()
    case "tree-sitter":
        return NewTreeSitterParser()
    default:
        // Auto-detect based on env
        if os.Getenv("MINZ_USE_RDP") != "" {
            return NewRDPParser()
        }
        return NewTreeSitterParser()
    }
}
```

### Step 2: Incremental Migration

1. Parse with both parsers
2. Compare AST output
3. Fix discrepancies
4. Run full test suite

```bash
# Compare outputs
MINZ_USE_RDP=1 ./minzc file.minz -ast > rdp.ast
./minzc file.minz -ast > ts.ast
diff rdp.ast ts.ast
```

### Step 3: Validation

```bash
# Run regression suite
./ast-compare -parser 'MINZ_USE_RDP=1 ./minzc' tests/parser_corpus

# Target: 90%+ pass rate for clean files, 80%+ for grammar-bug files
```

### Step 4: Cutover

Once at 95%+ compatibility:
1. Make RDP the default
2. Keep tree-sitter as `--parser=tree-sitter` option
3. Eventually remove tree-sitter

---

## Success Metrics

| Metric | Target |
|--------|--------|
| Regression pass rate | ≥95% |
| Memory (10MB file) | <50MB |
| Parse speed | ≥ tree-sitter |
| Error messages | Line:col + context |
| Dependencies | 0 external |

---

## Timeline

| Week | Focus | Deliverable |
|------|-------|-------------|
| 1 | Lexer + Expressions | Tokenizer, expression parser |
| 2 | Statements + Declarations | Full parser skeleton |
| 3 | Special features | Metafunctions, asm, interpolation |
| 4 | Integration + Testing | 90%+ regression pass |

---

## Files to Create

```
minzc/pkg/parser/rdp/
├── lexer.go          # Tokenizer
├── lexer_test.go     # Lexer tests
├── parser.go         # Main parser
├── expr.go           # Expression parsing
├── stmt.go           # Statement parsing
├── decl.go           # Declaration parsing
├── special.go        # Metafunctions, asm, etc.
├── errors.go         # Error handling
└── parser_test.go    # Parser tests
```

---

## References

- Go's parser: `go/parser/parser.go`
- MinZ z80asm parser: `minzc/pkg/z80asm/parser.go`
- Pratt parsing: https://matklad.github.io/2020/04/13/simple-but-powerful-pratt-parsing.html
- Crafting Interpreters: https://craftinginterpreters.com/parsing-expressions.html
