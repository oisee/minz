# Пять исходных typed showcase: VM и assembled Z80

Полностью проверены неизменённые March14 `ex21_enum`, `ex22_type_alias`, `ex26_enum_assert` и March15 `ex32_value_pipe`, `ex33_lambda_inference`. До измерений source/domain/model зафиксированы в `08d2595a`, base `11105fb3`. Все исходные функции, типы, enum значения и assertions сохранены. Ни у одного из пяти исходников нет main: не приписываем им startup/DI/HALT.

Независимая модель основана на semantic chapters Book v7: §16.1 auto-increment/explicit enum values, §16.5 transparent aliases, §2.16 left value становится первым аргументом pipe-вызова, §2.8 lambda, Chapter6 countdown range, §3.1 ширина u8/u16. Тестовые результаты не выводятся из сгенерированного IR. Исходные enum assertions дополнительно исполняют точные значения State/Color/Direction/Priority, включая HIGH=6/CRITICAL=7 после NORMAL=5.

| Исходник | Проверка | Native calls на режим | Module bytes |
|---|---|---:|---:|
| ex21_enum | identity все256; constants3/5 | 258 | 7 |
| ex22_type_alias | damage все65 536 пар; double_score все65 536 u16 | 131 072 | 3 |
| ex26_enum_assert | identity все256; add_prio все65 536 пар | 65 792 | 3 |
| ex32_value_pipe | double/inc все256; add все65 536 пар; piped11/piped_args16 | 66 050 | 12 |
| ex33_lambda_inference | add_acc все65 536 пар; sum_doubled30 | 65 537 | 20 |

Обычный, HIR+MIR Grace Size и HIR+MIR Grace Speed с ContractOpt=true: **986 127 реальных native CALL**, столько же исходной и final VM проверки каждого результата. Отдельно 12 original assertions проходят исходную VM и оба backend в каждом режиме (36 дополнительных native assertions). Double_score проверяет wrap на всём u16-домене; байтовые суммы — все overflow сочетания.

У ex33 поднятая `lambda_0(x)` проверена на всех256 значениях в raw VM в каждом режиме (768 проверок). Final optimizer её prunes после fusion: native loop всё ещё реально считает двойную сумму 5,4,3,2,1 →30. Мы не объявляем удалённый callback исполняемой native функцией. У pipe-примера вызовы внутри piped/piped_args constant-folded в 11/16; внешние CALL каждого исходного callable проверены, но внутренние CALL не сохранены.

Native harness использует реальные parameter homes allocation и Contract.Returns, восстанавливает исходный caller PC/SP (0xf000), а затем проверяет всё64KiB. На каждом шаге разрешён только конкретный audited instruction set этих модулей; единственная возможная запись памяти — CALL, у него сразу проверяются SP-2 и два точных байта адреса возврата. VM data observer подтверждает отсутствие data loads/stores. Native code/guard/остальная RAM неизменны; spills отсутствуют, максимальная наблюдаемая stack глубина2B. Это узкая проверка реально выпущенного кода, не общая ISA модель. Если будущий backend введёт другой opcode, тест требует повторного обзора effects.

Во всех трёх режимах каждый module BIN совпадает. Этот пакет закрывает исходный corpus behavior, не доказывает новый выигрыш Grace или heldout experiment. Production исправлений не понадобилось. Финальное исполнение точного сохранённого executable: PASS1,30s. Полный pipeline не запускался.

Все source snapshots, raw/final MIR, original/driver ASM/BIN, allocation/ABI/symbols, JSON counts/costs и сжатый frozen executable сохранены с SHA manifest. Шара: `/mnt/safe/minz/grace/2026-10-11/typed-showcase-native/REPORT.md`. Исторические showcase исходники не изменялись.
