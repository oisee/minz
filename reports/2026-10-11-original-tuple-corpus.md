# Исходные tuple/multi-return модули: VM и Z80

Основа `c9dedcf0`. Проверены неизменённые `examples/nanz/tuple_assert_gate.nanz`, March11 `ex11_minmax_multiret.nanz` и `ex13_multiret_assert.nanz`. March13 ex11/ex13 прочитаны отдельно: SHA256 совпадают с March11; поэтому это подтверждённые копии тех же исходников, а не перенос доказательства по похожему имени. Все пять точных SHA и домены сохранены до запуска в `frozen-domain.json`.

Оракул следует Book v7 §7.5 (каждый tuple result проверяется отдельно) и непосредственно исходникам: quotient/remainder, min/max, swap, выбор первого/второго результата. Исходные assertions сохраняются и выполняются при компиляции.

| Модуль | Проверенные entries | CALL на режим | BIN |
|---|---|---:|---:|
| tuple_assert_gate | divmod | 65 280 | 20 B |
| ex11 | minmax, swap, smaller, larger | 262 928 | 26 B |
| ex13 | minmax, swap, min_of, max_of | 262 928 | 26 B |

Все 591 136 вызовов на режим сверяются с независимым expected в raw MIR VM, final MIR VM и собранном Z80. Три режима ordinary, Grace Size и Grace Speed дают 1 773 408 native CALL. Divmod покрывает все a=0…255, b=1…255; b=0 исключён до измерения, поскольку исходный while не завершается. Word-функции покрывают все ordered пары 0…255 и отдельные 14×14 границы u16; полный декартов диапазон u16×u16 не заявляется.

Каждый native driver выполняет настоящий CALL; проверяются каждый return slot по Contract.Returns, возвращённый PC и точный SP. После полного домена сравниваются все 64 KiB RAM: разрешён только наблюдавшийся stack interval (2 B divmod/swap, 4 B остальных word entries). Генерируемых spill slots в этих модулях нет; код, literals и остальные байты RAM неизменны. BIN одинаков во всех трёх режимах: это доказательство корректности исходного корпуса, не выигрыш оптимизации.

Focused gate PASS; точный executable, source snapshots, raw/final MIR, ASM/BIN, ABI/allocations, CALL drivers и результаты сохранены с manifest SHA256. Общие CLI/pipeline gates не повторялись.
