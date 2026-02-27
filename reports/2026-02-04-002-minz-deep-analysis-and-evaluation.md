# MinZ: Глубокий Анализ и Оценка Языка

**Дата:** 2026-02-04
**Версия:** v0.18.0
**Автор:** Claude Opus 4.5 (Code Review)

---

## 1. Что такое MinZ?

### 1.1 Определение

**MinZ** — это экспериментальный компилируемый язык программирования, предназначенный для разработки программ для винтажного оборудования (Z80, 6502) и современных платформ (Crystal, WASM). Язык сочетает современный синтаксис в стиле Ruby/Rust с генерацией высокооптимизированного машинного кода.

### 1.2 Слоган и Миссия

> *"Modern abstractions, vintage performance"*
> *"Write modern code. Deploy everywhere. From 1978 Z80 to 2026 Crystal."*

**Миссия:** Сделать разработку для ретро-платформ такой же приятной, как современное программирование, сохраняя при этом полный контроль над железом и максимальную производительность.

---

## 2. Философия и Идеология

### 2.1 Основные принципы

| Принцип | Описание | Реализация |
|---------|----------|------------|
| **Zero-Cost Abstractions** | Высокоуровневые конструкции без накладных расходов | Лямбды → прямые CALL, интерфейсы без vtable |
| **Developer Happiness** | Ruby-style синтаксис, приятное программирование | String interpolation, UFCS, overloading |
| **TRUE SMC** | Самомодифицирующийся код как first-class feature | Параметры патчатся в immediates (3-10x быстрее) |
| **Multi-Backend** | Один исходник — много платформ | 8 бэкендов: Z80, 6502, Crystal, WASM, C... |
| **CTIE** | Compile-Time Interface Execution | Мономорфизация трейтов при компиляции |

### 2.2 Позиционирование

MinZ — это **"анти-C для 8-битных систем"**:

| Аспект | C / SDCC | MinZ |
|--------|----------|------|
| Синтаксис | 1972, устаревший | 2026, современный |
| SMC | Вручную в asm | Native, автоматический |
| Абстракции | Макросы, препроцессор | Zero-cost lambdas, interfaces |
| String | Null-terminated | Length-prefixed (25-40% быстрее) |
| Error handling | errno, возврат -1 | `?` operator, CY flag ABI |

### 2.3 Целевая аудитория

1. **Ретро-энтузиасты** — ZX Spectrum, C64, CP/M разработчики
2. **Демосцена** — создание демо и intro
3. **Embedded-разработчики** — понимание низкоуровневого программирования
4. **Образование** — изучение компиляторов и оптимизации

---

## 3. Синтаксис и Фичи Языка

### 3.1 Базовый синтаксис

```minz
// Типы данных
let x: u8 = 42;           // 8-bit unsigned
let y: u16 = 1000;        // 16-bit unsigned
let z: i16 = -100;        // 16-bit signed
let flag: bool = true;    // Boolean
let ptr: *u8 = &buffer;   // Pointer

// Функции (fun или fn)
fun add(a: u8, b: u8) -> u8 {
    return a + b;
}

// Структуры
struct Point { x: u8, y: u8 }
let p = Point { x: 10, y: 20 };

// Глобальные переменные
global counter: u8 = 0;
```

### 3.2 Ruby-style String Interpolation

```minz
const NAME = "Alice";
const SCORE = 9001;
@print("Player #{NAME} scored #{SCORE}!");  // → "Player Alice scored 9001!"
```

### 3.3 UFCS и Operator Overloading (v0.17+)

```minz
struct Vec2 { x: i16, y: i16 }

impl Vec2 {
    fun add(self, other: Vec2) -> Vec2 {
        return Vec2 { x: self.x + other.x, y: self.y + other.y };
    }

    fun eq(self, other: Vec2) -> bool {
        return self.x == other.x && self.y == other.y;
    }
}

// Использование:
let v3 = v1 + v2;       // → CALL Vec2.add (zero-cost!)
if v1 == v2 { ... }     // → CALL Vec2.eq
let v4 = v1.add(v2);    // UFCS: эквивалентно Vec2.add(v1, v2)
```

### 3.4 Error Propagation

```minz
enum FileError { None, NotFound, Permission }

fun read_file?(path: u8) -> u8 ? FileError {
    if path == 0 {
        @error(FileError.NotFound);  // Sets CY flag + A register
    }
    return path;
}

// Caller:
let data = read_file?(5);  // CY flag indicates success/failure
```

### 3.5 CTIE (Compile-Time Interface Execution)

