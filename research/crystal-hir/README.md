# Crystal → HIR

**Текущий шаг:** [Итерация 9: общий byte buffer/view API](ITERATION9.md).
Импортируемая Nanz-библиотека без allocation; 22 проверки на обоих MIR2/Z80
и 22 native Crystal reference сценария. Crystal-фасад пока следующий шаг.

**Итерация 8:** [Итерация 8: native yield → общий generator/pull](ITERATION8.md).
2048 наборов на native Crystal, raw/optimized MIR2 и Z80: drain, break/next,
nonlocal return, demand-preserving filter, RLE, finite prefix of infinite producer
и empty source. Исправлено общее завершение генератора для Nanz/HIR.

**Итерация 7:** [Итерация 7: общий borrowed-view descriptor](ITERATION7.md).
Локальные Slice, copy/rebind и живые поля при изменении view внутри each.
1566 наборов на четырёх engine; общий HIR descriptor также читает Nanz
(256 проверок на raw/optimized MIR2 и Z80). Target 4 B, native host 16 B.

**Итерация 6:** [Итерация 6: Slice#each → общий HIR foreach](ITERATION6.md).
1822 набора на native Crystal, raw/optimized MIR2 и Z80: captures, break/next,
nonlocal return, alias mutation, nested/empty loops, word stride и length 256.

**Итерация 5:** [Итерация 5: scalar Crystal → Go](ITERATION5.md).
Существующий MIR2→Go backend прошёл 7 функций × 256 byte inputs = 1792
набора на generated Go и native Crystal. Это unsigned scalar/meta corpus;
Go port самого compiler и record/arena Go backend этим не проверены.

**Итерация 4:** [Итерация 4: macro/meta и records в одном модуле](ITERATION4.md).
Три сценария × 256 seed values, всего 768 наборов на каждый из четырёх
исполнителей. Nanz meta-генератор получает Z80 offset/sizeof/pointer width;
поле marker имеет offset 7 на target и 20 на native host.

**Итерация 3:** [Итерация 3: структуры, указатели и реальная arena](ITERATION3.md).
2372 случая на native Crystal, raw/optimized MIR2 и Z80; target ByteArena — 7 байт
против 24 на native host, указатель — 2 против 8.

**Итерация 2:** [Рабочий мост Crystal macro ↔ Nanz meta](ITERATION2.md).
Проверены 1792 случая на native Crystal, raw/optimized MIR2 и Z80.

**Продолжение:** [Итерация 1: native inference, мономорфизация, merge и raise/rescue](ITERATION1.md).
Она уже проверяет 844 случая на native Crystal, raw/optimized MIR2 и Z80,
а также 23 случая существующего Nanz CY ABI. GC не входит в целевую модель:
ориентируемся на arena и статически разрешённые специализации.

Ниже — устройство и результаты первоначальной, syntax-only итерации.

Проверено локально 2026-10-25 на `f7b5d9e5d`, Crystal 1.20.1 / LLVM 22.1.4,
macOS arm64. Ветка `spike/crystal-hir`, отдельный worktree
`/tmp/minz-crystal-hir`. На i7 выполнялись только чтение статуса и исходников.

## Вывод

Подключить Crystal к нашему HIR возможно. Для небольшого явно типизированного
подмножества уже работает настоящий Crystal parser → AST → Lanz → HIR → MIR2
→ Z80. Изменений в HIR, MIR2 и backend не понадобилось.

Прототип использует **только синтаксическую часть** родного компилятора,
`require "compiler/crystal/syntax"`. Типы параметров и результатов берутся
из аннотаций; типы локальных переменных — из простого прохода по присваиваниям.
Это ещё не переиспользование Crystal type inference и не полный Crystal.

Доказательство: 26 одинаковых проверок успешно прошли нативный Crystal,
MIR2 VM и собранный Z80 в эмуляторе. Ещё 11 отрицательных контролей проверяют
отказ за границами подмножества. Обновлять установленный Crystal для этой
итерации не потребовалось. В исходниках 1.21.0 тот же syntax-only вход есть.

## Воспроизведение

Нужны Python 3, Go из `minzc/go.mod` и Crystal с исходниками компилятора
в стандартном `CRYSTAL_PATH` (у установленного Homebrew Crystal они есть).
Из корня этого worktree:

```sh
python3 research/crystal-hir/verify.py
```

Результаты по умолчанию в `/tmp/minz-crystal-hir-results`:
`scalars.lanz`, `scalars.hir`, `scalars.mir2`, `scalars.a80`, `native.cr`,
`results.json`, логи сборки и отрицательных контролей.
Можно задать `--out /tmp/other-results`.
Никаких обращений к i7, установки пакетов или изменения системы скрипт не делает.
Первый запуск Go может загрузить отсутствующие зависимости обычным способом.

Файлы эксперимента:

- `export.cr`: нативный parser и небольшой AST → Lanz адаптер.
- `scalars.cr`: 7 обычных Crystal-функций.
- `assertions.lanz`: базовые ожидаемые результаты.
- `verify.py`: сборка, общий корпус для трёх исполнителей, отрицательные контроли.
- `../../minzc/cmd/crystal-hir-spike/main.go`: исследовательский Lanz → HIR consumer.

Consumer убирает предварительные объявления сигнатур перед HIR verification.
Они нужны для однопроходного Lanz, чтобы вызов функции ниже по тексту уже имел
правильный тип. Это локальная деталь прототипа, не изменение Lanz.
Команда `mz` пока не принимает `.cr`.

## Что реально прошло

| Пример | Что проверяет |
|---|---|
| `add_byte(250, 10) == 4` | UInt8 и wrapping overflow |
| `sum_to(255) == 128` | while, локальные переменные, присваивания, вызов, переполнение |
| `abs_diff(0, 255) == 255` и обратный порядок | ветвление, обе ветки, неявный возврат |
| `add_word(65535, 2) == 1` | UInt16 без сужения до байта |
| `negative(-128) == true`, `negative(127) == false` | Int8 и signed comparison |
| `forward_word(1000) == 1300` | forward call с UInt16 результатом |

Всего: 7 функций, 26 native + 26 MIR2 + 26 Z80 проверок, 11 отказов.
Среди отказов: checked `+`, Int32 literal по умолчанию, отсутствие аннотаций,
union return, смешанные разрядности, overload, class, утечка переменной из
ветки, integer truthiness, array literal и синтаксическая ошибка.

Один повторный запуск с прогретыми кешами: сборка exporter 1.86 s,
сборка consumer 0.33 s, экспорт 0.12 s, весь HIR/MIR2/Z80 проход с 26
проверками 0.25 s, нативная сборка и исполнение 1.15 s. Это замер удобства
эксперимента, не сравнительный benchmark компиляторов.

Пример результата нашего обычного backend:

```asm
add_byte:
    ADD A, C
    RET

abs_diff:
    SUB C
    RET NC
.abs_diff_if_else3:
    NEG
    RET
```

HIR-оптимизации уже узнали абсолютную разность, а вызов `add_byte` внутри
цикла был заинлайнен. Специальных Crystal-оптимизаций не добавлялось.

## Совместимость и сложность

| Возможность Crystal | Соответствие нашему HIR | Работа |
|---|---|---|
| Def, scalar args, locals, return, if, while | Func, Param, VarDecl, Return, If, While | Низкая; путь уже проверен |
| Implicit last-expression return | Явный ReturnStmt, CondExpr/ветки | Низкая для простых выражений; нормализация для общего случая |
| UInt8/16, Int8/16, Bool | Уже имеющиеся MIR2/HIR типы | Низкая; следить за coercions и signedness |
| Int32 по умолчанию и более широкие числа | i32/u32 в MIR2 есть | Нельзя молча сделать u8/u16; отдельно проверить покрытие Z80 операций и helpers |
| Struct, StaticArray, Pointer | StructTy, ArrayTy, field/index/load/store | Средняя; нужен наш layout и правила value/reference semantics |
| Methods, overloads, generics | Мономорфные Func + mangled names + явный self | Средняя/высокая; лучше использовать Crystal semantic resolution |
| Blocks и iterators | Captures, function calls, ForEach — полезные строительные блоки | Средняя/высокая; nonlocal return, break и lifetime нельзя потерять |
| Macros, require, constants | До HIR должны стать конкретными узлами | Переиспользовать native normalize/semantic; сырого AST недостаточно |
| Nil unions и flow narrowing | Нет готовой универсальной Crystal tagged-union модели | Высокая; tagged layout + dispatch + проверки либо явный отказ |
| Class, inheritance, virtual dispatch | Structs и прямые вызовы покрывают только часть | Высокая; object layout, type IDs, dispatch, allocation и lifetime |
| Exceptions, fibers, GC, stdlib | Нужны согласованные runtime/ABI решения | Очень высокая; самостоятельный проект, особенно на Z80 |

Критические различия семантики:

1. Crystal `+`, `-`, `*` проверяют overflow, а наш обычный integer HIR
   оборачивает результат. В spike разрешены явные `&+`, `&-`, `&*`.
   Для обычных операторов нужны проверки и определённое поведение ошибки.
2. `if 0` в Crystal истинен. Spike требует именно Bool, а не подменяет
   Crystal truthiness числовым условием нашего IR.
3. Деление `/` нельзя автоматически считать integer division. В spike
   вообще нет деления, сдвигов, casts/conversions или mixed-width coercions.
4. Native host pointer size/alignment не подходят Z80. Нужно заново
   вычислять target layout; нельзя переносить размеры host-объектов.
5. Crystal String/Array — объекты с runtime-контрактом. HIR string literal
   или указатель на массив сами по себе не реализуют этот контракт.

Spike намеренно консервативен: явные типы функций, одинаковые разрядности
операндов, явно суффиксированные literals. Переменные, впервые объявленные
внутри if/while, не экспортируются наружу. Возврат assignment/while как
значения отклоняется. Объекты, aliases, modules, methods, union narrowing,
closures, macros, require и stdlib пока не реализованы. Source locations
используются в диагностике exporter, но ещё не переносятся в HIR lineage;
source hashes/транзитивные зависимости для PGO также не записываются.

## Как переиспользовать больше нативного frontend

Изученный код Crystal 1.20.1 содержит `Compiler#no_codegen`, возвращаемый
`Compiler::Result(program, node)`, а также `Program#normalize` и
`Program#semantic`. В `semantic.cr` явно разделены top-level declarations,
инициализаторы, main semantic и cleanup. Это подходящая точка следующего
эксперимента: получить **семантически разрешённый граф**, пройти reachable
специализации методов и экспортировать их в HIR через небольшой процесс-адаптер.

Важно: `no_codegen` означает отсутствие генерации кода **исследуемой программы**,
а не автоматическую независимость самого адаптера от LLVM. В установленной
версии `program.cr` прямо требует `llvm`, а `Compiler` создаёт target machine.
В проверенном исходнике 1.21.0 эти зависимости тоже есть. Полный semantic
адаптер в этом эксперименте не собирался и не проверялся — его лёгкость и
границы надо подтвердить отдельным запуском, не обещать по одному имени флага.

Typed Crystal AST также не является сериализуемым HIR: `+` и методы сначала
представлены Call, должны быть разрешены primitives/target defs, default args
и macros расширены, overload/generic instantiations названы однозначно.
Публичной стабильной JSON AST-схемы spike не предполагает: маленький exporter
использует Crystal API и должен проверяться на каждой поддерживаемой версии.

Мой следующий выбор — не расширять собственный мини-typechecker до Crystal,
а проверить native semantic frontend на 3–5 маленьких случаях: untyped function,
overload, generic scalar, struct method, compile-time macro. После этого
зафиксировать простой versioned interchange и подключить `.cr` к CLI.
Lanz достаточно для spike; полноценный importer должен сохранять source locations,
linkage/effects и не зависеть от текстовых трюков с сигнатурами.

Ориентировочная инженерная оценка, не обещание сроков:

- parser-only scalar subset — небольшой spike; нулевая версия уже здесь;
- удобный явно типизированный embedded subset с CLI, диагностикой, structs,
  static arrays, pointers и проверками — примерно 1–3 недели;
- native semantic exporter с inference/overloads/ограниченными generics —
  несколько недель и отдельный риск по внутреннему API/target semantics;
- произвольная host-stdlib не переносится автоматически; для нашей целевой
  модели нужны arena-совместимые примитивы и выбранные модули stdlib.
  GC не требуется. Ограниченное raise/rescue уже проверено в итерации 1.

## Источники

Локальные `minzc/pkg/hir/hir.go`, `minzc/pkg/hir/verify.go`,
`minzc/pkg/lanz/lower.go`, `minzc/pkg/pipeline/pipeline.go`,
`minzc/pkg/mir2/types.go`, `docs/HIR_Guide.md`.

Upstream, проверенные при исследовании:

- [Crystal 1.20.1 syntax entry point](https://github.com/crystal-lang/crystal/blob/1.20.1/src/compiler/crystal/syntax.cr)
- [Crystal 1.20.1 semantic pipeline](https://github.com/crystal-lang/crystal/blob/1.20.1/src/compiler/crystal/semantic.cr)
- [Crystal 1.21.0 Program](https://github.com/crystal-lang/crystal/blob/1.21.0/src/compiler/crystal/program.cr)
- [Crystal 1.21.0 Compiler](https://github.com/crystal-lang/crystal/blob/1.21.0/src/compiler/crystal/compiler.cr)
- [Crystal operators](https://crystal-lang.org/reference/1.21/syntax_and_semantics/operators.html)
