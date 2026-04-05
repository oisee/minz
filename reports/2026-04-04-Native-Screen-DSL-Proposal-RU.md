# Proposal: Нативный `@screen` DSL без `screen_gen.go`

Дата: 2026-04-04

## Кратко

Текущий `@screen` полезен как bootstrap, но архитектурно это временный слой:

- DSL объявлен в языке,
- но реализован как special-case generator в Go,
- layout и semantics зашиты в compiler helper,
- а не выражены через нормальную метапрограмму и промежуточное представление.

Правильная цель:

- `@screen` должен стать обычной compile-time metafunction capability,
- работающей на структурированном AST/block IR,
- с нативным screen/UI builder API,
- без прямого string emission из Go.

`screen_gen.go` надо рассматривать как совместимый переходный слой, а не как финальную архитектуру.

---

## Что не так сейчас

### 1. Специальный случай в compiler

Сейчас `@screen` живёт не как обычная пользовательская метафункция, а как встроенный generator path в Go.

Это плохо потому что:

- язык показывает пользователю одну модель,
- а сам compiler пользуется другой, привилегированной,
- и реальные возможности meta-layer не проверяются на таком же сложном кейсе.

### 2. String-emission вместо структуры

`screen_gen.go` генерирует Nanz source строками.

Минусы:

- хрупкость formatting/escaping,
- сложнее анализировать и оптимизировать,
- трудно делать composition,
- легко плодить скрытые ad hoc соглашения.

Это нормальный bootstrap trick, но плохой долговременный foundation.

### 3. Layout knowledge зашит в generator

Сейчас decisions вроде:

- где title bar,
- где label,
- где button row,
- как рендерить status bar,
- как располагать table,

живут в Go-коде генератора.

Это означает:

- UI semantics не являются first-class данными,
- пользователь не может естественно переопределять layout,
- DSL остаётся template-like, а не composable.

### 4. DSL слишком keyword-template, а не composable

Сейчас есть набор фиксированных форм:

- `field`
- `int`
- `button`
- `table`
- `column`

Но почти нет:

- вложенности,
- section/group semantics,
- row/column/layout hints,
- локальной композиции.

Это делает DSL рабочим, но не “приятным”.

---

## Цель

Нужен нативный meta-DSL, где:

1. block body доступен как структурированные nodes, а не через хрупкий ad hoc parsing
2. метафункция может строить screen IR/builder-объекты, а не печатать строки
3. builtin `@screen` не нужен для выразительности, только максимум как sugar/compat layer
4. layout можно выражать в DSL, а не только в compiler helper
5. backend-специфичный render остаётся в `tui_*` / screen runtime, а не в мета-генераторе

---

## Правильная архитектура

### Слой A: Meta Block IR

Нужно дать compile-time code доступ к нормальному structured block representation:

- список nodes
- `keyword`
- positional args
- named args
- optional nested block

Минимальная модель:

```nanz
type MetaNode = struct {
    keyword: ^u8
    argc: u8
    // arg accessors
    // kwarg accessors
    // child block access
}
```

Не обязательно как literal struct в user code, но semantics должны быть именно такими.

Минимальный API:

- `block_len() -> u16`
- `node_keyword(i) -> ^u8`
- `node_arg_str(i, k) -> ^u8`
- `node_arg_int(i, k) -> u16`
- `node_has_kw(i, name) -> u8`
- `node_kw_str(i, name) -> ^u8`
- `node_kw_int(i, name) -> u16`
- `node_child_len(i) -> u16`
- `node_child_keyword(i, j) -> ^u8`

Критично:

- nested blocks должны быть first-class,
- иначе красивый DSL всё равно не построить.

### Слой B: Screen/UI IR

Нужен не string emitter, а screen builder / IR.

Например:

```nanz
type ScreenSpec

fun screen_new(title: ^u8) -> ScreenSpec
fun screen_add_field(self: ^ScreenSpec, label: ^u8, length: u8, def: ^u8) -> void
fun screen_add_int(self: ^ScreenSpec, label: ^u8, def: u16) -> void
fun screen_add_button(self: ^ScreenSpec, label: ^u8, key: u8) -> void
fun screen_begin_section(self: ^ScreenSpec, title: ^u8) -> void
fun screen_end_section(self: ^ScreenSpec) -> void
fun screen_emit(self: ^ScreenSpec) -> void
```

`screen_emit()` может:

- либо порождать HIR/MIR nodes,
- либо порождать Nanz source как transitional step,
- но это уже будет происходить из normal meta API, а не из compiler builtin.

