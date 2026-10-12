# Дополнительные границы прежних native composition примеров

На базе `6729c9e1` усилены **неизменённые** `nativeCompositionFixtures` a1/a2/a4/a8. Исходники сопоставлены побайтно; независимые модели записаны до запуска в [freeze.json](native-composition-boundaries/freeze.json), SHA каждого source сохранён там. Production-код не изменялся. Это усиление старого тестового домена, не перенос потерянного LIR feature и не новая прикладная программа.

Для a1/a4/a8: все 256×256 a/b при c=0,1,127,128,254,255. Для a2: все a/b при каждой из 36 пар c/d из того же набора. Независимые модели — `(b-c)&255`, `(a-b+c-d)&255`, `(c-(a>>1)+(b<<1))&255`, `(b-a+(c<<1))&255`; промежуточное переполнение u8 соответствует modulo256. Полного 256³/256⁴ Cartesian не заявляем. Прежний suite с c/d=3/4 остаётся активным отдельно.

| Entry | Native CALL в одном режиме | Полный BIN | Workload T, включая CALL | Входы → результат |
|---|---:|---:|---:|---|
| a1 | 393 216 | 10 B | 24 772 608 | A,C,B → A |
| a2 | 2 359 296 | 8 B | 129 761 280 | A,C,B,D → A |
| a4 | 393 216 | 11 B | 26 345 472 | C,D,A → A |
| a8 | 393 216 | 6 B | 18 481 152 | C,A,D → A |

Ordinary, Grace Size и Grace Speed дали одинаковые BIN/такты/ABI. Всего **10 616 832 actual CALL**; столько же независимых raw VM и final VM сравнений. Retained executable suite PASS **15.18s** на этом host; это время проверки, не универсальный compile benchmark. Во всех случаях возвращение проверено одновременно по PC=0x7003 и SP=0xf000; глубина stack2B. Costs включают инструкции сохранения/перемещения operands (например a2 LD E,A, LD B,A и LD A,E), а не только арифметику. Оптимальность последовательности не утверждается.

На каждом c/d slice проверены все64KiB памяти и code image кроме наблюдённых stack bytes и exact named spill bytes (здесь spills нет); profiler запрещает любые другие записи. Driver CALL bytes постоянны. IX/IY проверены на сохранение, если actual code их не использует; комментарии ASM исключены из этого определения. Main A/F/BC/DE/HL финальные состояния хешированы в фиксированном input order; A сверяется с независимым oracle. Caller-clobbered scratch bytes и flags не объявляются неизменными. Этот byte suite отвергает code с EXX/AF', поэтому не заявляет проверку shadow-bank arithmetic или interrupt semantics.

[Exact ASM/BIN/raw+final MIR/options/results](native-composition-boundaries/summary.json), log, source и compressed retained executable находятся в packet. SHA uncompressed executable — `executable-identity.json`. Для воспроизведения распаковать pipeline.test.xz (`xz -dk`), затем `COMPOSITION_BOUNDARY_EXPORT=/new/path ./pipeline.test -test.run '^TestNativeCompositionBoundaryLattice$' -test.count=1 -test.v`. Все manifest payloads должны совпасть с SHA; evidence IDs не подменены.

Новых production ошибок в этом домене не обнаружено. Проверка не закрывает arbitrary shape-table composition, все call/CFG/ISA constraints или всю Grace-цель.
