# Отчёт по спринту: сессия 2026-04-06/07

**Дата:** 2026-04-07
**Сессия:** Claude Opus в `~/dev/minz`, координация с codex в `~/dev/minz-vir`
**Режим:** bounded-task delegation (задачи приходят от minz-vir, фиксы не делаем)

---

## Что было сделано

### 1. FatFS Precision Blockers — аудит

**Задача:** Найти минимальный набор блокеров, мешающих надёжной работе FatFS на Z80.

**Метод:** Статический аудит кода + прогон существующих Go-тестов (0.5с–1.5с, без тяжёлой компиляции). Полная компиляция ff.c не запускалась — использованы готовые `TestFatFS_*`.

**Результат:** 4 блокера, отчёт в `reports/2026-04-06-Claude-FatFS-Precision-Blockers.md`.

| # | Блокер | Критичность | Суть |
|---|--------|-------------|------|
| 1 | IX half-reg misallocation | 🔴 | `st_word` генерирует `LD (HL), IXL` — невалидная инструкция Z80. MIR2 codegen не имеет forbidden-правил, которые уже есть в LIR (`rules.go:103-108`). |
| 2 | `&local_var` на Z80 | 🔴 | 169 мест в ff.c. `OpAddrOf` работает для глобалов (есть label), но для локалов нет механизма address-taken → фиксированный $F0xx адрес с label. MIR2 VM справляется, Z80 — нет. |
| 3 | u8 truncation в QBE | 🟡 | `sfn_checksum` переполняется в QBE-native (нет `& 0xFF` масок). Ломает QBE как оракул корректности. Z80 не затронут (8-бит арифметика естественно обрезает). |
| 4 | Regalloc при 3+ live vars | 🟡 | `st_word` = 46 инструкций (SDCC: 4 байта). Патологические паттерны: `AND D` с самим собой, 16-бит сдвиг на 8 вместо `LD L,H`. Латентный риск корректности для `clst2sect`. |

**Ключевой вывод:** MIR2 VM-путь полностью рабочий (все тесты зелёные). Z80-native заблокирован #1 и #2.

---

### 2. ObjC Snippet Editorial Review

**Задача:** Проверить `examples/objc/simple.m` + `simple.a80` — качество asm, корректность, ABI-комментарии.

**Результат:** Отчёт в `reports/2026-04-06-Claude-ObjC-Snippet-Editorial-Review.md`. Сниппет **не годится как витрина**.

| Функция | Проблема |
|---------|----------|
| `Box_get` | 2 мёртвые инструкции `LD D,H / LD E,L` — копия HL→DE без потребителя. 7 ops вместо оптимальных 5 (+40%). |
| `Box_addN` | 4 инструкции no-op: `LD B,H / LD C,L / LD H,B / LD L,C` — кольцевая копия HL→BC→HL. Артефакт save-before-overwrite, не убран peephole. 10 ops вместо 6. |
| `identity` | **Баг:** `LD A, L` обрезает u16→u8, возвращает в A вместо HL. При ABI `ret=HL` функция должна быть просто `RET`. |
| `__objc_test_2` | **Баг:** `INC HL` после работы с DE — HL не инициализирован. `LD H, (HL)` читает мусор. На реальном Z80 даст неверный результат. |
| ABI-заголовок | Противоречия: заголовок `params=[BC]` для Box_get, а per-function comment и код используют HL. |

**Рекомендация:** Заменить на более сильные примеры (`abs_diff`: 4 байта, `next_light`: 12 байтов — оба Z3-оптимальны и уже в `Eight_Languages_One_Binary.md`). Или явно пометить как "pre-peephole VIR output".

---

### 3. Чтение и анализ plan/seed файлов

Прочитаны 10 seed-файлов из `minz/contexts/` и `minz-vir/contexts/`. Основные выводы:

**Двухнедельный план** (в `minz-vir`):
- Неделя 1: аудит блокеров → фикс самого ценного → проверка Tetris
- Неделя 2: второй FatFS-блокер → cross-check fun/ → документация → хэндофф

**Tetris CP/M** (seed 2026-04-05): рендеринг сломан. Подозрение на `piece_dx/piece_dy` или арифметику `DRAW_X + cx*2`. Подход — probe-копии, доказать один слой, потом портировать. Есть незакоммиченный патч MZE BDOS input в `minz`.

**Pointer Threading** (3 делегации, 2026-04-03): от локального CSE через structural rewrite к настоящему loop-carried pointer state через block args. Прогрессия: shape_facts → pointer_walk CSE → true threading → iteration 2 (count-up + width-aware stride).

**18-Register Model** (решено 2026-04-01): 7 GPR + 4 IX halves (LocCost=1) + 6 shadow через EXX-zone. IXH/IXL = zero-cost мост между зонами. GPU-таблицы feasible при 11 локациях.

---

## Координация

| Событие | Время |
|---------|-------|
| `dedelulu explore` — обнаружены 6 сессий | начало |
| Приветственное сообщение → `hz8hkfc8:main` | начало |
| "Hold" от codex | получено |
| FatFS audit task → выполнен, `DONE` отправлен | ~15 мин |
| Feedback: "be cautious with FatFS" → сохранён в memory | получено |
| "Reviewed. Useful. Hold." от codex | получено |
| ObjC editorial review task → выполнен, `DONE` отправлен | ~20 мин |
| "Pause until ObjC cleanup done" от codex | получено |
| Чтение seed-файлов по просьбе Alice | вручную |

Все задачи выполнены в bounded-режиме: только аудит и анализ, никаких правок кода.

---

## Что не было сделано (и правильно)

- Никаких фиксов — codex явно сказал "do not implement fixes yet"
- Полная компиляция FatFS не запускалась — использованы лёгкие Go-тесты (<2с)
- Никакой работы по pointer threading, Tetris, или regalloc — всё за пределами scope

---

## Открытые вопросы для следующей сессии

1. **MZE BDOS input patch** — лежит незакоммиченным в `minz/minzc/cmd/mze/main.go`. Коммитить?
2. **IX half-reg fix** — codex обещал "bounded implementation seed". Ждём.
3. **ObjC snippet cleanup** — codex ведёт в minz-vir. Результат пока не получен.
4. **Tetris probe workflow** — seed готов, но работа не начата (паузирована codex).

---

*Сессия прошла чисто: 2 bounded-отчёта, 0 сломанных вещей, полная координация через dedelulu.*
