# ZSQL ZX full main: ограниченный native допуск

Исходный main выполняет CREATE/INSERT/SELECT и выводит 247 точных байт в ZX screen RAM; full RAM, SQL port protocol и DI/HALT проверены в трёх режимах. Результаты одинаковы: 1127 байт / 312 516 тактов. [Полный отчёт](../../reports/zsql-zx-real-main/README.md).

Вход ограничен 39 символами: исходный 40-байтовый буфер меньше заявленного readline limit 60. Проверка не утверждает безопасность всех входов.

Все 194 пути остаются в каталоге. Независимое/ограниченное VM/Z80 покрытие — 153; отдельно native ASM — 17, native CP/M — 10, native SMC — 2. Inline ASM audit осталось 4, Agon — 5, pending oracle — 2, иной memory model — 1. Полный pipeline 94a41120 ещё выполняется и предшествует этой проверке.
