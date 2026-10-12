# Оригинальные console sources: исходный результат

База 0cb00145. Два ex18 (11 и 13 марта) имеют одинаковый SHA256 8c5e935ba7284418db1200c822948c90e1c8d8b57b4fe283cbf3e77ea6f8e7f0. Их неизменённые whole-main прошли assembled Z80 в ordinary, Grace Size и Grace Speed: stdout `AB\nsame thing!\n`, stderr `ERR\n`, фактические порты OUT23/25, восстановленные PC/SP. Дополнительно каждый режим проверяет 256 значений каждого byte-output helper и 512 NUL-string случаев (длины 0..255, адреса 4000/fffe с wrap). VM ASM no-op здесь не доказательство.

Неизменённый examples/zx/console_demo.nanz **не проходит**: все три режима не возвращаются за 2 000 000 инструкций, экран расходится уже в 4000, cursor row24/col0, клавиатура ещё не читалась. Лог теста PASS означает, что исходный отрицательный результат сохранён; это не успешное выполнение ZX demo.

Два source/API дефекта: stdlib/zx/console.nanz:183 ожидает B=col,D=row,C=ch без pin annotations, текущая фактическая ABI A/C/B. Кроме того con_print:250 читает NUL-terminated pointer, а demo передаёт обычные counted литералы. Book docs/Nanz_Language_Book.md:215 различает plain expression string и c-string; глава9 задаёт explicit register contracts. Другие ASM entry consumers библиотеки (zx_poke/peek/in_fe/border, screen_addr, attr_addr) уже имеют подходящие explicit pins; con_clear не принимает аргументов.

Проспективная модель screen/keyboard записана freeze.json до native исполнения. Ошибка первоначального подсчёта row15 исправлена до исполнения на row11 по числу newline в исходнике. Исходные snapshots/SHA, ASM/BIN/MIR и результаты сохраняются отдельно; исправления source будут отдельными коммитами с новыми SHA. Это port/ZX proof, не CP/M runtime proof.