```minz
@ctie
fun fibonacci(n: u8) -> u8 {
    if n <= 1 { return n; }
    return fibonacci(n-1) + fibonacci(n-2);
}

let fib10 = fibonacci(10);  // → LD A, 55 (вычислено при компиляции!)
```

### 3.6 Inline Assembly

```minz
fun wait_vblank() -> void {
    asm {
        EI
        HALT
    }
}
```

### 3.7 TRUE SMC (Self-Modifying Code)

```minz
@smc
fun draw_sprite(x: u8, y: u8, sprite: *u8) -> void {
    // x, y патчатся как immediate операнды в инструкции
    // 3-10x быстрее чем стек!
}
```

---

## 4. Уникальные Фичи (Killer Features)

### 4.1 TRUE SMC — Нигде больше нет!

MinZ — единственный язык с native поддержкой Self-Modifying Code:

| Традиционный вызов | TRUE SMC | Выигрыш |
|-------------------|----------|---------|
| PUSH params, CALL | Patch imm, JP | 3-10x faster |
| 44+ T-states | 7-20 T-states | Memory bandwidth |
| Stack allocation | Zero allocation | No stack growth |

**Спецификация SMC** (из MinZ SPEC v0.1):
- SMC-якоря: первое употребление параметра как immediate
- PATCH-TABLE: таблица адресов для патчинга
- Undo-log для рекурсии/реентерабельности
- ABI режимы: `@abi(smc)`, `@abi(slot)`, `@abi(reg)`

### 4.2 Zero-Cost Lambda Iterators

```minz
numbers.iter()
    .map(|x| x * 2)
    .filter(|x| x > 5)
    .forEach(|x| print_u8(x));
```

Компилируется в прямые CALL с DJNZ оптимизацией — никаких vtables или heap allocations!

### 4.3 MZV Virtual Machine

Виртуальная платформа для тестирования:
- Framebuffer 320×240
- I/O ports
- Поддержка spectrum, agon, headless, terminal
- **Демо:** Raymarched 3D sphere с diffuse lighting на 8.8 fixed-point math!

### 4.4 DZRP Live Testing

```bash
mzrun game.minz --reset  # Compile → Assemble → Upload → Run на эмуляторе!
mztap game.tap           # 500x быстрее чем эмуляция ленты
```

---

## 5. Архитектура Компилятора

### 5.1 Pipeline

```
Source (.minz)
    ↓
Parser (Tree-sitter, 95% accuracy)
    ↓
AST (Abstract Syntax Tree)
    ↓
Semantic Analyzer (85% coverage)
    ↓
MIR (Mid-level IR, 100% complete!)
    ↓
Optimizer (90% - SMC, peephole, DCE)
    ↓
Codegen (8 backends)
    ↓
Output (.a80, .cr, .c, .wat, ...)
```

### 5.2 Статистика кодовой базы

| Компонент | LOC | Статус |
|-----------|-----|--------|
| Semantic Analyzer | ~11,600 | 85% |
| Z80 Codegen | ~5,200 | Stable |
| Parser | ~9,500 | 95% |
| MIR/IR | ~4,000 | 100% |
| Optimizer | ~3,500 | 90% |
| **Total Go code** | **~90,000** | Active |

### 5.3 Поддерживаемые бэкенды

| Backend | Статус | Примечание |
|---------|--------|------------|
| Z80 | Production | Primary target |
| 6502 | Stable | C64, NES, Apple II |
| Crystal | Stable | Modern testing |
| C | Stable | Portable |
| WASM | Stable | Browser |
| LLVM | Beta | IR syntax issues |
| Game Boy | Beta | SM83 variant |
| 68000 | Beta | Amiga/Atari ST |

---

## 6. Статус Реализации Фич

### 6.1 Feature Matrix

| Feature | Parsing | Semantic | Codegen | E2E | Оценка |
|---------|:-------:|:--------:|:-------:|:---:|:------:|
| Functions | ✅ | ✅ | ✅ | ✅ | 100% |
| Structs | ✅ | ✅ | ✅ | ✅ | 100% |
| Enums | ✅ | ✅ | ✅ | ✅ | 100% |
| Arrays | ✅ | ✅ | ✅ | ✅ | 100% |
| Loops (for/while/loop) | ✅ | ✅ | ✅ | ✅ | 100% |
| String interpolation | ✅ | ✅ | ✅ | ✅ | 100% |
| UFCS methods | ✅ | ✅ | ✅ | ✅ | 100% |
| Operator overloading | ✅ | ✅ | ✅ | ✅ | 100% |
| Error propagation | ✅ | ✅ | ✅ | ✅ | 95% |
| CTIE | ✅ | ✅ | ✅ | ✅ | 95% |
| Inline assembly | ✅ | ✅ | ✅ | ✅ | 100% |
| TRUE SMC | ✅ | ✅ | ✅ | ✅ | 95% |
| Lambdas | ✅ | ✅ | 🟡 | 🟡 | 80% |
| Pattern matching | ✅ | 🟡 | 🟡 | 🔴 | 50% |
| Generics | ✅ | 🟡 | 🟡 | 🟡 | 70% |
| Nested functions | ✅ | 🔴 | 🔴 | 🔴 | 10% |
| Module stdlib | ✅ | 🟡 | 🟡 | 🟡 | 75% |

