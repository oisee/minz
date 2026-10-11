# Полное удаление LIR: конкретный инвентарь

Проверен код `ea4039e7`, без запуска тестов и без изменения production. JSON рядом перечисляет каждый прямой импорт и все 27 нетестовых Go-файлов LIR. В каталоге LIR 47 Go-файлов и 72 объявления тестов верхнего уровня. Исторические копии исходников внутри доказательных пакетов не являются живыми зависимостями и остаются неизменными.

**VIR может остаться после удаления LIR:** его единственный прямой импорт LIR нужен для имени ASM-метки. Production pipeline дополнительно связан с необязательной диагностикой convergence. Включение native LIR уже отключено; удаление LIR не требует замены текущего production backend.

## Прямые потребители

| Файл | Что используется | Действие |
|---|---|---|
| `minzc/pkg/pipeline/pipeline.go` | `SanitizeAsmLabel`, `ConvergenceResult`, `CheckModuleConvergence` | Вынести метки; убрать LIRCheck и LIRResults либо заменить явно новой диагностикой native/VIR |
| `minzc/pkg/vir/pipeline.go` | Только `SanitizeAsmLabel` при глобальных данных | Переключить на общий пакет меток; VIR не зависит от WFC/LIR IR |
| `minzc/pkg/pipeline/lir_test.go` | Convergence, research codegen, disabled-backend поведение | Сохранить исходный корпус и native ожидания; убрать тесты удалённой возможности и сверить provenance |
| `minzc/pkg/hir/exhaustive_judge_test.go` | Multi-block research emitter, LIRCheck/Results | Сохранить GCD/арифметические/shift/negative-control native оракулы; удалить только LIR-ветви |
| `minzc/pkg/hir/lir_research_judge_test.go` | Codegen, Z80 locations, hints | Перенести полезные expected-value cases и composition tests в native/allocator gates |
| `minzc/pkg/hir/lir_allowlist_judge_test.go` | Codegen и fallback allowlist | Отделить независимый oracle/ordering от теста удалённого backend |
| `minzc/pkg/z80spec/spec_test.go` | LIR locations, `LocIndex`, сравнение namespace | Оставить canonical и VIR/MIR проверки; исторический namespace не должен импортировать удалённый пакет |

## Самое первое и безопасное

1. Вынести **точную** реализацию `lir.SanitizeAsmLabel` в общий пакет ASM-меток и переключить два внешних потребителя. Внутри LIR временно можно оставить wrapper. Проверить зарезервированные имена, `@ . - $`, globals и empty-function stubs, а также неизменность BIN на существующих fixtures.
2. Не подменять helper текущим `mir2.sanitizeIdent`: у него другая политика — заменяет **все** небуквенно-цифровые символы; LIR заменяет только четыре. Имена с апострофом, пробелом или Unicode могут измениться. Полную унификацию провести отдельно с явным контрактом символов.
3. Затем убрать optional convergence coupling из `pipeline.Options.LIRCheck`, `Steps.LIRResults` и соответствующих тестовых ветвей. Это не подтверждение native корректности: независимые VM/Z80 ожидания должны остаться.
4. После переноса тестов и доказанной пользы удалить весь `pkg/lir`, включая `pkg/lir/isle`, и проверить импорты. Сохранить архив, если нужен воспроизводимый research baseline; архив не остаётся частью Go build.

## Интерфейсы, которые легко пропустить

- `Options.UseLIR` сейчас ничего не включает: печатает предупреждение и записывает `PBQP (lir disabled)` в traces. CLI `--lir` и `--z3` есть в `cmd/minzc/main.go`; `--z3` тоже no-op. Удаление пакета может сохранить ограниченный deprecated CLI shim, но цель полного исключения требует решения о сроке удаления этих флагов/полей, а не бесконечного wrapper.
- `FuncTrace.Backend`, summary order, ASM marker extraction и комментарии pipeline ещё перечисляют LIR и LIR+PBQP fallback. Некоторые helpers остаются нужны VIR/PBQP; убрать только LIR-форматы после call-site аудита.
- `lir.UseZ3` применяется только самим research emitter. `lir.UseLIRContracts` не имеет живого внешнего потребителя; не переносить мёртвый глобальный переключатель.
- `z80spec.FromLIR/ToLIR` не импортируют LIR: исторические индексы находятся в независимой таблице. Их можно архивировать/удалить отдельно, сохранив canonical overlaps, halves, widths и VIR/MIR maps.
- **ProfileCompilationID хеширует JSON всего Options** (`pkg/pipeline/profile.go:25`). Удаление даже всегда-false UseLIR/LIRCheck меняет идентификатор сборки. Это допустимая новая конфигурация, но старые профили надо явно отклонять/пересобирать. Не переписывать старые evidence IDs и не обещать неизменный replay. `ProfileVariantSetEvidence.Options` также сохраняет Options.
- HIR fingerprint использует reflection по полям семантических структур (`hir/profile_compat.go`); перенос utility его не меняет. В проходах есть reflection обход HIR; это не скрытая зависимость от LIR options.

