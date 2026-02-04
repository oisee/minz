# Parser Alternatives: Детальное сравнение

**Source:** `reports/2026-01-08-001-parser-alternatives-research.md`
**Decision needed by:** Week 2, Day 5

---

## Текущая проблема

```
Tree-sitter CLI → shells out → reads S-expression → парсит
                      ↓
              60GB+ RAM на больших файлах
                      ↓
                    OOM
```

---

## Comparison Matrix

| Критерий | Tree-sitter | ANTLR4 | Hand-Written | Participle | Pigeon |
|----------|-------------|--------|--------------|------------|--------|
| **Memory (10MB)** | OOM | 50-100MB | 10-20MB | ~10MB | 20-30MB |
| **Dependencies** | CLI + npm | Runtime | None | 1 lib | Generated |
| **CGO needed** | No* | No | No | No | No |
| **Effort** | Done | 2 weeks | 4 weeks | 2-3 weeks | 3 weeks |
| **Error messages** | Poor | Excellent | Full control | Good | Limited |
| **Incremental** | Yes | No | No | No | No |
| **Grammar exists** | Yes | Yes (5%) | No | No | No |

*Tree-sitter CLI вызывается как subprocess

---

## Option A: Hand-Written Recursive Descent

### Характеристики
- **Memory:** O(n) — линейно от размера файла
- **Dependencies:** Zero — чистый Go
- **Effort:** 4 weeks
- **Сложность:** Medium

### Архитектура
```
Source → Lexer → Tokens → Parser → AST
           ↓
    minzc/pkg/parser/rdp/
    ├── lexer.go      # Токенизация
    ├── tokens.go     # Token types
    ├── expr.go       # Expression parsing
    ├── stmt.go       # Statement parsing
    ├── decl.go       # Declaration parsing
    └── parser.go     # Main orchestration
```

### Плюсы
- Полный контроль над памятью
- Идеальные error messages с позициями
- Паттерн из Go stdlib
- Уже есть пример: `minzc/pkg/z80asm/parser.go`
- Rob Pike's lexer pattern

### Минусы
- Много кода писать вручную (~3000-5000 LOC)
- Precedence climbing для операторов
- Нет incremental parsing

### Пример: Precedence Climbing
```go
var precedences = map[TokenType]int{
    TokenOr:     1,
    TokenAnd:    2,
    TokenEq:     3, TokenNe: 3,
    TokenLt:     4, TokenGt: 4, TokenLe: 4, TokenGe: 4,
    TokenPlus:   5, TokenMinus: 5,
    TokenStar:   6, TokenSlash: 6, TokenPercent: 6,
}

func (p *Parser) parseBinaryExpr(minPrec int) ast.Expression {
    left := p.parseUnaryExpr()

    for {
        prec, ok := precedences[p.tok.Type]
        if !ok || prec < minPrec {
            break
        }

        op := p.tok
        p.next()
        right := p.parseBinaryExpr(prec + 1)
        left = &ast.BinaryExpr{Left: left, Op: op, Right: right}
    }

    return left
}
```

---

## Option B: Participle

### Характеристики
- **Memory:** O(n) — линейно
- **Dependencies:** `github.com/alecthomas/participle/v2`
- **Effort:** 2-3 weeks
- **Сложность:** Low

### Архитектура
```
Source → Participle → AST (defined by struct tags)
           ↓
    minzc/pkg/parser/participle/
    ├── grammar.go    # AST structs with tags
    ├── lexer.go      # Custom lexer (optional)
    └── parser.go     # Parser instantiation
```

### Плюсы
- AST = грамматика (один источник правды)
- Меньше кода (~1000-1500 LOC)
- Хорошие error messages из коробки
- Активная поддержка (2024+ commits)
- 3x быстрее Pigeon

### Минусы
- Внешняя зависимость
- Сложные грамматики требуют workarounds
- Меньше контроля над деталями

