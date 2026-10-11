# Grace на второй реальной программе: HashMap, выбор на training, выигрыш на heldout

Неизменённый исходник `examples/nanz/21_hashmap_test.nanz`: 16-slot mutable map, u16 keys/values, открытая адресация и глобальные 32+32+16+2 = 82 байта. Все исходные функции и assertions сохранены. Это отдельная программа от Self-Lanz и циклов суммы.

До профилирования зафиксированы два набора:

- Training: 15 последовательных вызовов, пять записей/чтений, overwrite, отсутствующий key и map_has.
- Heldout: 48 вызовов, все 16 слотов, ключи i*256 с общей стартовой позицией, чтения в обратном порядке, отказ записи в полную таблицу, u16 границы, value=65535 (семантика sentinel map_has), overwrite нулём, clear и повторные операции.

Heldout не подаётся в selector. `hir.RunLoopProfile` собирает входной профиль training; `MeasureOrderedByteRWFunctionVariantSets` перечисляет четыре сочетания source-bound Grace преобразований `rd16`/`wr16`, проверяет все кандидаты и выбирает по фактическим Z80 T-states либо размеру BIN. После выбора `RunZ80ProfileWorkload` исполняет сохранённый Steps/ASM на heldout: новый профиль используется только для heldout oracle/freshness, выбора по нему нет. SHA выбранного BIN совпадает с training winner; all-original heldout BIN совпадает с training candidate 0.

| Цель | Выбор | Training T | BIN | Heldout T |
|---|---|---:|---:|---:|
| Speed | оба helper | 25 378 → 23 136 | 981 → 962 | 255 642 → 236 938 |
| Size | оба helper | 25 378 → 23 136 | 981 → 962 | 255 642 → 236 938 |

Training быстрее на 8,83%, heldout — на 7,32%; размер меньше на 19 байт. T-states считают вызываемую функцию от входа до RET, включая реальные вложенные CALL, но исключая bootstrap. Нет обещания выигрыша для всех workload и других программ.

Независимый Go HashMap host oracle проверяет результаты, все 82 байта глобального состояния и точные counts чтений/записей после каждого вызова: raw MIR VM, все training кандидаты и реальные baseline/selected Z80 runs. Второй Python oracle в `audit.py` независимо повторяет алгоритм и проверяет сохранённый пакет, включая SHA каждого кандидата и отсутствие замены winner на heldout. Native workload — 8 training candidates ×15 +4 heldout runs ×48 = 312 вызовов; saved raw runs также включают VM исполнения.

Тест `TestHashmapMeasuredTrainingAndUnseenWorkload`: PASS, 3.25 s из сохранённого test executable. Независимый Python audit: PASS. Сохранены исходник, два заранее выбранных workload, отдельные независимые oracles, профиль, четыре кандидата для обеих целей, выбор и его доказательства, выбранные ASM/BIN/MIR/allocation, baseline ASM/BIN, per-call snapshots/access/T и executable SHA. Сам ELF сохранён постоянно в `grace-checkpoints/hashmap-heldout.test`; публикационный пакет не дублирует его большой объём.

Это API интеграция существующей Grace, без изменения production правил или выбора оптимизированного исходника вручную. Read/write helper pass использует профиль как freshness binding, а measured objective — для выбора; счётчики циклов не превращены в статистическую модель helper. Старые assertions в исходнике объявлены via mir2; цель теста шире этих assertions: вызовы map API на обоих workload действительно исполнены на Z80 с независимым oracle. CP/M, реальное железо, полный showcase и общий долгий pipeline этим прогоном не проверялись.
