# Хэндовер: MIR2→Z80, PFCCO/PBQP, LIR/WFC, VIR и таблицы

**Дата/база:** 2026-10-01, `main` на `a5918c5c` после PR #52; этот файл добавлен следующим PR. После перезапуска сначала проверить `git status --short --branch && git log -5 --oneline` и поправить baseline, если main ушёл вперёд. Не добавлять посторонние untracked-файлы. Полный проектный контекст и R2 correctness track — в [основном хэндовере](HANDOVER-2026-10-01.md). Развёрнутое обоснование решений — в [архитектурном отчёте](../../reports/2026-10-01-Backend-Portfolio-Synthesis-RU.md).

## Принятое решение

Портфель backend имеет смысл как **offline сравнение кандидатов**, а не как срочная замена production. Default Z80 остаётся `MIR2 → OptimizeContracts (greedy PFCCO) → PBQPAllocate → Z80Codegen`. При `UseLIR` PBQP всё равно вычисляется, даёт WFC мягкие hints и код для per-function fallback. `vir-oracle` отдельно использует VIR/Z3 и таблицы; его меньший ASM не является доказательством корректности. Три ветки **можно** запускать над логически одинаковым MIR2, но **нельзя** безопасно запускать их goroutines на одном изменяемом `*mir2.Module` или выбирать минимум по ASM-строкам.

Пока работа по backend-портфелю подчинена основному R2.2: ABI-aware MIR2 VM↔собранный Z80 parity. Не переключать default и не объединять всё в новый solver до общего legality/ABI контракта и честного holdout.

## Карта слоёв

```text
frontends → typed HIR → MIR2 + общие passes/Verify
                       → inlining/pruning → PFCCO contract (межфункциональный ABI)
                       → liveness/coalescing → PBQP (физические регистры)
                                               ├─ Z80Codegen → ASM (default)
                                               └─ hints → LIR Combine(ISLE) → isel → WFC → ASM
                                                         └─ при failure: PBQP ASM

                       MIR2 → VIR table/Z3/PIR → ASM (offline oracle)
```

ISLE — механизм правила/выбора в LIR, WFC — распространение доменов регистров и последовательный collapse; это не четвёртая независимая стадия IR. В `pkg/lir` и `pkg/rewrite` есть разные ISLE реализации. PFCCO выбирает соглашение вызова между функциями, PBQP — регистры внутри функции, VIR пробует совместное решение инструкций/регистров. Таблицы — кандидаты allocation под моделью генератора. Поэтому части могут помогать друг другу, но не заменяют друг друга по одному интерфейсу.

## Проверено в коде на этом срезе

- `pipeline.CompileHIRSteps`: `LIRCheck` расположен **до** позднего inlining/contracts/coalescing; `UseLIR` расположен **после** PBQP, получает `pbqpToLIRHints` и fallback в PBQP по функции. `LIRResults.Match` в этом режиме присваивается `r.OK` (успех генерации), не результат сравнения исполнения.
- `lir/wfc.go`: `Propagate` с forward/backward/consistency/clobber/interference; `Collapse` обходит клетки по порядку, а `pickPreferred` берёт PBQP hint только в допустимом домене. Не называть это entropy-driven backtracking или глобальным оптимумом.
- `vir/pipeline.go`: `CodegenFunc` меняет входной MIR2 (`FuseAbsDiff`/Grace, при опции inline). `lir/pipeline.go` имеет package-global опции. Для честного parallel run — отдельные процессы или независимые deep copies с зафиксированными опциями.
- `cmd/vir-oracle` сейчас сравнивает **число инструкций** и вычисляет усреднённое модульное время как суррогат per-function; semantics оно не доказывает. VIR Z3-PFCCO отличается от production и помечена проблемной в ADR-0043. Для сопоставления сначала фиксировать production ABI.
- Таблицы: PR #51 исправил учёт binary entries и reader; 4v файл (156 506 записей) прочитан с простым lookup. Заголовок 5v полный, но записи не replay-validated. PR #52 уточнил, что IX-6v dense был создан через unordered parallel feed без shape ID; существующий бинарник нельзя адресовать надёжно, он отклоняется.
- Датированный [аудит 2026-08-21](../../reports/2026-08-21-117-Solver-Architecture-Audit.md) зафиксировал wrong-code GCD в LIR/VIR и fallback 18/39 C-файлов на старом SHA; это **повод для текущего regression test**, а не актуальный процент успеха. Текущий PR gate проверяет стабильные пакеты и несколько smoke, но не пакет `pkg/lir`/`pkg/vir` как полный backend matrix.

## Что значит «лучший» и где поставить границу

