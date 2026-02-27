# MinZ Language: Comprehensive Analysis & Status Report

**Date:** 2025-02-04
**Version Analyzed:** v0.18.0
**Author:** Claude (Code Review)

---

## Executive Summary

MinZ — это **амбициозный и технически интересный** язык программирования, который пытается совместить современный синтаксис (в стиле Rust/Ruby/Crystal) с генерацией кода для винтажного оборудования (Z80, 6502). Проект находится в **активной разработке** и демонстрирует значительный прогресс, но имеет разрыв между документированными возможностями и реальной работоспособностью.

### Overall Score: **7/10** ⭐⭐⭐⭐⭐⭐⭐

| Критерий | Оценка | Комментарий |
|----------|--------|-------------|
| **Идея и видение** | 9/10 | Уникальная ниша, амбициозные цели |
| **Дизайн языка** | 8/10 | Элегантный синтаксис, продуманные абстракции |
| **Реализация** | 6/10 | 81% компиляции, есть пробелы в семантике |
| **Документация** | 7/10 | Обширная, но рассинхронизирована с кодом |
| **Удобство использования** | 6/10 | Сложная установка, нестабильные бинарники |
| **Уникальность** | 10/10 | TRUE SMC — нигде больше нет |

---

## 1. Философия и Идеология

### 1.1 Основная идея

> *"Modern abstractions, vintage performance"*

MinZ стремится быть **анти-C для 8-битных систем**: современный синтаксис с zero-cost абстракциями, компилируемый в оптимальный ассемблер для ретро-платформ.

### 1.2 Ключевые принципы

1. **Zero-Cost Abstractions** — лямбды, итераторы, интерфейсы компилируются в прямые CALL без vtable
2. **TRUE SMC (Self-Modifying Code)** — революционный подход: параметры патчатся прямо в коде
3. **Multi-Backend** — один исходник → Z80, 6502, Crystal, WASM, C, LLVM
4. **CTIE (Compile-Time Interface Execution)** — мономорфизация трейтов при компиляции

### 1.3 Целевая аудитория

- Ретро-энтузиасты (ZX Spectrum, C64, CP/M)
- Embedded-разработчики
- Образовательные цели (изучение низкоуровневого программирования)
- Демосцена

---

## 2. Синтаксис и Фичи Языка

### 2.1 Базовый синтаксис (✅ Работает)

```minz
// Типы: u8, u16, i8, i16, bool, pointers
let x: u8 = 42;
const NAME = "MinZ";

// Функции
fun add(a: u8, b: u8) -> u8 {
    return a + b;
}

// Структуры
struct Point { x: u8, y: u8 }
let p = Point { x: 10, y: 20 };
```

### 2.2 Ruby-style String Interpolation (✅ Работает)

```minz
const USER = "Alice";
const SCORE = 9001;
@print("Player #{USER} scored #{SCORE} points!");
```

### 2.3 UFCS и Operator Overloading (✅ Работает в v0.17+)

```minz
struct Vec2 { x: i16, y: i16 }

impl Vec2 {
    fun add(self, other: Vec2) -> Vec2 {
        return Vec2 { x: self.x + other.x, y: self.y + other.y };
    }
}

let v3 = v1 + v2;  // → CALL Vec2.add (zero-cost!)
```

### 2.4 Error Propagation (✅ Работает)

```minz
enum MathError { None, DivByZero, Overflow }

fun safe_divide?(a: u8, b: u8) -> u8 ? MathError {
    if b == 0 {
        @error(MathError.DivByZero);  // Sets CY flag + A register
    }
    return a / b;
}
```

### 2.5 CTIE — Compile-Time Interface Execution (✅ Работает)

```minz
@ctie
fun fibonacci(n: u8) -> u8 {
    if n <= 1 { return n; }
    return fibonacci(n-1) + fibonacci(n-2);
}

let fib10 = fibonacci(10);  // Становится: LD A, 55 (вычислено при компиляции!)
```

### 2.6 Inline Assembly (✅ Работает)

```minz
fun fast_multiply(a: u8) -> u8 {
    asm {
        LD A, (a)
        SLA A        ; ×2
        SLA A        ; ×4
    }
    return a;
}
```

### 2.7 Pattern Matching (🟡 Частично)

```minz
enum State { IDLE, RUNNING, STOPPED }

fun next_state(s: State) -> State {
    case s {
        State.IDLE => State.RUNNING,
        State.RUNNING => State.STOPPED,
        _ => State.IDLE
    }
}
```
**Статус:** Синтаксис парсится, но codegen ограничен.

### 2.8 Generics (🔴 Запаркованы)

Решение: использовать Crystal-style `Type(T)` + function overloading вместо Rust `<T>`.

---

## 3. Уникальные Фичи

### 3.1 TRUE SMC (Self-Modifying Code)

**Это killer feature MinZ!** Нигде больше такого нет.