### Пример: MinZ Grammar
```go
package parser

import "github.com/alecthomas/participle/v2"

type File struct {
    Imports  []*Import     `@@*`
    Decls    []*Declaration `@@*`
}

type Import struct {
    Path string `"import" @(Ident ("." Ident)*)`
}

type FunctionDecl struct {
    Attrs   []*Attribute `@@*`
    Name    string       `("fun" | "fn") @Ident`
    Params  []*Param     `"(" (@@ ("," @@)*)? ")"`
    RetType *Type        `("->" @@)?`
    Body    *Block       `@@`
}

type Param struct {
    Name string `@Ident`
    Type *Type  `":" @@`
}

type Type struct {
    Pointer bool   `@"*"?`
    Name    string `@Ident`
    Array   *int   `("[" @Int "]")?`
}

type Block struct {
    Stmts []*Statement `"{" @@* "}"`
}

type Statement struct {
    Let    *LetStmt    `  @@`
    If     *IfStmt     `| @@`
    While  *WhileStmt  `| @@`
    For    *ForStmt    `| @@`
    Return *ReturnStmt `| @@`
    Expr   *Expression `| @@`
}

type Expression struct {
    // ... binary, unary, call, etc.
}

// Parser creation
var Parser = participle.MustBuild[File](
    participle.Lexer(minzLexer),
    participle.UseLookahead(2),
)
```

---

## Option C: ANTLR4 (Fallback)

### Текущий статус
- Грамматика есть: `minzc/grammar/MinZ.g4`
- Success rate: **5%** (visitor incomplete)

### Если выбрать
- Нужно дописать visitor (~1900 LOC в `antlr_parser.go`)
- Память: 2-10x от размера файла
- Runtime dependency: `antlr4-go`

### Когда имеет смысл
- Если нет времени на full rewrite
- Как временное решение
- Grammar уже готова

---

## Option D: Pigeon (PEG)

### Характеристики
- **Memory:** O(n) с memoization
- **Dependencies:** Generated code
- **Effort:** 3 weeks

### Минусы для MinZ
- PEG не поддерживает ambiguity
- 27 конфликтов в текущей грамматике
- Нужно переписывать грамматику

**Verdict:** Не рекомендуется для MinZ

---

## Recommendation

### Для MinZ рекомендую: **Participle**

| Фактор | Почему Participle |
|--------|-------------------|
| **Время** | 2-3 weeks vs 4 weeks |
| **Код** | ~1500 LOC vs ~4000 LOC |
| **Поддержка** | Активный проект |
| **Memory** | O(n) — решает OOM |
| **Dependency** | Одна, чистый Go |

### Alternative: Hand-Written если...
- Нужен абсолютный zero-dependency
- Очень сложные error messages
- Хочется полного контроля

---

## Implementation Plan (Participle)

### Week 1: Core Grammar
```
Day 1: Setup + File/Import
Day 2: Function declarations
Day 3: Statements (let, if, while, for)
Day 4: Expressions (binary, unary, call)
Day 5: Types + testing
```

### Week 2: Special Features
```
Day 1: Structs, Enums
Day 2: Impl blocks
Day 3: Metafunctions (@print, @ctie)
Day 4: Inline assembly
Day 5: String interpolation
```

### Week 3: Integration
```
Day 1: MINZ_USE_PARTICIPLE=1 flag
Day 2: Compare with tree-sitter output
Day 3: Fix edge cases
Day 4-5: Switch default, cleanup
```

---

## Migration Path

```bash
# Phase 1: Parallel parsers
export MINZ_USE_PARTICIPLE=1
./mz examples/fibonacci.minz  # Uses new parser

# Phase 2: Compare outputs
./scripts/compare_parsers.sh examples/

# Phase 3: Switch default (when >90% match)
# Remove tree-sitter dependency
```

---

## References

- [Participle GitHub](https://github.com/alecthomas/participle)
- [Participle Examples](https://github.com/alecthomas/participle/tree/master/_examples)
- [Go Parser Package](https://pkg.go.dev/go/parser)
- [Rob Pike: Lexical Scanning in Go](https://www.youtube.com/watch?v=HxaD_trXwRE)
- [Existing Report](../reports/2026-01-08-001-parser-alternatives-research.md)

---

**Decision:** _________________ (заполнить после Week 2, Day 5)

**Rationale:** _________________
