# Z80: unsigned word >> 8 и divide by 256

Общий backend теперь извлекает старший байт unsigned16 значения двумя `LD`:
нижний байт получает прежний верхний, верхний становится нулём. Это заменяет
восемь пар `SRL high` / `RR low` для постоянного unsigned `>> 8` и `/ 256`.
Signed arithmetic и остальные shift counts/divisors сохраняют прежние пути.
Преобразование не специализируется под hashmap и не меняет исходник программы.

На неизменённом полном hashmap с 36 последовательными вызовами actual Z80:

| Измерение | До | После |
|---|---:|---:|
| T-states всего workload | 138 440 | 133 994 |
| Байты всего модуля | 1013 | 984 |

Сэкономлены **4446 T-states (3.21%) и 29 байт**. Новое измерение получено на
текущем parent compiler, а не рассчитано из предыдущей PC-атрибуции. Новый
бинарник: SHA256
`0390aa22f7b028216e55bca731f3ca1bb4e91ab06e70ddead41e756235034860`.
Исходник, workload и host oracle побайтово совпадают с baseline. Независимый
Python oracle повторно проверил все результаты, полные 82 байта global state и
чтения/записи каждого global byte на raw, precompile, postcompile и actual Z80.
Compiler-context SHA внутри raw profile меняется вслед за backend: равенство
полного JSON не заявлено, наблюдаемая семантика и traffic проверены отдельно.

В исходном `wr16` вычисление `val / 256` исполняло 16 shift instructions:
128 T-states перед окружающими transfers. Две byte loads стоят 11 T-states;
38 наблюдавшихся вызовов `wr16` дают по 117 T экономии. Actual whole-caller run
подтвердил итог 4446 T. `rd16` этим изменением не оптимизирован.

542 machine regression cases проверяют boundary values и физические пары
регистров, aliases destination/source и spill destination. 540 комбинаций:
две операции × пять source pairs × шесть destination pairs/spill × девять values,
с независимым результатом `(u16value >> 8) + 103` и живым accumulator после
извлечения. Ещё два случая сохраняют исходную DE и постороннюю HL живыми.
Focused gate прошёл за 0.707 с; полный MIR2 — за 6.420 с. Проверяется также
отсутствие прежней повторяющейся shift sequence.

Существующая word copy/staging сохраняет byte aliases и spill paths. Division
path использует прежнее сохранение written scratch registers. Dispatch сбрасывает
cached comparison flags до shift/division, поэтому flag-preserving `LD` не
разрешает использовать старые arithmetic flags. MIR операции не имеют отдельного
наблюдаемого carry-output contract. Более общий вопрос живой HL при IX staging
предшествует этому изменению; эти тесты не заявляют новый глобальный register ABI.

Hashmap coverage остаётся прежним: start slots 5/11/15, count 0..16, full insertion
failure и overwrite, sentinel `map_has`, clear/refill. Счётчики доступа
агрегированы и не доказывают ordered native event/trap sequences; bootstrap,
stack, code fetch и initialization исключены. Выигрыш относится к измеренному
workload и не объявляется универсальным.

Финальные проверки прошли: pipeline269.139с, HIR18.572с/MIR4.666с/Nanz1.710с/C895.329с, CLI19.771с; showcase31прежнее падение, новых нет. Pipeline исключает только отдельно проверяемый долгий budget competition. Все4tokenizerкандидата заново проверены текущим CLI для обоих режимов.

[Артефакты](grace-word-shift8-artifacts/README.md) ·
[Независимая проверка hashmap](grace-word-shift8-artifacts/independent-hashmap-verification.txt) ·
[Результаты до/после](grace-word-shift8-artifacts/before-after-comparison.json)

Финальный CLI повторно измерил все4сочетания на30вызовах tokenizer. Без copy/region2257650T4317B,region2201516T4703B,copy2247229T4446B,оба2191095T4832B. Speed выбираетоба,sizeисходныйвариант. Отдельные saved-artifact host и typedSHA проверки проходят для всех8кандидатов: результаты, полные globals/resources, каждый resource read/write; globaltrafficVM/Z80agreement. Это новый измеренный результат текущего backend, а не перенос старых замеров.

Отдельный финальный audit доказал точное равенство исходника/всех4ASM/собранныхBIN предыдущему этапу:344/344/232/344байта. Старый722.080-секундный machine gate сохранён для этих неизменных бинарников; нового выполнения долгого workload не заявляем.
