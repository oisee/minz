# Интеграция134: unsigned widening и независимый аудит цели

Код9561d7f3 включает donor b212c6cc: unsigned u8/u16/u24 расширяется до 16/24/32 бит на Z80 с сохранением A/F и перекрывающихся исходных байтов. [Полный отчёт и замороженные source/ASM/BIN/MIR](../grace-wide-extension/REPORT.md). Независимые direct-native physical placements и ordinary/Grace Size/Grace Speed исходниковые проверки проходят на root. [Лог](unsigned-extension-root.txt). Native-модуль embed504B одинаков во всех трёх режимах; это исправление корректности, не заявленный выигрыш.

Полный original ZVM finish проверен raw/final VM по full-memory oracle. Его story[131072] действительно достижима; original wholemodule Z80 продолжает явно отказываться из-за 16-битного пространства. VM-проверка не переименовывается в native PASS. Signed extension valid homes проверяются отдельно.

[Независимый аудит всей цели](../../reports/2026-10-11-080-objective-completion-b1adbf09.md) интегрирован7b282474. Прикладная польза PFCCO/таблиц, несколько frozen heldout семейств и физическое удаление LIR имеют конкретные доказательства. Полная цель остаётся открытой: оставшееся независимое исполнение корпуса и актуальный общий gate. Следующий без внешнего runtime шаг: неизменённый widemath.nanz и все26 callable entries с независимыми оракулами. Fullpipeline a7954042 остаётся выполняющимся checkpoint, не объявляется текущим PASS.
