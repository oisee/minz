# ZX Tetris: 31 кадр исходного main и первый шаг gravity

Предыдущая проверка инициализации расширена до31 no-key кадра обоих неизменённых исходных main в трёх режимах. После каждого кадра независимо проверяются frame_count, drop_timer, cur_y, весь board и экран, ghost/current I-piece и previews. На кадре30 timer сбрасывается и cur_y становится1; кадр31 проверяет дальнейшее продолжение. Все155 keyboard reads на запуск имеют точные пять ZX row addresses в каждом кадре и возвращают no-key FF. Всего186 native игровых кадров и930 keyboard reads.

Вся RAM сверяется после каждого кадра, с исключениями лишь конкретных spill slots и наблюдаемого стека. Источник запускается настоящим CALL main; game HALT вызывается настоящим CALL zx_halt. Тест задаёт пробуждение переходом к следующей инструкции HALT: это контролируемая frame boundary, не interrupt-service/ROM/contention доказательство. Окончание — перед HALT кадра32, без возврата main.

Пока не проверены key transitions, rotations, hard/soft drop, hold, locking/line clearing и game-over. Обе inline ASM audit категории поэтому сохранены. Исходники frozen в [предыдущем пакете](../zx-tetris-initial-main/frozen/); старые результаты первого кадра не переписаны. Совместный прогон PASS7,387с. ASM/BIN, allocation/MIR, initialization RAM и RAM кадров1/30/31 сохранены на шаре в `compiler-reports/zx-tetris-gravity/executed/`.

[Root log](root.txt).
