# Исходный tui_demo: whole main и piped selection host

Неизменённый `examples/nanz/tui_demo.nanz` целиком прошёл raw/final VM и linked native в Ordinary / Grace Size / Grace Speed. Этот исходник регистрирует COUNT=42, вызывает selection host, читает индекс0 и печатает младший байт полученного u16. Он не содержит keyboard event loop или ANSI-rendering: выбран реальный MZV piped mode из `cmd/mzv/screen_host.go`, где одна строка ввода заменяет поле, а пустая сохраняет default.

До исполнения заморожены source/API SHA и четыре входа: `7\n`→`7`, `\n`→`42`, `256\n`→`0`, `65535\n`→`255`. Все 12 source/mode случаев совпали в raw VM, final VM и native. Проверены точный порядок registration/show/get, readonly NUL-terminated COUNT, default/parsed state, stdout и целая RAM. Native host читает actual allocated ABI: name HL/default DE, index A, returns A/u16HL. Unknown extern clobber stress меняет основные регистры/F между вызовами. Native print работает реальными OUT23; адрес порта включает ожидаемый A в high byte.

Связанный native содержит только test-owned RET slots для внешних функций; программа не переписана. Исходный main завершается штатным HALT со SP `effe` после CALL из driver `7000`; host call/return не оставляет дополнительных слов на стеке. Не заявляется OS-return или настоящий CP/M selection runtime. Образ 80 байт во всех режимах. Native caller T-states: 237 (`7` и `0`), 434 (`42`), 582 (`255`); host input callback вне тактов процессора.

Focused PASS 0.022 s, retained executable repeat PASS. Source не менялся, production changes отсутствуют, новых broad gates не запускали. Tui_commander — самостоятельная программа, этим тестом не покрыта; interactive selection renderer также не покрыт piped сценарием.

Freeze `834af0a4`, test `a53fdc0e`. Source/API snapshots, ASM/BIN, raw/final MIR, allocation, events, stdout, registers и summaries перечислены в `reports/tui-demo-main/manifest.json`. Exact executable сохранён с SHA в `executable.json`. Share `/mnt/safe/minz/grace/2026-10-12/agent-artifacts/tui-demo-main/`.
