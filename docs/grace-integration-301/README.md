# Level 3: оригинальная метафункция до Z80

Root `1ed2d763`: L3, Commander и selection demo совместно **PASS 0,330 с**. Неизменённая пользовательская `@screen` метафункция и исходный декларативный блок действительно участвуют в компиляции. Сгенерированный renderer и исходный main исполняются raw/final VM и native во всех трёх режимах. Независимая модель по исходной метафункции даёт точные 36 событий и 399 ANSI bytes; Esc завершает один кадр.

Все native варианты — 467 байт/992 T; test-owned TUI hosts исполняются вне измеряемых Z80 тактов. Проверены фактический extern ABI, clobber stress, RAM/image и возврат PC7003/SPf000. Исходные default строки преобразуются compile-time в CString и корректные emitted c-literals, source fix не нужен. Это proof одного блока, не всех возможных метафункций или публичного CP/M. [Подробный отчёт](../../reports/2026-10-12-tui-commander-l3-main-native.md).

Shared execution harness изменился; Commander replay прошёл заново, reviewed hash активного test обновлён явно. Исторические payload/manifest не переписаны. Полный pipeline и исходный screen_report остаются отдельными незавершёнными проверками.

Root каталог **PASS 3,168 с**: 148 из 194 путей с независимыми полными или ограниченными VM/native проверками.
