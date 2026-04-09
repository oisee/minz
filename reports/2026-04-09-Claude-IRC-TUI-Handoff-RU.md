# Claude Handoff: IRC Client, TUI, `@screen`

Дата: 2026-04-09

## Что прочитать сначала

Сначала прочитай предыдущий архитектурный report:

- [`2026-04-09-Bruto-IDE-Lessons-For-MinZ-TUI-and-Screen-RU.md`](/home/alice/dev/minz-vir/reports/2026-04-09-Bruto-IDE-Lessons-For-MinZ-TUI-and-Screen-RU.md)

Полезный более ранний companion report:

- [`2026-04-04-Native-Screen-DSL-Proposal-RU.md`](/home/alice/dev/minz-vir/reports/2026-04-04-Native-Screen-DSL-Proposal-RU.md)

Они нужны, чтобы не перепутать:

- проблемы raw TUI runtime,
- проблемы конкретного `irc_client`,
- и долгосрочную архитектуру `@screen` / `Screen`.

## Краткий вывод

`irc_client.nanz` сейчас не проверяет `@screen` как систему.

Он использует только raw TUI primitives из `tui.render`:

- `tui_clear`
- `tui_goto`
- `tui_color`
- `tui_putch`
- `tui_puts`
- `tui_read_key`

То есть текущий чёрный экран / странное поведение в `mzv` не надо интерпретировать как "сломана архитектура `@screen`". Это либо:

- баг raw TUI path в `mzv`,
- баг логики самого IRC клиента,
- либо просто неудобная immediate-mode схема рисования.

## Где смотреть

Исходник клиента:

- [`examples/nanz/irc_client.nanz`](/home/alice/dev/minz-vir/examples/nanz/irc_client.nanz)

Низкий TUI слой:

- [`stdlib/tui/render.nanz`](/home/alice/dev/minz-vir/stdlib/tui/render.nanz)

Средний Screen API:

- [`stdlib/tui/screen.nanz`](/home/alice/dev/minz-vir/stdlib/tui/screen.nanz)

Документация по TUI слоям:

- [`stdlib/tui/README.md`](/home/alice/dev/minz-vir/stdlib/tui/README.md)

MZV host-side TUI implementation:

- [`minzc/cmd/mzv/tui_host.go`](/home/alice/dev/minz-vir/minzc/cmd/mzv/tui_host.go)

## Что именно не так с `irc_client`

### 1. Он сидит на raw `tui_*`, а не на `Screen` / `@screen`

Это важнейшая вещь.

`irc_client` не использует:

- `Screen.init`
- `Screen.add_*`
- `Screen.render`
- `@screen(...)`

Он вручную рисует UI в immediate-mode стиле.

Следствие:

- мы пока не знаем, удобен ли `Screen` для chat/log/list приложений,
- мы знаем только, что forms DSL есть,
- а живые приложения пока естественно тянутся к голым `tui_*`.

### 2. Архитектурный gap: forms vs live apps

Текущий `@screen` / `Screen` хорошо подходит для:

- selection screens
- forms
- кнопок
- таблиц как статических layout-блоков

Но IRC-клиент — это другой класс UI:

- status bar
- scrolling log
- input line
- asynchronous network updates
- cursor/line redraw

Сейчас для таких программ нет такого же удобного middle-level API.

### 3. `status_msg()` и `draw_input_line()` рисуют напрямую

См. [`irc_client.nanz`](/home/alice/dev/minz-vir/examples/nanz/irc_client.nanz):

- `status_msg(msg)`
- `draw_input_line()`
- `scroll_msg()`

Они напрямую эмитят курсор, цвет и символы.

Это working style, но:

- трудно переиспользовать,
- трудно тестировать,
- трудно переносить на другие backends без риска subtle behavior drift.

### 4. Current loop is already `while 1 == 1`

В текущем состоянии файла `main()` уже использует:

```nanz
while 1 == 1 {
    poll_network()
    poll_keyboard()
}
```

То есть старый hypothesis про `loop {}` как immediate culprit в этом checkout уже не актуален.

### 5. `poll_keyboard()` uses `tui_read_key()` directly

Это значит:

- поведение клиента сильно зависит от runtime semantics `tui_read_key()`,
- любые различия между `mzv`, `mze`, CP/M и host-stdin path сразу влияют на app behavior.

Для IRC/TUI apps это усиливает потребность в отдельной, более semantic input abstraction.

## Что уже выглядит НЕ главным виновником

### `tui_goto()` host mapping в `mzv`

В [`minzc/cmd/mzv/tui_host.go`](/home/alice/dev/minz-vir/minzc/cmd/mzv/tui_host.go)
`tui_goto(x, y)` делает:

```go
fmt.Fprintf(out, "\033[%d;%dH", y+1, x+1)
```

То есть:

- логические координаты 0-based
- ANSI координаты 1-based

Это выглядит правильно.

Следовательно:

- `status_msg(0, 0)` не off-screen сам по себе,
- black screen надо искать выше по стеку, а не сразу валить на `tui_goto`.

## Правильная интерпретация текущей ситуации

`irc_client` сейчас говорит нам не "сломана `@screen`", а вот что:

1. raw TUI используется активно
2. `Screen`/`@screen` ещё не стал естественным путём для non-form interactive apps
3. для chat/log/list/status-input интерфейсов нужен middle layer богаче, чем current form DSL

Это ценный архитектурный сигнал.

## Что стоит исследовать дальше

### A. Не трогать сразу `@screen` ради IRC

Не надо насильно натягивать form DSL на IRC.

Сначала надо понять, какой intermediate API нужен именно для live text apps:

- `StatusBar`
- `LogView`
- `InputLine`
- maybe `ListView`

### B. Проверить raw runtime issue отдельно от architecture

Если цель локальная и прагматичная:

- отдельно добить конкретный black-screen/runtime problem в `mzv`
- не смешивать это с redesign `@screen`

### C. Подумать о screen/view layer между `Screen` и `tui_*`

Вероятная правильная эволюция:

- forms остаются на `@screen`
- live apps получают lightweight view API

Например:

- `StatusBar.set_text(...)`
- `LogView.append(...)`
- `InputLine.edit(...)`

Это уже больше похоже на usable middle layer для IRC/TUI apps.

## Что НЕ делать

- не объявлять по `irc_client`, что `@screen` провалился
- не переписывать `irc_client` сразу на текущий forms DSL
- не путать raw-host runtime bugs с мета-архитектурой

## Практическая задача для тебя

Если будешь продолжать по IRC/TUI линии, разделяй работу на два класса:

### 1. Runtime debugging

Цель:

- понять, почему именно под `mzv` экран может оставаться чёрным / неинформативным

Здесь смотри:

- `minzc/cmd/mzv/tui_host.go`
- stdin/render loop behavior
- конкретные ANSI sequences from app

### 2. Architecture/design

Цель:

- понять, какой middle layer нужен между raw `tui_*` и high-level forms DSL для non-form apps

Это уже не про баг, а про следующий полезный API layer.

## Bottom line

`irc_client` показал не провал `@screen`, а пробел в нашей TUI story:

- forms path у нас уже есть,
- raw primitives у нас уже есть,
- а удобного live-app middle layer пока почти нет.

Это и есть главный takeaway.
