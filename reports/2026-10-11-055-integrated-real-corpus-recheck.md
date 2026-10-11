# Общий повторный прогон реальных программ

На 2e20d2e4 повторены четыре прежних heldout эксперимента, полные main SelfLanz и tokenizer, оба original filter-map entry, native accumulator/constant-multiply exhaustion, byte packing с вызовами и полный FAT12 word/wrap диапазон. MIR2 PASS 0,293 с, pipeline PASS 31,103 с. Это перечисленные реальные workloads и regressions, не весь pipeline. Карта всех 193 exact paths отдельно PASS2,107с; 24 имеют bounded независимое behavioral evidence.

Полный pipeline-cc502-full выполняется в прежнем подтверждённом процессе. Его checkpoint предшествует новым фиксам; terminal PASS не заявляется. Независимый PFCCO аудит нашёл production cost-model bias на multi-call с pinned ABI, прототип исправления и native comparison готовятся отдельно. Логи сохранены в docs/grace-integration-055 и на /mnt/safe/minz/grace/2026-10-11/compiler-docs/.
