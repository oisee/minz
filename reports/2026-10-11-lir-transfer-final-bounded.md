# LIR: финальная ограниченная сверка переноса

Срез `354a47efb3b734e5ef0d1a9c9f1eb20a6f8a3393`. После интеграции typed identities **не обнаружен конкретный рабочий production feature или production expected-value test архива, потерянный при удалении LIR** в проверенных ниже доменах. Это bounded audit: ни полнота ISA constraints, ни вся Grace-цель этим не доказаны. Новые runtime gates не запускались; исходники только прочитаны и зафиксированы.

Архив: `docs/grace-lir-final-removal/research-lir-c3864cf2.tar.xz`, исходный commit `c3864cf2e3f2ab379e92c98c001ab4341586c5cd`, SHA256 **7c68901fcbb4e319b1ed4380e8a1e73fa2270e8e72180bc8778d8330c73adfc4**. Повторно проверен каждый payload по исходному archive-manifest. Это integrity check, а не доказательство функций по числу файлов. Exact excerpts и текущие source hashes: [verification.json](lir-transfer-final-bounded/verification.json).

## Рабочая польза и активные замены

Все Go-пути далее относительно `minzc/pkg`; архивные пути лежат в указанном tar.xz.

| Архивная возможность | Текущая реализация и смысл проверки |
|---|---|
| Constant byte multiply; word ×10/12, `lir/combine.go:44–81`, native instruction patterns | `mir2/z80codegen_muldiv.go:351,356`; `mir2/mul16_small_constants_test.go:14`. Original-source/native и physical preservation evidence в `docs/grace-mul16-10-12/REPORT.md`: HL 1079→96T и 32→9B. Byte coefficients уже были в обычном backend; повторный перенос рецептов не нужен. |
| Value zero identities, `lir/combine.go:75–93` | `mir2/constprop.go:475,628`, `typed_zero_identity_test.go:21,58,112`; `pipeline/typed_zero_identities_native_test.go:17`. Same canonical u/i8/16 operand types, typed stored-zero, hard/effect/flag guards. Byte MUL0 сохраняет дешёвый native XOR; mixed i8/u8 остаётся вне допуска. `reports/2026-10-11-lir-typed-zero-identities.md`: frozen **новый** source, raw/finalVM+trueCALL Z80; all byte values/14word boundaries; BIN175→86B, workload158812→145384T во всех трёх режимах, без ухудшенных entry. Это не original application gain. |
| Little-endian load combining, `lir/combine.go:30–38,101` | `mir2/byte_word_fusion.go:6`; `byte_word_fusion_test.go:37,127,197,240`; `fusion_independent_audit_test.go:21,87`; `pipeline/fat_word_fusion_test.go:19,197`. Сохранены sequential byte origins/address proofs, callbacks, wrap/fault prefix и CY; barred/shared-DAG формы отвергаются. Original FAT native packet: `docs/grace-fat-word-fusion.md`, 115200→89088T, fullBIN9543→9397B. Это проверенная ограниченная форма, не generic fusion. |
| CFG loop rotation/DJNZ/layout/merge/branch patterns, `lir/cfgrules.go:137,165,192,216,243,265` | Нужное поведение находится в `mir2/grace_runner.go:76,88,193`, `layout.go:14,73`, `condret.go:32,197,249`, `z80codegen_term.go` и DJNZ emitter. Native GCD/sums и edge/counter-home tests сохранены; exact case mapping — `reports/2026-10-11-lir-test-transfer-completion.md`. Не требуем того же IR layout или counts удалённого solver. |
| Param/return conventions, live call results, stores/spills, physical aliases/destructive ties | Canonical overlap — `z80spec/locations.go:129,137,154`; ordinary constraints — `mir2/pbqp.go:121,144,495`; actual parallel/spill/call bridge — `mir2/z80codegen.go:864`, `z80codegen_parallel_overlap_test.go:47,187`, `z80codegen_dword_parallel_test.go:11`, `accumulator_exhaustion_test.go:8`, `accsaves_lifetime_test.go:8,100`. Checked local admission — `vir/table_checked.go:107`, `table_admission_test.go:144,224,242`, `table_scratch_test.go:10,49,79`, `table_abi_binding_test.go:24`. Это конкретные active constraints/native tests, не утверждение эквивалентности всех WFC решений. |
| Shared facts/parser/matcher, ASM name and validation utilities | `rewrite/{sexpr,datalog,isle}`, `asmutil`, `z80validate` сохранены; нет живого LIR import. `pipeline/lir_retirement_config_test.go` оставляет historical config rejection; старые IDs не переписаны. |

