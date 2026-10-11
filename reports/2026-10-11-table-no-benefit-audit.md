# Где таблицы распределения регистров не дают выигрыша

Проверены **12 неизменённых corpus examples, 118 функций**, база `7719dffb`, одинаковые `ContractOpt=true, OptSize=true, AssertMode=none`, ordinary Z80. Производственные ключи и admission не менялись. Это разбор ограниченной выборки, не общий показатель всего compiler corpus.

Главное различие: **исторические shape tables и нынешний checked function cache — разные вещи**. Доступные repo assets: `regalloc_table.json` — 36 legacy JSON entries; `minzc/pkg/vir/regalloc_exhaustive.json` — 56 GPU entries. У них нет checked schema/costmodel/context. Их точные SHA и git objects записаны. Явно загрузили обе и попробовали десять реально подходящих leaf problems: legacy ComputeSignature raw hits **0**, legacy multi-format Lookup hits **0**. Это не «старые таблицы ускорили компиляцию».

Большие ENRT v1 / Z80T v2 / enriched JSONL поддерживаются reader и research lookup, но в проверенных default directories `/home/alice/dev/z80-optimizer/data`, `/home/alice/dev/minz-vir/data`, repo data их нет. Комментарий про 37.6M описывает историческую возможность; наличие таких файлов на этой машине не подтверждено. Их не загружали и не выводим RAM/coverage из обещаний комментария. Production function path использует только LookupChecked, legacy wrappers не дают ему admitted entries.

Нынешняя польза weighted/triple получена **новой offline генерацией точного случая**, затем сохранением и использованием сертификата. Это доказательство useful function cache, **не** broad precomputed-shape reuse. Checked ключ включает нормализованные operands, immediate, descriptor/templates/cost/scratch/overlaps, actual ABI и полный function context (включая имя). Переключить lookup на старый shape hash нельзя: shape/op bag не сохраняют все operand constraints, clobbers, ABI и фактические moves.

Из 118 функций:

- **55**: подготовка требует один блок и один return. Среди них loops/branches, void/externs, а не 55 одинаковых «CFG багов».
- **24**: hard class либо неподдерживаемая definition/type. Здесь, например, bool comparisons и inline ASM; diagnostic общий, op facts сохранены отдельно.
- **9**: неподдерживаемый contract parameter. Типичные pointer receiver (`Arena_used`) и callable (`apply`), не доказательство duplicate-parameter miscompile.
- **19**: неподдерживаемые операции — ext/trunc chains, shifts, signed div/mod, sext и call. `str_len` — один прямой peek call, но его memory/extern effects нельзя снять вместе с guard.
- **1**: hash16 подготовлен, но нет word XOR pattern.
- **10**: действительно генерируется admitted table; **3** строго меньших, **7** равного размера.

| Реальная функция | Baseline → candidate | Почему нет/есть эффект |
|---|---:|---|
| weighted | 36 → 12 B | +control, устраняются лишние moves при repeated accumulator use |
| triple (09/tiny) | 7 → 4 B | +control, меньше копирований; две source occurrences одного типа задачи |
| double (09/tiny), tiny add | 2 → 2 B | уже ADD + RET, таблица не сокращает |
| StrRef hash3 | 6 → 6 B | candidate буквально те же пять ADD + RET |
| StrRef test_eq/test_neq | 3 → 3 B | calls уже folded в constant result; LD + RET |
| low_nibble | 3 → 3 B | AND immediate + RET, та же стоимость |
| hash16 | нет candidate | missing16bit XOR; baseline уже11B55T; прежний isolated experiment добавил XOR, но unsafe BC scratch был отвергнут и безопасная folded цепочка хуже |
| Arena_used | нет problem | pointer receiver + load, domain не поддержан |
| self_tokenizer str_len | нет problem | direct extern memory call, нужен effects/ABI contract |
| promoted pressure | нет problem | signed division/modulo, вне narrow op domain |

StrRef hash3 дополнительно проверен на VM и Z80 с независимым `(a+2b+3c) mod256` оракулом: **65 536 различных tuples на каждый baseline/candidate путь** (не все 2²⁴), каждый **30 T**, весь модуль **45 B**. Код/literals и SP сохранены. Его безопасный cache hit существует, но size selector правильно оставляет baseline при равенстве.

Контроль shape reuse на реальном low_nibble: отдельная **изменённая probe-копия** mask15→7 (не выдаём её за неизменённый corpus). Legacy signature та же, checked key другой, поэтому exact cache miss. Старое assignment прошло **AddChecked заново для нового target problem**; native все256 значений подтвердили `x&7`. Но размер снова3B — ни reusable shape candidate, ни его успешная перепроверка сами по себе не создают выигрыш. Изменение ABI A→C меняет key, а старое assignment независимо отвергается `ABI or location hint mismatch`. Это конкретный путь безопасного candidate reuse, **не реализованный параметрический cache** и не доказательство универсальности.

Moves/save pressure: positive weighted/triple покрывают реальные saves/moves и callers. В этой выборке нет admitted отрицательного случая, который стал больше именно из-за добавленных saves: семь admitted negatives равны baseline. У hash16 экспериментальная tied add-immediate цепочка показывает дополнительное сохранение исходного word и BC scratch; evidence report068 сохраняет отказ. Не приписываем остальные preparation refusals «плохому allocator» без native сравнения.

Полные BIN и стоимость инфраструктуры приведены в `summary-table.md`: weighted36→12, 09 34→31, tiny11→8; остальные девять модулей без уменьшения. Timings — один warm in-process запуск на этой машине, parse+compile; generation включает **повторный compile и solver**, table-compile включает reading/admission и ASM measurement. Это wall observations, без RAM claim и без статистического benchmark. Например StrRef2.88/50.80/6.18ms и tokenizer601.77/693.72/639.03ms. Zero-entry generation tokenizer почти целиком повторяет compile; нет O(1) выигрыша всей сборки. В artifact есть per-function solve timings. Колебания порядка/нагрузки не интерпретируются как ускорение.

Финальные focused checks **PASS 6.085s**: scan, StrRef native, exact-binding/ABI probes, прежние weighted+actual caller, triple+indirect caller, hash16 все65536+eightcaller controls. Полный pipeline не запускали. Архив exact source/ASM/candidate/table/MIR по каждой функции, `audit.json`, historical assets inventory, binding-transfer evidence, logs и SHA manifest сохранены в `reports/artifacts/table-negative/`.

Практический вывод: сначала расширять safely reusable **candidate search**, который перепроверяется по текущему problem/ABI/descriptor и сравнивается с baseline, а не ослаблять checked key. Для equal ADD/AND/StrRef никакая более широкая форма ключа не улучшит BIN. Для loops/calls/memory нужна отдельная доказуемая область; более общий hash не заменяет effects, edge moves и live-value proof.
