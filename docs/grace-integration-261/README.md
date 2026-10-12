# Tetris: spawn, hold и составные последовательности

После интеграции общего byte-cycle emitter повторно проверены исходные CP/M/TUI модули. Frozen domains: 27 552 native вызова и 55 104 raw/final VM-вызова, включая 672 последовательности из восьми entry с сохраняющейся памятью. Source и независимый state-machine oracle не менялись. Точный terminal root log рядом.

Оракул сохраняет особенности исходного алгоритма: пустой hold вызывает spawn и сбрасывает hold_used, занятый hold не проверяет столкновение, game_over остаётся установленным. Тестовый контроллер явно задаёт спуск по независимо вычисленному ghost_y. Это вход нагрузки, а не эмуляция устройств.

Полные RAM, board read/write order, actual entry ABI, closure, code, return PC/SP проверены. Интерактивные main/rendering/keyboard/BDOS/TUI не заявлены. Категории корпуса остаются прежними; этот пакет расширяет доказательство computational entries. Артефакты и freeze сохранены в reports/tetris-sequences-native и на шаре.
