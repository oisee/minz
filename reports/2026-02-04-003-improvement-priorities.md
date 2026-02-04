# MinZ: Приоритеты Улучшений

**Дата:** 2026-02-04
**Версия:** v0.18.0

---

## Quick Wins (1-4 часа каждый)

Минимум усилий — максимум impact. Можно сделать за один день.

### QW-1: MIR Parser — Array Access ⭐⭐⭐
**Effort:** 1 час | **Impact:** HIGH

```go
// Добавить в mir_parser.go:
// r0 = r1[r2]  →  OpLoadIndex
// r0 = r1[5]   →  OpLoadElement
```

**Результат:** VM сможет выполнять программы с массивами
**Файл:** `minzc/pkg/ir/mir_parser.go`

---

### QW-2: MIR Parser — Struct Fields ⭐⭐⭐
**Effort:** 30 мин | **Impact:** HIGH

```go
// r0 = r1.field[2]  →  OpLoadField
// r1.field[2] = r0  →  OpStoreField
```

**Результат:** VM сможет выполнять программы со структурами
**Файл:** `minzc/pkg/ir/mir_parser.go`

---

### QW-3: MIR Parser — Parameter Loading ⭐⭐
**Effort:** 15 мин | **Impact:** MEDIUM

```go
// r0 = param x  →  OpLoadParam
```

**Результат:** Функции с параметрами заработают в VM
**Файл:** `minzc/pkg/ir/mir_parser.go`

---

### QW-4: VM Handlers для массивов ⭐⭐⭐
**Effort:** 1 час | **Impact:** HIGH

```go
case ir.OpLoadIndex:
    baseAddr := vm.registers[inst.Src1]
    index := vm.registers[inst.Src2]
    vm.registers[inst.Dest] = vm.memory[baseAddr+index]
```

**Результат:** Полная execution программ с массивами
**Файл:** `minzc/pkg/mirvm/vm.go`

---

### QW-5: Error Messages с Line Numbers ⭐⭐
**Effort:** 2-3 часа | **Impact:** HIGH (DX)

Сейчас ошибки cryptic без позиции:
```
error: unknown type 'foo'
```

Нужно:
```
error: examples/test.minz:15:10: unknown type 'foo'
```

**Файл:** `minzc/pkg/semantic/analyzer.go`

---

### QW-6: Document Actual Feature Status ⭐
**Effort:** 2 часа | **Impact:** MEDIUM (Trust)

Обновить README и docs чтобы claims соответствовали реальности:
- Pattern matching: "парсится, codegen частичный"
- Generics: "запаркованы, используйте overloading"
- Nested functions: "не работают"

---

## Quick Wins Summary

| ID | Task | Effort | Impact | ROI |
|----|------|--------|--------|-----|
| QW-1 | MIR array access | 1h | HIGH | ⭐⭐⭐⭐⭐ |
| QW-2 | MIR struct fields | 30m | HIGH | ⭐⭐⭐⭐⭐ |
| QW-3 | MIR params | 15m | MEDIUM | ⭐⭐⭐⭐ |
| QW-4 | VM array handlers | 1h | HIGH | ⭐⭐⭐⭐⭐ |
| QW-5 | Error line numbers | 3h | HIGH | ⭐⭐⭐⭐ |
| QW-6 | Docs sync | 2h | MEDIUM | ⭐⭐⭐ |

**Total Quick Wins:** ~8 часов = MIR полностью работает + error messages

---

## Mid Wins (1-5 дней каждый)

Существенные улучшения, требующие планирования.

### MW-1: Hand-Written Parser (Replace Tree-sitter) ⭐⭐⭐⭐⭐
**Effort:** 2-4 недели | **Impact:** CRITICAL

**Проблема:** Tree-sitter может сожрать 60GB+ RAM на больших файлах

**Решение:** Recursive descent parser на чистом Go:
- O(n) memory usage
- Zero dependencies
- Full control over error recovery

