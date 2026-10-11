# Pure byte-lane identity: useful FAT12 improvement

Production `SimplifyIdentities` теперь упрощает `trunc8(and16(x,255))` до `trunc8(x)`. Полный неизменённый `stdlib/fs/fat12.nanz` получает пользу без слияния памяти: `st_word` **220 → 93 T**, −57,7%, весь модуль **−23 B**. `ld_word` остаётся78T. Обычный режим, Grace Size и Grace Speed получают это чистое тождество; дополнительный флаг не нужен.

| Режим | До T st_word | После T | До BIN B | После BIN B |
|---|---:|---:|---:|---:|
| default | 220 | 93 | 9 686 | 9 663 |
| Grace Size | 220 | 93 | 9 864 | 9 841 |
| Grace Speed | 220 | 93 | 9 880 | 9 857 |

Только текущие production строки являются положительным результатом. Сохранённый direct16-study из предыдущего аудита остаётся несовместимым по callback; его36T не объявляются достигнутыми production. Оставшийся pure high-byte extraction/address setup можно исследовать отдельно.

Проверяется same-block dominating OpAnd, constant255 с шириной16, source conversion16→destination8, оба порядка operand. Hard class ограничения conservatively reject на AND/Trunc. Остальные ширины/masks/opcodes/недоминирующая constant не допускаются. Меняется только Src0 существующего Trunc; Dst, SrcTy, destination type/class, исходная позиция и остальная metadata сохраняются. AND с дополнительными потребителями остаётся; обычный DSE удаляет только действительно мёртвые вычисления. Получаемое low8bit равно исходному при signed/unsigned x, включая highbit.

Оригинальные OpStore остаются теми же объектами, с теми же Src/type/order/source metadata. Не перемещаются load/store/call, не вводится широкое memory access; volatile/MMIO/alias семантика обращения не меняется. Независимые callback tests проверяют разрешение именно двух original byte store и успешную запись34h перед запретом high-byte12h; internal VM partial-heap case сохраняет первый store до bounds отказа второго. ADR-0045 wrap сохранён.

Проверки:

- **262 144** VM случая: все65 536 значения × u16/i16 × mask operand order. Low-byte и отдельно используемое full-width AND наблюдаются по oracle input&255. Negative width/mask/ClsHard/order cases проходят.
- 80 assembled native/finalVM случаев прежнего FAT audit, включая по10 случаев каждого helper/mode (адрес4000h иffffh, значения0/1/1234h/80ffh/ffffh). Actual read/write byte counts, independent little-endian values/full64KiB/SP проверены. Counterfactual строка сохранена отдельно от production.
- Полная FAT12 `write_dir_entry`: все18 raw/finalVM/native случаев с exactdisk args,512window/allothermemory/SP — PASS8.892s.
- Полный MIR2 — PASS8.978s. Focused pipeline byte-store identity и byte-load spill — PASS0.163s; один optional full30 global-word test явно SKIP, не считается выполненным.

Такты от входа helper до RET, bootstrap исключён; BIN всего модуля включая storage. Это конкретная FAT12/helper нагрузка, не full disk/CPMOS/heldout PGO результат. `reports/byte-lane-extraction` содержит baseline/candidate exactsource/MIR/ASM/BIN/allocation/results/log; полный SHA manifest. `/mnt/safe/minz/grace/2026-10-11/byte-lane-extraction` хранит копию. Исторический refusal не переписан.
