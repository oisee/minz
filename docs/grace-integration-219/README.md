# C89 условные выражения и original CP/M main: root replay

C89 &&/||/?: теперь исполняют только выбранные побочные эффекты. Root повтор замороженных GCC/независимых моделей: PASS 23,630 с; 986 112 native вызовов и 1 972 224 raw/final VM проверки. Полный C89 PASS 6,323 с. Отдельный backend fix материализует предикат F при truncation; широкие Bool проверки root PASS 0,993 с. 27 expression, 4 backend и 4 lexical packet payloads сверены с SHA в Git и датированной шаре.

Root CP/M repeat PASS 2,630 с. Исходный CP/M main и все 256 значений print_u8 дополнительно проверены под явно тестовым ORG0100 загрузчиком, три режима и два BDOS2 host-контракта. Source/ASM не патчились. Рабочий RuntimeCPM всё ещё отвергает ASM и локальный stack allocation; обычный main завершает DI/HALT вместо OS RET. Этот пакет не повышает whole-runtime статус; новый production допуск разрабатывается отдельно.

Точные root логи, ASM/BIN, MIR и ledger сохранены. Общий pipeline29df08bb ещё live и предшествует последним frontend/backend исправлениям. Полная цель не закрыта.
