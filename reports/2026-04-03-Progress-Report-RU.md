# Отчёт о прогрессе за последние дни

Дата: 2026-04-03

## Коротко

За эти дни backend-путь заметно продвинулся от “идей и гипотез” к
реальным, встроенным и проверенным улучшениям.

Главное:

- намерение пользователя теперь лучше доживает до backend-а
- IX/IY-половинки закреплены как полноценные регистры
- появились настоящие structural Grace transforms
- появился compile-trace, который показывает, что реально сработало
- есть уже не только unit-level proof, но и живой source-level proof для
  pointer-threading на Nanz-коде

До финального сокращения `Che gap` ещё не дошли, но фундамент для этого уже
сделан и частично проверен на реальных программах.

## Что реально внедрено

### 1. Bit intent проходит через весь pipeline

Добавлено и доведено:

- синтаксис `x.N`
- синтаксис `ptr^.N`
- bit access для обычных scalar values и wide scalars
- direct MIR2 ops:
  - `bit_get`
  - `bit_set`
  - `bit_reset`
- VIR support для direct bit intent
- Z80 lowering в:
  - `BIT`
  - `SET`
  - `RES`

Это важный качественный сдвиг: теперь битовые операции не обязаны распадаться
в `shr/and/or` слишком рано.

### 2. IXH/IXL/IYH/IYL стали first-class citizens

Это закреплено и в policy, и в codegen:

- IX/IY halves учитываются как нормальные 8-bit regs
- это работает не только для bit ops
- backend теперь умеет рассуждать о них как о полноценной части Z80 target model

Это уже дало пользу в:

- bit lowering
- compare/move paths
- register-backed u8/u16 half-paths
- более честном modelling реального пространства регистров

### 3. Улучшены static data и LUT paths

Сделано:

- typed global emission для arrays
- `u16` LUT split в `lo/hi` таблицы
- layout стал заметно ближе к реальным нуждам Z80 codegen

Это уменьшает давление на runtime address math и делает таблицы более
естественными для target.

### 4. Есть groundwork для solver-friendly reshaping

Появились MIR2 shape facts:

- loop regions
- indexed accesses
- repeated address terms
- in-loop candidates

Это уже не просто “может быть когда-нибудь Grace”, а реальная база для
структурных transform-ов.

### 5. Появились первые реальные structural Grace passes

Теперь в `RunGracePasses` встроены:

- `ptr-threading`
- `ptr-add-cse`

То есть это уже не просто локальные эксперименты, а часть активного пути.

### 6. Появилась нормальная observability

Compile trace теперь показывает:

- `ptr-threading`
- `ptr-add-cse`

Это очень важный шаг. Теперь можно честно различать:

- pass существует, но не матчится
- pass матчится, но не даёт пользы
- pass не сработал из-за формы программы

Раньше эта граница была размыта.

## Что уже доказано end-to-end

### `pointer_threading.nanz`

Добавлен специальный живой пример:

- [`fun/pointer_threading.nanz`](/home/alice/dev/minz-vir/fun/pointer_threading.nanz)

На нём уже есть source-level proof:

- `--grace --compile-trace` показывает `ptr-threading=1`

Это означает, что structural transform уже реально срабатывает на Nanz source,
а не только на synthetic MIR2 tests.

### `che_cascade.nanz`

На `che_cascade` картина сложнее, но очень полезная:

- compile trace работает
- bottleneck’и теперь видны честно
- pointer-threading там пока не матчится автоматически
- зато ручные reshape-эксперименты уже показали, какие вещи реально помогают,
  а какие нет

Самый полезный локальный сдвиг:

- вынос row-base из `xor_pixel` в `xor_pixel_row(...)`

После этого:

- `xor_pixel` перестал быть `PBQP fallback`
- pressure ушёл дальше в draw-path

Это не финальная победа, но это реальный measurable shift в нужную сторону.

## Что оказалось важным уроком

### Хорошие гипотезы подтвердились

- intent-preserving lowering действительно стоит усилий
- IX/IY-half-aware model действительно нужен
- structural reshaping в Grace имеет смысл
- compile-trace был необходим и оказался очень полезным

### Не всё, что кажется красивым, реально помогает

Например:

- грубая block-size specialization в `che_cascade` оказалась неудачной
- она породила новые helper nodes и не дала чистого выигрыша

Это хороший результат тоже:

- теперь понятно, что именно не стоит автоматизировать в таком виде

## Что ещё не сделано

До финального сокращения `Che gap` пока не дошли.

Главные remaining bottleneck’и сейчас:

- `apply_buf`
- `apply_buf_row`
- `xor_blk`
- `xor_blk_row`
- `fill_buf`

То есть теперь задача уже не в том, чтобы “изобрести ещё один pass”, а в том,
чтобы правильно выбрать следующие reshapes и автоматизировать именно те,
которые уже показали value.

## Что параллельно исследовали

Через отдельный Claude-трек в соседнем репозитории исследовали `B6`:

- enriched-table / Path A
- IX/IY-expanded loc-set gap
- pre-split before enriched lookup

Вывод там честный:

- исследование полезное
- diagnostics полезны
- но production rollout этого пути пока рано делать

То есть этот трек дал нам знание и ограничения, но не стал immediate next move.

## Где мы сейчас

Текущее состояние проекта уже заметно лучше, чем несколько дней назад:

- backend стал честнее относительно hardware intent
- MIR2/Grace получили реальные structural transforms
- source-level proof для pointer-threading есть
- compile-trace наконец позволяет видеть реальные причины, а не гадать
- `che_cascade` уже можно разбирать по-настоящему, а не интуитивно

## Что делать дальше

Самый разумный следующий фокус:

1. продолжать `B2`-класс работы на `che_cascade`
2. выделить 1-2 reshape pattern’а, которые реально доказали value
3. автоматизировать именно их в Grace, а не делать ещё много ручных правок

Главные кандидаты на автоматизацию:

- row-base hoisting
- дальнейшее развитие pointer-threading для более богатых loop shapes
- более аккуратный call-heavy loop split

## Итог

За эти дни мы не просто “что-то подкрутили”.

Мы:

- улучшили семантическую честность pipeline
- встроили новые backend-aware transforms
- научились их наблюдать
- получили первый живой source-level proof
- и сузили область, где теперь реально надо бить для сокращения `Che gap`

Это уже не стадия разговоров. Это стадия направленного engineering work с
измеримыми промежуточными результатами.
