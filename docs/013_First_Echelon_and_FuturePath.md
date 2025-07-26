# 013_First_Echelon_and_FuturePath.md
**Version:** 0.2.2  
**Date:** July 26, 2025  
**Status:** Proposal → Adopt

---

## Purpose

Этот документ фиксирует **первый эшелон улучшений** MinZ (feature set с максимальным приростом эргономики при нулевых/около-нулевых накладных), а также **сводный FuturePath**, синхронизированный с:
- `SPEC v0.1` (SMC Anchors, ABI, Frames, Iterators),
- `ADR-001` (Истинный SMC через «якоря» в теле функции — дефолтный ABI для RAM),
- `012_Future_Roadmap.md` (стратегическая карта).

Фокус: **anti-C / Z80-native** — иммедиаты, DJNZ, таблицы переходов, статическая трансляция, SMC-якоря внутри функции.

---

## Design Principles (recap)

1. **Zero-/near-zero cost:** любой сахар обязан сворачиваться в эффективные паттерны Z80.
2. **Compile-time first:** всё, что можно, решается на компиляции; рантайм — минимален.
3. **SMC-friendly:** ничего не должно ломать или ухудшать истинный SMC с якорями.
4. **Z80-нативные циклы:** `DJNZ` как канон для обратных диапазонов.
5. **Простые ошибки:** без исключений; предпочтительно через флаги/регистры.

---

## Scope (First Echelon)

Фичи, которые **радикально улучшают эргономику**, не требуют рантайма/метапрога и естественно ложатся на SMC/DJNZ:

1. **Именованные параметры** (Ruby/Crystal-style)  
2. **`match` по константам/тегам** (Elixir/Crystal-вкус)  
3. **`defer` / `ensure`** (Go+Ruby)  
4. **Диапазоны / итераторы → DJNZ** (`for i in N..0`)  
5. **Ссылки на счётчик `&i`** c материализацией слота (осознанная деоптимизация)  
6. **Carry-flag error ABI + `with`** (Elixir-style ранний выход, Z80-native)  
7. **`smc_guard`** (авто-snapshot/restore якорей: SMC undo-log)  
8. **`smc_bind`** (частичная специализация якорей; «замыкание без рантайма»)  
9. **Диагностика/отчёты** (`-report-abi`, `-report-smc-anchors`, `PATCH-TABLE.json`)

---

## Normative Summary (MUST/SHOULD)

### 1) Именованные параметры
- **SHOULD:** поддерживаются на уровне синтаксиса как чистый сахар.  
- **Lowering:** перестановка в позиционные; патчатся SMC-якоря `p$imm0`.  
- **Errors:** дубликаты/пропуски — compile error.

### 2) `match` (константы/теги)
- **MUST:** плотные диапазоны → jump-table (`JP (HL)`), разрежённые → `CP`/`JR Z`.  
- **MUST NOT:** fallthrough; только константы (без сложного распаковочного матчинга).

### 3) `defer` / `ensure`
- **MUST:** статическая вставка кода перед каждым `RET`/выходом из блока/функции.  
- **MUST NOT:** скрытые вызовы/стек-фреймы.

### 4) Диапазоны / итераторы → DJNZ
- **MUST:** `for i in N..0 {}` при константном `N` → `LD B, N+1; ...; DJNZ`.  
- **SHOULD:** `.rev()` и другие обратные формы сводятся к `DJNZ` при возможности.

### 5) `&i` (ref на счётчик)
- **MUST:** при использовании `&i` — **материализовать** `i` в адресуемом слоте; либо деоптимизировать, либо добавить write-back (`i=B-1`).  
- **MUST:** срок жизни ссылки — лексическая область; escape запрещён.  
- **SHOULD:** компилятор выдаёт предупреждение о деоптимизации DJNZ.

### 6) Carry-flag error ABI + `with`
- **MUST:** принять гибридный ABI ошибок:
  - `u8`: `CY=0` → OK, `A=result`; `CY=1` → ERR, `A=err_code`  
  - `u16/ptr`: `CY=0` → OK, `HL=result`; `CY=1` → ERR, `A=err_code` *(или договорной `HL`)*
- **SHOULD:** `with` понижается в проверку `CY` (`JR C, ...`/`JP C, ...`) без лишней логики.
- **MUST NOT:** heap/tuple на рантайме для ошибок.

### 7) `smc_guard` (SMC undo)
- **MUST:** вход — начать undo-лог патчей якорей текущего блока; выход — откатить.  
- **SHOULD:** для `imm16` запись под `DI/EI`; поддержка вложенных guard.  
- **MUST:** совместимость с банками через временный `using bank[n]` при патче.

### 8) `smc_bind` (частичная специализация)
- **SHOULD:** зашивать выбранные якоря **раз и навсегда**; при вызове патчить только незашитые.  
- **MUST:** совместимость с `smc_guard`; зашитые значения неизменяемы.  
- **MUST NOT:** скрытый heap/closure.

