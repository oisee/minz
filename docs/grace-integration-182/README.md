# Интеграция 182: исходная Pascal-рекурсия

Root повторил неизменённый recursive_test.pas: все восемь callable functions по полному byte-домену, 14 assertions и исходный main проходят raw/final VM и Z80 в трёх режимах. Точный root лог рядом. 6 189 native calls; максимальный наблюдавшийся стек 1020 байт, возврат PC/SP и полный RAM/image защищены. Все 24 manifest payload проверены root, копия на canonical share сохранена.

Каталог сохраняет все 194 пути: 66 independent/bounded, 71 pending. Полный MIR2 e13e04a3 завершился PASS, полный CLI2612a266 тоже PASS; package gates имеют разные точные checkpoints. Старый fullpipelinecfaa7777 ещё выполняется. Новый Grace same-target ветвевой контрпример обнаружен отдельно и исправляется; цель не закрыта.
