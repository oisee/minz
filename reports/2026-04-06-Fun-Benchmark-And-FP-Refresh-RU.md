# Fun / Bench / FP Refresh

**Дата:** 2026-04-06
**Фокус:** что сейчас уже интересно показывать, какие цифры свежие, и какой реалистичный roadmap по floating point.

## 1. Что уже приятно показывать

Сейчас `fun/` уже не выглядит как случайная куча демо-файлов. Там сложился хороший короткий маршрут:

- `addr_field_basic.nanz`, `addr_field_method.nanz`, `addr_field_aos_walk.nanz`, `addr_index_basic.nanz`
  Это быстрый proof, что address-of / field / indexed-lvalue path реально живой.
- `bit_intent.nanz`
  Показывает, что bit intent не схлопывается сразу в тупой `shr/and/or`, а доживает до backend.
- `pointer_threading.nanz`
  Маленький real-source пример под свежие Grace/solver-friendly loop transforms.
- `tuple_return.nanz`, `triple_return_skip.nanz`
  Хороший короткий demo multi-return и `_`-skip unpacking.
- `frill_showcase.frl`, `frill_graphics.frl`
  Frill уже выглядит как зрелый frontend, а не эксперимент.
- `examples/lizp/functional.lizp`, `examples/lizp/showcase.lizp`, `examples/lizp/zx_rainbow.lizp`
  Lizp тоже уже можно показывать как живой путь до Z80, а не просто parser toy.

Практический вывод:

- если нужно показать "что MinZ уже умеет", лучше вести человека не только в `Nanz`, но и в `Frill`/`Lizp`
- особенно полезно сочетание:
  - `fun/README.md`
  - `examples/frill/*`
  - `examples/lizp/*`

### 1.1 Что по фронтендам кроме Nanz

Если ранжировать по текущей showability:

1. **Frill**
- уже выглядит зрелым frontend'ом
- есть сильные runnable examples
- есть собственный language guide
- хорошо показывает функциональную сторону MinZ

2. **Lizp**
- уже не только "S-expression parser"
- есть `functional.lizp`, `showcase.lizp`, ZX visual examples
- хороший способ показать, что backend реально общий для очень разных surface syntaxes

3. **ObjC**
- да, он у нас действительно хороший showcase
- не как "production ObjC", а как очень необычный и сильный MinZ frontend:
  - static message syntax
  - inheritance/protocol story
  - canvas/demoscene demos
  - dynamic dispatch experiments
- особенно сильные файлы:
  - `examples/objc/plasma.m`
  - `examples/objc/canvas_shapes.m`
  - `examples/objc/dynamic.m`
  - `examples/objc/pipeline.m`
- тут реальный вау-эффект выше, чем у Pascal

4. **Pascal**
- честно живой frontend
- есть asserts, сортировки, sieve, records, recursion
- но сейчас он больше "solid compiler breadth proof", чем flashy showcase
- хорошие файлы:
  - `examples/pascal/assert_test.pas`
  - `examples/pascal/bubble_sort.pas`
  - `examples/pascal/sieve.pas`

Вывод:

- если показывать "весёлое и сильное", я бы сейчас ставил связку:
  - `Nanz`
  - `Frill`
  - `Lizp`
  - `ObjC`
- `Pascal` скорее показывать как доказательство ширины и серьёзности компилятора

## 2. Свежий benchmark snapshot

Актуальная проверка:

```bash
go test ./pkg/pipeline -run 'TestGrace_SDCC_Full$' -count=1 -v
```

Результат на 2026-04-06:

| Program | SDCC | C89(Go) | C89(Grace) | Nanz(Grace) | Best | Winner |
|---------|-----:|--------:|-----------:|------------:|-----:|--------|
| abs_diff | 12 | 4 | 4 | 4 | 4 | MinZ −66% |
| abs_diff_u16 | 13 | 10 | 10 | 10 | 10 | MinZ −23% |
| chain | 8 | 15 | 5 | 5 | 5 | MinZ −37% |
| fib | 22 | 31 | 31 | 19 | 19 | MinZ −13% |
| foreach | 44 | 61 | 54 | 0 | 54 | SDCC −18% |
| gcd | 17 | 14 | 9 | 9 | 9 | MinZ −47% |
| minmax | 60 | 27 | 27 | - | 27 | MinZ −55% |
| swap | 20 | 11 | 11 | - | 11 | MinZ −45% |

Totals:

- `SDCC`: `196`
- `C89(Go)`: `173`
- `C89(Grace)`: `151`
- `Nanz(Grace)`: `47`
- MinZ wins: `7 / 8`

Что это значит:

- старые мартовские claims не устарели, а в целом подтверждаются
- `Grace` path реально даёт measurable value и на `C89`, и на `Nanz`
- `foreach` остаётся удобной честной дырой, а не местом для self-congratulation

Обновлять большой сравнительный report прямо сейчас не обязательно: цифры уже можно просто сослать из этого snapshot.

## 3. Floating Point: что уже обсуждали и что выглядит правильно

Сейчас у нас есть три реальных линии, и только одна из них выглядит как ближайшая практичная.

### Линия A: existing fixed-point first

Это уже есть в языке и примерах:

- `f8.8`, `f16.16` и родственные fixed-point types уже фигурируют в type/model/docs
- `fun/raymarcher.nanz` уже показывает практический `8.8` путь

Плюсы:

- самый дешёвый путь к полезной графике/геометрии
- хорошо ложится на Z80
- уже сочетается с текущими оптимизациями

Минус:

- это не floating point в привычном смысле, а fixed-point discipline

### Линия B: soft `fp16`

Это самый интересный next-step candidate из уже обсуждавшихся.

Что уже было зафиксировано:

- в старом seed есть идея `fp16` с byte-aligned exponent
- ожидаемый плюс: `x2 = INC H` как очень дёшевый exponent bump
- идея из roadmap/book:
  - exponent в отдельном байте
  - mantissa обслуживается через быстрые mul8-like paths / precomputed tables

Почему это выглядит сильнее всего:

- даёт "настоящий float-like" workflow
- не уходит сразу в тяжёлый IEEE754 swamp
- естественно сочетается с GPU-precomputed mantissa tables

Практически это выглядит как лучший следующий FP research track.

### Линия C: full IEEE-style float

Это сейчас выглядит преждевременным.

Минусы:

- большой runtime
- дорогая нормализация/denormals/rounding
- слишком большой blast radius для Z80 на текущем этапе

Вывод:

- не надо сейчас прыгать в full `float`/`double`
- сначала либо:
  - усилить fixed-point user story,
  - либо делать маленький `fp16` soft-float track

## 4. Самый правильный FP roadmap

Если делать это прагматично:

1. Зафиксировать short note/ADR по `fp16` format
- layout
- exponent bias
- mantissa width
- NaN/Inf policy or explicit "no NaN/Inf" policy

2. Сделать tiny math surface
- `fp16_from_u8`
- `fp16_add`
- `fp16_mul`
- `fp16_mul_const`
- maybe `fp16_cmp`

3. Сделать один маленький `fun/` пример
- не 3D сразу
- а что-то вроде:
  - easing
  - simple decay
  - tiny physics step

4. Только потом думать о более широком language exposure

## 5. Что делать прямо сейчас

Самые разумные короткие next steps:

- поддерживать `fun/README` как curated entrance, а не список всего подряд
- при желании сделать короткий Lizp-focused report/showcase
- по FP: не новый frontend, а маленький `fp16` design note + one tiny demo

Если выбирать один "весёлый, но не пустой" side quest:

- **не новый frontend**
- а **`fp16` mini-proposal + tiny demo path**
