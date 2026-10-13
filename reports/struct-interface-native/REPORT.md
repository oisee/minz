# Original struct / UFCS / interface family

Пять неизменённых исходников проверены целиком: March10 `ex1_struct`, `ex2_ufcs`, `ex3_iface`, `ex3b_iface_param` и March11 `ex0_test_struct`. Четыре March13 duplicate имеют побайтно тот же SHA и тот же callable module. Source snapshots `cc9a82ee`, prospective domain `61432f43` зафиксированы до исполнения, base `d61a8664`. Ни один источник не содержит main/assertions — их не выдумываем.

Semantic authority: Book v7 Chapter4 struct fields, pointer-self mutation/UFCS, single-implementor static interface dispatch; §3.1 u8 width. Независимая host-модель не вычисляется из HIR/native.

| Module | Полное покрытие на режим | Calls/mode | B |
|---|---|---:|---:|
| ex1_struct | 256 значений каждой RGB lane, все getters; 216 mixed boundary triples | 3288 | 24 |
| ex2_ufcs | add 256×256 при6000/ffff; reset256×2; sum_two256×256 | 197120 | 35 |
| ex0_test_struct | тот же полный domain, отдельный исходный module | 197120 | 35 |
| ex3_iface | Dog/Cat empty objects6000/ffff; demo | 5 | 9 |
| ex3b_iface_param | Dog_speak/feed empty objects6000/ffff | 4 | 6 |

Ordinary, HIR+MIR Grace Size и HIR+MIR Grace Speed: **1192611 native CALL**, столько же исходных/final VM executions. Полные byte objects проверены после каждого вызова. Acc_add должен сохранять `(old+amount)&255`; reset —0; sum_two —reset/add(a)/add(b) с stores0,a,(a+b)&255; RGB stores идут r,g,b. Explicit pointer ffff проверяет 16-bit адрес на границе. Empty structs имеют нулевой layout, у них нет читаемых/изменяемых object bytes.

Проверяется каждое исходное callable, фактический параметр ABI и return location, правильный caller PC со SP=f000. Наблюдаемая stack глубина2B. Source method calls внутри sum_two и interface demos оптимизатор inline; их отсутствие не объявляется внутренним native CALL доказательством. Вызов каждого отдельного метода проверен своим реальным native CALL.

Memory oracle проверяет все source stores в исходном порядке и их значения. Каждое сохранившееся data read сверяется с независимо симулированным состоянием bytes на этом шаге; redundant reads обычной RAM могут быть forwarded, им не приписан volatile/MMIO контракт. Native byte-access decoder покрывает реальные LD(HL)/LD(BC)/LD(DE)/indexed и absolute byteLD; исходные VM callbacks фиксируют реальные MIR data accesses. Полный объект и соседние исходные guard bytes проверяются в каждом вызове; весь64KiB после suite должен остаться исходным вне явно изменяемых object bytes и наблюдаемого stack interval. Точный CALL return-word проверяется сразу. Spills у этих пяти модулей отсутствуют.

В ранней версии локального harness guard записывался за границей global и портил соседний код/driver. Исправлено сохранением и сравнением фактического соседнего байта, не изменением source/codegen/oracle. Raw VM guards располагаются в её heap отдельно от executable code. Компиляторной ошибки здесь не найдено.

Три режима дают идентичный BIN для каждого module: пакет подтверждает correctness family, не новый measured Grace/heldout выигрыш. Фиксированы реальные ASM/BIN/raw/final MIR/ABI/allocation/symbols/results и сжатый точный executable. Шара: `/mnt/safe/minz/grace/2026-10-11/struct-interface-native/REPORT.md`. Полный pipeline/CLI gate не перезапускался.

Финальный frozen executable: --- PASS: TestOriginalStructInterfaceNative (8.07s).
