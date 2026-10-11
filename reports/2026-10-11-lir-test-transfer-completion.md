# LIR: аудит переноса полезных тестов

Срез текущего кода: `f758c0be75217e9ab9e9de496104e4b7efa56e65`.
В проверенных архивных expected-value, call и CFG случаях потерянного
production-оракула не обнаружено. Это заключение о сохранении тестовых доменов,
а не доказательство полноты переноса каждого ISA constraint или оптимизации.
Новые runtime прогоны для этого аудита не запускались.

Архив [research-lir-c3864cf2.tar.xz](../docs/grace-lir-final-removal/research-lir-c3864cf2.tar.xz)
взят из `c3864cf2e3f2ab379e92c98c001ab4341586c5cd`.
SHA256: `7c68901fcbb4e319b1ed4380e8a1e73fa2270e8e72180bc8778d8330c73adfc4`.
Повторно проверены содержимое и SHA256 всех 53 файлов: 47 внутри LIR,
шесть внешних consumers. [Проверка и точные выдержки](lir-test-transfer-completion/verification.json)
содержат исходные пути, хеши текущих источников и архивные тела тестов.

## Конкретное сопоставление

Все пути в таблице начинаются с `minzc/pkg/`. Архивный путь означает файл
в указанном tar.xz, текущий — активный Go-тест в данном срезе.

