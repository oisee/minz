# Перенос полезного K11 pattern в обычный Z80 backend

Полный исходник `examples/zx/plasma.nanz` сохранён без изменений: SHA256
`71113665c335bff7a20b34f581179c7055282e03f5332204f458a4578904991c`.
Base `ff6ff497`, freeze commit `1911770e`. На точном оригинальном `plasma_attr`
1024 независимых вызова ускорились **793332→331508 T (−58,21%)**. Полный BIN
модуля остался **1555 B**: другие функции всё ещё используют `__mul8`.
Это вычислительный helper, не весь экранный renderer/ZX runtime.

## Конкретный gap и перенос

`mir2/z80codegen_muldiv.go` раньше inline-умножал byte только на 0/1,
powers-of-two,3/5/6/7/9/10/12/15. Оригинальный `plasma_attr`, строка197,
делает `cy*11`; он уходил в PUSH BC/PUSH DE/CALL __mul8 со всеми сохранениями.
VIR `mul_table.go:tryConstMul` и `mulopt.go` показывают направление переноса:
подбирать короткую sequence с width/clobber/carry контрактом. Их внешние JSON
на этой машине отсутствуют; lookup GPU-таблицы в этом изменении не используется.
Архив LIR имел cost-pattern selection, но его WFC/solver не перенесены.

Самостоятельно выведенная sequence, A=x и temp=x:

```
LD temp,A
ADD A,A
ADD A,temp
ADD A,A
ADD A,A
SUB temp
```

Коэффициенты 1→2→3→6→12→11, modulo256; 6B/24T без окружающих moves/saves.
Входящий CY не читается. Используются существующие pickScratch8,
saveWrittenScratch и сохранение живого A; occupied scratch не объявлен свободным.
Не заявляется глобальная оптимальность: это проверенный полезный candidate.
Word whitelist не расширен.

## Обнаруженная и отдельно исправленная ошибка

Проверка `(x*K)+(x<y)` показала старую потерю carry-derived predicate:
x=0,y=1 получал0 вместо1 как при inline K3, так и runtime K11/K17.
`genMul` теперь до любого пути вызывает существующий
`materializePendingFlag`, который сохраняет A через PUSH/POP AF и переводит
истинность условия в свободный byte/spill. Исправление — отдельный коммит
`1ce77c08`, до K11 оптимизации. Новый тест не использует host no-op или изменение
ожиданий. Corrected plasma baseline имеет **точно прежние T/B/SHA**.

## Независимые проверки

Исходная Book v7 §3.1 byte семантика и исходное определение glsl_sin используются
как спецификация целочисленного quarter-wave approximation. Go-модель считает
q/oct/зеркальный param/corr и modulo byte sums самостоятельно; математический
floating-point sin не является оракулом этого исходника.

- 768 настоящих grid-вызовов (32×24): **595096→248728 T**.
- 256 additional boundary-вызовов cy0..255,cx7: **198236→82780 T**.
- Raw/final VM результаты, отсутствие любых heap writes; native все1024
  результаты, посторонняя RAM вне ограниченного actual стека, код, PC возврата
  и восстановленный SP проверены. Полный original source разобран и полный
  обычный module ASM/BIN собран; алгоритмы не вырезаны.
- Исторический public profile отказ `ASM unsupported` сохранён: оригинал
  содержит ZX ASM, поэтому для helper использован отдельный прямой runner.
  Это не ослабление production admission и не автоматическая PGO evidence.
- Exhausted scratch K11: all256x ×4entryF ×liveA yes/no =2048 native runs,
  пары BC/DE/HL/IX/IY, result и spill/liveA, SP и output flags against reference
  sequence проверены; **PASS0.263s**.
- Live predicate K3/K11/K17: all256x ×6y ×4F =18432 native runs,
  независимое `(x*K + (x<y)) mod256`, code/SP: **PASS0.186s** до оптимизации;
  окончательный полный MIR2 **PASS29.213s** включает K11 после оптимизации.
- Final exact original test **PASS0.107s** (повтор с heap-write guards также PASS).

| Backend | T | bytes | BIN SHA256 |
|---|---:|---:|---|
| Corrected ordinary |793332|1555|`0f4e9a4562ad3e6b22d01a678857a739efd0355ce84240841cc129de45167bec`|
| Corrected +K11 |331508|1555|`da0678eaaac073391b9637fe6a0c02c21707f17459f251ebe589bf88f7b90fd1`|

`before/` — до исправлений; `corrected-before/` — после generic flag fix,
до K11; `final/` — итог. `after/` и carry.txt — промежуточные observations,
сохранены честно; carry-before-original.txt подтверждает bug в baseline.
Полный медленный pipeline/CLI не запускался. Весь catalog не объявляется PASS
по одному helper, fullscreen/device/interrupt/frame timing не проверялись.

Воспроизведение из minzc:

```
go test ./pkg/pipeline -run '^TestOriginalPlasmaAttributeMul11$' -v
go test ./pkg/mir2 -run '^(TestMulLivePredicateNative|TestConstantByteMulExhaustedScratchNative)$' -v
go test ./pkg/mir2
```

`MINZ_PLASMA_MUL11_ARTIFACTS` сохраняет полный source/workload/MIR/ASM/BIN и
per-call измерения. Manifest фиксирует payload SHA256. Пакет переносит один
полезный cost-pattern; общие physical-constraint/CFG/call/table gaps остаются.
