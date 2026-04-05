# Session Wisdom: 2026-04-05

## Tetris / MZE

- Не путать два класса проблем:
  - emulator-side CP/M input semantics;
  - source/codegen render correctness.
- `mze` CP/M input был реальным blocker:
  - `BDOS 01` synthesizes fake `CR` on EOF;
  - `BDOS 0B` loses char.
- После локального patch'а `mze` игра уже может доходить до `BDOS 01`, значит следующий баг уже не в input core.

## What is already known-good

- `tui_goto` path жив:
  - hardcoded `tui_goto(12,1)`, `(14,1)`, `(16,1)`, `(18,1)` рисуют корректно.
- `draw_border()` лучше пока держать выключенным.
- `draw_info()` стоит держать минимальным.
- pointer-walk `board_get/board_set` and unrolled `can_place/lock_piece` были полезными workaround-формами.

## What is still dangerous

- Большие rewrite'ы `piece_dx/piece_dy` прямо в repo слишком рискованны.
- Nested render loops легко уезжают в broken PBQP/codegen shapes.
- Даже "честные" direct-form rewrites geometry lookup могут снова сломать `spawn_piece/can_place`.

## Best workflow from here

- Работать через temporary probe files first.
- В repo вносить только подтверждённые minimal fixes.
- Сначала prove:
  - `draw_cell` safe or not
  - `piece_dx/piece_dy` safe or not
  - `DRAW_X + cx * 2` safe or not
- Только потом patch main source.

## Claude usage

- Claude полезен как bounded diagnosis worker.
- Лучший format:
  - one narrow question
  - no code changes unless explicitly requested
  - one short conclusion / report
- Не давать mixed tasks.

See template:

- [`contexts/2026-04-05-claude-task-template.md`](/home/alice/dev/minz-vir/contexts/2026-04-05-claude-task-template.md)
