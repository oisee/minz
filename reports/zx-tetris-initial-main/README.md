# Два ZX Tetris: инициализация и первый кадр

Оба неизменённых исходных main исполняются до первого игрового HALT, затем тест задаёт одно пробуждение и наблюдает следующий HALT. Независимая модель проверяет очистку200 board bytes, весь экран, стены, preview pixels, два шага PRNG, initial I-piece queue; после пробуждения — пять точных keyboard port rows без нажатий, frame_count/drop_timer, ghost/current I и previews. Проверяется вся RAM с точными исключениями spill slots и фактически использованного стека; loader return word7003 остаётся целым.

HALT wake задаётся тестом через переход к следующей инструкции. Настоящие interrupts, ROM и contention не моделируются. Это bounded prefix, не возврат main и не полная игра. Вращения, hard/soft drop, hold, line clearing и game-over пока не покрыты этим тестом. Поэтому обе категории inline ASM audit сохранены, а helper/prefix evidence добавлен явно.

| Исходник | Режим | Полный BIN, байты | Инициализация + первый кадр, такты |
| --- | --- | ---: | ---: |
| examples/zx/tetris.nanz | Ordinary | 4548 | 2 936 452 |
| examples/zx/tetris.nanz | Grace Size/Speed | 4557 | 2 936 452 |
| ex34_tetris.nanz | Ordinary | 4979 | 1 987 741 |
| ex34_tetris.nanz | Grace Size/Speed | 4988 | 1 987 722 |

Такты не включают тестовое пробуждение или interrupt service; модуль содержит всю исходную игру. Результат не является prospective Grace selection или heldout gain. Совместный тест PASS7,679с.

Исходные `for i in lo..hi` имеют ascending half-open statement semantics, как показывают HIR ForRangeStmt и эквивалентные while в историческом исходнике; это отличается от countdown iterator `range(lo..hi)` из главы6 Language Book. Модель экрана использует исходные literal bounds и прямую формулу ZX адреса.

[Root log](root.txt) · [Текущий исходник](frozen/current.nanz) · [Исторический исходник](frozen/historical.nanz). ASM/BIN, allocation/MIR, RAM и регистры сохранены на шаре в `compiler-reports/zx-tetris-initial-main/executed/`.
