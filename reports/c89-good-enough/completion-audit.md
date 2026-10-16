# C89: практический допуск и доказательства

16 октября 2026 года. Механизмы PR [163](https://github.com/oisee/minz/pull/163)–
[170](https://github.com/oisee/minz/pull/170) интегрированы в main d43e8559.
Этот checkpoint добавляет полную небольшую width matrix, forwarding и volatile
проверки, исправление MUL с удалённой загрузкой константы и target-size/VLA
diagnostics. Финальное слияние checkpoint требует строгого CI на его head.

C89 теперь полезен для выбранного embedded/system подмножества: integer,
control, pointers, arrays, aggregate storage и реальные вызовы. Это не ISO
сертификат. Target ABI: signed char8, short/int/pointer/size_t16, long32,
little-endian, alignment/field alignment 1. Floating point, long long,
bitfield storage, runtime varargs и VLA не поддержаны и отклоняются в
проверенных сценариях. Hosted libc и весь preprocessing не обещаются.

| Условие objective.md | Проверяемое доказательство |
|---|---|
| Матрица и ограничения | [feature-matrix.md](feature-matrix.md): конкретные test families; C99/C11/import extensions отделены от C89 |
| System C семантика | Integer/pointer/lvalue/scalar/storage/enum/control/qualifier tests; [target limits](target-limits.md); K&R/sizeof/preprocessing admission |
| Независимые ожидания и настоящий Z80 | Checked-in C, Go models, GCC для переносимых численных результатов; raw/final MIR VM и assembled Z80; memory/events/PC/SP в control/storage suites |
| Реальный scalar ABI и цена | [width boundary](aggregate-width-boundary.md): 13 форм, 624 native / 780 VM; actual typed argument/result slots и no alloca. [Call cost](call-abi-and-cost.md): primary и heldout, весь caller/callee transfer |
| Copy/lifetime | Parameter mutation/direct/indirect/recursive suites; caller-owned return buffer; forwarding 81 native / 108 VM; [volatile source traffic](volatile-return-boundary.md), 6 native / 6 VM |
| Исправления и fast/deep разделение | Checked-in минимальные C inputs; forced-allocation native cases; frozen GCC expression oracle. Fast pipeline shards идут параллельно; deep expression campaign отдельно |
| Реальная программа, linking и main | FAT12 image reader и mixed C/Nanz/C imports; PR163–170 прошли required CI; clean-main d43 core и команды FAT12 подтверждены ниже; финальный checkpoint проходит собственный strict gate |

Приём/возврат eligible shallow structures до четырёх байт действительно
превращается в несколько MIR arguments/results. Все 11 порядков полей шириной
1/2 байта и две signed формы проверены в default/Grace/Size/no-contract.
Address escape, nested/overlapping storage и неподходящие call expressions
сохраняют memory ABI. Volatile source должен сохранять обращения; обычный
snapshot результата может вернуться как tuple. Forwarding сейчас проверяется
на буферном пути и не объявляется полностью scalarized.

Primary fixture: 193 → 90 bytes, 56 301 → 23 569 T-states на 49 входах.
Heldout: 255 → 141 bytes, 100 012 → 46 138 T на 73 входах. Повторный замер
этого checkpoint сохранил все цифры в default/Grace/Size. Это два конкретных
fixtures, не обещание ускорения каждого struct call. GCC подтвердил 3328
численных результатов 13 width forms; host layout не служит target-layout
оракулом.

## Итоговые проверки

На чистом main d43e8559: C89 38.784 s, HIR 32.478 s, MIR2 109.382 s,
Nanz 28.250 s, interop 0.007 s — PASS. Обе реальные команды FAT12 (default/Size)
прочитали 1537 bytes за 10 sector reads, byte-identical reference; BIN 7616/7631.
Это emulator/image-backed controller, не физический дисковод и не полная libc.

На checkpoint после MUL и target-limit fixes: parser 0.004 s, C89 62.816 s,
HIR 29.536 s, MIR2 107.443 s, Nanz 26.107 s, interop 0.002 s — PASS.
После MUL fix control/expression/FAT12 pipeline — PASS 34.051 s: expression
oracle проверяет 986112 native и 1972224 raw/final VM случаев с памятью и
порядком событий. Target-limit changes отдельно прошли native sizeof и полный
C89 корпус, включающий FAT12; они не меняют admission проверенной expression
программы. Старые и новые runs не складываются в процент уникального покрытия.

```sh
cd minzc
go test ./pkg/cparse ./pkg/c89 ./pkg/hir ./pkg/mir2 ./pkg/nanz ./pkg/interop -count=1
go test ./pkg/c89 -run '^(TestC89AggregateScalarWidthVMZ80|TestC89AggregateForwardingVMZ80|TestC89AggregateTransferCostVMZ80|TestC89TargetObjectLimits|TestC89SizeofTargetBoundaryVMZ80)$' -count=1 -v
```

Из корня repo, реальный image-backed пример:

```sh
python3 examples/system-c/fat12/create_image.py /tmp/minz-fat12.img
cd minzc
go run ../examples/system-c/fat12/run.go -image /tmp/minz-fat12.img -output /tmp/minz-fat12-default.txt
go run ../examples/system-c/fat12/run.go -size -image /tmp/minz-fat12.img -output /tmp/minz-fat12-size.txt
```

Полный заявленный объём — [objective.md](objective.md). Дальнейшие C99/C11,
runtime varargs, floating point, общий ресурсный анализ code/data/stack и
более широкий aggregate scalarization остаются самостоятельными задачами.
