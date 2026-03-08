# P4: Register Allocator Fix — Plan

## Problem

**Все** функции проходят через `generateSMCFunction()`, который использует `$F0xx` memory-backed виртуальные регистры. Физический аллокатор (`Z80RegisterAllocator`) существует и работает, но его результат **игнорируется** в SMC-пути.

### Root cause chain

```
base_backend.go:64  →  fn.IsSMCEnabled = true  (для ВСЕХ функций при --enable-smc)
         ↓
z80.go:683          →  if fn.IsSMCDefault || fn.IsSMCEnabled → generateSMCFunction()
         ↓
z80.go:934          →  generateSMCInstruction() → всё через getAbsoluteAddr() → $F0xx
         ↓
z80.go:732          →  skipInst (caller-save elim) живёт ТОЛЬКО в generateFunction(), не в SMC пути
```

Физический аллокатор (`register_allocator.go`) полностью реализован: linear scan, live intervals, hint-aware, spill — но вызывается только в `generateFunction()` (z80.go:673), а до этого вызова не доходим.

## Fix Plan (4 steps, ordered by impact)

### Step 1: SMC gate — не ставить IsSMCEnabled для обычных функций

**File:** `pkg/codegen/base_backend.go:63-66`

**Сейчас:**
```go
for _, fn := range module.Functions {
    fn.IsSMCEnabled = true  // ← ВСЕМ
}
```

**Нужно:**
```go
for _, fn := range module.Functions {
    if fn.UsesTrueSMC || fn.IsSMCDefault {
        fn.IsSMCEnabled = true
    }
    // Обычные функции → IsSMCEnabled = false → идут в generateFunction()
}
```

**Результат:** Обычные функции попадают в `generateFunction()` → физический аллокатор задействован → caller-save elim работает.

**Risk:** Функции с inline asm, `@define`-based SMC, recursive context push/pop — могут сломаться. Нужен guard по `fn.RequiresContext || fn.HasInlineAsm`.

### Step 2: Подключить физ. аллокатор в SMC путь (для оставшихся SMC функций)

**File:** `pkg/codegen/z80.go`, функция `generateSMCFunction()`

В SMC пути `generateSMCInstruction()` на каждый load/store вызывает `getAbsoluteAddr()`. Нужно:

1. В начале `generateSMCFunction()` вызвать `g.physicalAlloc.AllocateFunction(fn)` (как в z80.go:673)
2. В `generateSMCInstruction()` при load/store — проверять `g.getPhysicalReg(reg)` перед fallback на `$F0xx`
3. Подключить `eliminateUnnecessaryCallerSaves()` + `skipInst` в цикл генерации (z80.go:934)

**Результат:** Даже SMC-функции используют физические регистры где возможно, `$F0xx` только для спиллов.

### Step 3: Улучшить хинты в MIR для нетривиальных паттернов

**File:** `pkg/semantic/analyzer.go` (IR generation)

Сейчас хинты есть для iterator DJNZ (B для счётчика, HL для указателя). Нужно добавить для:

- Loop counter в `for i in 0..n` → hint B
- Accumulator в арифметике → hint A
- Pointer operand → hint HL
- Secondary operand → hint DE

**Результат:** Аллокатор получает предпочтения и кладёт значения в нужные регистры с первого раза.

### Step 4: Peephole cleanup для оставшихся round-trips

**File:** `pkg/optimizer/peephole.go` или новый MIR pass

После steps 1-3 останутся паттерны типа:
```asm
LD ($F00A), HL    ; store
LD HL, ($F00A)    ; immediate reload → eliminate
```

Peephole pass убирает redundant load после store в тот же адрес.

## Verification

После каждого шага:
1. `go test ./pkg/... -vet=off` → 20/20 pass
2. 11/11 E2E iterator tests → hex-identical output
3. fibonacci/basic_functions → correct output
4. `--dump-mir` + вручную сверить a80 output: $F0xx count должен уменьшаться

## Expected Impact

| Metric | Before | After Step 1 | After Step 2 | After Steps 3-4 |
|--------|--------|-------------|-------------|-----------------|
| $F0xx round-trips per iter element | ~12 | ~3-5 | ~1-2 | 0 |
| T-states per forEach element | ~207T | ~80T | ~50T | ~26-30T |
| fibonacci total round-trips | ~24 | ~6-8 | ~2-3 | 0-1 |
