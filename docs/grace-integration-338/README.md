# Заявленный корпус: точные границы доказательств

Каталог сохраняет все 194 пути. Статусы различают независимые VM/Z80 проверки, ограниченный native ASM, CP/M runtime, Agon MOS runtime и незакрытые случаи; наличие BIN или screenshot не означает полной корректности.

Обновление Agon: native-VDU TUI Hello, Demo и Mini NC выполняются в ordinary mode с реальными eZ80 CPU/MOS и полным VDP эмулятора. Кадры, клавиатура и следующий MOS echo сохранены. У NC дополнительно наблюдены Down/Enter, граница index 7 без wrap, Up до index 6 и Escape. Дополнительно точные native VDU/text потоки сверены с независимой моделью во всех трёх режимах (9 запусков): Hello218B, Demo489B, NC3151B. Это оракулы команд и текста для фиксированных сценариев ordinary/Grace Size/Grace Speed, не независимый pixel/RAM/SP оракул, все режимы/входы или железо. Старый независимый ANSI replay относится к сохранённой pre-VDU библиотеке.

Все inline примитивы ZX Tetris проверены; неиспользуемый peek прошёл 3072 дополнительных native чтения. Оба Tetris допускаются как native ASM bounded; категория включает 21 путь. [Аудит](../../reports/zx-tetris-asm-audit/README.md).

Источники и SHA256 reviewed evidence проверяет `TestShowcaseExecutableOracleCatalog`; `reviewed-updates.json` задаёт точные workload/model/boundary. Новый pipeline продолжает работать отдельным immutable checkpoint и не подменяется каталогом.

Дополнительно все три графических примера прошли отдельную матрицу точного возврата в MOS (9 запусков): SP/IY/IX, 24 байта caller frame, status HL0 и PC после RET из сохранённого трёхбайтного адреса. Это фиксированные early-exit сценарии, не аудит всех записей RAM.

Оба консольных Agon Hello также прошли exact text/return в трёх режимах с полным VDP (6 запусков). Все пять Agon-путей имеют bounded full-device командный/текстовый replay; это не общая RAM/SP или аппаратная гарантия.
