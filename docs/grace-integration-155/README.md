# Интеграция 155: callback ABI и эффекты константных вызовов

Root интегрировал `8c9fa926` (donor `149475c8`) и `4f58692b` (donor `4c2eeef3`). Известная цель callback сверяется с сигнатурой реально вызываемого pointer-параметра, включая forwarding wrappers; несовместимые ABI и fallible CY отклоняются до ASM. Константная свёртка допускает только скалярные вычисления без доступа к памяти и внешних эффектов, проверяя известные вызовы транзитивно. Чистая арифметика сохраняет свёртку.

Совместный root gate: MIR2 и pipeline **PASS**; [точные времена](root-gate.txt). Он включает исходный functional.lizp: 398 595 VM/native вызовов в трёх режимах, совместимый word callback, отказ несовместимому byte/word ABI, lifetime/store/load/transitive-effect регрессии и сохранение pure Double/chained folding. Это focused gate, не полный текущий pipeline.

На шаре SHA256 сверены 51 callback payload, 188 signed-comparison payload (включая source-тесты) и 8 purity payload. Манифесты отдельно сохранены. Полный исторический pipeline a7954042 завершился PASS за 3705,100 с; верхняя сводка исправлена. General struct-return buffers ещё в работе: вложенный native случай выявил повреждение стека, WIP сохранён в отдельной ветке, в root не интегрирован.
