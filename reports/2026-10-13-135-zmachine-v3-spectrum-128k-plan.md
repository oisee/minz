# Следующая цель: Z-machine V3 на Spectrum 128K

13 октября 2026 года. Цель задана Alice после завершения moving-sprite библиотеки:
Z-machine V3 на Spectrum 128K с историей в банковой памяти; сначала Zork I,
другие V3-истории проверяются отдельно.

В существующем backlog это J4/J5: [история и host VM evidence](../docs/project/BACKLOG.md).
Текущий host-путь уже исполнял MIT Zork I V3 (86 928 B), но host storage не
доказывает работу в 16-битной Spectrum-памяти. В Spectrum 128K банки доступны
через окно, а не единый 128-КБ указатель.

Порядок реализации:

1. Зафиксировать текущий interpreter/story/reference и проверить, какие global
   buffers и runtime зависимости мешают native target. Сохранить hashes и
   доступную лицензию; не объявлять поддержку произвольного copyrighted story.
2. Спроектировать resident layout и бюджет: interpreter, dynamic story RAM,
   стек Z-machine, native stack, текстовый экран/ввод, paging state. Учесть
   физические восемь банков по 16K, фиксированные окна и ROM IRQ.
3. Сделать библиотеку banked memory/story reader. VM PC/address могут быть
   шире 16 бит; доступ к физическому Z80-окну остаётся 16-битным. Проверить
   границы страниц, word reads, bank restoration и память вне окна.
4. Собрать настоящий Z80 interpreter и 128K loader/snapshot. Запустить в полной
   128K Spectrum-модели. Экран и клавиатура должны принадлежать этой платформе,
   а не host-only services.
5. Независимо проверить маленькие V3 fixtures, затем Zork I: startup, LOOK,
   OPEN MAILBOX, TAKE LEAFLET, READ LEAFLET, INVENTORY и выход. Сравнить игровой
   текст и описанные изменения состояния с отдельным эталоном.
6. Показать runnable demo, публиковать исходники/сборку/команды и bounded proof;
   main integration и сохранение на `/mnt/safe/minz/grace/`. Save/restore и
   следующие V3-игры описывать по реально реализованному контракту.

Первый шаг — аудит памяти и native build, а не механическое расширение указателей.
Архитектура banked library должна позволять последующее расширение на Scorpion/
Profi, но их разные карты и порты требуют отдельных backends/проверок.
