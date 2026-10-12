# Пять frozen heldout семейств после TermCondRet/Pascal Hello

Checkpoint `13d9c298`: ParserProspectiveHeldout, HashmapMeasuredTrainingAndUnseenWorkload, PFCCOProspectiveHeldout и PascalBubbleSortProspectiveHeldout — terminal PASS 32,927 с, exit 0. Отдельный ArithmeticProspectiveHeldout — PASS 1,409 с, exit 0.

Использованы прежние exact source/workload freezes и независимые semantic оракулы; новых входов/ожиданий не подгоняли. Arithmetic сохраняет исходный отрицательный результат для преобразований: выбор original корректен, выигрыш Grace не выдумывается. Это пять проверенных экспериментов, не универсальная оптимальность или весь native corpus.

Проверки предшествуют signed decimal Pascal fix, который уже интегрирован позже и проверяется отдельно. Полный core76945771 PASS; retained pipeline29df08bb всё ещё live. Start record242 сохраняет исходные запуски.
