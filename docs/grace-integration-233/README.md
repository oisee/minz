# CP/M: root-проверка после исправления типов локальных адресов

Checkpoint `9e56cef9`: production commits `246197e2` и `2b3c4e01`, перенесённые из `3c20e2d8` и `bab41a0e`. Конфликт разрешён сохранением обеих независимых возможностей: проверки типов/metadata адресов и numeric literal/implicit A BDOS adapter.

MIR2 targeted CP/M local/read-only/runtime — PASS 0,038 с. Pipeline original CP/M demo, README main/assertions, исходные self-Lanz/tokenizer runtime и live caller BDOS — PASS 8,081 с. Это проверки конкретных runtime контрактов, не весь pipeline.

Контрпример и точные before/after executable сохранены в [пакете](../../reports/2026-10-12-cpm-local-provenance.md). Narrowed pointer byte spill раньше принимался и писал 42 по адресу 0xcd67 вне локального объекта; теперь отвергается до генерации ASM. Обычный source/frontend miscompile этим контрпримером не заявляется. Положительные typed pointer/offset случаи сохраняются.
