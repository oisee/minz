# Tetris CP/M: исправленный целый main на Z80

Исправлены только CString литералы, передаваемые NUL API: три CP/M и семь TUI. Старые source snapshots, negative payload и manifests сохранены. Независимые historical модели сверяются по Go AST; старые source proofs не переносятся на исправленные исходники автоматически.

Root corrected CP/M main и old-source refusal/model controls прошли, точный лог рядом. Замороженный сценарий z/q/a даёт 326 байт ANSI, правильные globals/RAM и PC/SP во всех трёх режимах. Реальный imported ASM выполняется с test-owned BDOS1/2; public RuntimeCPM пока отказывает. Каталог сохраняет 194 пути, один CP/M путь получает bounded native ASM статус. TUI current whole-main остаётся explicit external-runtime gap.

Source identity/evidence hashes обновлены только после проверки diff, frozen scenario и root replay. Исторические sequence replays идут отдельно на сохранённых old-source bytes.