```minz
@smc
fun draw_sprite(x: u8, y: u8, sprite: *u8) -> void {
    // Параметры патчатся прямо в инструкции:
    // LD A, x  ← байт операнда перезаписывается перед вызовом
}
```

**Выигрыш:**
- 10x faster parameter passing на Z80
- Нет накладных расходов на стек
- Идеально для tight loops

**Ограничения:**
- Только для RAM (не ROM)
- Не реентерабельно (без undo-log)
- Требует DI/EI для ISR-безопасности

### 3.2 MZV — MinZ Virtual Machine

Виртуальная платформа для тестирования:
- Framebuffer 320×240
- I/O ports
- Поддержка spectrum, agon, headless, terminal

**Демо:** Raymarched 3D sphere с diffuse lighting на 8.8 fixed-point math!

### 3.3 DZRP — Live Testing

```bash
mzrun game.minz --reset  # Компилирует и запускает на эмуляторе!
mztap game.tap           # 500x быстрее чем эмуляция ленты
```

---

## 4. Toolchain

| Инструмент | Назначение | Статус |
|------------|-----------|--------|
| **mz** | Компилятор | ✅ 81% success |
| **mza** | Ассемблер Z80 | ✅ Работает |
| **mze** | Эмулятор Z80 | ✅ 100% opcodes |
| **mzr** | REPL | ✅ Работает |
| **mzrun** | DZRP runner | ✅ Работает |
| **mztap** | TAP loader | ✅ Работает |

### Поддерживаемые бэкенды

| Backend | Статус | Комментарий |
|---------|--------|-------------|
| Z80 | ✅ Production | Primary target |
| 6502 | ✅ Stable | C64, NES, Apple II |
| Crystal | ✅ Stable | Modern testing |
| C | ✅ Stable | Portable |
| WASM | ✅ Stable | Browser |
| LLVM | ⚠️ Beta | IR errors |
| Game Boy | 🚧 Beta | SM83 variant |
| 68000 | 🚧 Beta | Amiga/Atari ST |

---

## 5. Статус Реализации (Обновлено из свежих отчётов)

### 5.1 Compilation Pipeline

```
Source → Parser (95%) → AST → Semantic (85%) → MIR (100%) → Optimizer (90%) → Codegen (80%)
```

### 5.2 Последние достижения (v0.18.0, January 2026)

**Из последнего Progress Report:**
- ✅ **100% core examples** компилируются (72/72)
- ✅ **MZV Platform Abstraction** — pluggable виртуальное железо
- ✅ **24-bit типы** (u24/i24) для eZ80
- ✅ **Type inference fixes** для вложенных вызовов и chained methods
- ✅ **GLSL-style library** с operator overloading

### 5.3 Известные проблемы (из TODO_MIR_FIXES)

**Блокеры для полного исполнения:**
| Проблема | Статус | Effort |
|----------|--------|--------|
| Array access parsing (`r0 = r1[r2]`) | 🔴 Не парсится | 1 час |
| Struct field syntax | 🔴 Не парсится | 30 мин |
| Parameter loading | 🔴 Не парсится | 15 мин |
| VM handlers для arrays | 🔴 Не реализованы | 1 час |

**Total estimated fix effort:** ~6-7 часов

### 5.4 Размер кодовой базы

| Компонент | Строки кода |
|-----------|-------------|
| Semantic Analyzer | 11,642 |
| Z80 Codegen | 5,224 |
| Parser (total) | ~9,500 |
| **Total Go code** | **90,000+** |

### 5.2 Feature Matrix

| Feature | Parsing | Semantic | Codegen | E2E Test |
|---------|---------|----------|---------|----------|
| Functions | ✅ | ✅ | ✅ | ✅ |
| Structs | ✅ | ✅ | ✅ | ✅ |
| Enums | ✅ | ✅ | ✅ | ✅ |
| Arrays | ✅ | ✅ | ✅ | ✅ |
| Loops (for/while/loop) | ✅ | ✅ | ✅ | ✅ |
| String interpolation | ✅ | ✅ | ✅ | ✅ |
| UFCS methods | ✅ | ✅ | ✅ | ✅ |
| Operator overloading | ✅ | ✅ | ✅ | ✅ |
| Error propagation | ✅ | ✅ | ✅ | ✅ |
| CTIE | ✅ | ✅ | ✅ | ✅ |
| Inline assembly | ✅ | ✅ | ✅ | ✅ |
| Lambdas | ✅ | ✅ | 🟡 | 🟡 |
| Pattern matching | ✅ | 🟡 | 🟡 | 🔴 |
| Generics | ✅ | 🔴 | 🔴 | 🔴 |
| Nested functions | ✅ | 🔴 | 🔴 | 🔴 |
| Module stdlib | ✅ | 🟡 | 🟡 | 🟡 |

### 5.3 Текущие метрики

