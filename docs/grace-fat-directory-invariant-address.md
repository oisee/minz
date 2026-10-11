# FAT12: invariant address reassociation in production MIR Grace

Полный неизменённый `stdlib/fs/fat12.nanz`, `write_dir_entry`, cache-hit запись каталога: существующий MIR Grace не ускорял цикл копирования имени. После независимого исправления byte-load/живого A (`9e595c4b`, точные executables `6591ad22`) baseline стал корректным. Новый шаг выносит постоянную часть адреса, сохраняя каждое байтовое обращение на исходном месте.

| Режим | До, T | После, T | До, BIN B | После, BIN B |
|---|---:|---:|---:|---:|
| default | 13 828 | 13 828 | 9 691 | 9 691 |
| Grace Size | 13 828 | 13 828 | 9 869 | 9 869 |
| Grace Speed | 13 828 | 11 202 | 9 869 | 9 885 |

Все шесть слотов 0, 1, 5, 15, 16, 255 дают эту разницу. Выигрыш Speed: 2 626 T, 19,0%, цена 16 байт полного модуля. Размер включает spill storage и два ABI адаптера диска. Такты — от входа функции до RET с восстановленным SP, без bootstrap. Это одна процедура и одна cache-hit рабочая нагрузка; не реальный CP/M диск и не heldout PGO эксперимент.

Расширен существующий `HoistInvariantAddresses`, включённый под `UseGrace`, отключённый при `OptSize`. Выражение адреса `PtrAdd(base, Add(offset,index))` превращается в `PtrAdd(PtrAdd(base,offset),index)` только когда offset/base инвариантны, все участвующие операции имеют ширину 16, внутренний Add имеет ровно одно использование и определяется внутри естественного цикла. `ClsHard`, другие ширины и меняющийся offset не допускаются. Допускается `AddrOf` только существующего normal module-global: неизвестный символ может trap даже при zero-trip и потому не выносится. Несколько проходов перемещают зависимости в preheader в топологическом порядке, даже если AddrOf первоначально стоял после Add.

Single-latch/unique unconditional preheader и прежний запрет calls/conditional calls/indirect calls/ASM/ports сохранены. Перемещаются только адресные вычисления; loads/stores, их порядок, ширина, callback и отказ памяти не меняются. Арифметическая эквивалентность следует из target-width raw-pointer договора ADR-0045, modulo 65536; внутренний Add не имеет отдельного наблюдаемого потребителя.

Проверки: 13 MIR semantic случаев (обычный/глобальный адрес, ffff→0, zero-trip, alias-store, denied access, успешный low-byte/fault prefix в укороченной heap, extra-use, неизвестный global, меняющийся offset, hard/wide sum, mutating call); до/после совпадают значения, события доступа, отказ и полный snapshot памяти. Дополнительно шесть assembled native случаев wrap с копированием и порядко-чувствительной суммой `acc*3+x`, Speed/Size, n=0/1/3. Полный MIR2: PASS 8.406 s. Узкие pipeline FAT/byte-spill/существующий interning/wrap: PASS 9.473 s.

Все 18 FAT случаев проверяют raw/final VM и собранный Z80 по независимой модели 512-байтового окна, точным аргументам/числу disk_write, отсутствию disk_read, dirty=0, сохранности остальных globals и 64 KiB вне globals/spills/stack, PC/SP. Внешние disk_read/disk_write заменены только RET-адаптерами согласно исходному ABI; алгоритм FAT12 не переписан. Заморожены dirbase=7, cached-sector=7+slot/16, имя README  TXT, attr32, cluster1234h, size5678h и исходные байты window[i]=i*37+11.

`grace-fat-address-artifacts/manifest.json` перечисляет baseline и candidate исходник, ASM, BIN, MIR, allocation, все результаты и логи с SHA256. BIN — ORG 8000h измеряемый образ; не COM. Автоматический выбор между режимами существует отдельно; этот коммит расширяет production opt-in Grace и не объявляет общий оптимизатор завершённым.
