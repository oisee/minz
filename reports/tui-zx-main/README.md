# TUI ZX: исходный main и протокол ROM

Неизменённый `examples/nanz/tui_zx.nanz` прошёл собранный Z80 в Ordinary, Grace Size и Grace Speed: **PASS 0,055 с**. В каждом режиме BIN 603 байта и 9314 тактов; выигрыша Grace нет. Заранее заданные 194 байта включают текст и управляющие команды цвета/позиции.

Исполняются исходные ASM-тела `_p`, `tui_puts` и main. Только CALL 0x1601 и RST 0x10 имеют тестовую ROM-подмену, сохраняющую регистры. Проверены A=2 при инициализации, точный порядок всех output bytes, настоящие CALL/RST continuation words, полный RAM кроме точных spill и использованного стека. Main завершается своим DI/HALT с SP=EFFE и неизменным loader return word 7003; возврат main не заявляется. Время ROM callback не включено в такты.

Первая проба пропустила output: встроенный emulator RST-hook перехватывал инструкции до ручного ROM-перехвата. В корректном стенде `Hooks=nil`, RST выполняется процессором; первая ошибка стенда сохранена отдельно. Исходник и production код не менялись.

Это native проверка ROM output protocol, не VM/native differential, не модель экранных пикселей и не физический ZX ROM. Каталог пока не обновлён. [Лог](root.txt) · [Freeze](frozen/freeze.json) · [Первая проба](observer-before.txt).
