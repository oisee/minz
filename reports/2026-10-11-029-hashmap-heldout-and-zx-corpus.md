# Grace на HashMap и сокращение оставшихся сбоев корпуса

Неизменённый examples/nanz/21_hashmap_test.nanz: training15 вызовов и heldout48 вызовов сохранены до выбора кандидатов. Speed и Size выбирают оба ordered-byte helper среди четырёх вариантов. Training25378→23136T; тот же выбранный BIN на heldout255642→236938T (−7.32%), 981→962B. Независимые Go/Python оракулы сравнивают результаты, все82байта состояния и доступы памяти каждого вызова. На объединённой ветке повторный test PASS3.219s; независимый Python audit также PASS. Пакет docs/grace-hashmap-heldout содержит source, workload, profiles, allocations, каждый candidate BIN и выбранный ASM/BIN.

Три Lizp ZX примера в94518a86 проходят независимый экранный оракул6912байт на VM и Z80. Исправлены nested macros, typed palette initializer, wrappers через poke/peek и явный returnclamp. Полные Lizp/Lanz проходят. ULA/contension/настоящее железо не проверялись.

Архивный Tetris неправильно разбирал infix ^ как pointer dereference; parser теперь различает формы, source не изменён. Независимые VM/Z80 pointer/XOR cases и сборка Tetris проходят. Пропуск Tetris снят; ZVM пока остаётся известным nativeu32/runtime пробелом.

В98d0a993 исправлен подтверждённый табличный miscompile: скрытый A scratch для H→IXH возвращал5вместо37. Допуск проверяет реальные записи pattern и отвергает непроверенные repair moves; safe pattern реально исполняется. FullVIR остаётся красным из-за отдельного старого ожидания locSets8IX; baseline сохранён, исправление отдельно.

Большая цель активна: ZVM/u32, завершение безопасности/пользы таблиц и PFCCO, перенос effect-safe memory opportunities, расширение frozenheldout подтверждений. LIR уже удалён.