Первый эксперимент сравнивает **целые модули** при одном target, MIR2-снимке и конкретном ABI. Для каждой ветки хранить: вход/hash, compiler/model version, режим, native/fallback/unknown, assembler status, runtime observation, assembled bytes, measured cycles, compile time. Нельзя выбирать по размеру, пока не подтверждены результат, flags, memory effects, calls, stack и clobbers в заявленном домене. Unknown или timeout не считать pass. Смешивание отдельных функций затем требует структурированных artifacts с call edges, labels/relocations, data/runtime dependencies и проверки итогового модуля; нынешний текстовый ASM-splicing не годится как общий интерфейс портфеля.

Ни MIR2 verifier, ни MZA, ни один набор входов не доказывают универсальную семантическую эквивалентность. До такого допуска автоматический выбор победителя остаётся offline-экспериментом. Метрики bytes и cycles раздельны; победитель определяется профилем задачи, включая стоимость adapters/runtime.

## Дерево следующих работ

Легенда: `P0` — безопасность/достоверность, `P1` — ближайшая полезность, `P2` — после gate, `P3` — исследование; `Q` — узкий quick win, `F` — фундамент, `E` — эксперимент.

```text
B. Backend synthesis — общий MIR2/ABI, production PBQP остаётся опорой
├─ B0 [P0 F] Продолжить R2.2 ABI-aware VM↔Z80 dual-run
│  ├─ flags, calls, shadow/stack/spill, clobbers, multi-return/unsupported
│  └─ negative control неправильного ABI и независимый oracle
├─ B1 [P0 Q→F] Offline PBQP↔LIR portfolio для малого tracked corpus
│  ├─ одинаковый вход/target/ABI; отдельные процессы/копии; timeout
│  ├─ native LIR и PBQP fallback разнести в отчёте
│  └─ MZA+MZE, expected result, assembled bytes/cycles, JSONL
├─ B2 [P1 F] Общий legality/cost contract Z80
│  ├─ aliases, widths, flags, tied operands, calls, clobbers, stack
│  ├─ проверка полного assignment и emitted instruction sequence
│  └─ малая GPU↔Go↔SMT cost-model сверка
├─ B3 [P1 Q→F] Перенести ровно одно полезное ISLE-правило
│  ├─ side effects/widths/flags → MIR2 before/after oracle → Z80 execution
│  └─ ablation bytes/cycles; не переносить язык правил целиком заранее
├─ B4 [P2 F] Table/VIR offline candidates
│  ├─ 4v/5v hit/reject/replay; VIR с production ABI, mutation isolation
│  └─ 6v после keyed/deterministic rebuild, не из текущего unordered bin
├─ B5 [P2 F] Structured per-function artifacts + final-module checker
│  └─ только после B1/B2 и подтверждённого выигрыша двух native путей
└─ B6 [P3 E] Автоматический selector или единый solver
   └─ только при измеренной пользе, ясном cost budget и no-regression gate
```

**Первое действие:** добавить узкий offline PBQP↔LIR experiment к существующему [ABI dual-run helper](../../minzc/pkg/hir/production_dual_run_test.go), не менять default. Взять scalar single-block, signed flag, один call, один CFG/GCD и forced LIR-unsupported; сначала добиться наблюдаемого статуса и одинакового исполнения, затем измерять стоимость. Для GCD не предполагать исторический failure без воспроизведения на текущем main.

## Команды и критерии передачи

```sh
cd /home/alice/dev/minz
git status --short --branch
git log -5 --oneline
cd minzc
GOCACHE=/tmp/minz-go-cache go test -short -timeout 5m ./pkg/hir/... ./pkg/mir2/... ./pkg/pipeline/...
GOCACHE=/tmp/minz-go-cache go test ./pkg/lir -run 'TestWFC_|TestGCD_WFC' -count=1
```

Эти команды — стартовый smoke, **не** gate портфеля. При реализации B1 добавить проверку MZA+MZE для каждого кандидата, статус fallback и отрицательный контроль. PR должен содержать входы, actual/expected, trace backend, версии модели и отдельные counts native/unknown; после зелёного PR gate обновить этот handover ссылкой на commit и фактической метрикой, без переноса старых процентов.

## Ссылки

- [Архитектурный анализ](../../reports/2026-10-01-Backend-Portfolio-Synthesis-RU.md), [аудит таблиц](../../reports/2026-10-01-Regalloc-Table-Audit-RU.md), [основной хэндовер](HANDOVER-2026-10-01.md).
- [Production pipeline](../../minzc/pkg/pipeline/pipeline.go), [PFCCO](../../minzc/pkg/mir2/contracts.go), [PBQP](../../minzc/pkg/mir2/pbqp.go), [LIR](../../minzc/pkg/lir/pipeline.go), [WFC](../../minzc/pkg/lir/wfc.go), [VIR](../../minzc/pkg/vir/pipeline.go), [oracle CLI](../../minzc/cmd/vir-oracle/main.go).
