# Интеграция 080: guarded CP/M и каталог 34

На 7fe6cf8d повторены MIR2/runtime/CLI проверки: PASS. Новый compiler собран из основной рабочей ветки. Обычный CLI создал parser/tokenizer COM, которые отдельный тест сверил побайтно с API и исполнил с независимым full-state оракулом.

SelfLanz: 415614 T, 3968 B, 135 stdout bytes; tokenizer: 408919 T, 7397 B, 146 stdout bytes. Проверены все области памяти, code/literals, stack guards/status и возврат SP. Это Z80 emulator с BDOS hook, не настоящая CP/M OS. Baseline и пределы stack reserve находятся в ../grace-cpm-reserve-coalescing/README.md. На маленьком стеке controlled overflow может наступать раньше.

Свежий каталог: 194 пути, 34 с ограниченным independent VM/Z80 покрытием, один отдельный native CP/M demo, 102 pending, 32 ASM, 19 external runtime, 5 Agon и 1 host memory. Покрытие конкретных entry/workload не доказывает всё поведение программы.

Hash16 generation выполнен с явным Z3: PASS, ноль пригодных entries, 981 → 981 B. Отрицательный результат не превращён в production шаблоны. Полный pipeline 22e426de по-прежнему выполняется; process 1754951 подтверждён живым при интеграции. Предыдущий полный core a3cc555f PASS предшествует изменению guards, а новый focused gate не заменяет full gate.

Новые пакеты и report 068 побайтно сохранены на dated share, их manifests проверены. compiler.json идентифицирует новый frozen executable; прежние executable inventories остаются историческими.