### 6.2 Compilation Success Rate

| Версия | Success Rate | Комментарий |
|--------|-------------|-------------|
| v0.1.0 | ~2% | Genesis |
| v0.4.0 | 46% | SMC Revolution |
| v0.10.0 | 75-80% | Lambdas |
| v0.15.4 | 81% | Error propagation |
| v0.18.0 | **100% core** | 72/72 core examples |

### 6.3 Известные проблемы

**P0 Critical (Fixed):**
- ✅ Tree-sitter OOM (60GB RAM) → documented, workarounds available
- ✅ ANSI codes in output → fixed
- ✅ Binary architecture mismatch → build from source recommended

**P1 High (In Progress):**
- 🟡 Local/nested functions — не работают
- 🟡 LLVM backend — invalid IR syntax
- 🟡 Pattern matching guards — базовые только
- 🟡 Module aliases (`import x as y`) — не полностью

**P2 Medium:**
- Generic type parameters — ограниченные
- Testing framework — не реализован
- LSP server — не реализован

---

## 7. Toolchain

| Tool | Назначение | Статус |
|------|-----------|--------|
| **mz** | Компилятор | Production |
| **mza** | Z80 Assembler (100% coverage!) | Production |
| **mze** | Z80 Emulator | Production |
| **mzr** | Interactive REPL | Stable |
| **mzrun** | DZRP Remote Runner | Stable |
| **mztap** | Instant TAP Loader | Stable |

---

## 8. Standard Library

| Модуль | Функции | Статус |
|--------|---------|--------|
| `std/core` | Базовые типы | ✅ |
| `std/io` | I/O операции | ✅ |
| `std/print` | @print implementation | ✅ |
| `std/mem` | memcpy, memset | ✅ |
| `std/error` | Error handling | ✅ |
| `zx/screen` | Pixel graphics | ✅ |
| `zx/input` | Keyboard | ✅ |
| `math/fast` | Sin/cos/sqrt tables | ✅ |
| `glsl/` | Raymarcher, SDFs | ✅ |
| `cpm/bdos` | CP/M calls | ✅ |

**Total:** 10+ рабочих модулей, ~2,400 строк game-ready кода

---

## 9. Оценка Удобства

### 9.1 Сильные стороны

| Аспект | Оценка | Комментарий |
|--------|--------|-------------|
| Синтаксис | **9/10** | Ruby/Rust feel, очень приятно писать |
| Документация | **7/10** | 280+ файлов, но рассинхронизирована |
| Toolchain | **8/10** | Всё в одном пакете, zero dependencies |
| Error messages | **5/10** | Часто cryptic, нет line numbers |
| IDE support | **3/10** | VSCode extension есть, но без LSP |
| Installation | **5/10** | Требуется tree-sitter init-config |

### 9.2 Developer Experience

**Плюсы:**
- Одна команда `mz file.minz -o out.a80` для компиляции
- Crystal backend для быстрого тестирования логики
- DZRP для live testing на эмуляторе
- Inline assembly когда нужно

**Минусы:**
- Tree-sitter setup сложный (OOM на больших файлах)
- Некоторые фичи документированы но не работают
- Нет debugger integration (DAP planned)

---

## 10. Сравнение с альтернативами

| Критерий | MinZ | SDCC | z88dk | Millfork |
|----------|------|------|-------|----------|
| **Синтаксис** | Modern Ruby/Rust | C (1972) | C | Scala-like |
| **SMC support** | ✅ Native | ❌ | ❌ | ❌ |
| **Zero-cost lambdas** | ✅ | ❌ | ❌ | 🟡 |
| **String interpolation** | ✅ | ❌ | ❌ | ❌ |
| **Multi-backend** | ✅ 8 | ✅ Many | ✅ Z80 | ✅ 3 |
| **Maturity** | 🟡 | ✅ | ✅ | 🟡 |
| **IDE support** | 🔴 | ✅ | 🟡 | 🟡 |
| **Community** | Small | Large | Large | Medium |

---

## 11. Overall Score

