# Weeks 2-4: Mid Wins Sprint

**Цель:** Parser rewrite + Core features completion
**Expected Result:** No OOM, +10% compilation, nested functions work

---

## Week 2: Nested Functions + Parser Research

### Days 1-3: Nested Functions (MW-2)
**Effort:** 3 days | **Impact:** HIGH (+7 examples)

**Problem:**
```minz
fun outer() -> void {
    fun inner() -> u8 {  // ← не компилируется
        return 42;
    }
    inner();
}
```

**Solution:**
1. **Semantic:** Track nested scope in `analyzer.go`
2. **IR:** Emit nested as separate functions with mangled names
3. **Codegen:** `outer$inner` label generation

**Files:**
- `minzc/pkg/semantic/analyzer.go`
- `minzc/pkg/codegen/z80.go`

**Test cases:**
- `examples/archived_future_features/local_functions.minz`
- `examples/archived_future_features/nested_closures.minz`

---

### Days 4-5: Parser Alternative Research

**Goal:** Decide on tree-sitter replacement strategy

**Source:** `reports/2026-01-08-001-parser-alternatives-research.md`

---

## Parser Options Comparison (из репорта)

### Memory Usage на разных размерах файлов

| Parser | 1KB file | 1MB file | 10MB file |
|--------|----------|----------|-----------|
| **Tree-sitter (текущий)** | ~60KB | 60MB+ | **OOM** |
| ANTLR4 | 5-10KB | 5-10MB | 50-100MB |
| **Hand-written RDP** | 1-2KB | 1-2MB | **10-20MB** |
| **Participle** | ~1KB | ~1MB | **~10MB** |
| Pigeon (PEG) | 2-3KB | 2-3MB | 20-30MB |

---

### Hand-Written RDP vs Participle

| Критерий | Hand-Written RDP | Participle |
|----------|------------------|------------|
| **Memory** | O(n) | O(n) |
| **Dependencies** | None (pure Go) | 1 library |
| **Effort** | 4 weeks | 2-3 weeks |
| **Maintainability** | Manual code | Struct tags |
| **Error Recovery** | Full control | Good defaults |
| **Performance** | Excellent | 3x faster than Pigeon |
| **Learning Curve** | Medium | Low |

---

### Option A: Hand-Written Recursive Descent

**Pros:**
- Zero dependencies — полностью self-contained
- Полный контроль над error messages
- Паттерн из Go stdlib (`go/parser`, `text/template`)
- Уже есть пример: `minzc/pkg/z80asm/parser.go`

**Cons:**
- Больше кода писать вручную
- Нужно вручную обрабатывать precedence climbing
- Нет incremental parsing

**Пример кода:**
```go
func (p *Parser) parseExpression() ast.Expression {
    return p.parseBinaryExpr(0)
}

func (p *Parser) parseBinaryExpr(minPrec int) ast.Expression {
    left := p.parseUnaryExpr()
    for {
        op := p.currentToken
        prec := precedence(op)
        if prec < minPrec { break }
        p.advance()
        right := p.parseBinaryExpr(prec + 1)
        left = &ast.BinaryExpr{Left: left, Op: op, Right: right}
    }
    return left
}
```

---

### Option B: Participle (Struct-Based Parser)

**Pros:**
- AST определяется через struct tags — меньше кода
- Автоматическая генерация парсера
- Хорошие error messages из коробки
- Активно поддерживается

**Cons:**
- Внешняя зависимость (хотя одна)
- Меньше контроля над низкоуровневыми деталями
- Некоторые сложные грамматики требуют workarounds

**Пример кода:**
```go
type Expression struct {
    Left  *Term       `@@`
    Op    string      `@("+" | "-")?`
    Right *Expression `@@?`
}

type FunctionDecl struct {
    Name   string      `"fun" @Ident`
    Params []*Param    `"(" @@? ("," @@)* ")"`
    Return *Type       `("->" @@)?`
    Body   *Block      `@@`
}

// Создание парсера:
parser := participle.MustBuild[File]()
ast, err := parser.ParseString("", source)
```

**GitHub:** https://github.com/alecthomas/participle

---

### Рекомендация

