# 6502/6510 в MinZ: zero page, GPU-поиск и проверяемые таблицы

**Дата:** 2026-10-14. **MinZ:** `0ffddf8b465ce813319f274ae624559beee7dc73`, `origin/main` на момент начала исследования. **Ветка:** `feat/research-6502-gpu`. Исследование выполнено в отдельном worktree; производственный код не изменялся.

## Результат

6502 backend уже существует: MIR2 → allocation → assembly → Go-эмулятор выполняет простую 8-битную арифметику, сравнения, некоторые циклы и цепочки вызовов. Его нужно довести до полноценного target, а не начинать с чистого листа. На зафиксированной версии есть тихие ошибки для умножения, динамического сдвига, 16-битных констант и перекрывающихся ZP byte/pair. Добавление переключателя `--target 6502` до исправления этих случаев преждевременно.

Идея «256 регистров» разумна как **модель размещения компиляторных значений в 256 байтах памяти**. Реальные регистры — A, X и Y. ZP является адресуемой памятью с затратами на загрузку, запись, scratch, пары указателей и взаимодействие с платформой. На 6510 `$00/$01` — аппаратные регистры порта; на C64 ROM и IRQ используют значительную часть ZP. Поэтому «256 доступных регистров» не является универсальным ABI.

GPU-инфраструктура действительно есть, но в трёх разных местах:

1. `oisee/gpuforce` содержит CUDA-поиск Z80 и GPU allocator, однако его текущий Go checkout неполон: отсутствуют `go.mod`, `pkg/gpugen`, `pkg/regalloc`. Команда, рекламирующая 6502 generation, напрямую не собирается.
2. `oisee/z80-optimizer` содержит работоспособный генератор CUDA/Metal/OpenCL/Vulkan для **14-инструкционного 6502 multiply subset**. В модели A/X/Y/C и один `zp0`; проверяется результат A при заранее заданных начальных состояниях.
3. `oisee/6502-optimizer` содержит настоящий CUDA/Go peephole miner для **58 типов операций / 2 863 concrete instructions**, но память сведена к одному ZP-байту и двум stack slots. Это не полный CPU/system emulator и не 6510/C64 simulator.

Существующие многомиллионные allocation tables — **Z80-таблицы**. Готового 6502/ZP allocator table в исследованных checkout нет. Хороший путь: offline GPU генерирует кандидаты, независимый native checker допускает их в versioned tables; production compiler работает с таблицами и native fallback без CUDA/Z3 runtime dependency.

## 1. Что проверено и где

