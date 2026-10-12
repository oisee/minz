# ZX Tetris: keyboard edges, удержание и стены

Оба неизменённых main проходят31 кадр с заранее заданной клавиатурой: left held на кадрах1/2, release3, right held4/5, release6, left7; затем отдельные left presses до стены и right presses до другой стены. Независимая модель проверяет edge detection, отсутствие второго шага при удержании, остановку горизонтальной I у колонок0/6, ghost/current screen, таймер и gravity на кадре30. Вся RAM сравнивается после каждого кадра.

Повтор no-key сценария и movement в трёх режимах обеих программ:12 запусков,372 native кадра и1860 keyboard reads — PASS15,330с. Test HALT wake остаётся явным; interrupts/ROM/contention, rotation/drop/hold/locking/line clear/game-over не доказаны. Обе inline ASM audit категории сохранены.

Первое падение правой границы было ошибкой независимой модели: она допускала x7 для четырёхклеточной фигуры на поле0..9. Исправлено на x6; source/backend не менялись. Model-before сохранён как ошибка оракула, не compiler miscompile.

[Root log](root.txt). ASM/BIN, allocation/MIR и RAM кадров1/30/31 movement сохранены на шаре в `compiler-reports/zx-tetris-movement/executed/`. No-key повтор имеет root log; экспорт общий и итоговые payloads относятся к movement.