| Архивный путь / тест | Текущее покрытие | Что сохранено и что изменилось |
|---|---|---|
| `hir/lir_allowlist_judge_test.go`: `TestExhaustiveJudgeNativeLoweringCases`, `TestCodegenJudgeMismatchOrdering` | `hir/native_lowering_judge_test.go`: те же имена | Независимые модели mul8/mul3, word mul2/3/10, ext/ext2/trunc, byte/word AND/OR/XOR; настоящий production CompileHIRSteps и Z80. Byte пары — 65 536, word logic — 65 536 × 8 вторых аргументов. Детерминированный отрицательный контроль оставлен. |
| Тот же файл: fixtures `fc`, `h_c1`, `h_c4` | Тот же текущий файл и fixtures | Результат первого CALL и параметр живут через второй CALL; conditional callee и перестановка аргументов; word call plus live caller value. `fc` и `h_c1`: 65 536 входов; `h_c4`: все 65 536 значений первого аргумента, второй 7. Сейчас требуют ноль ошибок. |
| `hir/lir_research_judge_test.go`: `TestNativeCompositionExecution` | `hir/native_composition_test.go`: тот же тест | Все шесть sources/models `a1/a2/a4/a5/a8/a9`. Тело production-теста совпадает побайтно после переименования fixture-функции. Два аргумента исчерпываются; третий/четвёртый равны 3/4, unary — 256 входов. Это не уменьшение старого production-домена. |
| Тот же архив: `TestLIRResearchCompositions`, `TestLIRResearchByteArithmetic`, `TestLIRResearchComparisonValues` | Известные ошибки LIR оставлены в архиве; production byte arithmetic/comparisons в `hir/exhaustive_judge_test.go` | Полный 3/4-аргументный Cartesian до 2^32 был отдельным opt-in `MINZ_RUN_KNOWN_RED` исследованием. Он не объявляется текущим native PASS. |
| `hir/exhaustive_judge_test.go` | Тот же текущий путь: `TestExhaustiveJudgeGCD`, native single/wide arithmetic, byte shifts, negative controls, worker ownership, mismatch ordering | Независимые production VM/Z80 expected values сохранены. Удалена известная красная ветвь multi-block LIR, а не production GCD. |
| `lir/interblock_test.go`: `TestGCD_VM`, `TestGCD_Asm`, block params; `lir/loop_test.go`, `looprot_test.go`: суммы DJNZ/while | `hir/exhaustive_judge_test.go:TestExhaustiveJudgeGCD`; `mir2/examples_test.go:TestGCDZ80`, `TestSumRangeZ80`, `TestSumArrayDJNZZ80`; `mir2/z80codegen_djnz_location_test.go` | GCD и суммы имеют actual Z80 модели; counter homes, edge copies, explicit counter и long body проверяются непосредственно. WFC RPO/propagation/abstract machine convergence остаются архивной реализацией удалённого backend. |
| `lir/e2e_test.go:TestE2E_Fibonacci` и `convergence_test.go` arithmetic | `hir/exhaustive_judge_test.go:TestExhaustiveJudgeNativeSingleBlock`; настоящий `hir/lower_test.go:TestHIRFibonacci`; `pipeline/showcase_fib_iter_oracle_test.go:TestOriginalFibIterAllBytesVMZ80` | Архивный «Fibonacci» вычислял только `(40+3)-1=42`, не fib. Текущий HIR тест выполняет loop для n=0..7; оригинальный showcase имеет all-byte VM/Z80 модель. Abstract RISC8/RISC32/CISC/MICRO targets удалены вместе с LIR. |
| `lir/e2e_test.go:TestE2E_BitwiseOps`, `TestE2E_RegisterPressure` | Native lowering logic выше; `vir/solver_test.go:TestZ3SolverHighPressure`, `TestSpillInsertion`; `mir2/z80codegen_spill_copy_live_test.go`; текущие native wide edge/call tests | Арифметические значения и live spill/move correctness проверяются на actual ISA. Старые WFC allocation counts и результат его abstract VM не являются текущим API контрактом; точный старый fixture сохранён архивом. Это доменное сопоставление, не заявление идентичного allocator trace. |
| `lir/combine_test.go` little-endian ld_word: `TestCombine_Load16LE_Convergence`, `TestCombine_Load16LE_FullPipeline` | `mir2/byte_word_fusion_test.go`; `pipeline/fat_word_fusion_test.go:TestFATSequentialWordFusionNative`, `TestFATSequentialWordFusionNativeWrap`; `mir2/fusion_independent_audit_test.go` | Перенесена проверяемая same-block byte→word форма, native неизменённый FAT helper, wrap, original memory access/fault prefixes, callbacks, CY и typed constants. Старое уменьшение abstract LIR ops 8→2 не доказательство generic memory fusion. |
| `lir/combine_test.go:TestCombine_ISLE_RuleParsing`; `lir/isle/isle_test.go` parser/matcher/rewrite tests | `rewrite/isle/legacy_combine_test.go:TestLegacyCombineDeclarations`, `rewrite/isle/isle_test.go` | 32 declarations сохранены как `rewrite/isle/testdata/legacy-combine.isle`; shared matcher работает. Parser PASS не подтверждает семантику всех объявленных rewrite rules. |
| `z80spec/spec_test.go` widths/kinds/containment/overlaps, VIR aliases | Тот же текущий файл: `TestDocumentedWidthsAndKinds`, `TestContainmentAndAliases`, `TestOverlapProperties`, `TestVIRPairAliases`, `TestMIR2KnownAliasGaps` | Canonical ISA relation продолжает проверяться. FromLIR/ToLIR namespace и LIR-empty-gap checks относятся к удалённому adapter. |
| `pipeline/lir_test.go`: C89/Nanz/Lanz/Lizp corpora и output | Текущие source/native corpus suites `pipeline/showcase_*oracle_test.go`, `showcase_native_oracles_test.go`; обычный CompileHIRSteps | Старые тесты измеряли LIR conversion/codegen diagnostics, не независимый oracle каждой программы. Нельзя приравнять их старые counts к доказательству полного нового corpus PASS. |
| `lir/allowlist_test.go`, `bridge_honesty_test.go`, `bridge_test.go`; WFC/Z3/egraph/saturate/cfgrules tests | Архив; собственное admission `vir/table_checked_test.go`, `table_abi_binding_test.go`; shared rewrite tests | Запреты неполного flat-LIR bridge и его solver scaffolding не переносятся буквально в backend с более широкой семантикой. Current checked-table ABI/ISA/interference/flags проверки самостоятельны. |

## Предел заключения

Точный production composition-domain сохранён. Полного native перебора
третьего/четвёртого аргумента у этих шести fixtures нет: это существовавшее
ограничение, а не потеря при удалении LIR. Полезное следующее усиление — все
значения c для трёхаргументных fixtures и заранее выбранные c/d для `a2`,
с независимой моделью и проверкой реальных moves/saves.

Архивные multi-machine convergence, WFC/Z3 optimum counts, RPO propagation
и flat-bridge rejection fixtures оставлены воспроизводимыми research.
Ни эта таблица, ни отсутствие LIR imports не доказывают глобальную оптимальность
распределения, все возможные call/CFG shapes или полноту каждой старой pattern.
Актуальные ограничения переноса задокументированы отдельно в
[инвентаре](../docs/grace-lir-removal-inventory/README.md) и
[условиях удаления](../docs/grace-lir-final-removal/README.md).
