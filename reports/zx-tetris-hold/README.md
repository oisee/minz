# ZX Tetris: hold, обмен и блокировка

Оба неизменённых main проходят 31 кадр на Z80 в ordinary, Grace Size и Grace Speed. H нажимается на кадрах 1/2, 4, 6, 10, 12; SPACE — на кадре 8. Первое удержание не повторяет операцию. Первый захват I вызывает spawn и сбрасывает hold_used в 0. Обмен на кадре 4 устанавливает hold_used=1; кадр 6 блокируется. После hard drop на кадре 8 spawn снова разрешает hold: на кадре 10 текущая Z меняется на сохранённую I, Z попадает в hold. Кадр 12 блокируется, поэтому I остаётся текущей.

Это точное поведение обоих исходников. Правило «первый hold тоже запрещает повтор до фиксации» ими не реализовано: spawn обнуляет hold_used. Тест не меняет игру и не объявляет её соответствующей стандартным правилам Tetris.

Шесть запусков, 186 кадров и 930 keyboard reads — PASS (7,149 с). После каждого кадра независимая модель проверяет полную RAM, поле, счёт 38, RNG, текущую I, очередь J и hold Z, оба previews, ghost и таймер. Исполняется реальный ASM без Hooks. Ранее подтверждённые idle/movement/rotation/soft-drop/hard-drop сценарии перепроверяются отдельно.

ASM/BIN, MIR, allocation и RAM сохранены на шаре в compiler-reports/zx-tetris-hold/executed. Исходные freezes остаются в zx-tetris-initial-main. Проверка ограничена 31 кадром; line clear, gravity-lock, game over, wall kicks и произвольная игра впереди. HALT wake задаёт тест, реальные interrupts/ROM/contention/hardware не моделируются. Категория inline ASM audit сохранена.
