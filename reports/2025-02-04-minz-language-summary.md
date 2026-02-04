# MinZ: Краткое Описание Языка

## Что такое MinZ?

**MinZ** — это современный язык программирования для винтажного оборудования (Z80, 6502) и современных платформ (WASM, Crystal). Синтаксис вдохновлён Ruby, Rust и Crystal, но код компилируется в высокоэффективный ассемблер для 8-битных процессоров.

**Слоган:** *"Modern abstractions, vintage performance"*

---

## Ключевые Особенности

### 1. Современный Синтаксис

```minz
// Ruby-style string interpolation
let name = "Alice";
@print("Hello, #{name}!");

// Structs с dot-notation
struct Point { x: u8, y: u8 }
let p = Point { x: 10, y: 20 };
let sum = p.x + p.y;

// UFCS методы (zero-cost!)
let v3 = v1.add(v2);   // → прямой CALL, без vtable
let v4 = v1 + v2;      // → operator overloading
```

### 2. TRUE SMC (Self-Modifying Code)

Уникальная фича MinZ — параметры функций патчатся прямо в машинный код:

```minz
@smc
fun draw_pixel(x: u8, y: u8) -> void {
    // x и y встраиваются как immediate операнды
    // 10x быстрее чем передача через стек!
}
```

### 3. Compile-Time Execution (CTIE)

Функции выполняются во время компиляции:

```minz
@ctie
fun factorial(n: u8) -> u16 {
    if n <= 1 { return 1; }
    return n * factorial(n - 1);
}

let f10 = factorial(10);  // → LD HL, 3628800 (вычислено при компиляции!)
```

### 4. Error Propagation

Rust-style ошибки с ABI через CY flag:

```minz
enum FileError { NotFound, Permission }

fun read_file?(path: u8) -> u8 ? FileError {
    if path == 0 { @error(FileError.NotFound); }
    return path;
}
```

### 5. Inline Assembly

Прямой доступ к железу:

```minz
fun wait_vblank() -> void {
    asm {
        EI
        HALT
    }
}
```

### 6. Multi-Backend

Один исходник → много платформ:

| Платформа | Команда |
|-----------|---------|
| ZX Spectrum (Z80) | `mz game.minz -o game.a80` |
| Commodore 64 (6502) | `mz game.minz -b 6502` |
| Crystal (тестирование) | `mz game.minz -b crystal` |
| WebAssembly | `mz game.minz -b wasm` |
| C (портируемый) | `mz game.minz -b c` |

---

## Типы Данных

| Тип | Размер | Описание |
|-----|--------|----------|
| `u8` | 1 byte | Unsigned 8-bit |
| `i8` | 1 byte | Signed 8-bit |
| `u16` | 2 bytes | Unsigned 16-bit |
| `i16` | 2 bytes | Signed 16-bit |
| `u24` | 3 bytes | 24-bit (eZ80) |
| `bool` | 1 byte | Boolean |
| `*T` | 2 bytes | Pointer to T |
| `[T; N]` | N×sizeof(T) | Fixed array |

---

## Управляющие Конструкции

```minz
// Условия
if x > 10 {
    @print("big");
} else {
    @print("small");
}

// Циклы
for i in 0..10 { ... }      // range-based
while condition { ... }      // while
loop { ... }                 // infinite

// Pattern matching (частично)
case state {
    State.IDLE => State.RUNNING,
    _ => State.IDLE
}
```

---

## Что Работает (81%)

✅ Функции, структуры, перечисления
✅ Массивы и указатели
✅ Ruby string interpolation
✅ UFCS методы и operator overloading
✅ Error propagation (`@error`)
✅ CTIE (compile-time execution)
✅ Inline assembly
✅ TRUE SMC
✅ Все базовые типы
✅ Циклы for/while/loop

🟡 Lambdas (работают, оптимизация в процессе)
🟡 Pattern matching (парсится, codegen частичный)
🟡 Module stdlib (в разработке)

🔴 Generics (решено использовать function overloading)
🔴 Nested functions
🔴 LSP server

---

## Toolchain

```bash
mz file.minz -o out.a80     # Компилятор
mza file.a80 -o out.bin     # Ассемблер
mze file.bin                 # Эмулятор
mzrun file.minz --reset     # Live testing (DZRP)
mztap file.tap               # TAP loader (500x faster)
mzr                          # REPL
```

---

## Пример: Hello World

```minz
fun main() -> void {
    @print("Hello from MinZ!");

    loop {
        asm { EI; HALT }
    }
}
```

Компилируется в ~20 строк Z80 ассемблера.

---

## Пример: Game Loop

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
        if is_key_pressed(KEY_UP)    { y = y - 1; }
        if is_key_pressed(KEY_DOWN)  { y = y + 1; }

        set_pixel(x, y);
    }
}
```

---

## Установка

```bash
# 1. Зависимости
npm install -g tree-sitter-cli
tree-sitter init-config

# 2. Сборка из исходников (рекомендуется)
git clone https://github.com/oisee/minz.git
cd minz/minzc
go build -o mz cmd/minzc/main.go
sudo mv mz /usr/local/bin/

# 3. Проверка
mz --version
```

---

## Когда использовать MinZ?

**Да:**
- Разработка для ZX Spectrum, C64, CP/M
- Демосцена
- Изучение низкоуровневого программирования
- Эксперименты с SMC

**Подождать:**
- Production-критичные проекты
- Нужна IDE интеграция
- Требуются generics

---

## Ресурсы

- **Репозиторий:** github.com/oisee/minz
- **Документация:** 280+ файлов в `docs/`
- **Примеры:** 140+ в `examples/`
- **Версия:** v0.18.0

---

*MinZ: Where Modern Dreams Meet Vintage Reality™*