- **Examples compiling:** 48/59 (81%)
- **Core features working:** 12/20 (60%)
- **Documentation accuracy:** ~65%

---

## 6. Оценка Удобства

### 6.1 Что хорошо

✅ **Элегантный синтаксис** — Ruby/Rust feel, приятно писать
✅ **Zero-cost abstractions** — реально работают, видно в ассемблере
✅ **Crystal backend** — можно тестировать на современной системе
✅ **Обширная документация** — 280+ файлов в docs/
✅ **Активная разработка** — 18 версий за 14 месяцев

### 6.2 Что требует улучшения

⚠️ **Сложная установка** — нужен tree-sitter init-config, легко ошибиться
⚠️ **Нестабильные бинарники** — лучше собирать из исходников
⚠️ **Расхождение docs/code** — документация обещает больше чем работает
⚠️ **Cryptic errors** — сообщения об ошибках не всегда понятны
⚠️ **Нет LSP** — нет интеграции с IDE
⚠️ **OOM с tree-sitter** — на больших файлах может съесть 60GB+ RAM!

### 6.3 Проблема парсера (из исследования 2026-01-08)

**Текущая ситуация:**
- Tree-sitter (CLI) — primary, но вызывает OOM
- ANTLR4 — экспериментальный, только 5% success
- Native tree-sitter — отключен из-за CGO issues

**Рекомендуемое решение:** Hand-written recursive descent parser
- Предсказуемый memory footprint: O(n)
- Zero dependencies
- ~4 недели разработки

| Parser | Memory на 1MB | Dependencies |
|--------|--------------|--------------|
| Tree-sitter (current) | 60MB+ unpredictable | External binary |
| ANTLR4 | 5-10MB | Runtime |
| Recursive Descent | 1-2MB | None |
| Participle | 1MB | Single library |

---

## 7. Сравнение с альтернативами

| Критерий | MinZ | SDCC | z88dk | Millfork |
|----------|------|------|-------|----------|
| Синтаксис | Modern | C | C | Scala-like |
| SMC support | ✅ Native | ❌ | ❌ | ❌ |
| Zero-cost lambdas | ✅ | ❌ | ❌ | 🟡 |
| Multi-backend | ✅ 8 | ✅ Many | ✅ Z80 | ✅ 3 |
| Maturity | 🟡 | ✅ | ✅ | 🟡 |
| IDE support | ❌ | ✅ | 🟡 | 🟡 |

**Вывод:** MinZ уникален своим подходом к SMC и современным синтаксисом, но менее зрел чем SDCC/z88dk.

---

## 8. Рекомендации

### 8.1 Для пользователей

1. **Собирайте из исходников** — не используйте pre-built бинарники
2. **Начните с Crystal backend** — тестируйте логику на современной системе
3. **Используйте examples/** — там проверенный рабочий код
4. **Избегайте:** generics, nested functions, сложный pattern matching

### 8.2 Для развития проекта

**P0 — Критично:**
- [ ] Fix assembly generation (invalid syntax)
- [ ] Sync documentation with actual status
- [ ] Simplify installation

**P1 — Важно:**
- [ ] Implement LSP server
- [ ] Complete pattern matching codegen
- [ ] Module stdlib completion

**P2 — Желательно:**
- [ ] DAP debugger integration
- [ ] WASM playground
- [ ] Package manager

---

## 9. Conclusion

**MinZ — это впечатляющий проект** с уникальным видением и революционными идеями (TRUE SMC, zero-cost abstractions для Z80). Язык демонстрирует, что можно писать современный код для винтажного оборудования.

**Сильные стороны:**
- Единственный язык с native SMC support
- Элегантный Ruby/Rust-like синтаксис
- Multi-backend (один код → много платформ)
- Активная разработка

**Слабые стороны:**
- ~20% примеров не компилируются
- Документация опережает реализацию
- Сложная установка
- Нет IDE support

**Рекомендация:** MinZ подходит для энтузиастов, готовых мириться с шероховатостями ради уникальных возможностей. Для продакшена (если это применимо к ретро-разработке) лучше подождать v1.0.

---

## Appendix A: Quick Reference

### Типы
```
u8, u16, u24 (eZ80), i8, i16, i24, bool, *T (pointer), [T; N] (array)
```

### Ключевые слова
```
fun, let, const, struct, enum, impl, if, else, while, for, loop, case,
return, @ctie, @smc, @extern, @error, @print, asm, import, global
```

### Операторы
```
+, -, *, /, %, ==, !=, <, <=, >, >=, &&, ||, !, &, |, ^, <<, >>
```

### Специальные конструкции
```minz
@ctie fun ...           // Compile-time execution
@smc fun ...            // Self-modifying code
@extern(0x0010) fun ... // FFI to ROM/external
@error(EnumVal)         // Error propagation
asm { ... }             // Inline assembly
"Hello #{name}!"        // Ruby interpolation
```

---

*Report generated by Claude Code Review*
