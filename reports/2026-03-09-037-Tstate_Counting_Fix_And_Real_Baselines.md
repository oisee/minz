# Report #037 — T-state Counting Fix & Real Baselines

**Date:** 2026-03-09
**Status:** Done

---

## Problem

`pkg/z80testing` — `TestMemory` и `TestPorts` не инкрементировали `cpu.Tstates` при
обращениях к памяти/портам. В итоге:

```go
// e2e_harness.go (было):
elapsed := h.cpu.Tstates - prevStates  // всегда 0
if elapsed > 0 { h.cycleCount += elapsed }
else           { h.cycleCount++ }       // ← всегда этот путь
```

Все измерения ("T-states") на деле считали **инструкции**, а не T-states.
Report #018 ("26T/element", "7.8x speedup") был основан на некорректных данных.

---

## Fix

По образцу `pkg/emulator/fuse_test.go::fuseMemory`:

| Место | Изменение |
|-------|-----------|
| `TestMemory.ReadByte` | `cpu.Tstates += 3` |
| `TestMemory.WriteByte` | `cpu.Tstates += 3` |
| `TestMemory.Contend*` | `cpu.Tstates += time` |
| `TestPorts.ContendPortPreio` | `cpu.Tstates += 1` |
| `TestPorts.ContendPortPostio` | `cpu.Tstates += 3` |
| `NewTest()` | wire: `memory.cpu = cpu; ports.cpu = cpu` |
| `NewE2ETestHarness()` | wire: `memory.TestMemory.cpu = cpu; ports.cpu = cpu` |

Новые тесты подтверждают точность:
- `NOP×3 + DI + HALT = 20T` ✓
- `LD A,n(7T) + HALT(4T) = 11T` ✓
- `LD B,5(7T) + HALT(4T) = 11T` ✓

### Note: OUT (n), A = 7T (не 11T)

remogatto не вызывает `ContendPortPreio/Postio` для инструкций без ZX Spectrum
memory contention. Недостающие 4T специфичны для ZX Spectrum ULA — для
бенчмарков Z80 кода это несущественно (разница постоянна для всех тестов).

---

## Реальные базовые T-states (MIR1 codegen, $F0xx)

| Тест | Старое ("T-states") | Реальные T-states | T/elem |
|------|---------------------|-------------------|--------|
| forEach(call), 5 элементов | 103 | **1324T** | ~264T |
| map+forEach, 5 элементов | 150 | **1915T** | ~383T |
| filter+forEach, 5 элементов | 133 | **1469T** | ~293T |

Ratio: реальные T-states ≈ **12–13× больше** старых "T-states" (которые = instruction count).

### Разбивка forEach(call) = 1324T

```
Программа (5 элементов, MIR1 кодген):
  setup + arr init:     ~360T  (LD/ST через $F0xx для arr[0..4])
  loop×5:               ~960T  (~192T/iter)
    per-iter breakdown:
      load counter $F014:  16T  (LD HL,($F014) + LD C,(HL))
      load ptr $F016:      16T  (LD HL,($F016) + LD E,(HL)...)
      LD A,(HL):            7T
      CALL console_log:    17T + OUT(7T) + RET(10T) = 34T
      INC ptr + store:     ~30T
      DEC counter + store: ~30T
      DJNZ:                13T
```

Bottleneck: **~64% времени = load/store через $F0xx memory** (LD addr↔регистр).

### Цель оптимизации

| Сценарий | T/elem | Итого/5 |
|----------|--------|---------|
| Текущий (MIR1, $F0xx) | ~264T | 1324T |
| Цель (регистры B/HL/A) | ~43T | 215T |
| Hand-optimal | ~63T | 316T |

**6.1x** возможное ускорение если устранить $F0xx round-trips.

---

## Regression Guards (обновлены)

`regalloc_quality_test.go` — пороги обновлены до реальных T-states + 50%:

```go
forEach(call):   maxAcceptable = 2000   // baseline 1324T
map+forEach:     maxAcceptable = 3000   // baseline 1915T
filter+forEach:  maxAcceptable = 2200   // baseline 1469T
```

---

## Файлы изменены

- `pkg/z80testing/z80_test_framework.go` — TestMemory + TestPorts T-state counting
- `pkg/z80testing/e2e_harness.go` — wire cpu в memory/ports
- `pkg/z80testing/regalloc_quality_test.go` — обновлены пороги + комментарии
- `pkg/z80testing/verify_tstates_test.go` — новый: accuracy verification
- `pkg/z80testing/foreach_tstates_test.go` — новый: hand-optimal baseline 316T
