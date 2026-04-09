# Bruto IDE: что это подсказывает для MinZ `@screen` и TUI

Дата: 2026-04-09

## Кратко

Статья про Bruto IDE полезна не как “вот так надо сделать IDE”, а как подтверждение
одной архитектурной мысли:

- верхний DSL / language layer должен зависеть от маленького абстрактного контракта,
- а не от конкретной низкоуровневой реализации.

Для MinZ это значит:

- наш трёхслойный вектор уже правильный,
- но `screen_gen.go` надо считать переходным bootstrap-слоем,
- а конечная цель — нативный `@screen`, который работает через обычную
  метасистему языка и structured screen/UI IR.

---

## Что у них устроено хорошо

В Bruto IDE есть очень чистое разделение:

1. framework crate
2. language implementation crate
3. tiny binary that просто соединяет одно с другим

Ключ в том, что их IDE зависит не от Pascal, а от маленького `Language` trait:

- `name()`
- `file_extension()`
- `sample_program()`
- `create_highlighter()`
- `build()`

То есть язык для IDE — это plugin through a narrow contract.

Это и есть главная сильная идея статьи.

---

## Что у нас уже есть аналогичного

У нас уже фактически есть такая же трёхслойная схема для TUI/screens.

### Слой 1: низкие TUI primitives

[`stdlib/tui/render.nanz`](/home/alice/dev/minz-vir/stdlib/tui/render.nanz)

Там живут:

- `tui_goto`
- `tui_color`
- `tui_putch`
- `tui_puts`
- `tui_clear`
- `tui_read_key`
- `tui_read_line`

Это backend-facing слой.

### Слой 2: Screen API / builder-like layer

[`stdlib/tui/screen.nanz`](/home/alice/dev/minz-vir/stdlib/tui/screen.nanz)

Там живёт UFCS/API уровень:

- `Screen.init`
- `Screen.add_field`
- `Screen.add_int`
- `Screen.add_button`
- `Screen.render`
- `Screen.show`

Это уже screen semantics, а не raw terminal ops.

### Слой 3: `@screen`

[`minzc/pkg/nanz/screen_gen.go`](/home/alice/dev/minz-vir/minzc/pkg/nanz/screen_gen.go)

Там живёт декларативный верхний слой:

- `field`
- `int`
- `button`
- `table`
- `column`

То есть формально архитектура “primitive -> screen -> DSL” у нас уже есть.

И это хорошо.

---

## Где у них урок для нас

Самое полезное из статьи — не конкретный UI, а форма зависимости.

У них:

- framework не знает Pascal
- Pascal знает framework contract
- binary only wires them

У нас долгосрочно должно стать так:

- runtime renderer не знает `@screen`
- `@screen` не знает CP/M/ZX/MZV details
- `@screen` знает только ScreenSpec / UI builder contract
- backend знает только `tui_*`

То есть наш аналог `Language` trait — это будущий screen builder contract.

---

## Где у нас сейчас архитектурный долг

Проблема не в том, что `@screen` вообще существует.
Проблема в том, как он сейчас реализован.

Сейчас:

- `@screen` — special-case builtin path в compiler
- [`screen_gen.go`](/home/alice/dev/minz-vir/minzc/pkg/nanz/screen_gen.go) печатает Nanz source строками
- layout policy зашита в generator
- это не настоящий обычный пользовательский meta-layer

Именно здесь мы слабее желаемой архитектуры.

То есть сегодня у нас:

- surface syntax already good
- layering idea already good
- implementation still privileged and template-like

---

## Что у нас уже сильнее, чем у них

Это важно: наш endgame потенциально интереснее, чем их.

Bruto решает задачу:

- IDE framework + language plugin

Мы целимся в:

- cross-backend declarative screen system inside the language itself

То есть если довести нашу линию до конца, получится не просто “TUI framework”,
а часть самой языковой metaprogramming story.

