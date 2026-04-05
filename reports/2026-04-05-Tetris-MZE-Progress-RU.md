# Tetris on MZE: Progress Report

Дата: 2026-04-05

## Кратко

За эту сессию мы довели расследование до полезной архитектурной развилки:

- `tetris_cpm.nanz` больше не выглядит как "полностью мёртвая программа";
- часть критичных source-shape багов уже обойдена;
- подтверждено, что один из главных blockers сидел не в Tetris, а в `mze` CP/M input semantics;
- после локального патча `mze` игра уже доходит до keyboard input path;
- оставшийся основной баг теперь локализован в render/geometry layer, а не в TUI core.

## Что было исправлено в `minz-vir`

### 1. TUI input path

В [`stdlib/tui/render.nanz`](/home/alice/dev/minz-vir/stdlib/tui/render.nanz):

- `tui_read_key()` переведён на blocking path;
- добавлен decode VT100 arrows `ESC [ A/B/C/D`;
- расширены `clob all` для CP/M BDOS asm blocks, чтобы убрать часть скрытых clobber-ошибок.

Это убрало ранний хаос в input semantics и сделало CP/M TUI path заметно честнее.

### 2. Source-level hardening для `tetris_cpm`

В [`examples/nanz/tetris_cpm.nanz`](/home/alice/dev/minz-vir/examples/nanz/tetris_cpm.nanz) были сделаны source-side workarounds против токсичных PBQP/codegen shapes:

- `board_get/board_set` переписаны в pointer-walk form;
- `piece_dx/piece_dy` временно были переведены в safer shapes в отдельных probe-ветках;
- `can_place()` и `lock_piece()` развернуты без 4-итерационных loops;
- board init переписан как pointer walk;
- `draw_border()` временно отключён;
- `draw_info()` срезан до минимального useful path;
- render path был последовательно упрощён для narrowing.

### 3. Native `@screen` direction documented

Добавлен отдельный proposal:

- [`reports/2026-04-04-Native-Screen-DSL-Proposal-RU.md`](/home/alice/dev/minz-vir/reports/2026-04-04-Native-Screen-DSL-Proposal-RU.md)

Главная идея:

- `screen_gen.go` считать bootstrap hack;
- в будущем заменить его нативным meta-DSL / block-IR / builder path.

### 4. Claude task template added

Добавлен reusable task template:

- [`contexts/2026-04-05-claude-task-template.md`](/home/alice/dev/minz-vir/contexts/2026-04-05-claude-task-template.md)

Он нужен, чтобы Claude получал узкие bounded tasks с проверяемыми done criteria.

## Что было подтверждено в `minz` / `mze`

Локально в соседнем репо [`/home/alice/dev/minz`](/home/alice/dev/minz) был сделан и проверен compile-safe patch в:

- [`/home/alice/dev/minz/minzc/cmd/mze/main.go`](/home/alice/dev/minz/minzc/cmd/mze/main.go)

### Root cause

`mze` CP/M BDOS handler делал это:

- `BDOS 01` на EOF/pipe подставлял fake `CR`;
- `BDOS 0B` терял символ при status-check.

Из-за этого `tetris_cpm` под `mze` free-run'ил даже без реального input.

### Локальный патч

Сделано:

- `BDOS 01` больше не synthesizes fake `CR` на EOF;
- добавлен `pendingInput` buffer;
- `BDOS 0B` больше не теряет символ;
- `go test ./cmd/mze` зелёный;
- `go run ./cmd/mze --help` зелёный.

Это пока не закоммичено в `minz`, но локально проверено.

## Самое важное narrowing

### Что уже не виновато

1. Не `tui_goto`

Hardcoded probe с прямыми `tui_goto(12,1)`, `tui_goto(14,1)`, `tui_goto(16,1)`, `tui_goto(18,1)` рисует правильно.

Вывод:

- TUI coordinate path жив;
- базовая VT100 emit logic жива.

2. Не общий `mze` input path как единственный blocker

После локального `mze` patch'а игра уже доходит до `BDOS 01`.

Значит:

- endless redraw больше не объясняется только input semantics;
- следующий remaining bug лежит выше.

### Что осталось подозреваемым

Основной оставшийся подозреваемый слой:

- `piece_dx/piece_dy`
- и/или arithmetic/call shape вокруг `draw_cell(...)`

Прямой safe-form rewrite `piece_dx/piece_dy` в repo оказался слишком агрессивным:

- он снова провоцировал immediate `GAME OVER`;
- значит этот слой нельзя чинить "в лоб" большим переписыванием без дополнительных probes.

## Что удалось доказать end-to-end

Самый полезный факт:

- patched local `mze` + simplified Tetris probes уже могут завершаться cleanly через scripted input (`qq` path).

Это значит:

- runtime path ожил;
- debugging теперь идёт не вслепую.

## Что пока не готово

Пока ещё не достигнуто:

- нормально играемый `tetris_cpm` под `mze`;
- корректный piece render;
- возврат board render и полноценного HUD;
- чистый committed patch в `minz` для `mze` input fix.

## Следующий правильный шаг

Не делать больших переписываний в repo сразу.

Надо идти так:

1. продолжать через временные probe files;
2. отдельно изолировать:
   - `draw_cell` call-shape
   - `piece_dx/piece_dy` path
   - arithmetic `DRAW_X + cx * 2`;
3. только после этого вносить один подтверждённый source fix в `tetris_cpm`;
4. отдельно закоммитить `mze` BDOS input fix в `minz`.

## Verdict

Сессия дала не "готовый тетрис", а более ценную вещь:

- мы убрали ложные объяснения;
- отделили emulator bug от source/codegen bug;
- доказали, что TUI base жив;
- и сузили remaining problem до маленького geometry/render layer.

Это хороший stopping point для следующей сессии.
