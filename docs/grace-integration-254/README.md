# ZX reports и assembler quoted operands

Root checkpoint3cdd32e8: original SAP report и source-corrected hello_zx independent whole-main native proof — PASS 2,657 с. Exact fullscreen/attributes/keyboard/PCSP/fullRAM guard домены сохранены; ASM VM не используется как доказательство.

Assembler fix7c721b8c сохраняет operand tail после mnemonic/directive, поэтому DB строка не теряет пробелы/tab внутри кавычек. Source oracle до исправления обнаруживал сжатые поля hello; ожидаемый экран не нормализовался. Unit whitespace controls PASS, полный assembler suite повторяется отдельно. Hello source9e87c077 передаёт CString явно; старые source/BIN failures сохранены.

Исходный SAP проходит без правок. MARA color font содержит только32байта вместо заявленного набора glyphs; модель отказывает отсутствующим glyphs, это не compiler verdict. Native SQLite bridge и hardcoded parameter ABI ещё не доказаны. Эти ограничения не превращены в PASS по compile/assemble. [Полные отчёты](../../reports/2026-10-12-zx-apps-and-assembler-fix.md).