Особенно сильна формулировка из [`stdlib/tui/README.md`](/home/alice/dev/minz-vir/stdlib/tui/README.md):

- screen is data, not code

Если это сделать честно, то:

- `@screen` будет порождать structured screen data / IR,
- runtime будет рисовать его через `tui_*`,
- backends будут менять только primitives.

Это уже мощнее, чем hand-written IDE shell.

---

## Что именно стоит перенять из их архитектуры

### 1. Маленький контракт вместо special cases

Нам нужен маленький screen builder contract.

Примерно такой уровень:

- `screen_new(title)`
- `screen_add_field(...)`
- `screen_add_int(...)`
- `screen_add_button(...)`
- `screen_begin_section(...)`
- `screen_end_section(...)`
- `screen_emit()`

Это и есть наш аналог их `Language` trait.

### 2. Чёткое разделение ролей

Нужно отделить:

- screen description
- screen runtime behavior
- backend rendering primitives

Сейчас это местами уже так, но `screen_gen.go` ещё склеивает слои слишком рано.

### 3. Не протаскивать layout policy в compiler helper

Если layout rules живут в Go generator’е, это не composable system.

Layout policy должна жить:

- либо в ScreenSpec builder layer,
- либо в userland/meta implementation,
- но не как compiler privilege.

---

## Что не стоит слепо копировать

### 1. Не надо копировать “IDE-first” framing

Наша цель шире.

Нам не нужен framework только для IDE.
Нам нужен reusable language-native screen/TUI system.

### 2. Не надо увлекаться crate/plugin splitting ради splitting

Для MinZ суть не в количестве репозиториев.
Суть в правильной границе между:

- meta
- screen spec
- runtime
- backend

### 3. Не надо оставаться на template expansion

Это самая опасная ловушка.

Можно вдохновиться их trait boundary, но нельзя ответить на это
ещё более сложным `screen_gen.go`.

---

## Рекомендуемая целевая архитектура для MinZ

### Слой A: `tui_*` primitives

Остаётся как есть:

- portable backend contract
- CP/M / MZV / ZX / Agon / native adapters

### Слой B: ScreenSpec / UI IR

Новый честный промежуточный слой.

Он должен уметь описывать:

- title
- fields
- ints
- buttons
- labels
- tables
- sections / groups
- maybe layout hints

### Слой C: `Screen` runtime/builder

Либо текущий `Screen` evolve’ится в этот слой,
либо рядом появляется более structured builder API.

### Слой D: Native `@screen`

Конечная форма:

- не privileged Go generator
- а обычная compile-time metafunction
- работающая на block IR + ScreenSpec builder API

---

## Практический migration path

### Phase 0

Признать явно:

- [`screen_gen.go`](/home/alice/dev/minz-vir/minzc/pkg/nanz/screen_gen.go) — bootstrap hack

### Phase 1

Не расширять aggressively Go-side special generator.

То есть:

- минимум новых hardcoded keywords
- минимум новых layout special-cases

### Phase 2

Усилить native meta APIs:

- nested block traversal
- typed kwargs
- predictable node kinds

### Phase 3

Ввести structured ScreenSpec / builder contract.

### Phase 4

Переписать `@screen` поверх normal meta layer.

### Phase 5

Оставить builtin только как compatibility sugar или убрать.

---

## Вывод

Bruto IDE не даёт нам “готовый рецепт”, но хорошо подтверждает один принцип:

- мощная система строится вокруг маленького контракта между верхним слоем и нижним.

Для MinZ это означает:

- наш current 3-layer shape уже хорош,
- но реализация верхнего слоя пока слишком привилегированная,
- и следующий правильный шаг — не усложнять `screen_gen.go`,
- а двигаться к native meta-driven `@screen` через ScreenSpec/UI IR.

Именно тогда наш TUI/screen story станет не просто удобным bootstrap’ом,
а реальным доказательством силы языка.
