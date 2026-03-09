# Report #044 — Proposal: HIR → WAT (WASM) + HIR → QBE

**Date**: 2026-03-09
**Type**: Research proposal + implementation plan

---

## 0. Зачем два новых бэкенда?

Текущее состояние: один production бэкенд (Z80), один сломанный (C).
Два новых бэкенда дают разные вещи:

| Бэкенд | Что даёт | Сложность |
|--------|----------|-----------|
| HIR → WAT | Верификация MIR2 семантики + браузерный playground | 2–3 дня |
| HIR → QBE | Нативный x86-64/ARM64 без LLVM зависимости | 5–7 дней |

Оба генерируют **текстовые форматы** (`fmt.Fprintf`) — никакого C++ API, никакого CGO.

---

## 1. HIR → WAT: WASM Text Format

### 1.1 Что такое WAT

WAT (WebAssembly Text format) — читаемый текстовый вид WASM байткода.
`wat2wasm` (уже установлен: `/Users/alice/dev/bin/wat2wasm 1.0.35`) компилирует его в бинарный `.wasm`.

```wat
(module
  (func $gcd (export "gcd") (param $a i32) (param $b i32) (result i32)
    (local $tmp i32)
    (block $exit
      (loop $loop
        ;; if b == 0: break
        (br_if $exit (i32.eqz (local.get $b)))
        ;; tmp = a % b; a = b; b = tmp
        (local.set $tmp (i32.rem_u (local.get $a) (local.get $b)))
        (local.set $a  (local.get $b))
        (local.set $b  (local.get $tmp))
        (br $loop)
      )
    )
    (local.get $a)
  )
)
```

### 1.2 Маппинг HIR → WAT

HIR структурный — WASM структурный. Маппинг почти 1:1:

```
HIR node              WAT construct
────────────────────────────────────────────────────────────
hir.Module            (module ...)
hir.Func              (func $name (param ...) (result ...) ...)
hir.Param             (param $name i32)          ; всё через i32
hir.RetTy             (result i32) / ничего
hir.VarDeclStmt       (local $name i32)
hir.AssignStmt        local.set $name + expr
hir.ReturnStmt        expr + return
hir.IfStmt            (if (then ...) (else ...))
hir.WhileStmt         (block $brk (loop $cont (br_if $brk ¬cond) body (br $cont)))
hir.ForRangeStmt      то же + local counter
hir.BreakStmt         br $brk
hir.ContinueStmt      br $cont
hir.SwitchStmt        chain of (if (then ...) (else ...))
hir.StoreStmt         i32.store (ptr) (val)
hir.BinExpr +         i32.add
hir.BinExpr -         i32.sub
hir.BinExpr *         i32.mul
hir.BinExpr /         i32.div_u / i32.div_s
hir.BinExpr %         i32.rem_u / i32.rem_s
hir.BinExpr &         i32.and
hir.BinExpr |         i32.or
hir.BinExpr ^         i32.xor
hir.BinExpr <<        i32.shl
hir.BinExpr >>        i32.shr_u / i32.shr_s
hir.BinExpr ==        i32.eq
hir.BinExpr !=        i32.ne
hir.BinExpr <         i32.lt_u / i32.lt_s
hir.BinExpr <=        i32.le_u / i32.le_s
hir.BinExpr >         i32.gt_u / i32.gt_s
hir.BinExpr >=        i32.ge_u / i32.ge_s
hir.UnaryExpr -       i32.sub (i32.const 0) x
hir.UnaryExpr ~       i32.xor x (i32.const -1)
hir.UnaryExpr !       i32.eqz x
hir.CallExpr          call $fname
hir.IntLitExpr        i32.const N
hir.IndexExpr arr[i]  i32.load (i32.add base (i32.shl i stride))
DerefExpr *p          i32.load ptr
AddrOf                → глобальный оффсет в linear memory
```

**Ключевой момент — типы:**
WASM не имеет `i8`/`i16` как value types. Всё идёт через `i32`.
При присваивании u8 → добавляем `(i32.and (i32.const 255))` для трункации.
Для верификационных тестов это не проблема: `popcount(7)` вернёт `3` в `i32`.

### 1.3 Вопрос который ты задал: где I/O в WASM?

