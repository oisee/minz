# Byte ForEach countdown: VM и машинный Z80

Byte ForEach без break/continue/return/goto/label теперь проверяет нулевую длину один раз и использует MIR TermDJNZ на обратном ребре. Wide lengths и bodies с control transfers остаются на general trampoline lowering. Первый body parameter получает декрементированный counter; остальные loop-carried переменные передаются явными аргументами. Source loop lineage сохранена.

Z80 codegen учитывает фактическое расположение counter: B использует DJNZ, другие byte registers — DEC и условный переход; spill сохраняет A/HL и флаги декремента. Копии выполняются на соответствующем ребре. Явный Counter в BodyArgs/ExitArgs сохраняет исходное SSA значение до декремента, как MIR VM. Дальние DJNZ используют соседнюю taken label и relaxable JRS; маленькие тела сохраняют прямой DJNZ. Grace forwarding actions перенаправляют DJNZ exit и BrIf2 edges; DJNZ body forwarding консервативно запрещён.

Machine regression исполняет 70 случаев: register homes A/B/C/D/E/H/L/IXH/IXL/IYH/IYL/spill, разные body/exit copies, implicit counter с другим home, явные counter edges с VM comparison, 200 NOP long body. Отдельный ForEach regression проверяет loop-carried u16 сумму и control fallback. Вызовы функции в теле проверены на VM и Z80.

Seeded caller: 18 ordered calls, seeds0/1/65535 × lengths0/1/5/16/64/255; каждый кандидат проверяется по independent modulo65536 result, snapshots и exact reads. Original148452T/62B, pointer139581T/156B. Speed выбирает earliest pointer candidate1 (−8871T, около6%); size сохраняет original0. Предыдущий pointer241596T/164B сохранён в истории отчёта, новая версия сокращает его стоимость примерно42%. VM address domain15360 cases и Z80 boundaries48 configs сохраняют результаты и адресную семантику.

Полный standalone sum_array workload0/1/5/16/64/255:48495→13879T (около71%); extra55modulebytes.

Standalone sum_array модель после нового lowering: original69+141n; fast n=0:212, n>0:210+37n; equality добавляет7; fallback231+143n. Module bytes128/183. Все1022 address/length rows проверяются actual assembled execution и raw VM. Profile schema теперь minz.input-hir-loop-observations/v5, semantics loop-events/v2: required fast_zero_entries отделяет zero-trip от positive entries. Model требует FastZeroMeasured и FastZeroEntry; malformed, stale, missing/null, impossible zero observations отклоняются. Measured policy использует точный zero count и positive affine costs, не подгоняет одну прямую через discontinuity. Старые профили требуют пересбора.

Общая цель остаётся проверяемой оптимизацией реальных программ до бинарного Z80; hardware проверки ещё впереди. Артефакты и окончательные gate результаты: [локальный набор](grace-pgo-djnz-artifacts/README.md).

Составной caller с двумя независимыми source sites: original1417060T/249B, scalar-only726480T/220B, pointer-only1244100T/303B, both553520T/274B (прежний both645640T/276B). Speed выбираетboth3, size scalar-only2; все8candidates сохраняют результат и память. Built CLI seeded search выбрал exactpointer candidate1; selectedbinary SHA256 сверён.

Финальные проверки: весь pipeline196.898sPASS и отдельный бюджетный regression517.187sPASS; HIR/MIR2/Nanz8.234/4.415/1.648sPASS, profileCLI13.427sPASS. Showcase31existingfailures0new. Общая цель продолжается; следующий bounded этап — weighted pointer/scalar composition.
