# Итерация 3: структуры, указатели и arena с реальной памятью

Продолжение: [итерация 4](ITERATION4.md) соединяет этот record path с macro/meta.
Ниже описан самостоятельный корпус итерации 3.

Работает следующий сквозной путь без GC:

```text
native Crystal semantic AST
 → crystal-record-hir-v1 (типы, поля, вызовы, выражения)
 → Go importer с нашим target layout
 → typed HIR → MIR2 → Z80
```

Это отдельный исследовательский exporter/importer. Он дополняет предыдущий
scalar/Lanz путь; совместное использование macro↔meta и records в одном модуле
ещё не подключено. Новых изменений HIR/MIR2/backend в этой итерации нет.

## Реальная arena

Исходник — [arena_showcase.cr](arena_showcase.cr). `ByteArena` — обычный
Crystal struct с `initialize`, `alloc`, `reset`, `used` и другими методами:

```crystal
struct ByteArena
  @tag : UInt8
  @base : Pointer(UInt8)
  @cursor : UInt16
  @capacity : UInt16

  def alloc(size : UInt16) : Pointer(UInt8)
    if size > (@capacity &- @cursor)
      return Pointer(UInt8).null
    end
    result = @base + @cursor
    @cursor = @cursor &+ size
    result
  end
end
```

Теперь это не только учёт cursor: локальный `StaticArray(UInt8,32)` владеет
памятью; arena работает в его внутреннем диапазоне. Результат alloc используется
для настоящих pointer loads/stores. Четыре контрольных байта вокруг диапазона
проверяются после записей. Heap allocator, GC и host object headers в target
не переносятся.

## Layout задаёт наш compiler

| Поле | Z80 offset | Размер |
|---|---:|---:|
| tag | 0 | 1 |
| base | 1 | 2 |
| cursor | 3 | 2 |
| capacity | 5 | 2 |
| **ByteArena** | | **7** |

На проверенном native host Crystal `sizeof(Pointer(UInt8)) == 8`,
`sizeof(ByteArena) == 24`. В Z80 продукте указатель — 2 байта, ByteArena — 7.
Host размеры/offsets не входят в wire protocol: Go строит `mir2.StructTy` и
`mir2.PointerTo`, считает offsets через `ByteOffset` и размеры через `ByteWidth`.
Имена `record_0` и т. п. локальны одному экспорту, это ещё не стабильный library ABI.

Synthesized native `new` распознаётся отдельно: target получает локальную
структуру и прямой вызов `initialize` с явным self pointer. Методы читают/пишут
поля через typed `FieldExpr`. Локальное `second = first` создаёт копию структуры,
а не alias cursor; pointer-поле при этом намеренно продолжает ссылаться на тот
же backing buffer, как в нативном Crystal.

`StaticArray` создаётся как HIR `ArrayTy` с автоматическим storage. Fill выполняется
при каждом вызове функции; путь legacy static arrays не используется. Переход
`to_unsafe` сохраняет pointee type; шаг `Pointer(UInt16) + 1` вычисляется как
два target байта.

## Порядок вычисления

Importer раскладывает вычисления на временные HIR значения в исходном порядке.
Это нужно и для указателей, и для методов, меняющих память. Проверены:

```crystal
arena.alloc(1_u16)[0_u16] = arena.allocate_echo(seed)
sum = raw[2_u16] &+ arena.overwrite(raw + 2_u16, seed &+ 2_u8)
```

В первом случае LHS allocation должна произойти до RHS allocation. Во втором
первое чтение должно сохранить прежний байт до вызова overwrite. Проверка
только конечной pure-арифметики не обнаружила бы эти ошибки.

## Truthiness

Native reference выявил важную границу: числовой ноль истинен, но null pointer
ложен (как Nil и false). Importer сохраняет побочные эффекты условия; для
указателей генерирует сравнение с typed zero. Проверены все 256 byte seeds
для integer/null-pointer truthiness и effectful conditions, а также 10
размеров alloc в условии, включая OOM.

## Проверки

```sh
python3 research/crystal-hir/verify_records.py
```

Проверены **2372 случая на каждый из четырёх исполнителей**:
native Crystal, raw MIR2, optimized MIR2, собранный Z80 в эмуляторе.

- Все 256 значений seed для записи/чтения arena, reset, копирования структуры,
  повторных вызовов с заново заполненным локальным массивом и порядка эффектов.
- Размеры alloc 0, 1, 2, 3, 27, 28, 29, 255, 256 и 65535: успех, точная граница,
  OOM и неизменность cursor на отказе. Рабочая ёмкость — 28 байт.
- Word pointers: 264 UInt16 значения, включая переходы между байтами и overflow
  при `&+ 1000`; отдельно проверяется сохранность дальнего элемента массива.
- Layout проверяется явно, независимо от native host layout.
- **8 отрицательных контролей**: GC class, возврат указателя из free function,
  aggregate by-value argument, checked narrowing, dynamic array fill, локальный
  array в методе, pointerof в методе и поле типа Nil.

Artifacts: `/tmp/minz-crystal-record-results`: wire module, cases, HIR, MIR2,
Z80 assembly, native verification source, target layout и логи. Копии итоговых
receipt и layout лежат в `evidence/iteration3-*`. На одном прогретом запуске
target-проход со всеми 2372 проверками занял около 107 s; это не benchmark
скорости скомпилированного arena API.

## Границы

Поддержан проверяемый subset: scalar поля, pointers на 8/16-bit integers,
адресуемые локальные struct receivers, scalar параметры/результаты методов,
локальные копии structs, literal-filled scalar StaticArray длиной 1..256,
pointer indexing/arithmetic, unsigned byte→word widening и structured control flow.

Aggregate arguments/returns, classes, вложенные агрегатные поля, union dispatch,
произвольные constructors, pointer casts и checked narrowing пока отклоняются.
Распознаётся synthesized native `new` с одним зарегистрированным initialize;
перестановка/default adaptation аргументов constructor ещё не поддержана.

Полной lifetime/escape analysis нет. Поэтому pointer returns из free functions,
automatic arrays и pointerof внутри instance methods сейчас отклоняются.
Pointer arguments остаются обычным raw-memory контрактом: compiler не обещает
безопасность произвольного unsafe Crystal-кода. На нашем корпусе владелец backing
buffer живёт дольше arena и всех выданных указателей.

Новый record protocol пока не объединён с scalar exception lowering и meta
bridge. Попытка использовать `minz_meta_repeat` в нём явно отклоняется.
Top-level вызовы в исходнике служат inference roots; target `main` не генерируется.
Source hashes/transitive dependencies для production PGO и внешнее ABI тоже
ещё требуют отдельного подключения.

Следующая цель — объединить record path с meta-мостом и передавать генераторам
наши target типы/layout. Тогда macro сможет вызывать общий генератор для структуры
или arena, уже зная реальные Z80 размеры.