Это ключевой вопрос. Ответ: **WASM sandbox — прямого I/O нет совсем.**

WASM модуль — это изолированная функция без доступа к ОС.
Любой I/O приходит через **host functions** — функции которые хост (браузер/рантайм) предоставляет модулю через импорт:

```wat
(module
  ;; Импортируем putchar от хоста
  (import "env" "putchar" (func $putchar (param i32)))

  (func $hello (export "hello")
    (call $putchar (i32.const 72))   ;; 'H'
    (call $putchar (i32.const 105))  ;; 'i'
  )
)
```

Три сценария для нас:

#### Сценарий A: Верификация тестов (никакого I/O не нужно!)

Самый простой случай. Тест-функция принимает аргументы и возвращает значение.
Хост (Go/Python/JS) вызывает функцию и проверяет результат:

```bash
# wasmtime CLI напрямую вызывает экспортированную функцию:
$ wasmtime run --invoke popcount out.wasm 7
3

$ wasmtime run --invoke gcd out.wasm 48 18
6

$ wasmtime run --invoke fibonacci out.wasm 10
55
```

Это работает прямо сейчас — никакого I/O не нужно. Наши 53 MIR2 теста
все имеют форму "вызов функции → проверить возвращаемое значение".

```go
// Тест в Go через exec:
func runWasm(t *testing.T, wasmFile, funcName string, args ...int) int {
    strArgs := make([]string, len(args))
    for i, a := range args { strArgs[i] = strconv.Itoa(a) }
    cmd := exec.Command("wasmtime", append([]string{"run", "--invoke", funcName, wasmFile}, strArgs...)...)
    out, err := cmd.Output()
    // ...
    result, _ := strconv.Atoi(strings.TrimSpace(string(out)))
    return result
}

// Использование:
got := runWasm(t, "popcount.wasm", "popcount", 7)
assert(t, got == 3)
```

#### Сценарий B: WASI — стандартный I/O для CLI программ

**WASI** (WebAssembly System Interface) — стандарт для системных вызовов.
`wasmtime` поддерживает WASI из коробки. Если WASM модуль импортирует WASI функции,
он получает доступ к stdin/stdout/файлам:

```wat
(module
  (import "wasi_snapshot_preview1" "fd_write"
    (func $fd_write (param i32 i32 i32 i32) (result i32)))
  ;; ... реализуем putchar через fd_write
)
```

Для наших тестов WASI не нужен — `--invoke` достаточно.
Для полноценного "запустить MinZ программу как WASM CLI" — WASI нужен, но это Phase 2.

#### Сценарий C: Браузерный playground (JavaScript host)

В браузере хост — это JavaScript. JS загружает `.wasm` и вызывает функции:

```javascript
const wasm = await WebAssembly.instantiateStreaming(fetch('out.wasm'), {
    env: {
        // предоставляем host functions если они нужны
        putchar: (c) => { output += String.fromCharCode(c); }
    }
});

// Вызываем функцию напрямую:
const result = wasm.instance.exports.fibonacci(10);
console.log(result); // 55
```

Для playground без I/O (просто вычисления) — даже `env` не нужен.
Пользователь вводит код в редакторе, нажимает "Run", браузер:
1. Отправляет Nanz код на сервер (или компилирует в WASM на сервере)
2. Загружает `.wasm`
3. Вызывает `main()` или указанную функцию
4. Показывает возвращаемое значение

#### Резюме по I/O:

```
Сценарий                    I/O механизм              Сложность
────────────────────────────────────────────────────────────────
Верификация тестов          нет I/O (--invoke)         ★☆☆☆☆
CLI программы               WASI (fd_write/fd_read)    ★★★☆☆
Браузерный playground       JS host functions          ★★☆☆☆
Полный CP/M эмулятор в JS   JS host (все BDOS calls)   ★★★★☆
```

### 1.4 Как тестировать WAT бэкенд

Три уровня:

**Уровень 1: wat2wasm валидация (уже можем)**
```bash
$ mz program.nanz --emit=wat > out.wat
$ wat2wasm out.wat -o out.wasm
# Если не упало — WAT синтаксически корректен
```

