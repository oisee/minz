# Исходная механика Tetris: интеграция и root повтор

Freeze `5e2bc1c7`, proof `2777b63e`, уточнение истории `cd0f7f07` (donors049f7faa/20bd7da8/946d6113). Два unchanged original source: tetris_cpm и tetris_tui. Двенадцать mechanics entry, три режима; агентский terminal PASS 274,043 с: 1 430 736 native и 2 861 472 raw/final VM вызова. Отдельный production coalesce fix уже интегрирован35ee3d6f.

Root повтор начат на `cd0f7f07`: TestOriginalTetrisFamily, count1, timeout20m, session20540, журнал tetris-family-root.txt. Terminal ещё неизвестен, частичный результат не переименован в PASS. Exact board/global/input модели и memory/event/PC/SP guards сохранены.

Проверены исходные ограниченные mechanics, не whole composed game: spawn/hold/main/terminal/device и ZX/archived variants остаются отдельной задачей. Стоимость неверного before-кода не используется как benchmark выигрыша. [Scope/frozen домены](../../reports/tetris-family-native/REPORT.md).