| Источник | Зафиксированный commit | Проверяемые файлы |
|---|---|---|
| MinZ | `0ffddf8b465ce813319f274ae624559beee7dc73` | [m6502codegen.go](../../minzc/pkg/mir2/m6502codegen.go), [m6502cost.go](../../minzc/pkg/mir2/m6502cost.go), [sim6502run.go](../../minzc/pkg/sim6502run/sim6502run.go) |
| GPUFORCE | `622109c75657d6930a79ff1ed54178215ddc9f10` | [z80_regalloc.cu](https://github.com/oisee/gpuforce/blob/622109c75657d6930a79ff1ed54178215ddc9f10/cuda/z80_regalloc.cu), [cmd/gpugen](https://github.com/oisee/gpuforce/blob/622109c75657d6930a79ff1ed54178215ddc9f10/cmd/gpugen/main.go) |
| z80-optimizer | `7701e474705b0a2d58364764870ff98d3c6390b7` | [mos6502.go](https://github.com/oisee/z80-optimizer/blob/7701e474705b0a2d58364764870ff98d3c6390b7/pkg/gpugen/mos6502.go), [emit.go](https://github.com/oisee/z80-optimizer/blob/7701e474705b0a2d58364764870ff98d3c6390b7/pkg/gpugen/emit.go), [regalloc](https://github.com/oisee/z80-optimizer/tree/7701e474705b0a2d58364764870ff98d3c6390b7/pkg/regalloc) |
| 6502-optimizer | `c643bb9074c5e5743016691a80466238d8199030` | [6502_common.h](https://github.com/oisee/6502-optimizer/blob/c643bb9074c5e5743016691a80466238d8199030/cuda/6502_common.h), [6502_search_v2.cu](https://github.com/oisee/6502-optimizer/blob/c643bb9074c5e5743016691a80466238d8199030/cuda/6502_search_v2.cu), [verifier.go](https://github.com/oisee/6502-optimizer/blob/c643bb9074c5e5743016691a80466238d8199030/pkg/search/verifier.go) |

Измерения выполнены на NVIDIA RTX 2070, 8 GiB, SM 7.5; driver 550.120, nvcc 12.0.140. MinZ использует свой Go toolchain 1.24.3. Во внешних репозиториях `go 1.25` вызывает попытку загрузить отсутствующее имя toolchain `go1.25`; для воспроизводимой сборки использован уже установленный `GOTOOLCHAIN=go1.26.8` без изменения `go.mod`.

Исторические [proposal 6502](../../research/abi-paper/PROPOSAL_6502_backend.md) и [report 067](../2026-03-13-067-6502_Backend_E2E_Harness.md) помогают понять историю, но их scope/списки TODO устарели. Нынешний emitter — 1 003 строки; часть branching/loop fixes уже реализована. Утверждение proposal «zero changes to MIR2 core / PBQP 100% reusable» не подтверждается текущими alias и target-specific paths.

### MinZ: реальные границы поддержки

| Компонент | Факт на текущем commit | Ограничение |
|---|---|---|
| `M6502CostTable` | 18 locations: A/X/Y, 8 ZP bytes `$02–$09`, 4 pair views, stack, abs, P | Это не 256-slot model; byte/pair aliases должны моделироваться отдельно |
| `NewM6502CodegenCostTable` | Фильтрует stack/P/absolute locations, оставляет 15 locations | Сохраняет byte/pair views одного и того же storage; общий allocator всё равно может fallback-spill в `$F000+` |
| `M6502Codegen` | 8-bit constants/moves/add/sub/neg/logic/cmp, branch/block args, shifts through A, JSR, RTS | Width-specific lowering, memory, mul/div, SMC отсутствуют; неизвестный op становится TODO comment |
| `sim6502run` | Compile/Run API для cross-backend tests; tiny assembler; bootstrap по allocation | Возвращает только `uint8 A`; нет C64 system state; emulator errors/step limit в `run()` не возвращаются как ошибка |
| MIR2 tests | 35 top-level tests по `6502|M6502` PASS | Это ограниченный corpus, а не доказательство полного языка |
| Showcase | 7 tests PASS | SMC showcase сам печатает отсутствие lowering; не native SMC check |
| Production pipeline | [pipeline.go:365](../../minzc/pkg/pipeline/pipeline.go#L365) выбирает Z80/eZ80 costs; main allocation — PBQP | 6502 не подключён сюда; тестовый 6502 path использует `Allocate`, greedy colouring |
| Contract selection | [contract_strategy.go:21](../../minzc/pkg/pipeline/contract_strategy.go#L21) допускает явный PBQP strategy только для Z80 | Нельзя автоматически объявить PFCCO/PBQP 6502 проверенным |
| Old backend | [codegen/_deprecated](../../minzc/pkg/codegen/_deprecated/m6502_backend.go) | Старые SMC/TSMC implementations не production support |

Проверяемый результат выполнения тестов: [6502 log](evidence/minz-6502-tests.log), [showcase log](evidence/minz-showcase-tests.log). Console C64/Apple II/BBC Micro tests подставляют `STA $F001; RTS` по адресам ROM entry points. Они доказывают работу JSR/output stub, а не работу настоящей KERNAL ROM, memory banking или её ZP clobbers.

### Свежие контрпримеры

Все примеры собраны из корректного MIR2 через существующий API, без изменения compiler. [Код](probes/minz_probe.go), [вывод](evidence/minz-probes.log).

| Вход/условие | Получено | Ожидалось / вывод |
|---|---|---|
| `shift(3,3)` через `Builder.Shl` | 6, nil error | 24; [codegen:236](../../minzc/pkg/mir2/m6502codegen.go#L236) при `Src[1]` принудительно считает shift count = 1 |
| `multiply(3,3)` | 3, nil error | 9; вместо `OpMul` emitted TODO comment |
| `Const(0x1234, TyU16, ClassPair)` | `LDA #$34; STA $02` | Старший байт `$12` не записывается, Compile nil error |
| Одновременно живые ptr и u8 byte | `zp23@2` и `zp2@2`, `interfere=true` | Оба используют `$02`; pointer low byte уничтожается |
| `PhysicalAliasesForTest(zp23)` | пустой список | [alloc.go:578](../../minzc/pkg/mir2/alloc.go#L578) моделирует Z80 aliases, но не memory spans |

Ещё прямые code findings:

- [m6502codegen.go:606](../../minzc/pkg/mir2/m6502codegen.go#L606): `zpScratch = "$00"`. На 6510 это data-direction register, не RAM scratch.
- [m6502codegen.go:764](../../minzc/pkg/mir2/m6502codegen.go#L764): backups начинаются с `$F0`; [cyclic block copies:948](../../minzc/pkg/mir2/m6502codegen.go#L948) используют `$D0+`. На C64 оба диапазона заняты системными переменными. Нет единого target ZP lease и контроля конца диапазона.
- [genInst:187](../../minzc/pkg/mir2/m6502codegen.go#L187): `OpCall` эмитит только `JSR Sym`, не переносит Args/return Dst по контракту и не исполняет caller-save policy. Проверенные A→A call chains не покрывают общий ABI.
- [genCmpBool:526](../../minzc/pkg/mir2/m6502codegen.go#L526): signed/unsigned comparison conditions используют одни и те же carry branches. Для signed i8 сравнения через обычный CMP это требует отдельного lowering.
- Генератор не устанавливает `D=0` через CLD и не имеет runtime/extern contract для decimal flag.

Cost table тоже пока отражает предпочтения, а не точную стоимость emitted schedule. Например, [m6502CostIndex:297](../../minzc/pkg/mir2/m6502cost.go#L297) считает ZP→Y как `LDA_zp + transfer = 5`, хотя прямой `LDY zp` занимает 3 cycles. [Counter cost](../../minzc/pkg/mir2/m6502cost.go#L206) использует not-taken BNE baseline, тогда как обычная loop back edge taken. Для точного GPU cost oracle нужны затраты по реальным patterns и layout.

## 2. Zero page как register file

### Что означает «256 регистров»

NMOS 6502 имеет 8-bit A/X/Y, status P, 8-bit stack pointer S и PC. Ни ZP-байт, ни ZP-пара не является дополнительным hardware ALU register. Но компилятор может назначить 256 байтов `$0000–$00FF` виртуальному storage pool и генерировать для них короткие инструкции.

| Операция NMOS 6502 | Bytes | Cycles | Следствие для allocation |
|---|---:|---:|---|
| `TAX/TAY/TXA/TYA` | 1 | 2 | A↔index дешёвый; меняет N/Z |
| `TXA; TAY` / `TYA; TAX` | 2 | 4 | X↔Y проходит через A и clobbers его |
| `LDA/LDX/LDY zp`, `STA/STX/STY zp` | 2 | 3 | ZP storage не бесплатный register move |
| `LDA abs`, `STA abs` | 3 | 4 | ZP экономит 1 byte/1 cycle на такой доступ |
| `ADC/SBC zp` | 2 | 3 | ALU destination/input accumulator — A |
| `INC/DEC/ASL/LSR/ROL/ROR zp` | 2 | 5 | Нативный memory update иногда дешевле load→ALU→store; это RMW |
| `LDA (zp),Y` | 2 | 5 или 6 | Требует pointer pair + Y; read page crossing влияет на cost |
| `STA (zp),Y` | 2 | 6 | Pointer + Y; всегда 6 |
| `LDA (zp,X)` | 2 | 6 | X индексирует **адрес пары в ZP**, затем читает pointer |
| `PHA; PLA` | 2 | 7 | Hardware stack нужен для LIFO/save; это не arbitrary indexed register pool |

Стандартные ISA/addressing сведения сверены с [6502 instruction reference](https://www.masswerk.at/6502/6502_instruction_set.html), разделом 6510 того же источника, [C64 memory map](https://www.pagetable.com/c64ref/c64mem/) и независимым [py65 NMOS implementation](https://github.com/mnaberez/py65/blob/master/py65/devices/mpu6502.py). HTML/source snapshots и SHA-256 сохранены вместе с внешними артефактами; противоречивые общие формулировки о BCD flags не используются как oracle.

Полезная модель — `CPU register / ZP byte / ZP contiguous span / absolute span / stack save`, с lifetime, physical overlap и instruction access costs. Совместно живущие byte `$20`, pair `$20/$21`, u32 `$20–$23` конфликтуют. Неявное A clobber любого memory→memory ALU pattern должно участвовать в liveness/scratch validation.

### 6510 и C64

6510 использует ту же documented NMOS 6502 arithmetic ISA; его on-chip I/O port, bus control и C64 banking не добавляют CPU arithmetic registers. Поэтому можно разделять ISA semantics, platform memory/ABI profile и timing environment.

`$0000` — data-direction register, `$0001` — peripheral data register; на C64 его биты управляют ROM/I/O mapping и cassette. `$02–$09` не становятся свободными автоматически после исключения двух аппаратных адресов. C64 memory map включает BASIC/KERNAL variables в ZP; явно помеченные user bytes `$FB–$FE` дают всего четыре байта, если выбран консервативный ROM-compatible профиль. Это не исчерпывающий список всех потенциально арендуемых байтов: для конкретного runtime можно доказать безопасность других диапазонов или целиком управлять ZP.

Практические target profiles:

| Профиль | Ownership | Что требуется |
|---|---|---|
| Bare-metal 6502 | Выбранный runtime владеет ZP pool | Явная bitmap reserved/scratch/global/soft-stack bytes |
| C64 KERNAL/BASIC compatible | Частично арендуемая ZP, ROM/IRQ alive | Документированные ZP clobbers каждой extern/IRQ path, сохранение арендованных spans |
| C64 takeover demo | Runtime владеет почти всей ZP кроме `$00/$01` и своих служебных bytes | Явная IRQ/NMI/ROM-call policy и restoration при возврате; общий размер pool всё равно меньше 256 |
| 65C02 / NES 2A03 | Другие ISA/decimal contracts | Отдельные descriptors; 65C02 имеет дополнительные ops, 2A03 не выполняет decimal adjustment |

IRQ/NMI может выполняться в середине carry chain, а системный handler — использовать ZP или D. Программа и handler должны иметь совместимый save/restore contract; простой function-local allocator этого не обеспечивает. Reentrant/recursive functions также не могут безусловно разделять один статический ZP frame.

### Пары указателей, wrap и addressing

Чётность ZP pointer pair не обязательна: `$21/$22` допустима. Нужны два корректно leased байта. Для `(zp),Y` high byte pointer после `$FF` читается из `$00`, а для `(zp,X)` pointer address и high-byte fetch тоже оборачиваются в zero page. Поэтому pair at `$FF` в обычной linear span model недопустима без специального wrap representation. На 6510 её high byte попадает в hardware `$00`.

Обычная multi-byte memory value at `$FF` с байтом по `$0100` — другое размещение, оно не равнозначно indirect pointer pair. ZP indexed effective address оборачивается в пределах страницы; `(zp),Y` добавляет Y уже к 16-bit pointer. NMOS `JMP ($xxFF)` имеет отдельный page-wrap bug для high byte fetch; 65C02 отличается.

Адресные modes меняют лучший выбор A/X/Y и ZP. Для buffer copy важнее держать base pointer в ZP pair и index в Y, чем одинаково предпочитать X/Y как «general registers». `LDX zp,Y` и `STX zp,Y` существуют, но `LDX abs,X` нет; `LDY zp,X`, `STY zp,X` существуют, обычных симметричных вариантов для всех троек нет. Эти ограничения должны быть patterns, а не одним scalar cost class.

Hardware stack — `$0100–$01FF` с S, который оборачивается modulo 256. JSR/RTS/interrupts потребляют тот же pool. `TSX; LDA $0101,X` позволяет явно обращаться к stack memory, но использует X и требует layout/wrap reasoning; на NMOS нет прямого SP-relative mode, PHX/PLX/PHY/PLY. Для широких локальных значений/recursive frames обычно полезнее отдельный soft stack или статический frame при доказанной non-reentrancy.

Подход не новый: в [LLVM-MOS MOSRegisterInfo.td](https://github.com/llvm-mos/llvm-mos/blob/0f031168a7cc8e81b7b40c0ec0b1f7b3c90b8a63/llvm/lib/Target/MOS/MOSRegisterInfo.td#L119) imaginary registers являются ZP storage; указан `MaxImag8Regs=256`, пары, soft-stack pointer и linker placement. [Calling convention](https://github.com/llvm-mos/llvm-mos/blob/0f031168a7cc8e81b7b40c0ec0b1f7b3c90b8a63/llvm/lib/Target/MOS/MOSCallingConv.td#L16) различает caller/callee saves. Это полезный prior art; утверждать новизну самого ZP register file или отсутствие альтернативных ABI работ нельзя.

## 3. Широкая арифметика

16/32/64-bit arithmetic реализуется цепочкой byte ops. Little endian addition:

В нынешнем [MIR2 types.go](../../minzc/pkg/mir2/types.go#L35) есть u8/u16/u24/u32 и signed counterparts, но нет `TyU64/TyI64`. Поэтому 16/32-bit lowering может опираться на существующие scalar types, а u64 потребует расширения frontend/type/VM/ABI contracts либо library representation из двух u32/восьми bytes. Raw 64-bit probe ниже доказывает работоспособность byte chain, а не наличие u64 в языке MinZ.

```asm
    CLC
    LDA lhs+0
    ADC rhs+0
    STA dst+0
    LDA lhs+1
    ADC rhs+1
    STA dst+1
    ; ещё 2 или 6 таких byte steps для u32/u64
```

LDA/STA сохраняют C; поэтому carry передаётся между bytes. Для subtraction — один `SEC` перед всей цепочкой и SBC; C=1 означает отсутствие borrow. Повторный CLC/SEC внутри цепочки разрушит широкую арифметику. Частично перекрывающиеся spans требуют отдельного анализа; `dst=lhs` допустим для такого порядка, `dst=lhs+1` может уничтожить следующий input byte.

Baseline для двух ZP inputs и ZP output, без call/RTS и прочих setup:

| Width | Bytes | Cycles | Независимая проверка |
|---|---:|---:|---|
| 16 | 13 | 20 | ADC и SBC: по 2 056 boundary/seeded input pairs PASS |
| 32 | 25 | 38 | ADC и SBC: по 2 056 pairs PASS |
| 64 | 49 | 74 | ADC и SBC: по 2 056 pairs PASS |

Формулы: `1 + 6*N` bytes, `2 + 9*N` cycles для N bytes. Это стоимость конкретной hand-written baseline, а не нынешнего MinZ codegen и не доказанный минимум. [Research code](probes/native_arithmetic.go) запускает raw machine bytes на `cjbearman/sim6502`; [log](evidence/native-arithmetic.log). Проверены результаты modulo width; all-input proof, полный набор terminal flags и device timing этим не доказаны.

После последнего ADC/SBC C относится к whole-word carry/borrow, V и N соответствуют high-byte signed arithmetic. Z показывает только последний обработанный byte, а не `whole_word == 0`. Нужен отдельный zero test с OR-reduction либо условная проверка всех bytes. Compare, signed ordering, saturating ops и extension/truncation должны иметь собственные contracts.

Мulti-byte left shift: ASL low byte, ROL последующих. Right shift: LSR high byte, ROR остальных в обратном порядке; arithmetic right shift требует sign fill. Shift count 0 и count >= width — отдельные семантические случаи. Memory RMW имеет dummy write на NMOS: применять такие patterns к volatile/I/O нужно только согласно memory-effect contract.

Integer ABI должен задавать **D=0** на entry и после extern/asm calls либо emitter должен восстановить D. При D=1 ADC/SBC выполняют BCD; CLC/SEC этого не исправляет. На NMOS N/Z/V в decimal mode связаны с промежуточной ALU арифметикой и не являются простыми flags от скорректированного decimal результата. Для BCD нужны отдельная спецификация, CPU variant и отдельные проверенные tables. Текущий sim6502 wrapper годится как binary result oracle; его decimal flags implementation не следует назначать golden NMOS oracle.

После базовых add/sub/compare/shift разумны shift-add multiplication, signed/unsigned div/mod helpers, constant multiply/divide patterns. GPU offline search особенно полезен для маленьких constant ops, переносов между A/X/Y и branch idioms; полный 64-bit all-input sweep имеет пространство `2^128` для двух inputs и не является практичным GPU доказательством. Для широких ops нужны композиционное byte-level доказательство и native differential tests.

## 4. Что GPUFORCE действительно предоставляет

### 4.1 Неполный GPUFORCE checkout и исходный generator

В `gpuforce` [cmd/gpugen:23](https://github.com/oisee/gpuforce/blob/622109c75657d6930a79ff1ed54178215ddc9f10/cmd/gpugen/main.go#L23) регистрирует `gpugen.MOS6502Mul`, но пакет и `go.mod` отсутствуют. `go build ./cmd/gpugen` завершается `cannot find main module`. Это факт конкретного checkout, а не вывод об отсутствии 6502 работы вообще.

В исходном `z80-optimizer` пакет есть. [mos6502.go](https://github.com/oisee/z80-optimizer/blob/7701e474705b0a2d58364764870ff98d3c6390b7/pkg/gpugen/mos6502.go) определяет:

- A/X/Y/C и один symbolic `zp0`;
- ASL/LSR/ROL/ROR A, TAX/TXA/TAY/TYA, CLC/SEC, STA zp, ADC/SBC zp, EOR #$FF;
- цель `A*K mod 256`; entry X=Y=zp0=0, C=false;
- quick inputs 1/2/127/255 и затем все 256 A inputs;
- cost — статические cycles; score сначала instruction length, затем cycles.

Нет PC/S/full P, 256-byte ZP, branch/address execution, decimal flag, pointer memory или 6510 port. Не проверяются arbitrary incoming carry, X/Y preservation и final flags. Shader generation unit tests проверяют наличие строк; реальную CUDA compilation/runtime здесь проверили дополнительно.

Команды generation и object compilation прошли; [kernel smoke host](probes/mos6502_cuda_smoke.cu) выполнил len 1–4, 41 370 кандидатов. На этом запуске найден:

```asm
    ROL A
    STA scratch
    ROR A
    ADC scratch
```

4 instructions, 6 bytes, 10 cycles. Для `A=3` native simulator даёт 9 при C=0 и 11 при C=1. Среди всех 256 A inputs: **0 ошибок при C=0, 256 ошибок при C=1**. [GPU log](evidence/mos6502-cuda-smoke.log), [native log](evidence/native-arithmetic.log). Шаблон условно корректен под declared C=0, D=0; перед произвольным call можно добавить CLC, но стоимость и flags/clobber contract нужно заново проверить.

Измеренные 0.093931 s — четыре kernel launches и transfers после CUDA allocation; маленький smoke с launch/module overhead. Это не steady-state throughput benchmark и не обещание времени построения полной таблицы.

### 4.2 Отдельный 6502 peephole miner

В [6502-optimizer](https://github.com/oisee/6502-optimizer/tree/c643bb9074c5e5743016691a80466238d8199030) есть работоспособные Go CPU executor и CUDA executor. Проверены `go test ./...`, nvcc build и ограниченный CUDA search slice.

Модель в [6502_common.h:28](https://github.com/oisee/6502-optimizer/blob/c643bb9074c5e5743016691a80466238d8199030/cuda/6502_common.h#L28): A/X/Y/P/S/M/S0/S1. **M — один общий byte для всех ZP ops; S0/S1 — два абстрактных stack slots.** Поэтому multiple independently addressed memory cells, pointer pairs, arbitrary stack depth, stack/RAM aliases, self-modifying code и C64 devices этим не покрыты.

ADC/SBC явно binary-only в [cpu/exec.go:7](https://github.com/oisee/6502-optimizer/blob/c643bb9074c5e5743016691a80466238d8199030/pkg/cpu/exec.go#L7) и CUDA executor. CLD/SED при этом присутствуют, но ADC/SBC игнорируют D. Например, valid BCD `09 + 01` имеет результат `$10`, модель даёт `$0A`. Такие sequences нельзя принимать по этой модели с произвольным D или SED внутри.

Проверка называется exhaustive, но имеет границы:

- [verifier.go:272](https://github.com/oisee/6502-optimizer/blob/c643bb9074c5e5743016691a80466238d8199030/pkg/search/verifier.go#L272): sweep A=0..255 и C=0/1, остальные P bits фиксированы; это не sweep всего P.
- До двух extra modeled regs используются полные 256-value sweeps.
- Для трёх и более extra regs [Go reduced sweep:377](https://github.com/oisee/6502-optimizer/blob/c643bb9074c5e5743016691a80466238d8199030/pkg/search/verifier.go#L377) использует 32 representative values. CUDA host [search_v2:945](https://github.com/oisee/6502-optimizer/blob/c643bb9074c5e5743016691a80466238d8199030/cuda/6502_search_v2.cu#L945) отправляет такие cases на CPU, который тоже reduced. Fallback не превращает выборку в доказательство.
- Даже полный sweep абстрактной модели не доказывает full-memory аппаратную equivalence.

Slice `--first-op-start 2 --first-op-end 3 --max-target 2` обработал **2 601 pruned targets**, нашёл **8 rules**, внешнее wall time **0.65 s**. Среди них `TXA; TAX → TXA`. Стандартный Go `verify-jsonl` подтвердил 8/8. [CUDA log](evidence/6502-cuda-smoke.log), [Go verification log](evidence/6502-smoke-cpu-verification.log), [rules](evidence/6502-smoke-rules.jsonl). Это cross-check двух реализаций общей модели; независимый device/memory oracle нужен отдельно.

README сообщает 1 078 897 rules / 403 families / 1 041 s на RTX 4060 Ti. В checkout нет соответствующей rule table/data directory, эти числа не переизмерялись. Поиск 6510-specific port/bus implementation и 6502 regalloc/table consumer в трёх checkout результатов не дал; нельзя называть miner готовым CUDA simulator 6510 или готовой 6502 allocation database.

### 4.3 Allocation tables — другая задача

[GPUFORCE z80_regalloc.cu](https://github.com/oisee/gpuforce/blob/622109c75657d6930a79ff1ed54178215ddc9f10/cuda/z80_regalloc.cu) не исполняет CPU instructions. Он перебирает static vreg→location assignments, проверяет interference/width/param constraints, выбирает pattern cost и делает minimum reduction. Schedule и pattern vocabulary заранее заданы. Это allocation solver, тогда как MOS6502Mul/peephole miner ищут instruction sequences.

Ограничения нынешнего kernel: max 16 vregs, 64 ops, 15 locations, uint16 masks; widths и location IDs жёстко завязаны на Z80 A/B/C/D/E/H/L/BC/DE/HL/IX/IY halves/mem0. Без изменения ISA/alias/width model нельзя просто передать ему A/X/Y и 256 ZP bytes.

Kernel собрался и `--server` работает. [JSON probe](probes/gpu_regalloc_probe.py), [log](evidence/gpu-regalloc-probes.log):

| Вход | Выход |
|---|---|
| add с A/B params и tied result A | `[0,1,0]`, cost=4, searchSpace=6, feasible=1 |
| Interfering 8-bit B и 16-bit BC params | `[1,7]`, cost=0, feasible=1 — physical overlap ошибочно принят |

[check_interference:149](https://github.com/oisee/gpuforce/blob/622109c75657d6930a79ff1ed54178215ddc9f10/cuda/z80_regalloc.cu#L149) сравнивает только равенство location IDs. Такие candidates должны отбрасываться независимо в MinZ. `--demo`/одиночный input path отдельно неисправен: constrained loc maps не вычисляются, demo сообщает 0 assignments. Это не препятствует работающему server path, где [compute_constrained_locs:1071](https://github.com/oisee/gpuforce/blob/622109c75657d6930a79ff1ed54178215ddc9f10/cuda/z80_regalloc.cu#L1071) вызывается.

### Какие таблицы лежат в репозиториях

В z80-optimizer реально присутствуют ENRT artifacts, Z80T symlinks и OFB sidecar. В GPUFORCE symlinks остались, основные ENRT/large binary targets отсутствуют. [Table header probe](probes/table_headers.py), [результат](evidence/table-headers.jsonl):

| Artifact z80-optimizer | File bytes | Records из header | Проверка body |
|---|---:|---:|---|
| `enriched_4v.enr` | 4 104 402 | 156 506 | Все records распарсены, 123 453 feasible |
| `enriched_5v.enr.zst` | 22 829 072 | 17 366 874 | Только header; full body не проверялся |
| `enriched_6v_dense.enr.zst` | 58 012 779 | 66 118 738 | Только header; full body не проверялся |

Сумма stored record counts — 83 642 118. Это не independent unique coverage, не число доказанных native functions и не новое измерение корректности; lower-vreg enumeration может повторяться между файлами. Исторический [audit 118](../2026-08-21-118-GPU-Superoptimizer-Artifacts-Audit.md) исследовал другие snapshots, поэтому его количества и carry bugs не переносятся автоматически на текущий MinZ.

Shape keys кодируют widths/allowed loc sets/interference graph. ENRT record хранит одну assignment и 12 heuristic metrics; [enrich-regalloc](https://github.com/oisee/z80-optimizer/blob/7701e474705b0a2d58364764870ff98d3c6390b7/cmd/enrich-regalloc/main.go#L173) — CPU postprocessor, не новая GPU optimization. OFB — operation-feasibility metadata. Наличие average ALU cost/OFB не доказывает лучший allocation для конкретного order, frequency, scratch, flags и ABI. Для operation-aware оптимальности нужна Pareto family assignments или новое решение конкретного problem, а не enrichment одной прежней assignment.

## 5. Как связать это с MIR2, PBQP, PFCCO и checked tables

MIR2 types/CFG/optimizations/liveness действительно переиспользуемы. Но общий allocator сейчас содержит Z80 assumptions: memory span aliases, implicit A scratch, byte-vs-word eligibility, fallback spill storage и exact access patterns нужно вынести в target-aware contracts. PBQP — подходящий native fallback, но эвристические RN reductions не следует рекламировать как global optimum для arbitrary graphs.

PFCCO выбирает parameter/return storage по всем call sites. На 6502 это особенно полезно: pointer in ZP pair, offset in Y, accumulator value in A, loop counter in X; wrong choice порождает loads и clobbers. Нужен target-aware move matrix, concrete call frequency/size objective, ROM extern ABI, ZP live-across-call lease и return policy для u16/u32/u64. Текущий `OpCall=JSR` сначала нужно заменить настоящим call lowering. [vir/pfcco.go](../../minzc/pkg/vir/pfcco.go) — отдельный Z3 research path; production [contracts_pbqp.go](../../minzc/pkg/mir2/contracts_pbqp.go) не тождественен ему.

Нынешние MinZ checked-local tables уже дают правильный архитектурный образец:

- [CheckedTableKey](../../minzc/pkg/vir/table_checked.go#L20) включает descriptor, costs, concrete ops/immediates/symbols, ABI, context, overlap и emission options.
- [AddChecked / LookupChecked](../../minzc/pkg/vir/table_checked.go#L186) независимо допускают candidate и повторно проверяют lookup; legacy signature не считается certificate.
- [table_admission.go](../../minzc/pkg/vir/table_admission.go) проверяет liveness/physical overlap и direct patterns; [table_generate.go](../../minzc/pkg/vir/table_generate.go) трактует solver result только как candidate.
- [table_composition.go](../../minzc/pkg/vir/table_composition.go) и [table_readmission.go](../../minzc/pkg/vir/table_readmission.go) проверяют fragment domains/boundaries и remapped candidates.

**Этот механизм пока Z80-specific**: key явно требует `desc.Name == "z80"`, overlap/emission binding используют z80spec и Z80 emitter. Его принципы нужно перенести с новым schema ID; импорт 6502 JSON в существующую schema будет ошибкой. Native table admission доказывает stated local contract, а не автоматически optimality search или полный CPU semantics.

Для 6502 candidate certificate стоит хранить:

| Поле | Зачем |
|---|---|
| CPU variant + exact ISA hash | NMOS 6502/6510 versus 65C02/2A03; invalidation при изменении semantics |
| Platform/ZP ownership bitmap, byte/span overlap model | Port/ROM/runtime reservations и фактическая pair adjacency |
| Ordered MIR/patterns, immediates, memory effects/alias domains | Один abstract M не заменяет множество различимых адресов |
| Entry/output A/X/Y and C/D/N/Z/V contracts | Не допустить C=0-only mul3 на arbitrary carry; отличить word Z |
| Read/write/clobber/scratch sets, lifetime, call boundaries | Scratch обязан быть свободен на всей execution interval |
| ABI and function context | Pointer/index/result locations, preserved registers, live-through storage |
| Bytes, base cycles, page predicates / relocation fixups | `(zp),Y` crossings, branch range/page и code address changes |
| Oracle/test version + proof domain + provenance | Различать exhaustive byte proof, reduced sweep и measured case |

Composability: fragment output locations/flags/memory должны удовлетворять следующему fragment input; промежуточный C — live value. Scratch lease не должен пересекаться с boundary live spans. Relocation символического `scratch` допустима только в разрешённый ZP byte; indirect base — в разрешённую pair; branch relocation требует range/page recalculation. Independently recompute emitted bytes/cost, а не доверять GPU's `cost` field.

Production pipeline: `MIR2 → native allocation baseline → admitted table candidates → emit/assemble/check → выбрать допустимый результат по objective`. Table miss/failed admission использует native baseline. GPU/Z3 нужны только offline tools; compiler installation и demo runtime от них не зависят.

## 6. Размер поиска и разумная архитектура GPU oracle

Наивный перебор 256 ZP slots для восьми одновременно живых virtuals уже даёт `256^8 = 2^64 ≈ 1.84e19` assignments, до учёта pairs, modes и order. Добавлять 256 IDs к существующему base-15 kernel и надеяться на GPU speedup бессмысленно.

Большинство внутренних byte slots симметричны. Полезно разделить:

1. **Abstract instruction/allocation shape:** A/X/Y, несколько symbolically numbered ZP bytes/spans, adjacent pointer pair classes, liveness/ABI/flag states.
2. **Concrete placement:** материализация symbolic spans в leased target ZP с проверкой reserved bytes, pointer wrap, memory aliases и relocation.

Renaming quotient уменьшает поиск; canonical labeling учитывает span overlap/adjacency и фиксированные ABI bytes. Symmetry неприменима к ports, user-visible addresses, indexed ZP table layouts или page-cross predicates. Local shape table должна хранить несколько недоминируемых candidates: минимум bytes, минимум cycles и варианты preserved/clobbered A/X/Y/C. Иначе local optimum ломается на composition boundary.

Первый GPU oracle разумно ограничить небольшим straight-line domain: до 4–6 byte values, A/X/Y + 4–8 symbolic ZP bytes, одна/две pointer pairs; exact documented NMOS subset, D=0, no opaque calls/SMC/volatile. Для такого domain можно проверить все byte operands/flags, cross-check native instruction bytes и раздельно сертифицировать placement. CFG/page-sensitive timing и широкий memory search добавляются после этого.

Необходимые falsification checks до генерации больших tables: entry C=0/1, all modeled P bits, X/Y live-through, independent ZP bytes, pointer pair at boundary, stack side effects, input/output overlap, long branch relocation и decimal variant exclusion. Сегодняшние witnesses показывают, почему эти dimensions нужны.

## 7. Roadmap и быстрые выигрыши

Оценки ниже — engineering scope, не обещание календарных сроков или benchmark speedup.

| Приоритет | Изменение | Условие завершения |
|---|---|---|
| P0 | Fail explicitly на unsupported op/type/count; 6502 capability validation до emission | Mul, dynamic shifts, u16 const и signed compare перестают тихо выдавать неверный код |
| P0 | Единый target ZP map: reserved bitmap + scratch leases + span aliases | Witness `zp23/zp2` отвергается; `$00/$01`, `$D0+`, `$F0+` не используются скрыто |
| P0 | Run harness возвращает emulator stop/error/step-limit; независимый binary oracle | Ошибочный instruction/timeout не выглядит успешным u8 result |
| P1 | Byte-exact u16 const/move/add/sub/compare, pointer indirect load/store | VM↔native corpus содержит carry/borrow, pair alias и page/wrap cases |
| P1 | Настоящие call/return moves и saves; target-specific PBQP/PFCCO cost patterns | Multi-arg/call chain с live-through A/X/Y/ZP проверяется на native bytes |
| P1 | Bare-metal CLI target и assembler path после capability gates | Source→MIR2→6502→binary→emulator для поддержанного subset; unsupported language cases diagnostic |
| P2 | u32/u64 lowering через проверенные byte helpers; mul/div library | Boundary/random differential checks, byte-level contracts и exact width semantics |
| P2 | 6502 checked-table schema/admission; offline corrected GPU shape search | Certificates отделяют abstract shape/placement/flags/ABI; production без GPU/Z3 |
| P3 | C64 takeover/KERNAL-compatible profiles, PRG loader, real system emulator | CHROUT/banking/IRQ/port/ROM ZP contracts проверены в VICE или equivalent |
| P3 | Page-aware timing, richer patterns, SMC/patch admission | Benchmark на конкретной platform profile с actual assembled layout |

Небольшие полезные optimizations до большого GPU run:

- `a+a → ASL A`: текущая `CLC; STA scratch; ADC scratch` — 5 bytes/8 cycles, ASL — 1 byte/2 cycles. Results/C/Z/N совпадают для binary doubling, но **V differs**; допустимо только если V dead или явно учтён.
- `JSR leaf; RTS → JMP leaf`: 6 bytes/12 cycles до callee против 3 bytes/3 cycles; проверить stack-observable context и ABI.
- `DEC zp; BNE` / `DEX; BNE` без лишнего LDA/compare, если flags lifetime допускает; taken branch имеет 3 cycles, crossing 4.
- `LDY zp` непосредственно вместо transfer via A для index materialization; сохраняет A.
- Native u16 ADC/SBC chains и `(zp),Y` loads/stores дадут более полезное language coverage, чем преждевременная миллиардная table enumeration.

Для оценки качества backend нужен held-out source corpus: byte/word arithmetic, memcpy/fill/indexed loads, string loops, multiple calls, recursion/IRQ contracts, u32 hashing primitives. Сравнивать actual assembled bytes, base/conditional cycles, ZP consumption и корректность с cc65/LLVM-MOS на одинаковом platform/ABI. Здесь таких сравнений не проводилось; «оптимальнее cc65» и «полный 6510» результатами исследования не являются.

## 8. Воспроизведение и сохранённые артефакты

Из MinZ `minzc/`:

```bash
go test ./pkg/mir2 -run '6502|M6502' -v -count=1
go test ./pkg/mir2 -run 'Showcase_' -v -count=1
go run ../reports/research-6502-gpu/probes/minz_probe.go
go run ../reports/research-6502-gpu/probes/native_arithmetic.go
```

Во внешнем `z80-optimizer/`:

```bash
GOTOOLCHAIN=go1.26.8 go test ./pkg/gpugen ./pkg/regalloc -count=1
GOTOOLCHAIN=go1.26.8 go run ./cmd/gpugen -isa 6502 -backend cuda > mos6502_mulopt.cu
nvcc -O3 -c mos6502_mulopt.cu -o mos6502_mulopt.o
# Скопировать probes/mos6502_cuda_smoke.cu рядом с generated kernel:
nvcc -O3 mos6502_cuda_smoke.cu -o mos6502_cuda_smoke
./mos6502_cuda_smoke
```

Во внешнем `6502-optimizer/`:

```bash
GOTOOLCHAIN=go1.26.8 go test ./... -count=1
nvcc -std=c++14 -O2 -o 6502search_v2 cuda/6502_search_v2.cu
./6502search_v2 --max-target 2 --first-op-start 2 --first-op-end 3 > slice.jsonl
GOTOOLCHAIN=go1.26.8 go run ./cmd/6502opt verify-jsonl slice.jsonl
```

GPUFORCE server: build `cuda/z80_regalloc.cu` с nvcc, затем `python3 probes/gpu_regalloc_probe.py /absolute/path/z80_regalloc`. ENRT metadata: `python3 probes/table_headers.py /absolute/path/z80-optimizer/data`.

Локальное research workspace: `/home/alice/dev/minz-artifacts/research-6502-gpu/`. Отчёт, логи, исходные probes, generated CUDA, external source snapshots, compressed table artifacts/reference snapshots и checksum manifest сохранены также в `/mnt/safe/minz/grace/2026-10-14/research-6502-gpu/`. [manifest.json](evidence/manifest.json) фиксирует inputs/tool versions; [SHA256SUMS](evidence/SHA256SUMS) фиксирует evidence и external archive hashes. Ветка содержит исследование; PR/merge в main и изменения действующей Z80 demo/DSL задачи не выполнялись.