**Уровень 2: wasmtime --invoke (нужно установить wasmtime)**
```bash
$ brew install wasmtime   # или curl из wasmtime.dev
$ wasmtime run --invoke popcount out.wasm 7
3
```

**Уровень 3: Go тест, сравнение с Z80 эмулятором**
```go
// pkg/hirwat/wasm_test.go
func TestWasmMatchesZ80(t *testing.T) {
    cases := []struct{ fn string; args []int; want int }{
        {"popcount", []int{7}, 3},
        {"gcd", []int{48, 18}, 6},
        {"fibonacci", []int{10}, 55},
        {"abs_diff", []int{10, 3}, 7},
        {"clamp", []int{150, 10, 200}, 150},
    }
    for _, c := range cases {
        wasmGot := runWasm(t, c.fn, c.args...)
        z80Got  := runZ80(t, c.fn, c.args...)   // через MZE
        if wasmGot != z80Got {
            t.Errorf("%s(%v): WASM=%d Z80=%d — semantics diverge!", ...)
        }
    }
}
```

Это и есть **бесплатный оракул семантики MIR2**: если WASM и Z80 согласны на всех тестах, оба бэкенда корректны.

### 1.5 Структура нового пакета

```
minzc/pkg/hirwat/
    codegen.go      — основной кодогенератор (HIR → WAT текст)
    types.go        — mir2.Ty → WAT type string
    names.go        — санитизация идентификаторов для WAT
    codegen_test.go — тесты: генерация + wat2wasm валидация
```

```go
// API:
package hirwat

// Compile generates WAT text for the given HIR module.
// All functions are exported by their HIR name.
func Compile(m *hir.Module) (string, error)
```

Интеграция в `mz` CLI:
```bash
mz program.nanz --emit=wat         # выводит WAT на stdout
mz program.nanz --emit=wasm        # wat2wasm | бинарный .wasm
mz program.nanz -o program.wasm    # то же, в файл
```

### 1.6 Размер реализации

Оценка: **~350 строк Go**.

```
codegen.go:
  compileModule()       ~20 строк
  compileFunc()         ~40 строк
  compileBlock()        ~30 строк
  compileStmt()         ~80 строк  (switch по всем Stmt типам)
  compileExpr()         ~100 строк (switch по всем Expr типам)
  emitType()            ~20 строк
  имена/escape          ~30 строк
```

Самая сложная часть — `WhileStmt` с `break`/`continue`: нужно правильно именовать
`$brk` и `$cont` блоки для вложенных циклов. Решается стеком строк (как loopCtx в lower.go).

---

## 2. HIR → QBE

### 2.1 Что такое QBE

QBE — маленький (~20K строк C) компилятор-бэкенд из проекта cproc (Quentin Rameau).
Принимает собственный текстовый IR, выдаёт x86-64, ARM64, или RISC-V asm.
Потом `as` + `ld` дают нативный бинарник.

QBE спроектирован именно для таких компиляторов как MinZ: маленькое фронтенд → QBE → нативный код.
Используется: cproc (C компилятор), hare lang, myrddin lang.

```qbe
# QBE IR для gcd:
function w $gcd(w %a, w %b) {
@start
    jmp @loop
@loop
    %b_z =w ceqw %b, 0
    jnz %b_z, @done, @body
@body
    %tmp =w remu %a, %b
    %a   =w copy %b
    %b   =w copy %tmp
    jmp @loop
@done
    ret %a
}
```

### 2.2 Ключевое отличие от WAT: SSA + phi

QBE ожидает **SSA форму с phi nodes** (или copy-based SSA).
HIR — structured, not SSA. Поэтому нужна конвертация.

Два подхода:

**Подход A: mem2reg через alloca (проще)**
Каждая HIR переменная → QBE `alloc4` (стековый слот).
Чтение = `loadw`, запись = `storew`. QBE сам делает mem2reg.

```qbe
function w $clamp(w %x_arg, w %lo_arg, w %hi_arg) {
@entry
    # аллоцируем стековые слоты
    %x  =l alloc4 4
    %lo =l alloc4 4
    %hi =l alloc4 4
    storew %x_arg,  %x
    storew %lo_arg, %lo
    storew %hi_arg, %hi
    jmp @check_lo
@check_lo
    %xv  =w loadw %x
    %lov =w loadw %lo
    %lt  =w csltw %xv, %lov
    jnz %lt, @ret_lo, @check_hi
@ret_lo
    %r1 =w loadw %lo
    ret %r1
    # ...
}
```

