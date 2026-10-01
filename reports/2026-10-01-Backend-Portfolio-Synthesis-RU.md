# MIR2→Z80: параллельные пути и осмысленный синтез

**Дата/база:** 2026-10-01, `main` после PR #52 (`a5918c5c`). **Метод:** чтение текущих entry points, сверка с датированными аудитами и независимый архитектурный review ASTRA. Это проект решения, не новый runtime-бенчмарк и не claim о превосходстве backend.

## Решение

Три пути можно запускать над одним **логически одинаковым** оптимизированным MIR2 и сравнивать готовые результаты, но текущий pipeline не является честным турниром. На первом этапе нужен *offline portfolio* как измерительный стенд. Автоматический выбор для пользовательского бинарника допустим только среди вариантов с одинаковым ABI, полной проверкой сборки/легальности и подтверждённой семантикой в определённом домене. Нельзя признать вариант лучшим по числу строк ASM или по ответу собственного solver.

Наиболее полезный синтез сейчас — маленькие, проверенные мосты: общая спецификация Z80 legality/cost, переиспользование безопасных ISLE-комбинаций до backend-развилки, PBQP как подсказка WFC, VIR/таблицы как независимые offline-кандидаты и источники контрпримеров. Не объединять четыре алгоритма в один новый backend до исполнения одинакового corpus и сравнения с production.

## Что именно существует

```text
source frontends → typed HIR → MIR2 raw → MIR2 passes/Verify
                                      → inlining/pruning → PFCCO contract
                                      → LUTGen/coalescing/liveness
                                      ├─ PBQP allocation → Z80Codegen → ASM       default
                                      └─ LIR: Combine(ISLE) → isel → WFC → ASM    --lir
                                           ↑ PBQP location hints; per-function PBQP fallback

VIR: MIR2 → VIR constraints → table / Z3 → PIR/ASM (offline vir-oracle)
```

| Компонент | Фактическая задача | Статус/граница |
|---|---|---|
| PFCCO (`mir2/contracts.go`) | Классы параметров/результата и стоимость адаптеров между функциями | Production `OptimizeContracts` — greedy DP; `OptimizeContractsPBQP` — отдельный эксперимент. VIR имеет другую Z3-PFCCO реализацию, которую [ADR-0043](../docs/adr/0043-vir-demoted-to-offline-oracle.md) не считает достоверной. |
| PBQP (`mir2/pbqp.go`) | Физические locations MIR2-регистров с liveness, стоимостью, aliases и spill | Production allocation; выполняется **даже при `UseLIR`**, затем превращается в мягкие LIR hints. |
| LIR/ISLE/WFC (`lir/`) | ISLE-комбинация MIROp, жадный выбор паттерна, распространение LocSet и последовательный collapse | `--lir` opt-in; при ошибке LIR функция берётся из PBQP ASM. Это не entropy-guided search и не доказанная оптимальность. |
| VIR (`vir/`, `cmd/vir-oracle`) | Совместный поиск паттерна/раскладки, таблицы и SMT | Offline oracle. CLI сравнивает число инструкций, не семантику; сам VIR codegen ещё мутирует MIR2-функции и может ошибаться в ASM. |
| GPU regalloc tables | Shape→assignment под моделью генератора | 4v lookup подтверждён; 5v полный по заголовку, не replay-validated; текущий IX-6v dense не имеет надёжного shape→row индекса. [Аудит](2026-10-01-Regalloc-Table-Audit-RU.md). |

**Существующее взаимодействие:** `pipeline.CompileHIRSteps` вычисляет PFCCO и PBQP, затем при `UseLIR` передаёт PBQP-регистры в WFC через `pbqpToLIRHints`; при отказе LIR подставляет PBQP-код по функции. `WFCState.pickPreferred` использует hint, только если он остаётся в допустимом `LocSet`. `LIRCheck` — ранняя, неразрушающая проверка сходимости **до** позднего inlining/contract/coalescing; в `UseLIR` `Match` означает `r.OK` генерации, а не эквивалентное исполнение. Показатели этих режимов нельзя без уточнения смешивать.

## Можно ли запускать всё одновременно и выбирать лучшее?