| Если... | То выбрать... |
|---------|---------------|
| Нужен полный контроль | Hand-Written RDP |
| Нужна скорость разработки | **Participle** |
| Нужен zero dependencies | Hand-Written RDP |
| Сложная грамматика с конфликтами | Hand-Written RDP |
| Простая итерация на грамматике | **Participle** |

**Для MinZ:** Оба варианта подходят. Participle быстрее в разработке (2-3 vs 4 weeks), Hand-Written даёт больше контроля.

---

### Decision Criteria

1. **Memory** — must not OOM on 10MB files ✓ оба
2. **Self-contained** — no CGO, no external binaries ✓ оба
3. **Error recovery** — good error messages ✓ оба
4. **Maintenance** — easy to modify grammar

**Deliverable:** ADR document with final recommendation

---

## Week 3-4: Parser Implementation (MW-1)

### Week 3: Lexer + Expression Parser

**Day 1-2: Lexer**
**File:** `minzc/pkg/parser/rdp/lexer.go`

```go
type TokenType int

const (
    TokenEOF TokenType = iota
    TokenIdent
    TokenNumber
    TokenString
    TokenKeyword  // fun, let, if, while, for, etc.
    TokenOperator // +, -, *, /, ==, !=, etc.
    TokenPunct    // (, ), {, }, [, ], ;, ,
    TokenComment
)

type Lexer struct {
    input   string
    pos     int
    line    int
    col     int
}

func (l *Lexer) NextToken() Token { ... }
```

**Day 3-5: Expression Parser**
**File:** `minzc/pkg/parser/rdp/expr.go`

```go
// Precedence climbing for binary expressions
func (p *Parser) parseExpression() ast.Expression {
    return p.parseBinaryExpr(0)
}

func (p *Parser) parseBinaryExpr(minPrec int) ast.Expression {
    left := p.parseUnaryExpr()

    for {
        op := p.currentToken
        prec := precedence(op)
        if prec < minPrec {
            break
        }
        p.advance()
        right := p.parseBinaryExpr(prec + 1)
        left = &ast.BinaryExpr{Left: left, Op: op, Right: right}
    }
    return left
}
```

---

### Week 4: Statement + Declaration Parser

**Day 1-2: Statements**
**File:** `minzc/pkg/parser/rdp/stmt.go`

```go
func (p *Parser) parseStatement() ast.Statement {
    switch p.currentToken.Type {
    case TokenKeyword:
        switch p.currentToken.Value {
        case "let":
            return p.parseLetStatement()
        case "if":
            return p.parseIfStatement()
        case "while":
            return p.parseWhileStatement()
        case "for":
            return p.parseForStatement()
        case "return":
            return p.parseReturnStatement()
        }
    }
    return p.parseExpressionStatement()
}
```

**Day 3-4: Declarations**
**File:** `minzc/pkg/parser/rdp/decl.go`

```go
func (p *Parser) parseDeclaration() ast.Declaration {
    switch p.currentToken.Value {
    case "fun", "fn":
        return p.parseFunctionDecl()
    case "struct":
        return p.parseStructDecl()
    case "enum":
        return p.parseEnumDecl()
    case "impl":
        return p.parseImplDecl()
    case "const":
        return p.parseConstDecl()
    case "global":
        return p.parseGlobalDecl()
    }
    // ...
}
```

**Day 5: Special Features**
- Metafunctions: `@print`, `@ctie`, `@smc`, `@define`
- Inline assembly: `asm { ... }`
- String interpolation: `"Hello #{name}!"`

---

## Verification Checklist

### Week 2
- [ ] Nested functions compile
- [ ] 7+ archived examples now work
- [ ] Parser ADR document written

### Week 3-4
- [ ] Lexer handles all MinZ tokens
- [ ] Expression parser handles all operators
- [ ] Statement parser handles all control flow
- [ ] Declaration parser handles all types
- [ ] Memory usage O(n) for large files
- [ ] `MINZ_USE_RDP=1` env var switches parser
- [ ] >90% success rate vs tree-sitter

---

## Success Metrics

| Metric | Before | After |
|--------|--------|-------|
| Compilation success | 81% | 91%+ |
| Memory usage (10MB file) | 60GB+ | <1GB |
| Parser dependencies | tree-sitter | None |
| Nested functions | Broken | Working |

---

**Next:** Weeks 5-6 — Pattern Matching + stdlib
