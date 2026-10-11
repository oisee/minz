# Публичный tokenizer, PFCCO literal demand и полный CLI

Tokenizеr commits 0816642a/56851abc интегрированы как aa0e4ccf/60de5579. Оригинальный исходник компилируется ordinary CLI --runtime cpm --asserts none в 5815-байтовый COM 0100. Native independent full-main proof: 226 178 T, 146 байтов вывода, globals/арены, весь 64 KiB допустимых изменений, SP/guard/неизменный код. Фокусированный повтор интеграции: MIR2/runtime stack, pipeline 0,782 с и CLI 0,054 с PASS. BDOS CALL5-hook; реальная ОС и TPA остаются отдельными предпосылками. Рекурсивный selfLanz ещё впереди.

PFCCO 39f21352 интегрирован как 2ed9d4bb. Byte literal commutative RHS с мёртвым operand может использовать A непосредственно. Проверка liveness и отрицательные Sub/Cmp/u16/live/cycle контроли сохранены; greedy не менялся. Обычный source: 84 327 → 82 283 T, 60 → 58 B; bounded native exhaustive примеры 3+x direct 44 288 → 43 264 T, 22 → 21 B и conditional 49 585 → 46 026 T, 27 → 26 B. Root focused MIR2+pipeline 1,100 с PASS; frozen PFCCO heldout сохраняет 5568 T / 52 B. Не доказана общая оптимальность регистров/частот.

Полный CLI после cleanup на 89fff011: PASS за 383,376 с. Он предшествует tokenizer/PFCCO изменениям; PASS всего нового дерева не заявляется. Full pipeline 22e426de выполняется, а Sites 62 отражает snapshot 89fff011. Новые пакеты отдельно сохраняются на шаре.
