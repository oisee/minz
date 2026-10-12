# ZX console: явные source контракты и native повтор

Library fix `0ea5f3bc` (donor3ec12f47) закрепляет B/D/C для col/row/ch, которые ASM читает напрямую. Demo source fix `c2843398` переводит восемь строк на CString. Исходный до правок failure сохранён: counted strings и несовпадающие ABI registers давали повреждённый экран/незавершение. Default string semantics не менялись.

Root `94fba0e4`: TestOriginalConsoleNative PASS 0,504 с. Два неизменных ex18 whole-main output/port случая плюс исправленный ZX demo во всех трёх режимах. Для ZX — точная bitmap/attributes модель полного 0x4000..0x5aff, 17 keyboard reads, lowercase a, cursor11/0, PC/SP/fullRAM и guarded writes. Это один frozen keyboard/font сценарий, не весь keyboard/scroll API или настоящее железо. VM ASM no-op не используется как proof.

Ordinary 2180 байт / 404 452 T; Grace Size/Speed 2173 байт / 404 501 T. −7 байт, +49 T: ускорение не заявлено. Old/new source и библиотечные SHA различаются и сохранены; старым dependent каталоговым строкам требуется refresh identity.

[Before отчёт](../../reports/2026-10-12-original-console-before.md), [source fixes и ограничения](../../reports/2026-10-12-console-source-contract-fixes.md).
