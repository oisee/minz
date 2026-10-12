# Полный main ZSQL со встроенным шрифтом

Проверены SQL-диалог,257 байт вывода, screen/full RAM и исходный DI/HALT. Grace:2137→2018 байт,318613→318424 такта на одной frozen нагрузке. Недостающие glyph ASCII123..126 добавлены явно; прежний источник и model FAIL сохранены. [Полный отчёт](../../reports/zsql-zx-embedded-main/README.md).

Все194 пути сохранены. Independent/bounded VM/Z80 остаётся153; отдельная native ASM категория выросла до18. Native CP/M10,SMC2; inline ASM audit осталось3, Agon5, pending oracle2, иной memory model1. Шесть native SQL main запусков и372 helper-вызова проходят. Полный pipeline94a41120 ещё выполняется и предшествует изменениям.
