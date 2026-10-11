# Границы таблиц и хранение bool-полей

Интегрирован `a9f21893`: таблицы отвергают входы без ABI-привязки, обязательные
классы регистров и некорректные определения. Реальный контрпример раньше читал
A=99 вместо параметра C=7 и возвращал 100 вместо 8. Теперь такая запись
не допускается. Production context v3 делает прежние записи обычными misses.
Стоимость LD IX/IY,nn исправлена на 14 T / 4 B; BC/DE/HL остаются 10 T / 3 B.
Время внешнего solver ограничивается host deadline, включая зависший executable.
Root repeat: VIR 2.124 s, pipeline 3.052 s, PASS.

Один warm замер собранного CLI: baseline 0.00 s / 19328 KiB RSS,
consumption 0.01 s / 20224 KiB, generation 0.03 s / 34432 KiB.
Точность wall coarse; это один запуск, не benchmark распределения.

Root дополнительно нашёл связанный storage bug: struct из трёх bool и u8
имел Width 11 bits, при отдельно адресуемых полях. Генерация global data
падала с slice bounds из-за буфера в два байта. `1e3b1bdd` округляет storage
каждого поля и parser offsets к byte; scalar bool сохраняет ширину один bit.
Полная VM/native модель четырёх полей и guard проходит. Проверки layout
включают nested struct и union. Full packages: MIR2 7.126 s, Nanz 12.446 s,
HIR 24.084 s, PASS. Родственные native showcase workloads тоже прошли.

[Таблицы](../docs/grace-checked-table-boundaries/README.md),
[root repeat](../docs/grace-table-boundaries-integrated.txt),
[struct before](../docs/grace-bool-struct-before.txt),
[struct after](../docs/grace-bool-struct-after.txt),
[package gate](../docs/grace-bool-struct-full-packages.txt).
Артефакты скопированы на шару и проверены SHA256. Общая цель активна.
