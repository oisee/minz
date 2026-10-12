# SAP printing и указатель локального массива

Root SAP whole main **PASS 0,577 с**, native string/newline регрессии **PASS 0,563 с**. Общий fix `2e055f0c` загружает actual print.str argument и сохраняет живые AF/HL; newline сохраняет живой accumulator. Исходный SAP MARA не менялся: реальная SQLite, все строки и точный601-byte output совпали raw/final VM/native в трёх режимах. [Пакет с original negative](../../reports/sap-mara-whole-main/README.md). Donor fullMIR PASS92,129с относится к его checkpoint; current root полный HIR/MIR ещё работает.

NC whole-main VM probe обнаружил независимый HIR miscompile: address локального массива tmp проходит через loop block param как u8. Raw и final VM теряли цифры format_size(5); source/expected files unchanged. Fix `0597fc9b` хранит локальный array binding как pointer, сохраняя byte element type. Minimal rawVM/Z80 regression8cases **PASS 0,064 с**, с array base выше255 и порядко-чувствительной суммой. NC raw/final VM в трёх режимах **PASS 2,643 с**: exact1453terminal events, disk reads0/2, filesystem/cursor state.

NC nativewholemain ещё не выполнен, поэтому его статус корпуса не повышен. HIR pointer fix подтверждён отдельной собранной Z80 регрессией, но это не proof всего браузера. Corpus adds only SAP exactworkload.

Root каталог **PASS 2,374 с**: 150 из 194 путей с независимыми полными или ограниченными VM/native проверками. Full HIR0597fc9b **PASS 25,602 с**; MIR того же gate ещё выполняется.
