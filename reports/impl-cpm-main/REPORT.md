# Original 51_impl_cpm_demo: отказ runtime и независимый native main

База компилятора 354a47ef. Исходник целиком неизменён; SHA и точные ожидаемые строки заморожены коммитом c83f75ec до исполнения.

Production `RuntimeCPM` во всех трёх режимах честно отказал: `CP/M runtime rejects ASM and fixed-address extern functions`. Помимо inline BDOS2 тело main содержит локальные Alloca 2+1 и адресные stores. Это незакрытый public-runtime путь, а не PASS компиляции COM приложения.

Отдельный **тестовый** загрузчик ORG 0100 задаёт SP fdfe и JP main. Все исходные функции и BDOS ASM сохранены. Обычный backend завершает void main через DI/HALT: возврата в ОС здесь нет. Бинарник test-loader.com не выдается за production linked COM.

Все три режима × два BDOS2 контракта прошли main с точным `C:75\r\nR:50\r\n` и все 256 значений print_u8 (00..99, 100..255): 1 542 native исполнения. Второй BDOS host портит A/F/BC/DE/HL/IX/IY; отдельный preserving host не подменяет этот стресс. Перехват PC0005 выполняет тот же контракт возврата A/HL и pop адреса, что emulator.Run. Затраты синтетического BDOS не включены в T-states.

Во всех режимах тестовый COM 266 байт, main 3650 T; сумма 256 print_u8 161556 T. Режимы дают одинаковый код: выгода Grace здесь не заявляется. Минимальный SP fdef: 15 байт ниже fdfe. Каждый фактический PUSH/CALL проверяет записанное слово; исходное слово входного стека сохранено. Полные 64KiB неизменны вне реально занятого стека и именованных spill-данных; код и соседние защитные байты сохранены. Возвраты helpers проверены по PC0/SPfe00, main по HALT/SPfdfe. VM ASM/BDOS вывод не симулируется.

Проверка: `go test ./pkg/pipeline -run '^TestOriginalImplCPM(MainNativeLoader|RuntimeAdmission)$' -v -count=1` — PASS 2.014s. `MINZ_IMPL_CPM_MAIN_ARTIFACTS` включает выгрузку всех контекстов и ledger; ASM, COM, raw/final MIR, allocation/options/symbols, все выходы сохранены. Отдельный следующий шаг — узкий production допуск точного BDOS2 body и доказанных локальных адресов, с отрицательными admission контролями и настоящим OS RET. Физическая CP/M машина не проверялась.