**Для исследования — да, как независимые кандидаты.** Зафиксировать входной MIR2 после общих passes и выбранный production contract; каждому backend дать собственную копию module или отдельный процесс. Последнее сейчас проще и безопаснее: `vir.CodegenFunc` вызывает мутирующие `FuseAbsDiff`/Grace и опциональный `InlineMIR2`; в LIR есть package-global `UseZ3`/`UseLIRContracts`. Concurrent goroutines над одним `*mir2.Module` не дадут честного эксперимента. Раздельные CLI invocations требуют также фиксации входного файла, compiler SHA и всех опций, а в перспективе — сериализуемого MIR2 snapshot.

**Для автоматического выбора при обычной компиляции — ещё нет.** Каждому кандидату нужен одинаковый внешний контракт: параметры/возврат, flags, clobbers, stack, globals, labels, runtime-зависимости и target mode. «Скомпилировалось» и «собралось MZA» не доказывают, что функция вернёт то же значение и сохранит ту же память. Исполнение всех входов при компиляции невозможно; сначала корпусные и независимые oracle-гарантии для допускаемых семейств, статические legality checks и явные unknown/fallback. Отдельные успешные функции можно смешивать в модуле лишь после проверки их call edges и адаптеров; начать проще с **целого модуля**.

Текущий `LIR Match` записывается как `r.OK`, а convergence checker пропускает часть call/memory/multiblock случаев; это **не** сертификат эквивалентности. PBQP-hint в WFC также не фиксирует фактический ABI: его можно отвергнуть, если location недопустима. Перед портфельным выбором нужен явный concrete ABI, а не предположение «один MIR2 — значит одинаковые регистры на входе». Нынешний per-function fallback собирается сплайсингом текстового ASM по комментариям; для дальнейшего смешивания нужны структурированные function artifacts с символами, данными и зависимостями, затем проверка всего собранного модуля. Рекомбинация внутри basic block ещё труднее: для каждого фрагмента потребуются live-in/live-out, flags, side effects и корректные edge moves; сейчас она не нужна.

**Что считать лучшим:** сравнивать размеры *собранного* кода и измеренные T-states на одном target/scenario, включая вызовы, адаптеры, runtime и fallback; компиляционное время считать отдельно. Размер и скорость — разные цели, поэтому сохранять Pareto-набор или явный профиль (`size`/`speed`), а не обещать единственного победителя. Timeout, unsupported, assembler error и semantic mismatch входят в denominator и не получают score.

Минимальная запись сравнения: `inputHash, targetModelVersion, concreteABI, backend/options, nativeOrFallback, code/data/dependencies, diagnostics, evidence, bytes, cycles, compileCost`. Состояния `unsupported/generated/assembled/checked/rejected/timeout/fallback` различать явно. Общая модель допустимости регистров может использоваться всеми генераторами; семантический oracle не должен вызывать их rewrite/cost helpers, иначе возможна общая ошибка.

## Где рекомбинация приносит пользу

1. **ISLE до развилки — наиболее дешёвый перенос.** `lir.Combine` уже содержит правила вроде LE-load fusion и умножения на константу; приём в общий MIR2 pass возможен по одному правилу после указания side effects, widths/flags и before/after oracle. Две реализации ISLE (`pkg/lir/isle`, `pkg/rewrite/isle`) пока не считать готовым общим API. Отдельный case может оказаться лучше как локальный pattern в production Z80Codegen — выбирать по измерению.
2. **Общая модель ISA и legality — фундамент.** PBQP, LIR, VIR и GPU различно описывают aliases, flags, clobbers, IX halves и стоимость переходов. Начать с проверяемых `overlap/valid move/valid pattern` и тестовых случаев `B↔BC`, `H↔HL`, carry, call-live, а не с большого нового DSL. Один источник истины для допустимости важнее объединённого solver.
3. **PBQP→WFC уже подключён.** Можно измерить ablation подсказок на том же MIR2. WFC полезен как локальный constraint propagator в LIR; его успешность и пользу измерять отдельно от успеха PBQP fallback. Не подменять это рассказом про глобальный оптимум или настоящий entropy/backtracking.
4. **Таблицы→PBQP как кандидат, не жёсткая граница.** Требуются совпадающие shape/loc namespace, версии стоимости, проверка типа, alias, interference, ABI, clobber и повторная оценка в PBQP cost model. PBQP использует редукции и эвристику, поэтому таблица может стать incumbent/seed/affinity; «отсечь остальные» безопасно только с обоснованной нижней границей при той же цели. Таблицы не выбирают межпроцедурные контракты PFCCO; теоретически ими можно оценить кандидат-контракт вместе с allocation, но это более дорогой эксперимент.
5. **VIR как контрпример и генератор кандидатов.** Сравнивать с PBQP при уже фиксированном production ABI, отчётливо разделяя SAT/UNSAT/error/timeout и стоимость под общей моделью. Если VIR предложил меньший код, заново собрать и исполнить его; обнаруженное правило или модельный gap переносить в маленький production pass. Слепое сплайсирование VIR ASM не нужно.

