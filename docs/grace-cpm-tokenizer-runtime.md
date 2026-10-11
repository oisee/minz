# Исходный tokenizer через production CP/M runtime

Обычная CLI теперь собирает неизменённый `examples/nanz/self_tokenizer.nanz` в COM с точкой входа 0100, выводом BDOS2 и приватным стеком 256 байт. Алгоритм и main не переписаны, ASM не исправляется текстовыми заменами.

```sh
mz examples/nanz/self_tokenizer.nanz -t cpm --runtime cpm --asserts none -o tokenizer.com
mz examples/nanz/self_tokenizer.nanz -t cpm --runtime cpm --asserts none -o tokenizer.a80
```

Явный runtime связывает только объявленные byte primitives: `peek(u16 в HL) → u8 в A`, `poke(u16 в HL, u8 в C) → void`. Несовместимый ABI и неизвестные extern отклоняются. Обычные функции разрешены. Spills уже использовали связанные символы внутри образа; прежний запрет LocMem снят, а генерируемые TSMC spills отключены в этом runtime, чтобы не изменять код.

Проверка итогового ASM обходит тела всех прямых CALL, включая rotate/math и console wrapper, учитывает return words, PUSH/POP, условные возвраты и баланс каждого кадра. Обмен EX (SP) разрешён только для собственного сохранённого слова над адресом возврата. Рекурсивные CALL, динамический SP, косвенные переходы и непривязанные RST отклоняются. Хвостовые переходы, включая restart цикла scan, не добавляют кадр и могут пройти проверку. Поддержка рекурсивного selfLanz остаётся следующим шагом; этот результат её не заменяет.

Фактический frozen CLI дал COM 5815 байт, SHA256 `ff30ec8eb3ab19f9a81044256e14e7ed48c706daa832bfc89de9714d100bfea4`. Повторная компиляция API совпала с сохранённым CLI COM побайтно. Native запуск: 226178 T-states, 146 байт stdout, наблюдаемый стек затронул 28 байт. Независимый host tokenizer проверил все глобальные переменные, все 4096 байт source, 4096 байт tokens и 8192 байт strings. Дополнительно проверены все 65536 байт памяти: изменения разрешены только в этих аренах, именованных globals/spills, saved-SP и приватном стеке; код и literals неизменны. BDOS hook намеренно портил все первичные регистры и flags. Возврат PC=0, исходный SP восстановлен, stack guard цел.

Byte primitive ABI проверен реальными Z80 инструкциями на адресах 0, 1, 7fff, 8000, fffe, ffff и значениях 0, 1, 127, 128, 255. Проверка сохраняет все нерезультатные первичные регистры/flags и баланс стека. Отрицательные проверки включают повреждённое тело rotate, условный несбалансированный RET, попытку снять return word, рекурсивный CALL и превышение стека вложенными вызовами. Полный MIR2: PASS 8.448s; focused CLI/stack и исходный tokenizer oracle проходят.

Это запуск машинного COM в Z80 эмуляторе с BDOS2 hook, а не запуск настоящей CP/M OS. Программа требует доступной RAM для её исходных арен 9000–cfff, TPA до d000 и доступного BDOS. Внутренние потребности BDOS в стеке не входят в доказанную границу application frames. Пустой runtime сохраняет прежний port23 путь и прежние профили; измеренные profile APIs явный application runtime не принимают.

[Артефакты](grace-cpm-tokenizer-runtime/artifacts/) содержат исходник, CLI ASM/COM, native state/oracle и логи. Frozen compiler: `mz-cpm-tokenizer-final`, SHA256 `b8152ed734eca9dce838f2ff174f4414e50327fae4b766484762e89f0336d6cc`. Проверка saved COM:

```sh
cd minzc
MINZ_CPM_TOKENIZER_COM=/absolute/path/tokenizer.com go test ./pkg/pipeline -run '^TestCPMRuntimeOriginalTokenizerMain$' -v
```