**План:**
1. Week 1: Lexer (`pkg/parser/rdp/lexer.go`)
2. Week 2: Expression parser (precedence climbing)
3. Week 3: Statement/Declaration parser
4. Week 4: Metafunctions, inline asm

**Файлы:** Новый `minzc/pkg/parser/rdp/`

---

### MW-2: Nested Functions ⭐⭐⭐
**Effort:** 2-3 дня | **Impact:** HIGH

7+ примеров не компилируются из-за этого.

**Реализация:**
1. Semantic: Track nested scope
2. IR: Closure capture (или просто inline)
3. Codegen: Emit nested как отдельные функции с mangled names

**Файл:** `minzc/pkg/semantic/analyzer.go`

---

### MW-3: Pattern Matching Codegen ⭐⭐⭐
**Effort:** 3-4 дня | **Impact:** HIGH

Сейчас: Парсится, но codegen частичный

**Нужно:**
- `case x { A => B, C => D }` → jump table или цепочка CP/JR
- Exhaustiveness checking
- Guard expressions

**Файл:** `minzc/pkg/codegen/z80.go`

---

### MW-4: LLVM Backend Fix ⭐⭐
**Effort:** 2-3 дня | **Impact:** MEDIUM

Текущий статус: Invalid IR syntax

**Проблемы:**
- Incorrect type annotations
- Missing function signatures
- SSA violations

**Файл:** `minzc/pkg/codegen/llvm_backend.go`

---

### MW-5: Module stdlib Completion ⭐⭐⭐
**Effort:** 1 неделя | **Impact:** HIGH

Текущее: 10 модулей работают, но не все функции

**Нужно:**
- `math.abs()`, `math.min()`, `math.max()`
- `string.split()`, `string.trim()`
- `collections.List`, `collections.Map`

**Файлы:** `stdlib/`

---

### MW-6: Self Parameter in impl Blocks ⭐⭐⭐
**Effort:** 2-3 дня | **Impact:** HIGH

```minz
impl Vec2 {
    fun length(self) -> i16 { ... }  // ← не работает
}
```

**Файл:** `minzc/pkg/semantic/analyzer.go`

---

## Mid Wins Summary

| ID | Task | Effort | Impact | Priority |
|----|------|--------|--------|----------|
| MW-1 | Hand-written parser | 2-4 weeks | CRITICAL | P0 |
| MW-2 | Nested functions | 3 days | HIGH | P1 |
| MW-3 | Pattern matching | 4 days | HIGH | P1 |
| MW-4 | LLVM backend | 3 days | MEDIUM | P2 |
| MW-5 | stdlib completion | 1 week | HIGH | P1 |
| MW-6 | Self parameter | 3 days | HIGH | P1 |

---

## Long Wins (2+ недель каждый)

Стратегические улучшения для v1.0.

### LW-1: LSP Server ⭐⭐⭐⭐
**Effort:** 2-3 недели | **Impact:** MASSIVE (Adoption)

**Фичи:**
- Autocomplete
- Go to definition
- Hover documentation
- Error highlighting
- Code actions (fix imports, etc.)

**Технологии:** `gopls`-style, или `tower-lsp` pattern

**ROI:** Без LSP язык трудно использовать в IDE

---

### LW-2: DAP Debugger Integration ⭐⭐⭐
**Effort:** 2-3 недели | **Impact:** HIGH

**Фичи:**
- Breakpoints
- Step through code
- Variable inspection
- Call stack view

**Интеграция:** MZV + VS Code Debug Adapter

**Файлы:** `minzc/pkg/dap/` (уже начато)

---

### LW-3: Full Generics with Monomorphization ⭐⭐
**Effort:** 3-4 недели | **Impact:** MEDIUM

**Сейчас:** Парсится, не работает

**План:**
1. Type parameter parsing (уже есть)
2. Constraint checking
3. Monomorphization pass (generate concrete functions)
4. Type inference for generic calls

**Alternative:** Crystal-style Type(T) уже задокументировано

---

### LW-4: Package Manager ⭐⭐
**Effort:** 2-3 недели | **Impact:** MEDIUM