## Почему большой «единый solver» сейчас плохая инвестиция

Существующий [августовский аудит](2026-08-21-117-Solver-Architecture-Audit.md) воспроизвёл wrong-code `gcd` в LIR и VIR при корректном PBQP на той базе; 18 из 39 C-файлов на LIR потребовали per-function fallback. Это исторический срез `77f8ed19`, **не текущий регрессионный счётчик**. Текущий код всё ещё содержит ранний `LIRCheck`, мутации VIR, LIR globals и разные описания ISA. Пока нет общего интерфейса ошибок/стоимости/ABI и holdout-evidence, монолит увеличит поверхность wrong-code и сделает атрибуцию хуже.

## Ранжированный план и критерии остановки

```text
P0 [F, полезность 5] Единый сценарий сравнения на фиксированном MIR2/ABI
   ├─ pinned input+SHA+flags+target, отдельные процессы/копии, timeout
   ├─ статус каждого кандидата: native/fallback/unsupported/error/timeout
   ├─ MZA+MZE, независимый expected observation, испорченный ABI/код как негативный контроль
   └─ размер/T-states/compile-time только для семантически допустимых
P1 [Q, полезность 4] Первое абляционное измерение PBQP ↔ LIR
   ├─ маленький корпус single-block + CFG/calls/flags/spills
   ├─ PBQP hints on/off, LIR native/fallback отдельно; GCD как regression
   └─ если нет выигрыша без регрессий — оставить LIR исследовательским
P1 [F, полезность 5] Общая Z80 legality и cost-contract
   ├─ aliases/widths/flags/clobber/ABI tests, затем единые валидаторы
   └─ проверка модельных расхождений GPU↔Go↔SMT на малом exhaustive subset
P2 [Q→F, полезность 3] Перенос одного ISLE-правила
   ├─ MIR2 before/after + VM/независимый oracle + MZA/MZE
   └─ holdout bytes/T-states; при нуле или wrong-code откатить именно правило
P2 [F, полезность 3] Table/SMT candidates
   ├─ 4v/5v hit/legality/replay, VIR с production ABI и machine-readable outcome
   └─ IX-6v только после keyed/deterministic rebuild и spot-check
P3 [E, полезность 2] Runtime portfolio selector / общий solver
   └─ обсуждать после подтверждённого выигрыша и одинаковых контрактов
```

**Первый конкретный срез:** расширить существующий `pkg/hir/production_dual_run_test.go` или сделать рядом offline runner на 5–10 маленьких tracked MIR2 функций, включая `gcd`, один flag-result и один call; запустить PBQP и opt-in LIR с одинаковым target, отдельно отметить нативный LIR и fallback, собрать MZA, исполнить MZE и сохранить JSONL-строку на функцию/кандидата. Случаи unsupported и заведомо испорченного ABI должны дать отказ, а не «победителя». VIR добавлять после фиксации ABI и изоляции его мутаций. Это не требует сначала переписывать регистровый allocator.

## Источники проверки

- Production route, PBQP hints, fallback, ранний `LIRCheck`: [`pipeline.go`](../minzc/pkg/pipeline/pipeline.go).
- PFCCO и PBQP: [`contracts.go`](../minzc/pkg/mir2/contracts.go), [`contracts_pbqp.go`](../minzc/pkg/mir2/contracts_pbqp.go), [`pbqp.go`](../minzc/pkg/mir2/pbqp.go).
- LIR path: [`pipeline.go`](../minzc/pkg/lir/pipeline.go), [`wfc.go`](../minzc/pkg/lir/wfc.go), [`combine.go`](../minzc/pkg/lir/combine.go).
- VIR mutation and oracle boundary: [`vir/pipeline.go`](../minzc/pkg/vir/pipeline.go), [`vir-oracle/main.go`](../minzc/cmd/vir-oracle/main.go), [ADR-0043](../docs/adr/0043-vir-demoted-to-offline-oracle.md).
- Таблицы: [аудит 2026-10-01](2026-10-01-Regalloc-Table-Audit-RU.md), [CI gate](../.github/workflows/ci.yml).
