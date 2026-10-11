# ContractOpt и native memory capacity

`5ab7e332` исправляет игнорирование ContractOpt=false в CompileHIRSteps. DefaultOptions сохраняетtrue. Оба публичных API проверены по реально выбранным классам и независимому VM/Z80 результату6. Старые замеры из прежнего контекста не переписаны. Повторные API, HashMap training/heldout и caret VM/Z80 проверки объединённой ветки PASS3.228s.

`9a1678a0` рано отвергает globaldata больше65536байт в nativeZ80Codegen. Три отрицательных случая: одна большая allocation, суммарные globals, большой initializer; обычный buffer допущен. ПолныйMIR2 PASS6.748s. Это необходимая data-onlyпроверка; code/alignment/runtimecapacity пока отдельны.

ZVM source запускается через mzv-H и содержит131072байта story сhostfile/UTF8/inputexterns. ДляflatZ80 он требует отдельного memory/runtime проекта. Реальныеu24/u32backendошибки не списаны наэто ограничение: матрица load/store и caller saves исправляется отдельно. Общая цель остаётся активной.