### 11.1 По критериям

| Критерий | Оценка | Вес | Взвешенная |
|----------|--------|-----|------------|
| Идея и видение | 9/10 | 15% | 1.35 |
| Дизайн языка | 8/10 | 20% | 1.60 |
| Реализация | 7/10 | 25% | 1.75 |
| Документация | 7/10 | 10% | 0.70 |
| Удобство использования | 6/10 | 15% | 0.90 |
| Уникальность | 10/10 | 15% | 1.50 |
| **ИТОГО** | | 100% | **7.8/10** |

### 11.2 Финальная оценка

# **7.8 / 10** ⭐⭐⭐⭐⭐⭐⭐⭐

**Вердикт:** MinZ — это впечатляющий экспериментальный язык с революционными идеями (TRUE SMC, zero-cost abstractions), уникальным позиционированием и активной разработкой. Несмотря на ~20% примеров с проблемами компиляции и расхождение docs/code, язык вполне пригоден для энтузиастов и демосцены.

---

## 12. Рекомендации

### 12.1 Когда использовать MinZ

**Да:**
- Разработка для ZX Spectrum, C64, CP/M
- Демосцена и intro
- Изучение низкоуровневого программирования
- Эксперименты с SMC и оптимизацией

**Подождать:**
- Production-критичные проекты
- Нужна IDE интеграция (LSP)
- Требуются generics в полном объёме
- Large codebase (>10K LOC)

### 12.2 Для development team

**P0 (критично для adoption):**
1. Fix tree-sitter OOM / implement hand-written parser
2. Sync documentation with actual feature status
3. Add line numbers to error messages

**P1 (важно):**
1. Implement LSP server for IDE support
2. Complete pattern matching codegen
3. Fix LLVM backend

**P2 (желательно):**
1. DAP debugger integration
2. WASM playground
3. Package manager

---

## 13. Примеры кода

### Hello World
```minz
fun main() -> void {
    @print("Hello from MinZ!");
    loop { asm { EI; HALT } }
}
```

### Fibonacci
```minz
fun fibonacci(n: u8) -> u16 {
    if n <= 1 { return n; }
    let mut a: u16 = 0;
    let mut b: u16 = 1;
    for i in 2..n+1 {
        let temp = a + b;
        a = b;
        b = temp;
    }
    return b;
}
```

### Game Loop
```minz
import stdlib.graphics.screen;
import stdlib.input.keyboard;

global x: u8 = 128;
global y: u8 = 96;

fun main() -> void {
    clear_screen();
    loop {
        wait_frame();
        clear_pixel(x, y);
        if is_key_pressed(KEY_LEFT)  { x = x - 1; }
        if is_key_pressed(KEY_RIGHT) { x = x + 1; }
        set_pixel(x, y);
    }
}
```

---

## 14. Заключение

**MinZ демонстрирует, что можно писать современный код для винтажного оборудования.**

Язык имеет уникальные killer features (TRUE SMC, zero-cost lambdas), которых нет ни в одном другом компиляторе для Z80/6502. 18 версий за 14 месяцев показывают активное развитие и commitment автора.

Для энтузиастов ретро-программирования MinZ — это глоток свежего воздуха после десятилетий C и ассемблера. Да, есть шероховатости, но потенциал огромен.

**Recommendation:** Try it, contribute, be part of the revolution!

---

*"MinZ: Where Modern Dreams Meet Vintage Reality"*

---

## Appendix A: Quick Reference Card

### Types
```
u8, u16, u24 (eZ80), i8, i16, i24, bool, *T, [T; N]
```

### Keywords
```
fun/fn, let, const, struct, enum, impl, if, else, while, for, loop,
case, return, @ctie, @smc, @extern, @error, @print, asm, import, global
```

### Operators
```
+, -, *, /, %, ==, !=, <, <=, >, >=, &&, ||, !, &, |, ^, <<, >>
```

### Metafunctions
```
@print("text")      - Output text
@ctie               - Compile-time execution
@smc                - Self-modifying code
@extern(addr)       - FFI binding
@error(EnumVal)     - Error propagation
@define("tmpl",a,b) - Text substitution
@minz[[[...]]]      - Compile-time block
@lua[[[...]]]       - Lua scripting
```

### CLI
```bash
mz file.minz -o out.a80       # Compile to Z80
mz file.minz -b crystal       # Compile to Crystal
mza file.a80 -o file.bin      # Assemble
mze file.bin -v               # Run in emulator
mzrun file.minz --reset       # Live testing via DZRP
```

---

*Report generated by Claude Opus 4.5 Code Review*
*2026-02-04 | MinZ v0.18.0*
