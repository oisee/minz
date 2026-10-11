# Byte/word fusion: measured gap and semantic refusal

Проверен `64538c3c`, полный неизменённый `stdlib/fs/fat12.nanz`. MIR2 сохраняет два byte OpLoad/OpStore в `ld_word`/`st_word`; native packing `genDisjointByteOr` уже убирает часть OR, не меняя память. Общие OpLoad16/OpStore16 существуют. Word multiply10/12 уже перенесены в native; в этом шаге они не повторяются.

| Полный модуль / режим | ld_word T | st_word T | BIN B |
|---|---:|---:|---:|
| default | 78 | 220 | 9 686 |
| Grace Size | 78 | 220 | 9 864 |
| Grace Speed | 78 | 220 | 9 880 |
| direct16-study, test-only final MIR | 80 | 36 | 9 646 |

Последняя строка — **недопустимый counterfactual**, не production-правило. Тест заменяет final MIR двух helper bodies на ширину16, оставляя настоящую allocation/calling convention и остальной модуль. Чтение даже дороже на2T, поэтому простой перенос старого VIR fuseLoad16LE не оправдан. Запись показывает полезный резерв184T, но слияние не проходит нынешнюю семантику callback.

`VM.CheckDataAccess` получает оригинальный `*Inst`; по нему доступны opcode, Ty, исходная позиция. Два byte loads могут быть разрешены при TyU8, а fused TyU16 отклонён **до первого байта**. Отдельный отрицательный тест сохраняет этот контрпример на ffff→0: original читает34h/12h, fused не проходит. Сохранения порядка байтов/endian/wrap недостаточно, чтобы считать такую замену эквивалентной. Для store аналогично меняется identity/width и контекст отказа; interleaved stores/calls/volatile нельзя удалять по одному совпадению DAG. Новый semantic marker и расширение сериализации/profile context здесь не вводятся.

80 случаев: четыре режима × read/write × адреса4000h/ffffh × значения0,1,1234h,80ffh,ffffh. VM сверяет два отдельных обращения по независимой little-endian модели; assembled native возвращает правильное слово/записывает оба байта, profiler подтверждает по одному доступу каждого ожидаемого вида, полный64KiB snapshot допускает только два целевых byte store и фактически используемый stack. Return PC/SP сохраняются. Void helper проверяется по памяти, не по несуществующему return value. Только baseline/current-safe modes являются production evidence; study не подтверждает effects совместимость.

Следующий безопасный полезный шаг: оптимизация **чистого извлечения байтов** `trunc(and(val,255))`, `trunc(and(shr(val,8),255))`. Она может убрать AND/copy/shuffle, оставив исходные два OpStore, их identity, порядок и fault prefix. Это конкретная возможность из реального st_word, не перенос unsafe fusion. Все исходники/final MIR/allocation/ASM/BIN/результаты/log сохранены в `reports/byte-word-fusion` и на `/mnt/safe/minz/grace/2026-10-11/byte-word-fusion` с SHA256. BIN ORG8000h не является COM/реальным дисковым driver.
