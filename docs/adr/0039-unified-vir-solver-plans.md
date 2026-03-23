# ADR-0039 Appendix: Implementation Plans

## Plan A: С НУЛЯ (Clean-room unified solver)

### Подход
Новый пакет `pkg/vir/` с чистым solver. Текущий `pkg/lir/` остаётся как fallback. Переключение через `--solver=unified` флаг.

### Этапы

| # | Что | LOC | Дни | Риск |
|---|-----|-----|-----|------|
| A1 | `VIROp` тип + bridge MIR→VIR | ~200 | 1 | Низкий — копия MIROp с чистым API |
| A2 | `PIROp` тип + trivial emit (template → asm) | ~150 | 1 | Низкий |
| A3 | `MachineDesc` v2 — patterns с полным LocSet (L1-L7) | ~300 | 2 | Средний — нужно правильно закодировать ВСЕ Z80 constraints |
| A4 | WFC solver на VIROps (isel+regalloc одновременно) | ~500 | 3 | **Высокий** — ядро, нужны правильные constraints |
| A5 | Move insertion (solver генерирует spill/reload moves) | ~200 | 2 | Средний |
| A6 | Z3 fallback (WFC fails → SMT solve) | ~300 | 2 | Низкий — Z3 encoding уже написан |
| A7 | Wire в pipeline + корпус-тест | ~100 | 2 | Средний — integration |
| A8 | TSMC tunnel analysis + L4/L5 tiers | ~200 | 2 | Средний |
| A9 | Убить старый pipeline (isel+wfc+fixups) | ~0 (delete) | 1 | **Высокий** — only if corpus 100% |

**Итого: ~1950 LOC, ~16 дней, 3-4 недели реально**

### Плюсы
- Чистая архитектура с первого дня
- Нет legacy debt
- Тестируемо в изоляции (VIROp → PIROp юнит-тесты)
- Можно параллельно с текущим pipeline (--solver=unified)

### Минусы
- **3-4 недели без улучшений** в production pipeline
- Текущие 82/87 файлов могут регрессировать если переключиться рано
- Риск: unified solver может быть хуже текущего на edge cases которые мы уже пофиксили
- Нужно заново находить и фиксить все Z80 encoding constraints
- A3 (MachineDesc v2) — самый рискованный шаг, неправильные constraints = все баги заново

### Контрольные точки
- После A4: `double16(x) = ADD HL,HL / RET` через unified solver?
- После A5: `test(a,b) { call f(a); ret b }` с правильным PUSH/POP?
- После A7: corpus >= 82/87?

---

## Plan B: ИТЕРАТИВНЫЙ (Incremental refactor)

### Подход
Убираем text fixups один за одним, переводя каждый в constraint. Текущий pipeline всегда рабочий. Каждый шаг — отдельный commit, тестируемый на корпусе.

### Этапы

| # | Что | LOC | Дни | Риск |
|---|-----|-----|-----|------|
| B1 | Убрать `fixInvalidZ80Template` — правила → pattern constraints | ~-100/+50 | 2 | **Низкий** — уже начали (isLD guard) |
| B2 | Убрать `spill_reload.go` — правила → LocSet constraints в WFC | ~-100/+80 | 2 | Средний |
| B3 | Убрать `validate-reject` loop — constraints достаточны | ~-50 | 1 | Средний — нужно быть уверенным |
| B4 | Убрать `selectTemplateForPhys` — solver выбирает pattern + reg | ~-80/+100 | 2 | Средний |
| B5 | Merge isel в WFC — WFC получает VIROps, не Insts | ~-200/+300 | 3 | **Высокий** — ядро изменение |
| B6 | Z3 fallback для противоречий | ~+100 | 1 | Низкий |
| B7 | Move insertion в solver (не post-hoc) | ~+200 | 2 | Средний |
| B8 | TSMC tiers (ADR-0038) | ~+150 | 2 | Средний |
| B9 | Rename MIROp → VIROp, Inst → PIROp | ~+50 (sed) | 0.5 | Низкий |

**Итого: ~-530/+1030 LOC (net +500), ~15.5 дней, 3 недели реально**

### Плюсы
- **Pipeline всегда рабочий** — каждый шаг тестируется на корпусе
- **Immediately visible** — каждый удалённый fixup = одним edge case меньше
- Низкий риск на каждом шаге
- B1-B3 можно сделать за неделю и получить measurable improvement
- Не нужно переписывать Z80 constraints заново — они уже в pattern table
- **Каждый шаг можно остановить** — pipeline остаётся рабочим

### Минусы
- Результат может быть "почти unified" но с legacy шрамами
- WFC might not be good enough even with all constraints (need Z3)
- B5 (merge isel в WFC) — переломный момент, может потребовать значительный рефакторинг
- Переименование (B9) может сломать imports в других пакетах

### Контрольные точки
- После B1: `fixInvalidZ80Template` удалён, corpus ≥ 82/87
- После B3: нет text fixups, нет validate-reject, corpus ≥ 82/87
- После B5: isel+WFC unified, corpus ≥ 82/87
- После B7: spill moves генерируются solver'ом

---

## Сравнение

| Аспект | Plan A (с нуля) | Plan B (итеративный) |
|--------|----------------|---------------------|
| Время до первого результата | 2 недели | 2 дня |
| Время до полного unified solver | 3-4 недели | 3 недели |
| Риск regression | Высокий (при переключении) | Низкий (каждый шаг тестируется) |
| Чистота архитектуры | Идеальная | Хорошая (legacy шрамы) |
| Параллельная работа | Да (два pipeline) | Нет (один pipeline) |
| Можно остановить на полпути | Да (fallback на старый) | Да (каждый шаг самодостаточен) |
| LOC total | ~1950 new | ~500 net change |
| Нужно ли заново фиксить constraints | Да, все с нуля | Нет, инкрементально |

## Рекомендация

**Plan B** — итеративный. Причины:

1. **Каждый шаг даёт measurable improvement** (удалённый fixup = удалённый edge case source)
2. **Pipeline никогда не ломается** (корпус тестируется после каждого шага)
3. **B1-B3 за неделю** — быстрый результат, снижает bug count
4. **Если B5 окажется слишком сложным** — можно остановиться после B4 и всё равно иметь лучший pipeline
5. **Z80 constraints уже закодированы** в pattern table — не нужно переписывать

Plan A лучше ТОЛЬКО если текущий код настолько запутан что инкрементальный рефакторинг невозможен. Но мы уже показали что можем инкрементально фиксить (isLD guard, INC filter, 16-bit CMP) — значит код поддаётся.

**Начинать с B1** — убрать `fixInvalidZ80Template`, перенести правила в constraints.
