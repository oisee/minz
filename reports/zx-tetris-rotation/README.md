# ZX Tetris: вращение I в исходном игровом цикле

Оба неизменённых main проходят31 кадр с Q на кадрах1/2,4,6,8,10. Удержание1/2 даёт только одно вращение; следующие presses проходят все четыре состояния и возвращаются к вертикальной I. Модель непосредственно задаёт геометрию I из обоих исходников: horizontal cells0..3, vertical rows0..3. После каждого кадра проверяются rotation, keyboard edges, ghost landing16/19, current piece, previews, board, timers и полная RAM. Gravity на кадре30 опускает вертикальную фигуру на одну строку.

Шесть native запусков,186 кадров и930 keyboard reads — PASS7,517с. Это проверка I в свободном центре поля: wall kicks, другие фигуры, lock/drop/hold/line clear/game-over пока не доказаны. HALT wake задаётся тестом, interrupt/ROM/contention и возврат main не заявляются. Категории inline ASM audit сохраняются.

ASM/BIN, allocation/MIR и RAM кадров1/30/31 сохранены на шаре в `compiler-reports/zx-tetris-rotation/executed/`. Исходные freezes остаются в zx-tetris-initial-main.
