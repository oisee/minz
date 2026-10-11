# ISA/ABI transfer: живое условие до записи F

Read-only аудит условия №2 roadmap017 сопоставил архивный LIR WFC с production.
Найден настоящий miscompile: `let p:bool=x<y;let r:u8=x>>1;return r+u8(p)`
при x0,y1 выдаёт raw/finalVM1, assembledZ800. CP сохранял CY, затем SRL заменял
его, и только после этого материализовался p. Аналогичный SLA и другие ALU
пути подтверждены до изменения. Expectation выводится из Bookv7 §3.1/3.2:
boolean — ранее вычисленное значение, арифметика не меняет его.

## Конкретная карта переноса

| Архив LIR | Нынешний production механизм | Итог аудита |
|---|---|---|
| `wfc.go:operandInterference` distinct inputs и dst/source | `mir2/alloc.go:BuildInterferenceGraph` simultaneous input liveness + def/live edges; `pbqp.go:physicallyConflicts`; `alloc.go:physicalAliases` | Main/shadow dword, pairs/halves, IX/IY containment представлены. Это механизм, не новое универсальное native доказательство |
| `wfc.go:destructiveWritePass` сохраняет живой src0 | SSA interference + `z80codegen_alu` staging, `saveWrittenScratch`, parallel copies | Имеющиеся exhausted scratch/mul/call native gates подтверждают конкретные случаи; WFC не нужен для этой обязанности |
| `wfc.go:clobberPass` проверяет pattern Clobbers (не только CALL) | `callerSavePairs`+parallel argument/results + declared extern clobbers; checked table independently rejects live overlap/clobber writes (`vir/table_admission.go`, `table_pattern_admission.go`) | Вызовы и table guards действуют; в обычных scalar emitters **пропущено live F перед shifts/ALU** |
| pattern F clobber | `materializePendingFlag` | До исправления вызывался перед CMP/MUL, ALU только при прямом flag operand. Перенесена недостающая обязанность на известных писателей |

`genShift`, `genBinOp`, `genDivMod`, OpNeg и OpBitGet теперь вызывают
существующее сохранение live predicate. Не введён all-instruction capture:
flag-preserving LD/CPL/SET/RES не обязаны сохранять весь F; direct comparison
имеет свой собственный consumer protocol. Обычный MUL уже исправлен отдельно.
`flagStillNeeded` теперь использует instruction/CFG liveness и текущие Uses,
включая Args и terminator: прежний Src-only scan пропускал эти потребители.
Материализация PUSH/POP AF сохраняет живой A; scratch exhaustion использует spill.

## Проверки

22 actual source variants: u8/u16 ×shr/shl/sar/add/sub/and/or/xor/div/mod/neg.
14 boundary values (с ограничением ширины),4y и 8shiftcounts0/1/7/8/15/16/31/32;
для div/mod5 ненулевых divisor. Независимая modular/signed-shift модель,
raw/finalVM, полный assembled native output, RAM вне bounded stack, code/SP/PC.
**PASS0.454s**, включая source branch consumer default+Grace.

Изолированный32бит: actual emitter Shl/Shr/Sar ×6counts0/1/7/16/31/32 ×9values
×2entryCY =324 native runs, реальные shadow registers, независимая uint32/int32
модель, условие после shift и unrelated liveA23, SP/PC/code. **PASS**.
Полный MIR2 **PASS20.899s**, terminal результат в `mir2.txt`.

## Выявленные отдельные широкие проблемы

Полная source32 matrix не объявляется PASS. `after.txt` сохраняет u32 gaps:
CMPu32 сравнивает только byte, `u8→u32` пишет TODO и не выполняет расширение;
generic spill argument требует actual symbol вместо псевдонима mem/Offset.
Из-за этого source32 ожидания расходятся даже на shift0. Убраны только из
этого focused test, их evidence сохранено и передано отдельным задачам.
Предмет этого коммита — перенос F-lifetime обязанности, а не снятие требования
wide32 correctness общей цели. CMP исправляет другой agent; extension следующий.

Начальный diagnostic `initial-audit.go.txt` намеренно имеет неверное assert1
для add3/~x; такие его падения не используются как proof. Правильный independent
model и полный before.txt отделяют их от реального miscompile.

Воспроизведение из minzc:

```
go test ./pkg/pipeline -run '^TestFlagWriterRetains' -v
go test ./pkg/mir2 -run '^TestWideShiftRetainsLiveFlagAndAccumulator$' -v
go test ./pkg/mir2
```

Исторические логи не изменены. Никакая compile-only/coverage карта не заменяет
эти bounded native доказательства; полнота всех ISA шаблонов не заявлена.
