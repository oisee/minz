# Неизменённый tui_commander: пять кадров и полный main

`examples/nanz/tui_commander.nanz` целиком прошёл raw VM, final VM и assembled linked Z80 в Ordinary / Grace Size / Grace Speed. До исполнения отдельно заморожены исходник SHA256 `11a31a7e49238da84a61e03e0a7d883670f787413e228c19616f9521035649bb`, API MZV TUI и независимый порядок терминальных операций. Исходник и production code не менялись.

Последовательность клавиш Down(129), F5(144), F3(142), Up(128), Esc(27) показывает list/cursor0 → list/cursor1 → detail/cursor1 → list/cursor1 → list/cursor0. Независимый source model учитывает два отдельных if в main: вход в detail и обработка его клавиши возможны в одной итерации. Все три исходные функции main/render_file_list/render_file_detail действительно исполняются. Совпали 204 host events и все 2729 ANSI байт, включая пробелы исходных строк. Финальные cursor/current_screen/running/depth равны нулю; неиспользованный stack-global остался исходным.

Native image содержит только test-owned RET slots внешних TUI функций. Caller ASM, формальные ABI и исходные функции сохранены. Host читает actual allocated parameter homes, возвращает key в A и намеренно затирает A/F/BC/DE/HL/IX/IY кроме возвращаемого значения. Каждая строка ограничена своим NUL-terminated literal. Проверена целая RAM, включая неизменность кода/литералов и globals, с явными исключениями только для emitted spill cells и наблюдённого стека. Main возвращается из реального CALL к PC7003/SPf000; минимум SPeff8.

| Режим | Весь linked BIN | Native caller T-states |
|---|---:|---:|
| Ordinary | 1572 B | 16407 |
| Grace Size | 1574 B | 16353 |
| Grace Speed | 1574 B | 16353 |

Grace здесь быстрее на 54 T и больше на 2 B; это наблюдение, не заявление о выигрыше Size. Host rendering/input callbacks выполняются вне тактов Z80. Не заявляется public CP/M runtime, интерактивный терминал ОС или все возможные последовательности клавиш.

Первичная проверка PASS0.226s; новый сохранённый executable повторно проверил commander и selection demo (PASS0.25s/0.02s). Freeze `76f1c6f8`, test `5606721d`. Manifest содержит snapshots source/API, независимые события, raw/final MIR, allocation, ASM/BIN, VM/native events/stdout/registers и журналы. Matching executable сохранён отдельно с SHA в executable.json. Пакет: `/mnt/safe/minz/grace/2026-10-12/agent-artifacts/tui-commander-main/`. Доказательство tui_commander_l3 этим пакетом не переносится.
