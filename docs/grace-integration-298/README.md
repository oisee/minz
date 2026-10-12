# ALV и selection demo: интегрированные whole-main проверки

Root `34082988`: совместный тест **PASS 2,216 с**. Проверены исходные `screen_alv`, `tui_demo` и ранее интегрированный `sqlite_demo`: raw/final VM и собранный Z80 в обычном режиме, Grace Size и Grace Speed.

ALV проверен на F8 и TAB/F8: пять исходных строк, вся таблица 184 байта, поля, focus/title и точная последовательность вывода. Native исполняет настоящий ASM копирования, padding и rendering; его формат координат заморожен отдельно от VM. [Подробности](../../reports/alv-report-main/ALV.md).

`tui_demo` — простой selection input example, а не интерактивный terminal loop. Четыре piped-входа `7`, пустая строка, `256`, `65535` дают вывод `7`, `42`, `0`, `255`. Проверены регистрация COUNT/default42, host lifecycle, фактический extern ABI, clobber stress и полная память. Все native образы — 80 байт; исходный main заканчивается HALT и сохраняет driver CALL на стеке (SP EFFE). Это явный test-owned runtime.

SQL result filter fix интегрирован `ddcbbaec`, отдельный real SQLite тест PASS 0,005 с. Дополнительная проверка неизвестных имён колонок и порядка LIMIT остаётся в работе; полный screen_report ещё не включён в этот checkpoint.

Reviewed каталог сохраняет все 194 пути. Старый полный pipeline продолжает работать; текущий полный pipeline этим focused gate не заявляется. Arena size-gain regression расследуется отдельно.

Root каталог **PASS 3,053 с**: 146 из 194 точных путей имеют независимые полные или ограниченные VM/native проверки.
