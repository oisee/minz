# Интеграция обычного recursive selfLanz COM

d4dc7aa4/17a52354 интегрированы как ecda27e1/5f5c7c52. Независимые whole-main oracle, forced shared recursive spill, guard threshold/overflow/fullmemory и прежний tokenizer/CLI runtime прошли на объединённом дереве. Лог сохраняет отдельные package времена.

Public CLI: mz examples/nanz/self_lanz_parser.nanz -t cpm --runtime cpm --runtime-stack 1024 --asserts none -o parser.com. Исходник/ASM не переписываются. COM 4727 B, 983 310 T, 17 AST nodes, 135 stdout bytes. BDOS CALL5 hook намеренно портит регистры; globals/арены, все 64 KiB, code, guard и incoming SP проверены. Это не исполнение настоящей CP/M OS.

Статическая проверка guard ещё проверяется дополнительным негативным CFG-контролем: лексическое соседство не должно позволять перейти прямо на рост стека в обход guard. Подагент укрепляет admission; successful native proof не объявляется доказательством всех CFG.