### Слой C: Renderer Runtime

Runtime слой остаётся нормальным:

- `tui_*`
- `Screen`
- `widget`
- `render`

То есть meta-layer описывает screen,
а runtime-layer его исполняет/рисует.

Это правильное разделение обязанностей.

---

## Как должен выглядеть DSL

### Минимальный target style

```nanz
@screen("Material Report") {
    field "Material", 18, "*"
    field "Plant", 4
    int "Count", 10
    actions {
        button "Execute", F8
        button "Back", F3
    }
}
```

### Следующий уровень

```nanz
@screen("Material Report") {
    section "Selection" {
        field "Material", 18, "*"
        field "Plant", 4
        int "Count", 10
    }

    section "Preview" {
        table "Items", rows: 8 {
            column "MATNR", 18
            column "MAKTX", 24
            column "QTY",   6
        }
    }

    actions {
        button "Execute", F8
        button "Back", F3
    }
}
```

Это уже ближе к “приятному DSL”, потому что здесь появляются:

- hierarchy
- grouping
- readable intent
- меньше template feeling

---

## Что не надо делать

### 1. Не тащить ещё больше special logic в `screen_gen.go`

Плохо:

- ещё 20 keywords,
- ещё больше hardcoded layout policy,
- ещё больше one-off parsing logic в Go.

Это ухудшит migration path.

### 2. Не делать сразу giant generic GUI system

Нужен не “универсальный UI framework на все случаи”, а:

- компактный form/screen DSL,
- который хорошо ложится на TUI / retro targets.

### 3. Не делать shell-like emit DSL

Плохо было бы заменить текущий string emission на просто более модный string emission.

Нужно именно:

- structured meta input,
- structured UI builder,
- а не другой синтаксис печати строк.

---

## Migration Plan

### Phase 0: Freeze bootstrap status

Надо явно зафиксировать:

- `screen_gen.go` = bootstrap compatibility layer
- new features туда добавлять только по крайней необходимости

### Phase 1: Expand native meta APIs

Добавить в compile-time meta layer:

- nested block access
- typed kwarg access
- predictable node traversal

Это полезно не только для `@screen`, но и для других DSL.

### Phase 2: Introduce ScreenSpec builder

Сделать нативный builder API:

- создать screen spec
- добавлять fields/buttons/tables/sections
- финализировать в generated program fragment

На этом этапе можно ещё временно внутри `screen_emit()` пользоваться source emission, но уже не через compiler builtin path.

### Phase 3: Reimplement `@screen` in userland/meta-Nanz

Сделать proof-of-concept:

- `fun @screen(title: ^u8) -> void { ... }`
- без `screen_gen.go`
- только на normal meta primitives

### Phase 4: Keep builtin only as sugar or remove

Если userland/meta implementation зрелая:

- builtin path либо выпиливается,
- либо остаётся thin compatibility alias.

---

## Quick Wins

Даже до полной миграции можно сделать 3 полезных шага:

1. объявить `screen_gen.go` transitional layer в docs
2. прекратить добавлять туда новые semantic special-cases без крайней нужды
3. расширить meta API именно в сторону nested block traversal

Это уже уменьшит архитектурный долг.

---

## Риски

### 1. Meta API может стать слишком сырым

Если дать только low-level block walkers, пользовательские DSL останутся неудобными.

Поэтому нужен не только raw AST access, но и builder layer.

### 2. Full IR emission может оказаться дорогим по implementation effort

Поэтому переходный вариант допустим:

- structured ScreenSpec
- потом source emission внутри `screen_emit()`

Но важно, чтобы это было уже userland/meta path, а не compiler special case.

### 3. Можно случайно потерять backward compatibility

Поэтому migration должна сохранять:

- старый `@screen` surface syntax,
- пока новая реализация не стабилизируется.

---

## Recommended Decision

Нормальное решение сейчас:

1. признать `screen_gen.go` bootstrap hack
2. не расширять его aggressively
3. сделать nested block meta APIs
4. сделать `ScreenSpec` / UI builder layer
5. переписать `@screen` как нормальную native metafunction

Это лучший путь, потому что он:

- убирает special-case magic из compiler,
- делает сам язык сильнее,
- даёт реально composable DSL,
- и при этом не требует big-bang rewrite.

---

## Verdict

Правильная конечная форма `@screen`:

- не Go-side template expander,
- не hardcoded compiler builtin,
- а обычный user-visible meta-DSL, построенный на нормальном meta block IR и screen builder API.

`screen_gen.go` стоит сохранить только как временный совместимый мост.
