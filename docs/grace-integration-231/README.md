# Исходные Hello-примеры через рабочий CP/M runtime

Checkpoint d823ddd2 допускает ту же точную BDOS2 инструкцию с literal numeric 5 spelling и неявным входом физического A, если однобайтовый parameter contract действительно размещён в A. Неизвестные символы/выражения/дополнительный ASM и неправильный input home отвергаются. Source примеров не изменён.

Root README CPM suite PASS 0,154 с: hello_cpm и hello_cpm_fib исполняют main в трёх режимах и под preserving/all-clobber BDOS2; точные строки, OS RET, PC/SP/guards/fullRAM. Оригинальные fib/gcd assertions отдельно проходят raw VM, final VM и Z80. В main вычисления могли constant-fold; standalone native assertions не заменены заранее вычисленным результатом. CPM MIR/admission suite PASS 0,148 с.

В root execution сохранены actual ASM/COM/MIR/runtime ledger. Полный CLI и старый pipeline ещё выполняются. Прикладного Grace speed gain эти одинаковые COM не заявляют; это расширение подтверждённого runtime корпуса.