```bash
mz get github.com/user/minz-lib
mz add math/random@1.0.0
```

**Компоненты:**
- Package registry (GitHub-based?)
- Dependency resolution
- Lock file

---

### LW-5: WASM Playground ⭐⭐⭐
**Effort:** 2 недели | **Impact:** HIGH (Marketing)

Online editor где можно:
1. Write MinZ code
2. Compile to WASM/JS
3. Run in browser
4. Share snippets

**Tech:** Monaco editor + mz compiled to WASM

---

### LW-6: MZA Assembler → 95% Coverage ⭐⭐
**Effort:** 3-4 недели | **Impact:** MEDIUM

Сейчас: 1-2% success rate

**План (из TODO_MZA.md):**
1. Phase 1: Memory LD instructions → 15-25%
2. Phase 2: Table-driven encoder → 40-60%
3. Phase 3: Complete instructions → 80-90%
4. Phase 4: SjASMPlus compatibility → 95%+

---

## Long Wins Summary

| ID | Task | Effort | Impact | Priority |
|----|------|--------|--------|----------|
| LW-1 | LSP Server | 3 weeks | MASSIVE | P0 |
| LW-2 | DAP Debugger | 3 weeks | HIGH | P1 |
| LW-3 | Full Generics | 4 weeks | MEDIUM | P3 |
| LW-4 | Package Manager | 3 weeks | MEDIUM | P3 |
| LW-5 | WASM Playground | 2 weeks | HIGH | P2 |
| LW-6 | MZA 95% | 4 weeks | MEDIUM | P2 |

---

## Recommended Roadmap

### Week 1-2: Quick Wins Sprint
```
Day 1: QW-1 + QW-2 + QW-3 (MIR parser complete)
Day 2: QW-4 (VM handlers)
Day 3: QW-5 (Error messages)
Day 4: QW-6 (Docs sync)
Day 5: Testing + Polish
```
**Result:** MIR layer полностью работает, ошибки читаемы

### Week 3-6: Mid Wins Sprint
```
Week 3: MW-1 начало (Parser lexer + expressions)
Week 4: MW-1 продолжение (statements + declarations)
Week 5: MW-2 + MW-6 (Nested functions + self)
Week 6: MW-3 (Pattern matching)
```
**Result:** Parser без OOM, основные фичи работают

### Week 7-12: Long Wins Sprint
```
Week 7-9: LW-1 (LSP Server)
Week 10-11: LW-5 (WASM Playground)
Week 12: LW-2 начало (DAP Debugger)
```
**Result:** IDE support, online demo

---

## Impact Matrix

```
                    EFFORT
              Low         High
         ┌─────────┬─────────┐
    High │ QW-1,2,4│ MW-1    │  ← DO FIRST
         │ QW-5    │ LW-1    │
IMPACT   ├─────────┼─────────┤
         │ QW-3,6  │ MW-2,3,5│  ← DO NEXT
    Low  │         │ MW-4    │
         │         │ LW-3,4  │  ← DO LATER
         └─────────┴─────────┘
```

---

## Total Effort Estimate

| Category | Total Effort | Expected Impact |
|----------|-------------|-----------------|
| Quick Wins | ~8 hours | +15% compilation, DX++ |
| Mid Wins | ~6 weeks | +10% compilation, OOM fix |
| Long Wins | ~12 weeks | Adoption++, Community++ |

---

## Top 5 Priorities (если времени мало)

1. **QW-1,2,3,4** — MIR fixes (6-7 часов) → VM работает полностью
2. **QW-5** — Error line numbers (3 часа) → DX улучшается
3. **MW-1** — Hand-written parser (2-4 недели) → OOM решена
4. **MW-2** — Nested functions (3 дня) → 7+ примеров заработают
5. **LW-1** — LSP Server (3 недели) → Adoption возможен

---

*"Quick wins first, then parser, then LSP. Everything else is nice-to-have."*

---

**Файл создан:** 2026-02-04
**Автор:** Claude Opus 4.5
