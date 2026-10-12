# Публичный CP/M Tetris: root интеграция

Root source3a7bddf8 плюс переход текущего admission-test к accepted. Native original сценарий, invalid store и hostile canonical bodies/ABI **PASS6,409с**. Все CPM MIR проверки **PASS0,127с**. Старый wholemain тест после явного перехода к новому admission **PASS2,256с**. Historical отказ/manifest не переписаны.

Только canonical exact TUI ASM bodies с actual LocReg/name/ABI допускаются; reachable read_line, неизвестные ASM/extern и ambiguous/narrowed pointers отказывают. Динамические stores с доказанным единственным global root получают bounds guard всего effective address и ширины. Invalid actual board_set(255,255) возвращает status2 до записи и OS PC/SP; без guardedruntime старый out-of-object эффект сохранён как counterexample. Это не general memory sandbox/BDOS10/hardware proof.

[Полный донорский отчёт и CLI parity](../../reports/2026-10-12-cpm-tui-public-runtime.md). Root focused gate не заменяет полный pipeline; старый retained executable ещё жив.
