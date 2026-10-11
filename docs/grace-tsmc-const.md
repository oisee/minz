# Z80: word constant TSMC сохраняет живой A

Исследование guarded `region_eq` обнаружило wrong code: вынос чистой арифметики guard проходил raw VM, но собранный Z80 возвращал другое состояние interning arena (str_bump 82 вместо 91 на втором вызове). Ошибка была в backend, а не в исходнике tokenizer.

`OpConst` для 16-битного spilled значения сохранял HL, затем `emitTSMCSpill` использовал A для записи двух immediate bytes. Живая длина сравнения в A терялась. Даже если orphaned TSMC reload позднее переписывался в обычный memory spill, промежуточные LD A,L/H оставались и портили значение.

Исправление ограничено word-constant TSMC path: PUSH/POP AF вокруг patch stores, внутри существующего сохранения HL. A, флаги и стек возвращаются к прежним значениям; payload константы записывается правильно. Другие spill paths не получают повторного сохранения AF.

## Проверенное поведение

Шесть минимальных actual-machine cases покрывают константы 1/0xABCD/0xFFFF, обычные TSMC reload и orphan rewrite. До исправления все теряли A=103 (получали 0/171/255), после него проходят с правильными A, carry, HL, SP и word payload. Полный MIR2 пакет проходит; исходные failing runs сохраняются.

Guarded region prototype проверяет 6144 raw случая: все u8 длины, две памяти, address/offset/alias combinations, ordered successful reads, first trap и unchanged whole memory. Полный typed tokenizer с 30 persistent calls и независимым state/resource-traffic oracle проходит assembled Z80 после исправления backend.

## Отрицательное измерение оптимизации

| Эксперимент на полном tokenizer | T-states | Module bytes |
| --- | ---: | ---: |
| Исходный baseline | 2 270 286 | 4 346 |
| Первоначальный two-pointer prototype | 2 435 966 | 4 723 |
| ForRange alternative | 2 401 051 | 4 702 |
| Hoisted guards после исправления backend | 2 341 065 | 4 644 |

Ни один prototype не выигрывает у исходника. Они не зарегистрированы как production Grace rule/PGO API и не включаются в выбранный tokenizer. Exact PC audit первого варианта: guard стоит 297260T за 178 входов, body 93674T; весь region_eq 390934T против исходных 224008T. Стоимость setup/проверок границ и spill превосходит экономию адресной арифметики на коротких mismatch-heavy строках.

Эксперимент сохраняет исходный fallback и строгие offset/physical end guards, включая one-past. Более широкие structural postproof, lineage и exhaustive equal/mismatch-position coverage ещё не реализованы; проверенный опыт не является production proof. Выгодное решение обязано измеряться на полном caller после независимой legality проверки.

## Артефакты

[Пакет](grace-tsmc-const-artifacts/README.md) включает минимальный before/after, patch и machine tests, все prototype версии, источники/workload/raw/target, ASM/binary/costs, PC audit и историю failing arena state. HIR/MIR/Nanz проходят за 8,073/3,962/1,517 с, CLI за 21,424 с; showcase сохраняет 31 прежнее падение, новых нет. Собранный CLI выбирает прежние tokenizer binaries (speed 2259865T/4475B, size 2270286T/4346B), независимые host/resource-traffic и typed evidence SHA проверки проходят. Общий pipeline проходит за 242,286 с, только отдельно проверяемый budget fixture исключён. Actual two-pass budget comparison подтверждает точное равенство четырёх ASM и собранных бинарников этапу cfda6368: 344/344/232/344 bytes. Прежняя 722,080-секундная проверка исполнения сохранена для неизменных бинарников; новое исполнение budget workload не заявляется. Настоящее железо и original extern tokenizer ABI не проверены этим этапом.