## Что полезно, что уже есть

| Компонент | Реальное состояние | Назначение и требуемая проверка |
|---|---|---|
| Shared parser/facts | `rewrite/sexpr`, `rewrite/datalog` уже извлечены; HIR/Grace/VIR ими пользуются | Сохранить; удалить LIR wrappers/duplicate FactDB после проверки тестов |
| ISLE matcher | LIR Combine вызывает `lir/isle`; `rewrite/isle` существует отдельно и пока без production caller | Не выдавать декларации за исполнение. Сверить guarded API/правила/fixtures, если matcher нужен; иначе архивировать LIR matcher |
| Byte constant multiply | Все коэффициенты LIR 0/1/2/3/4/5/6/8/10/12 уже покрыты native `z80codegen_muldiv.go`; native дополнительно 7/9/15 | Копирование не приносит пользу. У VIR есть таблица K=2..255: новые коэффициенты переносить только с верификацией sequence, scratch, flags и реальными T/bytes |
| Word constant multiply | Native 16-bit покрывает powers-of-two, 3/5/6/9; 10/12 уходят в runtime | Возможный конкретный gap. Сначала реальный источник, затем unsigned width/overflow oracle и native cost; LIR выражение не доказательство выгодности |
| Load16/store16 fusion | LIR Combine делает load fusion; VIR исполняет Go fuseLoad16LE/fuseStore16LE, S-expr лишь текст | Не переносить нынешние DAG удаления blindly. Требуются use counts, dominance, order, alias/effects, volatile/MMIO и wrap65535. Target: MIR rule + native lowering |
| CFG шесть правил | loop_rotate_djnz, loop_rotate, empty_block_elim, block_merge, branch_to_next, branch_inversion | Сопоставить с native/Grace существующим DJNZ/layout/ret passes; переносить gaps с zero-trip и edge-args oracle |
| KnownPatterns catalogue | sub_cmp, split_join_ret, cond_ret_sink, sub_swap_neg, abs_diff, inc_dec, cp_zero, dead_ret, forwarding, djnz | Значительная часть имеет native специализированные проходы/Grace runner; каталог — не дополнительная работающая оптимизация |
| WFC + location sets | Solver, call clobbers, same-op destructive ties, interblock propagation | Переносить **недостающие ограничения**, не solver: canonical z80spec/PBQP/VIR admission; пары/половины, flags, indexed prefix, scratch, stack, spills, call ABI |
| Z3 / e-graph selection | Research joint isel+regalloc, RISC32/RISC8/CISC/MICRO machines | При необходимости archive/research VIR. Не сохранять production LIR ради research solver; эталон optimality относится к полному формализованному cost/constraint problem |
| Outparam promotion | Только detector, нет production caller и нет переписывания call sites | Будущий PFCCO semantic transform; current detector проверяет Src, но не все Args/терминаторы/escaping alias. Не готовый quick win |
| Spill repair | LIR textual InsertSpillReloads | Не переносить workaround. Native correctness через allocator/codegen contracts и actual stack/call tests |
| Z80 validation | LIR wrapper уже delegates shared z80validate; per-template assembler admission отделён | Общий валидатор сохранить; actual assembly admission полезно table/VIR, но не доказывает semantics |

## Почему нельзя переносить peepholes целиком

`lir/z80peephole.go` утверждает доказанность всех правил, однако код не проверяет live flags/effects. `ADD A,1 → INC A` и `SUB 1 → DEC A` меняют carry; удаление `AND 255`/`OR 0`/`XOR 0`, двойного NEG/CPL/CCF и замена `CP 0 → OR A` меняют наблюдаемые флаги. Даже `LD r,X; LD r,Y` не проверяет, что второй источник использует r; `LD A,7; LD A,A` нельзя свернуть в один `LD A,A`. Повторные memory LD могут быть MMIO, а удаление PUSH/POP меняет записи памяти/наблюдения стека. Это находки по коду, без запущенного контрпримерного теста в этом инвентаре.

Переносить только с явно указанным read/write-set и dead-flags/dead-value/effect proof. Заявление GPU/exhaustive о равенстве A на 256 входах не заменяет проверку F, carry, всех scratch-регистров и памяти.

## Условия фактического удаления

- Production и сохранённый VIR не импортируют `pkg/lir` либо `pkg/lir/isle`; весь Go module собирается без каталога LIR.
- Все обещанные source cases и независимые ожидаемые значения сохранены в native/MIR gates; отрицательные controls остаются отрицательными.
- Новые полезные правила проверены на собранном Z80, имеют Speed/Size costs; уже имеющиеся правила не повторяются под новым именем.
- CLI/Options/Steps/provenance приведены к конечной архитектуре, old profile/configuration IDs не маскируются.
- Shared z80spec остаётся единой моделью physical overlap; проверенные таблицы VIR сохраняют versioned constraint/cost namespace.
- Исторические отчёты и raw artifacts сохранены на шаре; архив research LIR не входит в build graph.
