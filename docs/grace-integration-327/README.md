# ZX Tetris: подтверждён исходный initialization prefix и первый кадр

Оба полных модуля исполняют неизменённый main до двух игровых HALT: начальное состояние и один no-key frame проходят независимые screen/board/globals/full RAM проверки в трёх режимах. Пробуждение HALT задаёт тест, interrupt model не заявляется. [Отчёт](../../reports/zx-tetris-initial-main/README.md).

Оба пути остаются inline ASM audit, поскольку полный игровой цикл ещё не проверен. Категории всех194 путей неизменны; добавлено только точное bounded prefix evidence. Полный pipeline94a41120 продолжает работать.