Плюс: прямой маппинг, без SSA алгоритмов.
Минус: QBE должен убрать лишние load/store через mem2reg (он умеет).

**Подход B: версионирование переменных (правильнее)**
При каждом присваивании `x = expr` — создаём новое QBE имя `%x_v2`, `%x_v3`.
На merge points (после if/while) — QBE phi.

```qbe
@after_if
    %x_v3 =w phi @then_block %x_v1, @else_block %x_v2
```

Плюс: чистый SSA, оптимальный код.
Минус: ~100 строк конвертации, нужно отслеживать "текущую версию" каждой переменной.

**Рекомендация: начать с Подходом A** (alloca + mem2reg).
QBE делает mem2reg хорошо. Потом можно перейти на B если нужна максимальная производительность.

### 2.3 Маппинг HIR → QBE

```
HIR                   QBE
────────────────────────────────────────────────────
hir.Module            # QBE файл — просто конкатенация function/data блоков
hir.Func              function w $name(w %p1, w %p2, ...) { ... }
hir.VarDeclStmt       %v =l alloc4 4  +  storew init, %v
hir.AssignStmt        storew val, %var_slot
hir.ReturnStmt        ret val
hir.IfStmt            jnz cond, @then, @else + @merge
hir.WhileStmt         jmp @head + @head: jnz cond, @body, @exit
hir.ForRangeStmt      то же + counter alloc
hir.BreakStmt         jmp @exit_label
hir.ContinueStmt      jmp @head_label
hir.SwitchStmt        chain of jnz

mir2.TyU8/I8          w (word, 32-bit) — QBE не имеет i8
mir2.TyU16/I16        w
mir2.TyU32/I32        w
mir2.TyU64            l (long, 64-bit)
mir2.TyPtr            l (64-bit pointer на x86-64)
mir2.TyBool           w

BinExpr +             %r =w add %l, %r
BinExpr -             %r =w sub %l, %r
BinExpr *             %r =w mul %l, %r
BinExpr /             %r =w divu %l, %r  (или div для signed)
BinExpr %             %r =w remu %l, %r
BinExpr &             %r =w and %l, %r
BinExpr |             %r =w or  %l, %r
BinExpr ^             %r =w xor %l, %r
BinExpr <<            %r =w shl %l, %r
BinExpr >>            %r =w shru %l, %r  (или shr для signed)
BinExpr ==            %r =w ceqw %l, %r
BinExpr !=            %r =w cnew %l, %r
BinExpr <             %r =w cultw %l, %r (unsigned)
BinExpr <=            %r =w culew %l, %r
BinExpr >             %r =w cugtw %l, %r
BinExpr >=            %r =w cugew %l, %r
CallExpr              %r =w call $funcname(w %arg1, w %arg2)
IntLitExpr N          в QBE константы inline: add %x, 5 (нет отдельного const)
StoreStmt *p = v      storew %v, %p
DerefExpr *p          %r =w loadw %p
GlobalVar addr        data $name = { w value }  +  %r =l $name (глобальный адрес)
```

### 2.4 Как получить QBE

```bash
# Вариант 1: из исходников (рекомендуется, ~30 секунд)
git clone https://c9x.me/git/qbe.git /tmp/qbe
cd /tmp/qbe && make
sudo cp qbe /usr/local/bin/

# Вариант 2: homebrew (если есть формула)
brew install qbe

# Проверка:
qbe -h
```

### 2.5 Полный пайплайн после QBE

```bash
mz program.nanz --emit=qbe > out.qbe    # HIR → QBE IR
qbe -t amd64_apple out.qbe > out.s      # QBE → x86-64 asm (macOS)
# или:
qbe -t arm64 out.qbe > out.s            # QBE → ARM64 asm
as out.s -o out.o                       # asm → object
ld out.o -o out -e _main -lSystem       # link (macOS)
./out                                   # запуск
```

На Linux:
```bash
qbe -t amd64_sysv out.qbe > out.s
gcc out.s -o out -nostartfiles          # gcc берёт на себя линковку
./out
```

