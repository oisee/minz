# Целый TUI Tetris: raw/final VM и связанный native

После интеграции полный original corrected main прошёл все три режима: 1955 ordered API events,8669 UTF8 bytes,state/fullRAM/code/PCSP. Root joint TUI main+SQLite entry gate PASS1,847с. Source не менялся. Независимый terminal/render/state model заморожен до исполнения.

Native linker вставляет по одному RET в уникальные declared extern labels, сохраняет unlinkedASM отдельно и связывает только эти entry с actual formal ABI. Все игровые функции исполняются; callback обслуживает terminal API и реальный stack return. Public unlinked ASM/BIN не объявлен исполняемым. Native/readOnly pooledCString bounds и полные state/effect guards проверены.

Каталог сохраняет194пути. TUI current source теперь имеет bounded independent VM/native evidence вместо external-runtime ожидания. Это фиксированный конечный сценарий, не все интерактивные нагрузки/устройства. Дальше публичный CP/M admission и оставшиеся runtime программы.
