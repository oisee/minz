# Checked tables: ещё один существующий consumer и цена использования

Точная программа `examples/nanz/09_function_pointers.nanz`, без нового арифметического fixture, дала следующий полезный leaf: `triple(x)=x+x+x`. Таблица сохраняет address-taken ABI A→A, прямые caller moves и runtime `apply(f,x)` остаются production PBQP. Candidate меньше только после полного assembler size gate.

При независимой native проверке реального `apply(triple,x)` найден старый miscompile: Nanz `parseType` выбрасывал `fun(u8)->u8` signature, а `parseCall` принимал pointer type переменной f за return type вызова. Поэтому MIR был pointer-valued, и после корректного A-return исполнялся `LD A,L`. Baseline уже ошибался: x=0 давал 2. Исправление сохраняет callable signature в frontend metadata aliases/declared parameters/typed locals; pointer representation TyPtr не меняется. HIR получает Signature и настоящий return type; signature verifier теперь также проверяет arity. Нет hardcode A в backend: u16 callable по-прежнему возвращает HL. Unknown raw ptr historical fallback отдельно не расширен.

## Native результат после исправления сигнатуры

| Измерение | Baseline | Checked table |
|---|---:|---:|
| Полный исходный corpus module | 34 B | 31 B |
| Прямой triple, bootstrap+CALL | 66 T | 54 T |
| Реальный indirect apply(triple,x), bootstrap+CALL | 123 T | 111 T |

Прямой и косвенный пути проверены для всех 256 x по независимому `(3*x) mod 256` oracle с восстановлением SP=f000. Direct leaf дополнительно совпадает с MIR VM. Typed word callable checked на переходах 255/256, 32767/32768 и wrap 65535: результат x+37 в HL. Prefix copies, void, alias-chain, явные typed locals и неправильная arity проверяются frontend regression tests.

Actual CLI генерирует два valid entries (double/triple, файл 606 B). double уже оптимален и не заменяется. Exact ASM/BIN/table/source и before-failure сохранены. Поправка сигнатуры делает module 36→34 B независимо от table; выигрыш самой table — 34→31 B.

## Измеренная стоимость инструмента

Один warm filesystem host, новая compiler process на каждый запуск, 21 sample после одного warmup; perf_counter wall включает launcher GNU time, parse, compilation и запись результата. Не cold-cache и не общий benchmark. Median и диапазон wall:

| Режим | Median wall | min..max wall | Median max RSS |
|---|---:|---:|---:|
| Генерация offline | 43.764 ms | 42.459..49.742 ms | 33 920 KiB |
| Обычная сборка | 8.806 ms | 8.525..10.359 ms | 18 304 KiB |
| Table hit + admission + size gates | 11.898 ms | 11.380..15.338 ms | 20 864 KiB |
| Empty table miss + fallback | 9.856 ms | 9.248..10.309 ms | 20 096 KiB |

Таким образом на этой маленькой программе hit добавляет около 3.1 ms и 2.5 MiB к median compile, miss около 1.1 ms и 1.8 MiB. Consumption не запускает Z3; offline generation зависит от Z3. GNU time max RSS с subprocess/wait учитывает high-water child usage по Linux semantics; это не сумма одновременной памяти compiler+solver. Верхняя задержка generation ограничена существующим timeout per eligible leaf (30 s default), приведённые миллисекунды не обещают такую цену для больших программ.

Timings/source/ABI соответствуют финальному native варианту, таблица привязана к неизменённой функции и физическому параметру. Реальные ABI saves, explicit local save moves, return copies и scratch legality входят в admission/emission; преимущества не выводятся из cost metadata.

## Отрицательные результаты и следующая граница

`hash3` существующего StrRef corpus (`20_strref_test.nanz`) admitted solver, но полная сборка равна baseline: размер не меньше, production fallback сохранён. `hash16` HashMap corpus (`21_hashmap_test.nanz`) не имеет сертифицированного 16-bit XOR pattern, generation отказала: никаких entries, CLI не выдаёт фиктивную таблицу. Источники/диагностика сохранены; stdlib catalog нашёл в основном константные FAT12 stubs, а не новую полезную операцию.

Практический следующий безопасный gap — checked 16-bit XOR pattern с explicit scratch/flags cost и native word oracle для hash16, до расширения CFG. Общая CFG table admission сейчас требует отдельного per-edge ABI/live-value proof; этот этап его не расширяет. Unknown higher-order/nested callable contracts и imported alias preservation остаются границами frontend metadata, полного closure ABI здесь нет.

Проверки: full Nanz PASS 19.069 s; native/solver/word/regression focused PASS. Начальный full HIR встретил известный m18 multiply miscompile, воспроизведение сохранено root отдельно; final core rerun включает root fix ee5621ac (не часть этого изменения). Полный pipeline этим шагом не запускался. SHA256 packet: `reports/grace-table-practical-cost-artifacts/`.

Final core с ee5621ac: full HIR PASS 36.147 s, full MIR2 PASS 18.492 s. Production pkg/cmd compile-only PASS.