### 2.6 Тестирование QBE бэкенда

```go
// pkg/hirqbe/qbe_test.go
func TestQBEMatchesZ80(t *testing.T) {
    // Компилируем Nanz → QBE → нативный бинарник → запускаем
    // Сравниваем с Z80 результатами
    cases := []struct{ fn string; args []int; want int }{ ... }
    for _, c := range cases {
        qbeGot := runQBE(t, c.fn, c.args...)
        if qbeGot != c.want {
            t.Errorf("%s: QBE=%d want=%d", c.fn, qbeGot, c.want)
        }
    }
}

func runQBE(t *testing.T, funcName string, args ...int) int {
    // 1. mz → QBE IR (текст)
    // 2. qbe → asm
    // 3. as + ld → бинарник (или gcc wrapper)
    // 4. exec бинарника с аргументами через argv
    // 5. читаем exit code или stdout
}
```

**I/O в QBE/нативном коде**: обычный C ABI.
Нативный бинарник получает аргументы через `argc/argv` или через stdin.
Возвращает результат через exit code (для однобайтных значений) или stdout.

Для тестов проще всего: генерировать `main()` обёртку которая читает аргументы из argv,
вызывает тест-функцию, печатает результат через `write(1, ...)`:

```qbe
# Авто-генерируемый тестовый harness:
export function w $main(w %argc, l %argv) {
@entry
    # читаем argv[1] как число
    %arg1_ptr =l add %argv, 8    # argv[1]
    %arg1_str =l loadl %arg1_ptr
    %arg1     =w call $atoi(l %arg1_str)
    # вызываем тест-функцию
    %result   =w call $fibonacci(w %arg1)
    # exit(result) — QBE knows standard C functions via linkage
    call $printf(l $fmt_str, w %result)
    ret 0
}
data $fmt_str = { b "%d\n", b 0 }
```

---

## 3. Сравнительная таблица: WAT vs QBE

```
Характеристика        HIR → WAT                HIR → QBE
──────────────────────────────────────────────────────────────────
Сложность             ★★☆☆☆                   ★★★☆☆
Время реализации      2–3 дня                  5–7 дней
Зависимости           wat2wasm (есть)           qbe binary (нужно установить)
I/O                   host functions / WASI     C stdlib (printf, etc.)
Таргеты               браузер, wasmtime         x86-64, ARM64, RISC-V
SSA конвертация       не нужна                 нужна (alloca или phi)
Верификация MIR2      да (wasmtime --invoke)    да (нативный запуск)
Браузерный playground да                        нет
Нативная скорость     ~80% native               ~90% gcc -O1
Зрелость рантайма     wasmtime стабильный       QBE production (cproc использует)
Линковка              нет (sandboxed)           полный ELF/Mach-O
Тип I/O для тестов    wasmtime --invoke fname   argv + exit code / stdout
```

---

## 4. Рекомендуемый порядок реализации

### Спринт 1: WAT бэкенд + верификация (2–3 дня)

```
День 1:
  - pkg/hirwat/codegen.go: compileModule, compileFunc, compileBlock
  - Скалярные типы + VarDeclStmt + AssignStmt + ReturnStmt
  - Тест: fibonacci генерируется корректный WAT

День 2:
  - compileStmt: IfStmt, WhileStmt, ForRangeStmt
  - BreakStmt/ContinueStmt через loopCtx стек
  - compileExpr: все BinExpr, UnaryExpr, CallExpr, IntLitExpr
  - Тест: gcd, clamp, abs_diff

День 3:
  - StoreStmt, DerefExpr, IndexExpr, глобалы
  - --emit=wat флаг в mz CLI
  - Тестовый харнесс: wasmtime --invoke для 53 существующих тестов
  - Сравнение Z80 vs WASM результатов
```

### Спринт 2: QBE бэкенд + нативный x86 (5–7 дней)

