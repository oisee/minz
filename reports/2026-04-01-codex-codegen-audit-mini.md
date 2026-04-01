# Mini Report: Codegen State and Obvious Next Steps

Date: 2026-04-01
Author: Codex
Companion to: `reports/2026-04-01-codex-codegen-audit.md`

## Short Summary

1. Главный вектор по codegen сейчас уже не в сторону "сделать ещё один backend", а в сторону доведения качества существующего VIR-пути.
2. `CLAUDE.md` явно ставит в приоритет register allocation bugs, loop/codegen correctness, iterator-fusion gap и открытые MIR2 bugs.
3. VIR уже выглядит как основной quality-backend: table-first allocation, PFCCO, CFG-aware solving, `OpAsmBlock`, inline runtime.
4. MIR2 всё ещё важен, но в основном как legacy/fallback path и источник оставшихся correctness bugs.
5. В `minzc/pkg/vir/pipeline.go` уже есть реальный fast path: direct table lookup, затем cut-vertex decomposition, и только потом solver-heavy path.
6. В `minzc/pkg/vir/regalloc_table.go` уже заведены несколько lookup-режимов и автозагрузка 4v/5v/6v enriched/IX-expanded tables.
7. EXX-инфраструктура частично уже написана: есть `FindCutVertices()`, `IsBipartite()`, decomposition, diagnostic logging.
8. Но EXX пока в основном существует как architecture + diagnostics, а не как полноценный execution path в allocator pipeline.
9. Самый очевидный near-term gap: `intrinsics.go` концептуально рассчитан на целое семейство GPU-backed arithmetic idioms, но по факту сейчас там реально проведены только `tryConstDiv()` и `tryConstMod()`.
10. Это значит, что большой кусок ожидаемого quality gain уже почти лежит на поверхности: расширить intrinsic lowering на `mul8`, `mul16`, sat/abs/min/max, widening ops и `u32`.
11. MIR2 DAG inliner уже существует и выглядит как практический рычаг для loop-heavy workloads; его стоит не изобретать, а стабилизировать и измерить.
12. В `pipeline.go` сейчас много полезной arithmetic specialization делается постфактум на ASM-тексте; стратегически лучше переносить это раньше, в VIR lowering / intrinsic layer.
13. Самая большая структурная возможность сейчас: превратить EXX-zone model из "решённой идеи" в реальный allocation path через bipartite split + IX bridge channels.
14. Самый правильный режим работы с MIR2 сейчас: чинить только те куски, которые ещё реально бьют по correctness или по fallback safety, а не продолжать полировку его как главного quality path.
15. Если говорить совсем коротко: repo уже принял правильные архитектурные решения; теперь нужна не новая теория, а добивка интеграции в самых очевидных местах.

## Suggested Next Steps

1. Расширить `intrinsics.go` и bridge-level lowering на уже существующие GPU tables.
2. Добавить счётчики/отчётность по intrinsic hits и VIR→PBQP fallback причинам.
3. Прогнать и стабилизировать MIR2 DAG inliner на реальных stress cases.
4. Реализовать pragmatic EXX path: bipartite split, two-table solve, IX bridges, явная стоимость переходов.
5. Постепенно переносить arithmetic specialization из post-emit ASM rewrite в более ранние VIR passes.

## One-Line Take

Самые очевидные улучшения "смотрят в лицо" из трёх мест: недорасширенный intrinsic layer, недоактивированный EXX path и всё ещё слишком дорогая зависимость от fallback-путей.
