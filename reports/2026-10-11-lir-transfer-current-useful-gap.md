# LIR transfer на текущем коде: один конкретный полезный остаток

Срез `08be2950`, read-only аудит. Счёт 47 LIR файлов/53 archive consumers
не является критерием completeness: архив сохранён, но не все исследовательские
механизмы были полезны production. SHA архива повторно проверен:
`7c68901fcbb4e319b1ed4380e8a1e73fa2270e8e72180bc8778d8330c73adfc4`.
Выдержки из точных archived sources и SHA current consumers сохранены в
`lir-transfer-current-completeness/verification.json`.

Ранее перенесённые полезные области действительно есть:

| Область | Current code / evidence | Граница |
|---|---|---|
| Word ×10/12 | `mir2/z80codegen_muldiv.go:351`, `mul16_small_constants_test.go`; docs/grace-mul16-10-12/REPORT.md | Native HL1079→96T,32→9B с live register/stack proof. Старая фраза «уходят в runtime» относится к snapshot и исправлена внизу inventory. |
| Byte/word load/store combining | `mir2/byte_word_fusion.go:6`, `pipeline/fat_word_fusion_test.go:19/197`; fusion_independent_audit | Опциональная same-block/effect-safe форма, native FAT115200→89088T и9543→9397B. Не универсальная DAG memory fusion. |
| Shared Combine declarations / parser | `rewrite/isle/testdata/legacy-combine.isle`, shared matcher tests | Parser32 rules не означает, что все32 оптимизации исполняются. |
| CFG/DJNZ/calls | Grace runner, dedicated native edge/call/flag tests и новые original corpus replays | Source edge args, zero-trip, return/SP preserved. Требование идентичного WFC/RPO trace не было native API. |
| ISA aliases/ties/scratch/spills | canonical z80spec, MIR interference/parallel copies и `vir/table_pattern_admission.go:80` | Проверены конкретные contracts, не заявлена глобальная полнота каждого ISA constraint. |
| Composition expected values | `hir/native_composition_test.go:30` сохраняет шесть archived fixtures | c/d фиксированы3/4 — ранее существовавшее ограничение, не потеря при удалении. Усиление Cartesian native proof полезно, но не новый feature. |

**Подтверждённый остаток — часть старых MIR identities.** Archive Combine
и сохранённый shared fixture реально содержат `or(x,0)`, `xor(x,0)`,
`sub(x,0)`, `and(x,0)`, `mul(x,0)`, `shl(x,0)`:
`legacy-combine.isle:46–60`. Current `SimplifyIdentities` (`constprop.go:473`)
в основном обрабатывает ptr_add0, sub(0,x), add0, mul1. Обычный source probe
показывает оставшиеся OR0/XOR0/AND0 и SUB0 с лишним LD C,0; исходный/final MIR
и ASM сохранены в probe-output.txt. Shl0 уже не исполняет native shift,
но MIR операция остаётся и препятствует простому alias/coalescing.

Это возможность компактного preallocation преобразования, не miscompile.
Следующая реализация: typed same-width MIR value identities, byte/word
модели и native cost до/после. Нельзя копировать старую textual peephole:
архив утверждает «provably correct», но `LD A,0→XOR A` или `ADD A,1→INC A`
изменяют флаги без анализа liveness. Новое MIR правило должно явно отказаться
при opaque ASM/physical flag dependence, hard constraints, unsupported
width/type или несовместимом effect metadata. Старое unconditional flags
переписывание не считается полезной возможностью, которую надо воскресить.

Другие неперенесённые области не были готовыми production преимуществами:
`outparam_promote.go` реализует detector/report, а не rewrite caller/callee;
WFC/egraph multi-machine convergence — исследовательский solver scaffold.
Checked tables всё ещё ограничены single-result leaf block (`table_production.go:25`)
и не допускают canonical indirect bridge (`:22`); calls/CFG/islands требуют
нового proof, а не переноса старого placeholder. Historical assignments уже
переиспользуются как revalidated proposals, поэтому старое «prefix sample
только» описание не является актуальным статусом indexed lookup.

В этом аудите не найден потерянный независимый expected-value oracle сверх
известных сохранённых границ. Все useful optimisation features ещё нельзя
объявить завершёнными, пока указанная группа identities не проверена и не
внесена. Audit не запускает общий gate и не доказывает native speed у нового
правила; следующий production шаг согласован отдельно.