```
День 1–2:
  - pkg/hirqbe/codegen.go: функции + базовые блоки (Подход A: alloca)
  - Скалярные выражения + арифметика

День 3–4:
  - Все statement types: if/while/for/switch/break/continue
  - CallExpr с правильным QBE call syntax

День 5:
  - Глобальные переменные как QBE data секции
  - Указатели: load/store через QBE l type

День 6:
  - qbe → as → ld пайплайн в Go (exec)
  - Тестовый харнесс: компиляция + запуск нативного бинарника

День 7:
  - Сравнение QBE vs Z80 на всех тестах
  - --emit=qbe флаг в mz CLI
  - Документация
```

---

## 5. Что нужно установить

```bash
# wasmtime (нужен для тестирования WAT):
curl https://wasmtime.dev/install.sh -sSf | bash
# или:
brew install wasmtime

# QBE (нужен для QBE бэкенда):
git clone https://c9x.me/git/qbe.git /tmp/qbe && cd /tmp/qbe && make && sudo cp qbe /usr/local/bin/

# wat2wasm уже есть: /Users/alice/dev/bin/wat2wasm 1.0.35 ✓
```

---

## 6. Пример: gcd через все три пути

**Nanz источник:**
```nanz
fun gcd(a: u8, b: u8) -> u8 {
    while b != 0 {
        var tmp: u8 = a % b
        a = b
        b = tmp
    }
    return a
}
```

**WAT выход (HIR → WAT):**
```wat
(module
  (func $gcd (export "gcd") (param $a i32) (param $b i32) (result i32)
    (local $a_slot i32)  (local $b_slot i32)  (local $tmp i32)
    (local.set $a_slot (local.get $a))
    (local.set $b_slot (local.get $b))
    (block $brk
      (loop $cont
        (br_if $brk
          (i32.eqz (i32.and (local.get $b_slot) (i32.const 255))))
        (local.set $tmp
          (i32.rem_u (local.get $a_slot) (local.get $b_slot)))
        (local.set $a_slot (local.get $b_slot))
        (local.set $b_slot (local.get $tmp))
        (br $cont)
      )
    )
    (i32.and (local.get $a_slot) (i32.const 255))
  )
)
```

**QBE выход (HIR → QBE):**
```qbe
function w $gcd(w %a_arg, w %b_arg) {
@entry
  %a =l alloc4 4
  %b =l alloc4 4
  storew %a_arg, %a
  storew %b_arg, %b
  jmp @while_head_0
@while_head_0
  %b_val =w loadw %b
  %b_masked =w and %b_val, 255
  %cond =w ceqw %b_masked, 0
  jnz %cond, @while_exit_0, @while_body_0
@while_body_0
  %av =w loadw %a
  %bv =w loadw %b
  %tmp_val =w remu %av, %bv
  %tmp =l alloc4 4
  storew %tmp_val, %tmp
  storew %bv, %a
  %tv =w loadw %tmp
  storew %tv, %b
  jmp @while_head_0
@while_exit_0
  %ret =w loadw %a
  %ret_masked =w and %ret, 255
  ret %ret_masked
}
```

**Z80 выход (существующий MIR2 бэкенд):**
```asm
gcd:
    CP 0             ; b != 0?
    RET Z
.gcd_loop:
    ; ... DJNZ loop ...
    RET
```

**Тест-оракул:**
```bash
# Все три должны дать 6:
wasmtime run --invoke gcd gcd.wasm 48 18      # → 6
./gcd_native 48 18                             # → 6
mze gcd.com 48 18                              # → 6 (через Z80 эмулятор)
```

---

## 7. Долгосрочно: браузерный playground

После WAT бэкенда — браузерный playground (1–2 дня дополнительно):

```
Архитектура:
  Редактор (Monaco/CodeMirror) → Nanz код
      ↓ POST /compile
  Сервер (Go) → mz --emit=wasm → .wasm байты
      ↓ response
  Браузер: WebAssembly.instantiate(bytes) → вызов функции → результат
```

Никакого сервера не нужно если скомпилировать сам компилятор в WASM
(MinZ компилятор → WASM → запускается в браузере). Но это Phase N.

Короткий путь: простой Go HTTP сервер, компилирует на стороне сервера, отдаёт `.wasm`.

---

*Proposal: реализация HIR → WAT + HIR → QBE. Оба пути дают верификацию MIR2 семантики через независимый оракул. WASM добавляет браузерный таргет, QBE — нативный x86-64/ARM64.*
