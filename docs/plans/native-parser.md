# Plan: Native MinZ Parser (Participle)

## Overview

Replace tree-sitter with a **Participle**-based parser to eliminate OOM issues on large files.

**Participle:** https://github.com/alecthomas/participle - Go parser library using struct tags for grammar definition.

**Goal:** O(n) memory usage, single Go dependency, 95%+ compatibility with current AST.

**Why Participle over hand-written RDP:**
- Grammar defined via struct tags - less code, faster development
- Good error messages out of the box
- Actively maintained
- 2-3 weeks vs 4 weeks for hand-written

## Validation Commands

- `cd minzc && go build ./...`
- `cd minzc && go test ./pkg/parser/participle/... -v`
- `cd minzc && ./ast-compare tests/parser_corpus`

---

### Task 1: Setup Participle and Define Tokens

Set up Participle lexer with all MinZ tokens.

- [ ] Add `github.com/alecthomas/participle/v2` to go.mod
- [ ] Create `minzc/pkg/parser/participle/` directory
- [ ] Create `lexer.go` with custom lexer definition
- [ ] Define token rules: identifiers, numbers (dec/hex/bin), strings, operators
- [ ] Handle comments (// and /* */)
- [ ] Handle keywords: fun, fn, let, const, global, if, else, while, for, in, loop, return, struct, enum, impl, import, true, false
- [ ] Handle type keywords: u8, u16, i8, i16, bool, void, str
- [ ] Handle multi-char operators: ==, !=, <=, >=, &&, ||, <<, >>, ->, =>, .., ::, ??
- [ ] Create `lexer_test.go` with token tests
- [ ] Verify: `go test ./pkg/parser/participle/... -run Lexer -v`

---

### Task 2: Define Expression Grammar

Define expression AST structs with Participle tags.

- [ ] Create `expr.go` with expression types
- [ ] Define `Expression` as union type (Participle uses `@@` for alternatives)
- [ ] Define `BinaryExpr` with operator precedence via Participle's `@Expr` pattern
- [ ] Define `UnaryExpr` for prefix operators (-, !, ~)
- [ ] Define `CallExpr` for function calls
- [ ] Define `IndexExpr` for array indexing [i]
- [ ] Define `FieldExpr` for field access .field
- [ ] Define `Identifier`, `NumberLiteral`, `StringLiteral`, `BoolLiteral`
- [ ] Define `ArrayLiteral` [1, 2, 3]
- [ ] Define `StructLiteral` Point { x: 1, y: 2 }
- [ ] Define `LambdaExpr` |x, y| => expr
- [ ] Create `expr_test.go` with parsing tests
- [ ] Verify: `go test ./pkg/parser/participle/... -run Expr -v`

Example Participle syntax:
```go
type BinaryExpr struct {
    Left  *UnaryExpr `@@`
    Op    string     `@("+" | "-" | "*" | "/")`
    Right *BinaryExpr `@@`
}
```

---

### Task 3: Define Statement Grammar

Define statement AST structs with Participle tags.

- [ ] Create `stmt.go` with statement types
- [ ] Define `Statement` union type
- [ ] Define `LetStmt` - `"let" @Ident ":" @Type "=" @@`
- [ ] Define `ConstStmt` - `"const" @Ident "=" @@`
- [ ] Define `ReturnStmt` - `"return" @@?`
- [ ] Define `IfStmt` with else/elif chains
- [ ] Define `WhileStmt` - `"while" @@ @@Block`
- [ ] Define `ForStmt` - `"for" @Ident "in" @@ ".." @@ @@Block`
- [ ] Define `LoopStmt` - loop { } and loop array into/ref
- [ ] Define `AssignStmt` - `@@ "=" @@`
- [ ] Define `ExprStmt` - bare expression with optional semicolon
- [ ] Define `Block` - `"{" @@* "}"`
- [ ] Create `stmt_test.go`
- [ ] Verify: `go test ./pkg/parser/participle/... -run Stmt -v`

---

### Task 4: Define Declaration Grammar

Define top-level declarations with Participle tags.

- [ ] Create `decl.go` with declaration types
- [ ] Define `File` as top-level container
- [ ] Define `FunctionDecl` - fun/fn name<T>(params) -> Type { body }
- [ ] Define `TypeParam` for generics <T: Constraint>
- [ ] Define `Parameter` with name and type
- [ ] Define `Type` - primitives, arrays, pointers, named types
- [ ] Define `StructDecl` - struct Name { fields }
- [ ] Define `EnumDecl` - enum Name { variants }
- [ ] Define `ImplDecl` - impl Type { methods }
- [ ] Define `GlobalDecl` - global name: Type = value
- [ ] Define `ImportDecl` - import path
- [ ] Create `decl_test.go`
- [ ] Verify: `go test ./pkg/parser/participle/... -run Decl -v`

---

### Task 5: Handle Special Syntax

Handle metafunctions, asm blocks, and interpolation.

- [ ] Create `special.go` for special syntax
- [ ] Define `Metafunction` - @name(args) pattern
- [ ] Handle @print, @if, @define, @extern, @abi, @inline, etc.
- [ ] Define `AsmBlock` - asm { raw content } (capture raw text)
- [ ] Define `MirBlock` - mir { raw content }
- [ ] Handle string interpolation "Hello {name}!"
- [ ] Handle attributes @[inline], @[abi("z80")]
- [ ] Create `special_test.go`
- [ ] Verify: `go test ./pkg/parser/participle/... -run Special -v`

---

### Task 6: Build Main Parser

Create the main parser entry point.

- [ ] Create `parser.go` with Parser struct
- [ ] Build Participle parser with `participle.MustBuild[File]()`
- [ ] Implement `Parse(filename string, input string) (*File, error)`
- [ ] Implement `ParseFile(path string) (*File, error)`
- [ ] Add custom error formatting with file:line:col
- [ ] Handle parser options (e.g., elide comments)
- [ ] Create `parser_test.go` with full file parsing tests
- [ ] Test with sample MinZ files
- [ ] Verify: `go test ./pkg/parser/participle/... -v`

---

### Task 7: Convert to Existing AST

Convert Participle AST to existing `pkg/ast` types.

- [ ] Create `convert.go` for AST conversion
- [ ] Implement `Convert(pFile *File) (*ast.SourceFile, error)`
- [ ] Convert all expression types to ast.Expression
- [ ] Convert all statement types to ast.Statement
- [ ] Convert all declaration types to ast.Declaration
- [ ] Preserve source positions (line, column)
- [ ] Create `convert_test.go`
- [ ] Verify conversion matches tree-sitter output structure
- [ ] Verify: `go test ./pkg/parser/participle/... -run Convert -v`

---

### Task 8: S-Expression Output for Testing

Output S-expressions matching tree-sitter format.

- [ ] Create `sexp.go` for S-expression generation
- [ ] Match tree-sitter format: `(node_type [line, col] - [line, col] ...)`
- [ ] Handle all AST node types
- [ ] Create CLI wrapper for testing: `participle-parser file.minz`
- [ ] Compare with tree-sitter on sample files
- [ ] Create `sexp_test.go`
- [ ] Verify: `go test ./pkg/parser/participle/... -run SExp -v`

---

### Task 9: Regression Testing

Run against 220 test corpus files.

- [ ] Ensure corpus exists: `./ast-gen tests/parser_corpus ../stdlib ../examples ../tests`
- [ ] Create wrapper script/binary for ast-compare
- [ ] Run: `./ast-compare -parser './participle-parser' tests/parser_corpus`
- [ ] Fix failures iteratively (target: 95%+ clean files)
- [ ] Handle tree-sitter grammar bugs (74 files) - these should work better
- [ ] Document intentional differences
- [ ] Create regression summary report
- [ ] Verify: 90%+ pass rate on 220 files

---

### Task 10: Integration and Cutover

Make Participle the default parser.

- [ ] Add parser backend selection in `pkg/parser/parser.go`
- [ ] Add `--parser=participle|tree-sitter` CLI flag
- [ ] Add `MINZ_PARSER=participle` env var support
- [ ] Set Participle as default
- [ ] Update go.mod (add participle, can remove tree-sitter deps later)
- [ ] Update README.md - no more npm/tree-sitter-cli required
- [ ] Run full compilation test: `./compile_all_examples.sh`
- [ ] Verify memory on large files (<50MB for 10MB input)
- [ ] Update CLAUDE.md to mark parser complete
- [ ] Commit and tag release

---

## Success Criteria

- [ ] All 220 regression tests pass at 95%+ rate
- [ ] Memory usage < 50MB on 10MB input files
- [ ] Single dependency: github.com/alecthomas/participle/v2
- [ ] Error messages include file:line:col
- [ ] All existing examples compile successfully
- [ ] No npm/tree-sitter-cli required for installation
