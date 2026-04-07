# План следующего спринта

**Дата:** 2026-04-07
**Основа:** [claude-two-week-plan.md](/home/alice/dev/minz-vir/contexts/claude-two-week-plan.md), seed-файлы из обоих репо

---

## Неделя 1: Блокеры + Tetris

### День 1 — Аудит и привязка к репро

Собрать все открытые блокеры в единый список с конкретными репро и критериями остановки:

- **FatFS IX half-reg** ([отчёт](/home/alice/dev/minz/reports/2026-04-06-Claude-FatFS-Precision-Blockers.md), блокер #1) — `st_word` генерирует `LD (HL), IXL`. Репро: `fatfs_lowlevel.c`, функция `st_word`.
- **FatFS `&local_var`** ([отчёт](/home/alice/dev/minz/reports/2026-04-06-Claude-FatFS-Precision-Blockers.md), блокер #2) — 169 мест в ff.c. Репро: `fill(&x, 42)`.
- **ObjC snippet quality** ([отчёт](/home/alice/dev/minz/reports/2026-04-06-Claude-ObjC-Snippet-Editorial-Review.md)) — `identity` баг, мёртвые LD в Box_get/addN.
- **Tetris rendering** ([seed 2026-04-05](/home/alice/dev/minz-vir/contexts/2026-04-05-next-session-seed.md)) — подозрение на `piece_dx/piece_dy` или арифметику.
- **MZE BDOS input patch** — незакоммичен в `minz/minzc/cmd/mze/main.go`.

### Дни 2–3 — Фикс самого ценного блокера

По решению codex — скорее всего IX half-reg misallocation (блокер #1):

- Портировать forbidden-правила из LIR (`pkg/lir/rules.go:103-108`) в MIR2 Z80 codegen
- Добавить регрессионный тест
- `go test ./pkg/vir` + мини-репро `st_word`

**Источник:** [claude-two-week-plan.md](/home/alice/dev/minz-vir/contexts/claude-two-week-plan.md), Day 2–3

### День 4 — Tetris regression check

- Пересобрать `examples/nanz/tetris_cpm.nanz`
- Прогнать через `mze`/`mzv`
- Сравнить before/after
- Написать `reports/2026-04-0X-Tetris-Runtime-Check.md`

**Источник:** [claude-two-week-plan.md](/home/alice/dev/minz-vir/contexts/claude-two-week-plan.md), Day 4; [seed 2026-04-05](/home/alice/dev/minz-vir/contexts/2026-04-05-next-session-seed.md) — probe workflow

**Подход из seed:** НЕ переписывать repo source напрямую. Создать probe-копии в `/tmp`, изолировать один слой (draw_cell → арифметика → piece LUT), доказать, потом портировать минимальный фикс.

### День 5 — Рефлексия

- Гипотеза держится? Новые блокеры?
- Обновить seed для следующей сессии

---

## Неделя 2: Второй блокер + Hardening

### Дни 6–7 — Tetris runtime hardening

- CP/M entry stub
- BDOS flow (закоммитить MZE патч если ещё не)
- `tui_putch` проверка

**Источник:** [claude-two-week-plan.md](/home/alice/dev/minz-vir/contexts/claude-two-week-plan.md), Day 6–7

### Дни 8–9 — Второй FatFS блокер

Скорее всего `&local_var` (блокер #2) или u8 truncation (блокер #3):

- Address-taken locals: пометить переменную, выделить $F0xx с label, `OpAddrOf` разрешится
- Или: вставить `& 0xFF` маски в QBE-эмиттер для u8 операций
- Регрессионный тест + `go test ./pkg/vir` + `tetris_cpm`

**Источник:** [FatFS блокеры](/home/alice/dev/minz/reports/2026-04-06-Claude-FatFS-Precision-Blockers.md), блокеры #2–3

### День 10 — Cross-check fun/ frontend

- Прогнать сэмплер из fun/ примеров
- Отметить регрессии/улучшения
- Короткий отчёт `reports/2026-04-0X-Fun-Status-Check.md`

### Дни 11–12 — Документация

- Доказанные фиксы + safety checks
- Обновить seed для следующего спринта
- Emerging blockers → перерасставить приоритеты

### Дни 13–14 — Wrap-up

- Ревью reports/, CLAUDE.md
- Убрать stale промежуточные файлы
- Handoff note: победы + открытая очередь

---

## Критерии остановки

Из [claude-two-week-plan.md](/home/alice/dev/minz-vir/contexts/claude-two-week-plan.md):

1. Каждый целевой блокер имеет регрессионный тест + pass log
2. `tetris_cpm` доходит до первого видимого кадра без краша под патченным `mze`
3. Runtime matrix зелёная или обновлена с объяснениями

---

## Контрольные точки рефлексии

| Когда | Вопрос |
|-------|--------|
| Середина недели 1 | Оригинальная гипотеза всё ещё валидна? |
| Конец недели 1 | Какие новые блокеры требуют перерасстановки? |
| Конец недели 2 | Готовы снизить внимание Claude? Что дальше? |

---

## Параллельные треки (не в этом спринте, но на радаре)

### Pointer Threading

Прогрессия из трёх delegation-файлов:

1. [Grace solver-friendly shapes](/home/alice/dev/minz-vir/contexts/2026-04-03-grace-solver-friendly-shapes-seed.md) — концептуальная карта: indexed loop→pointer walk, row-helper extraction, IX/IY counter placement
2. [First pointer-walk delegation](/home/alice/dev/minz-vir/contexts/2026-04-03-claude-pointer-walk-delegation.md) — conservative MIR2 rewrite через shape facts, `pointer_walk.go`
3. [True pointer threading](/home/alice/dev/minz-vir/contexts/2026-04-03-claude-true-pointer-threading-delegation.md) — loop-carried pointer state через block args
4. [Iteration 2](/home/alice/dev/minz-vir/contexts/2026-04-03-claude-next-pointer-threading-iteration.md) — count-up loops + width-aware byte stride (u16 → stride 2)

Статус: инфраструктура в minz-vir, ждёт следующего цикла.

### 18-Register Model

[Архитектура решена](/home/alice/dev/minz/contexts/2026-04-01-18-reg-model.md): 7 GPR + 4 IX halves (LocCost=1) + 6 shadow через EXX-zone. IXH/IXL = zero-cost мост между зонами. GPU-таблицы feasible при 11 локациях (~280M entries).

Следующий шаг: Path A — shape→enriched table lookup в VIR (1–2 недели).

### GPU Enriched Tables

Из [seed 2026-04-02](/home/alice/dev/minz/contexts/2026-04-02-next-session-seed.md):
- 4v: 156K entries (default)
- 5v: 17.4M entries (via `VIR_ENRICHED_5V`)
- 5v 11-loc: 61M entries (с IX halves)
- 6v dense: ~66M shapes (было в процессе)

### Self-Hosting Pipeline

Из [seed 2026-03-30](/home/alice/dev/minz/contexts/2026-03-30-next-session-seed.md): Stage 1+2 готовы, баги в multi-function emit и print_ast рекурсии.

---

## Зависимости от codex (minz-vir)

| Что ждём | От кого |
|----------|---------|
| Bounded implementation seed для IX half-reg fix | codex hz8hkfc8 |
| ObjC snippet cleanup результат | codex hz8hkfc8 |
| Решение по Tetris probe workflow | codex hz8hkfc8 |

---

*План основан на двухнедельном roadmap из minz-vir + результатах аудита этой сессии.*
