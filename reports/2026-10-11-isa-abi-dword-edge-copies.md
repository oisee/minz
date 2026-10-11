# ISA/ABI migration: настоящий пробел wide edge copies

Base `c2b9a77f`. Read-only сравнение архивного LIR (`docs/grace-lir-final-removal/research-lir-c3864cf2.tar.xz`) с действующим backend привело к исправлению реального ordinary-source miscompile, а не восстановлению старого backend.

## Конкретный сбой и исправление

`emitParallelCopy` обрабатывал все width >8 word-cycle shortcuts как 16-битные. HL↔DE использовал только EX DE,HL; spill cycle сохранял только одну word-половину. Теневые старшие значения терялись. Forced native HL/DE u32 exchange давал `0x1234ef01` вместо `0xabcdef01`.

Обычный исходник с globals и while-loop, `old=x; x=y; y=old`, воспроизвёл production default ошибку: ina=0, inb=0x10000, count=1 -> outx=0,outy=0 вместо 0x10000,0. Source/final raw VM oracle при этом верен. Полные before ASM/логи — `dword-cycle-before.txt` и `dword-cycle-source-before.txt`; первый source probe с param ABI сохранён отдельно.

Теперь pending wide sources полностью снимаются на stack до записи любого destination. Для каждого значения сохраняются основной и теневой word; затем обычный resolver выполняет narrow copies и wide snapshots возвращаются в destinations. Memory snapshot использует EX (SP),HL и сохраняет рабочий HL; u24 memory high-byte не читает/пишет четвёртый байт. Wide immediate rematerialization теперь также записывает старшую shadow половину, а не только low word. Coalesced no-op не создаёт snapshot. Другие действующие overlap/byte/word правила сохранены.

Это bounded correctness fallback для wide edge/return copies. Не заявляются оптимальный код или единая descriptors-based allocation модель. Решение не трогает VM/call/terminator/error agent code. Whole-function checked tables пока отказывают CFG и width32; исправление нужно обычному production codegen.

## Карта ограничений и активных тестов

| Архивный механизм | Действующий путь | Evidence / остаток |
|---|---|---|
| `lir/z80.go` location namespace, pair/half/shadow | `z80spec/locations.go`, `mir2/alloc.go:physicalAliases`, PBQP physicallyConflicts | PBQP hard-constraint alias tests и z80spec emulator tests; namespace parity сама по себе не доказывает ISA legality |
| `lir/wfc.go:destructiveWritePass` | MIR interference/liveness + emitter repair; checked `vir/table_admission.go`, `table_pattern_admission.go` | Native A/scratch alias tests + checked tied/clobber admission; call multi-result simultaneous-def gap параллельно исправляет другой агент |
| `lir/wfc.go:clobberPass` | Production contract clobbers/caller save, PFCCO strategy; checked table ABI context | PFCCO byte adapter matrix, wide caller-spill tests; глобальная native optimality не доказана |
| Pattern ties, flags/scratch and actual costs | `vir/table_checked.go`, full ISA/context key, pattern admission | Dead/live tied operands, hard-clobber negatives, real weighted/triple native packets. Production leaf domain ограничен; calls/CFG/u32 не допускаются |
| CFG setup/parallel moves | `mir2/z80codegen_copy.go` | u16/half overlap tests были активны; **u32 full-bank cycle отсутствовал и ошибочен**, закрыт этим пакетом |
| LIR location costs / WFC quality | PFCCO/PBQP cost model + native measurement | Старые константы loc costs не считаются точными современными moves/saves; ранее retained multicall residual negatives остаются важны |

## Проверка

- Ordinary source 8×8×8=512 cases: raw/final VM/native, counts 0/1/2/3/7/8/254/255, boundary/high-bank values, exact PC/SP return. Code/literals immutable, исключения — объявленные globals и typed u32 spill slots.
- 30 forced native sets: pair swap, triple cycle, register↔spill, spill↔spill, old-source+wide immediate. Независимый initial-snapshot oracle проверяет все pairs в обоих банках и spill memory, exact AF, A, SP.
- Existing pair/half overlaps и u32 caller snapshot повторены: focused pipeline/MIR2 PASS 0.060/0.159 s.
- Full MIR2 PASS 30.622 s. Full pipeline здесь не запускался.

Точные ordinary source/ASM/BIN и SHA256 — `artifacts/dword-cycle/manifest.json`. Полезная миграция — реальная корректность wide edge movement; архив LIR не включён в Go build и не объявлен оракулом.

## Follow-up: valid mixed narrow/wide copies

Добавлены четыре раскладки на каждый из шести input masks (теперь 54 forced sets): HL32→DE32 + D8→A8; A8→H8 + HL32→DE32; A8→H8 + HL32→spill32; HL32→DE32 + coalesced BC32→BC32. Проверяются original-source reads до wide writes, narrow destination H при spill restore через HL, coalesced live BC, оба банка и F/stack. Когда A — явный destination, его новое значение сверяется с исходным D; F остаётся точным. PASS 0.071 s (`dword-cycle-mixed.txt`). Destination overlaps между двумя разными живыми outputs не считаются допустимой allocation.

u24 snapshot в register homes сохраняет полный физический pair, а memory payload имеет три байта; неиспользуемая четвёртая memory byte не становится частью значения. Отдельный u24 live-alias proof этим u32 packet не заявлен.