## Тестовый контракт сохранён

`hir/lir_allowlist_judge_test.go` → `hir/native_lowering_judge_test.go:95`: native arithmetic/casts/bitwise expected values и negative mismatch ordering. `fc`, `h_c1`, `h_c4` на строках 155–185 сохраняют результат/параметр через CALL, conditional path и word return с live caller value.

`hir/lir_research_judge_test.go:145` → `hir/native_composition_test.go:30`: **точное тело** `TestNativeCompositionExecution` совпало побайтно после единственного переименования `lirCompositionFixtures`→`nativeCompositionFixtures`. Шесть source/model fixtures a1/a2/a4/a5/a8/a9 сохранены; byte арифметика modulo256. Unary — все256; два аргумента — все65536; дополнительные c/d **3/4**. Полный 3/4-argument Cartesian был отдельным `MINZ_RUN_KNOWN_RED` research test (архив строки103–107); не выдаём его за текущий native PASS. Реальный Fibonacci, GCD, wordlogic и source corpus ожидаемые значения сопоставлены в прежнем exact transfer report; текущие источники проверены снова.

Таким образом, ограничение c/d не создано удалением LIR. Счётчик parser declarations и размер архива не используются как критерий выполнения.

## Что не было готовой production возможностью

- **Outparam:** `lir/outparam_promote.go:35–95` только DetectOutparams/OutparamReport; отсутствует actual signature/callsite rewrite. Detector смотрел Src, но не все Args/terminators/escaping aliases. Semantic outparam→tuple optimization остаётся возможной новой работой PFCCO с alias/effect proof, а не потерянной реализацией.
- **KnownPatterns:** `lir/rules.go:207,368` — каталог и четыре rewrite descriptions; запись имени не означает активный безопасный pass. Нативные condret/sub-swap/DJNZ и forwarding уже существуют, с более строгими flag/edge guards.
- **Все32 Combine declarations:** shared fixture сохранён, но `lir/combine.go:222–249` termToOp умел извлекать только load16_le/const; прочие outcomes могли возвращать error и оставлять старую op (`:135–143`). Поэтому parser PASS никогда не доказывал 32 working optimizations. Новые typed identities подтверждены отдельно.
- **Textual peepholes:** `lir/z80peephole.go:80–125,183` удалял ALU flags и PUSH/POP без liveness/effect proofs. ADD1→INC меняет carry; OR0/XOR0 removal меняет flags; PUSH/POP удаляет memory writes. Их буквальное возвращение не требуется и небезопасно.
- **WFC/Z3/egraph/multi-machine:** архив WFC действительно выполнял vreg consistency/clobber/inplace/same-instruction propagation (`lir/wfc.go:198,253,328,408`), это работающий **research solver**, не пустая заглушка. Его constraints покрыты конкретными canonical/allocator/admission/native tests выше, однако полного relation-by-relation equivalence нет. Не сохраняем старые optimum/count/abstract-machine results как native guarantees.
- **Composition/islands:** current standalone VIR research `vir/pipeline.go:1294` cut-vertex и `:1754–1783` island stitch не становятся production proof: второй island решается independently, boundary LD moves всё ещё placeholder. Production checked-local `vir/table_production.go:20–25,69` явно исключает canonical indirect entry bridge, calls/CFG/unsupported op и требует whole contract+return. This refusal is retained, а не скрытый перенос islands.

## Конкретный следующий шаг

Нового обязательного production переноса из архива не найдено. Самое небольшое полезное усиление доказательства — расширить **существующие unchanged** a1/a4/a8 native models на заранее зафиксированные дополнительные c (0/1/127/128/254/255), а a2 на пары c/d, затем при необходимости отдельный all256-c sweep. Проверять реальный CALL/return/SP, registers/memory и moves/saves. Это coverage gap, существовавший до удаления; не основание объявлять whole-goal blocked или реставрировать solver.

Если цель будет расширена до arbitrary shape composition, нужен самостоятельный separator/boundary-move contract и whole assignment AddChecked/native oracle. Нынешний research placeholder такого доказательства не имеет. Общую Grace/table/PFCCO цель данный отчёт не закрывает.
