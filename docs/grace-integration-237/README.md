# Исходные ASM-примеры и Pascal CP/M: интеграция

На `74727dc6` root повтор шести неизменных ASM исходников: PASS 7,723 с; 17 202 372 actual native CALL в трёх режимах и отдельный повтор 88 исходных assertions. VM ASM no-op не объявлен эквивалентным. PC/SP, память, ABI и точные порты проверены. `copy_n` в исходнике описан неверно: реальный LDIR копирует HL→DE; count0 вне positive proof. [Полные домены](../../reports/2026-10-12-original-asm-contract-corpus.md).

Pascal production commits `905522a5` и `5fe54a90` (donors `70df304f`/`f0fe0c27`) дают узкий допуск точных generated WriteStr/WriteCrLf и ordinary CLI .pas. Root runtime regression PASS 1,131 с; CLI PASS 0,068 с. Отдельный whole-main Hello/Factorial loader и FACT oracle повтор — в `pascal-whole-root.txt`.

Неизменный Hello COM исполняется от ORG0100 до OS RET, три режима × два BDOS host контракта, точный stdout и bounded dollar-string reads. Factorial FACT0..12 raw/final VM/native подтверждён отдельно; основной production decimal runtime пока отвергается. Полные pipeline/CLI этими focused проверками не подменяются. [Пакет](../../reports/pascal-cpm-main/README.md).
