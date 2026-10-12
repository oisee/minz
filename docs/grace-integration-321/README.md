# Полный plasma main закрывает ещё один inline ASM audit

Проверен весь неизменённый рендерер: 6912 упорядоченных записей, полный экран, RAM и возврат. Ordinary и оба режима Grace равны: 1555 байт / 7 562 297 тактов. Raw/final VM исполняют алгоритм с явным контрактом zx_poke; native исполняет настоящую ASM-инструкцию. [Полный отчёт](../../reports/plasma-whole-main/README.md).

Каталог сохраняет все 194 пути. Независимое/ограниченное VM/Z80 покрытие остаётся 153: plasma помещена в отдельную native ASM категорию (16 путей), чтобы не скрывать необходимость модели ASM на VM. Native CP/M — 10, SMC — 2; inline ASM audit — 5, Agon — 5, pending oracle — 2, иной memory model — 1.

Полный pipeline 94a41120 ещё работает; этот executable предшествует новым backend/assembler/validator изменениям и данному тесту.
