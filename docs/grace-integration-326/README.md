# Статический цветной MARA report проверен на native Z80

Программа не исполняет SQL: рисует заранее заданные команды и строки. Исправлен неполный шрифт; полный main проходит492 байта текста,20 attribute calls, screen/full RAM и DI/HALT в трёх режимах. Результат1699 байт/478352 такта, Grace gain отсутствует. [Отчёт](../../reports/zsql-mara-color-main/README.md).

Все194 пути сохранены. Independent/bounded VM/Z80 остаётся153; отдельно native ASM19, native CP/M10, SMC2. Inline ASM audit осталось2 (два ZX Tetris), Agon5, pending oracle2, иной memory model1. Полный pipeline94a41120 ещё работает и предшествует изменениям.