### 9) Диагностика/отчёты
- **MUST:** `-report-abi` (обоснование выбора smc/slot/reg per call-site).  
- **MUST:** `-report-smc-anchors` (адреса якорей, повторные загрузки из якорей, undo-операции).  
- **MUST:** `PATCH-TABLE.json`, `MEMORY-MAP.json` (черновые схемы).  
- **SHOULD:** аннотированный `.a80` с комментариями: `; x$imm0 %IMM8_PATCH`, `; reuse: LD A,(x$imm0)`.

---

## Lowering Details & Examples

### Carry-`with` пример
```minz
fn read_u8() -> {ok, u8} | {error, u8};

with {ok, a} <- read_u8(),
     {ok, b} <- read_u8(),
do {
  use(a, b);
}
```

**Lowering (схематично):**
```
CALL read_u8
JR   C, .err1         ; CY=1 → error
; A=a
CALL read_u8
JR   C, .err2
; A=b
; use(a,b)
JP .done
.err2: ; errpath
.err1: ; errpath
.done:
```

### DJNZ и `&i`
```minz
for i in 10..0 {
  let r: ref u8 = &i;  // материализация
  touch(r);
}
```
**Lowering:**  
- `LD B, 11`  
- В теле: `LD A, B` / `DEC A` / `LD (i_slot), A` (или экв.)  
- `DJNZ`

### `smc_bind`
```minz
@abi(smc)
fn plot(x: u8, y: u8, col: u8) -> void { ... }

let plot_white = smc_bind plot(col=7);  // зашит col$imm0=7
plot_white(10, 20);                     // патчатся только якоря x/y
```

---

## Tooling (Reports & JSON Draft)

- **`-report-abi`**: CSV/JSON со столбцами: `site`, `target_fn`, `abi`, `reason`, `est_gain_tstates`.
- **`PATCH-TABLE.json`** (draft):  
```json
{
  "functions": [{
    "name": "draw_point",
    "anchors": [
      {"symbol":"x$imm0","addr":33024,"size":1,"bank":0},
      {"symbol":"y$imm0","addr":33027,"size":1,"bank":0},
      {"symbol":"color$imm0","addr":33030,"size":1,"bank":0}
    ]
  }]
}
```
- **`MEMORY-MAP.json`** (draft): глобалы, слоты, банки, размеры.
- Аннотированный `.a80`: комментарии с `%IMM8_PATCH`, `%SMC_SAVE/RESTORE`, `%DEFER`.

---

## Alignment with SPEC / ADR / 012 Roadmap

- **SPEC v0.1:** полностью совместим: SMC-якоря внутри функций, DJNZ, `&i` материализация, SMC-undo, ABI выбор.  
- **ADR-001:** соответствует принятому решению: **TRUE SMC** уже **adopted**, не «future design».  
- **012_Future_Roadmap.md:**  
  - Carry-flag ошибки — **согласовано и конкретизировано** (ABI).  
  - TRUE SMC переведён из «design docs» в «реализация якорей/undo/отчётов».  
  - Добавлен **First Echelon** как ближайшие фичи, не требующие рантайма.

---

## Plan of Record (Phases)

```mermaid
flowchart LR
  P02[Phase 0.2<br/>Named args, match,<br/>defer, DJNZ, &i, reports] --> P03[Phase 0.3<br/>Carry ABI + with,<br/>smc_guard, smc_bind,<br/>annotated .a80]
  P03 --> P04[Phase 0.4<br/>TRUE SMC anchors impl,<br/>PATCH-TABLE stabilize]
  P04 --> P05[Phase 0.5<br/>Function pointers (min),<br/>smc_bind 2.0 for fn-ptr]
  P05 --> P06[Phase 0.6<br/>Tooling polish,<br/>source-maps, warnings]
  P06 --> P07[Phase 0.7<br/>Concurrency Lite,<br/>channels (ring), select]
  P07 --> P10[Phase 1.0<br/>MinZ-native meta,<br/>const-eval, specialization]
```

*(в диаграммах перенос строки — `<br/>`)*

---

## Acceptance Criteria (DoD)

- **Named args:** перестановка без аллокаций; ошибки на дубли/пропуски.  
- **match:** корректная генерация jump-table/цепочек; без fallthrough.  
- **defer:** вставка по всем путям; отсутствуют лишние прыжки.  
- **DJNZ:** тождественный T-states ручному циклу; `&i` → варнинг + корректная материализация.  
- **Carry ABI + with:** ветвления по флагу; эквивалент ручному коду.  
- **smc_guard:** корректный undo `imm8/imm16` (с `DI/EI`), вложенность работает.  
- **smc_bind:** зашитые якоря неизменяемы; патчатся только незашитые; совместимость с guard.  
- **Reports:** генерируются `-report-abi` и `-report-smc-anchors`; `PATCH-TABLE.json` валиден.

---

## Open Questions

1. Допустить ли режим **нескольких якорей** на параметр в сильно ветвистых CFG (vs. один синтетический)?  
2. Для `u16` ошибок: класть ли код ошибки в `A` всегда, либо допустить модульный вариант `HL=err_ptr`?  
3. Включать ли `-Witer-ref-deopt` как warning по умолчанию?

---

## Change Log

- **0.2.2 (this doc):** формализация First Echelon, Carry-ABI, план фаз и согласование с SPEC/ADR/012.
