# NC: дальнейшее сужение native расхождения

После flatRAM и tail-return коррекции filesystem globals верны: n_entries3, panel_h4, cursor/top0. На входе format_sfn directory entry уже нулевой; ошибка возникает раньше форматирования. Raw/finalVM entry сохраняется верно. Generated caller сохраняет spillbyte через HL до передачи HL→IY в get_dir_entry_full; этот участок требует дальнейшего независимого сужения, причина пока не доказана.

Диагностический read-only logging не меняет код/оригинальный source/expected output; после наблюдения удалён. Каталог остаётся150/194, NC не повышен. PFCCO repair rootPASS сохранён в306; currentfullMIR ещё выполняется.
