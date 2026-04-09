# Proposal: Нативный TUI View DSL для MinZ/Nanz

Дата: 2026-04-09

## Кратко

Текущий `@screen` хорош как DSL для selection screens и форм, но он не покрывает
живые TUI-приложения уровня IRC, monitor, console, commander.

Нужен следующий слой:

- data/state остаются обычными структурами Nanz,
- layout и bindings описываются декларативно через метафункцию,
- runtime/Z80 получает маленький и предсказуемый набор view renderers.

Цель не заменить `tui_*`, а подняться над ними на один уровень.

---

## Что не хватает сейчас

Сейчас у нас есть:

1. raw TUI primitives
- `tui_goto`
- `tui_putch`
- `tui_puts`
- `tui_color`
- `tui_clear`
- `tui_read_key`

2. forms-oriented `Screen` / `@screen`
- `field`
- `int`
- `button`
- `table`

Этого хватает для selection screens, но не хватает для:

- status bar
- scrolling log
- nick/user list
- input line
- live redraw by state

Именно поэтому `irc_client.nanz` уходит напрямую в raw `tui_*`.

---

## Принцип

Правильный endgame:

- **Data in Nanz**
- **Layout in metafunction DSL**
- **Rendering in tiny runtime widgets**

То есть:

```text
App state -> View DSL -> Screen/View IR -> widget runtime -> tui_* -> backend
```

---

## Data model

Публичная модель должна быть естественной для пользователя:

- `array of structs`
- string buffers
- counters/lengths

Не `struct of arrays` как внешний API.

### Пример

```nanz
type NickItem = struct {
    nick: [16]u8
    flags: u8
    color: u8
}

type ChatLine = struct {
    kind: u8
    color: u8
    nick: [16]u8
    text: [96]u8
}

global status_text: [64]u8
global nick_items: [64]NickItem
global nick_count: u8
global chat_lines: [128]ChatLine
global chat_count: u8
global input_buf: [128]u8
global input_len: u8
```

`struct of arrays` может быть внутренней оптимизацией renderer'а позже, но не основным user-facing контрактом.

---

## Базовые view types

Минимальный полезный набор:

- `status`
- `log`
- `list`
- `input`

Этого уже хватает для:

- IRC client
- serial monitor
- build/log console
- simple file/command browser

Дополнительно потом:

- `tabs`
- `table`
- `panel`
- `menu`
- `scrollbar`

---

## Layout DSL

Нужны не только widgets, но и композиция.

Минимальные layout nodes:

- `hsplit`
- `vsplit`
- `panel`
- `spacer`

### Пример target syntax

```nanz
@screen("MinZ IRC") {
    status top bind status_text

    hsplit {
        log main bind chat_lines, chat_count
        list side width 18 bind nick_items, nick_count
    }

    input bottom prompt "> " bind input_buf, input_len
}
```

Это уже:

- компактно,
- composable,
- выразительно,
- похоже на то, как человек реально мыслит экраном.

---

## Widget binding semantics

### `status`

Bind to one text buffer:

```nanz
status top bind status_text
```

### `input`

Bind to mutable text buffer + length:

```nanz
input bottom prompt "> " bind input_buf, input_len
```

### `list`

Bind to array of structs + count:

```nanz
list side width 18 bind nick_items, nick_count
```

Runtime contract для `list`:

- item count
- item renderer policy
- optional selected index later

### `log`

Bind to array of chat/event lines + count:

```nanz
log main bind chat_lines, chat_count
```

Runtime contract для `log`:

- visible window
- append-only or ring semantics
- line renderer by `kind/color/nick/text`

---

## IR / builder layer

Метафункция не должна печатать строки напрямую из Go helper'а.

Нужен structured screen/view IR.

Минимально:

```nanz
type ScreenSpec
type ViewSpec

fun screen_new(title: ^u8) -> ScreenSpec
fun screen_add_status(...)
fun screen_add_log(...)
fun screen_add_list(...)
fun screen_add_input(...)
fun screen_begin_hsplit(...)
fun screen_begin_vsplit(...)
fun screen_end_split(...)
fun screen_emit(...)
```

Переходный этап допустим:

- `screen_emit()` ещё может порождать Nanz code,
- но уже через обычный meta API,
- а не через compiler-only `screen_gen.go` as privileged hack.

---

## Runtime contract

Нижний runtime слой должен быть маленьким.

### Rendering

- `render_status(view, data)`
- `render_log(view, data)`
- `render_list(view, data)`
- `render_input(view, data)`

### Input handling

- `input_handle_key(...)`
- `screen_handle_key(...)`
- later: focus management

### Geometry

Каждый view знает только:

- `x`
- `y`
- `w`
- `h`

Никакой backend-specific магии на уровне DSL.

---

## What lowers to Z80

На Z80 это не должно превращаться в monster framework.

Lowering goal:

- region loops
- simple line renderers
- predictable buffer traversal
- direct `tui_*` calls

То есть итоговые building blocks:

- `render_status_bar`
- `render_log_line`
- `render_list_item`
- `render_input_line`

Это очень важно: DSL должен быть expressive сверху, но lowering должен оставаться тупым и прозрачным снизу.

---

## Почему это лучше текущего raw style

Текущий raw IRC style:

- много `tui_goto`
- много ручного redraw
- трудно тестировать
- трудно переносить
- трудно переиспользовать

View DSL style:

- данные отделены от layout
- widgets отделены от backend primitives
- redraw строится из state
- код приложения становится короче и понятнее

---

## Почему это не то же самое, что текущий forms DSL

Forms DSL отвечает на вопрос:

- где поля и кнопки

Live-app TUI отвечает на другой вопрос:

- как показать evolving state and input regions continuously

Это разные usage classes.

Поэтому:

- forms DSL не надо ломать,
- но его нельзя считать достаточным для IRC/console/monitor apps.

---

## Migration path

### Phase 1

Stabilize raw runtime:

- `mzv` stdin handling
- `tui_read_key`
- deterministic TUI tests

### Phase 2

Add widget runtime without DSL revolution:

- `StatusBar`
- `LogView`
- `NickList`
- `InputLine`

Можно сначала даже без метафункции, просто через обычный Nanz API.

### Phase 3

Add Screen/View IR + metafunction bindings.

### Phase 4

Re-express `@screen` and future IRC client on top of that IR.

### Phase 5

Retire or minimize special Go-side generator paths.

---

## Example end state

```nanz
@screen("MinZ IRC") {
    status top bind status_text

    hsplit {
        log main bind chat_lines, chat_count
        list side width 18 bind nick_items, nick_count
    }

    input bottom prompt "> " bind input_buf, input_len
}
```

And the app logic remains just data mutation:

```nanz
append_chat_line(...)
append_nick(...)
set_status(...)
input_handle_key(...)
```

That is the right separation.

---

## Recommendation

Следующий design/implementation target должен быть не “улучшить forms DSL”, а:

- сделать minimal live-TUI widget runtime,
- затем поверх него построить native view DSL,
- и только потом переписать IRC client на этот слой.

Это самый красивый и самый реалистичный путь одновременно.
