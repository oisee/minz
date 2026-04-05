# Tetris under MZE: Split of Remaining Compiler Bugs

Date: 2026-04-05

## Коротко

После расчистки `mze` input path и TUI-базы оставшийся `tetris_cpm` bug больше не выглядит как одна большая поломка.
Он распадается на два конкретных compiler/codegen класса багов и один подтверждённо безопасный shape.

Это важный поворот:

- дальше не нужно бесконечно уродовать исходник тетриса;
- нужно превращать найденные shapes в compiler regressions;
- source-side workaround имеет смысл только там, где shape уже доказан как safe.

## Что уже не является главным blocker

- `mze` CP/M input path:
  локально в sibling repo `minz` уже есть patch для `BDOS 01/0B`, убирающий fake `CR` и потерю символа.
- `tui_goto` / базовый TUI path:
  hardcoded probe рисует правильные координаты.
- runaway `draw_border` path:
  уже выведен из основного пути в текущем `tetris_cpm.nanz`.

Иными словами: теперь remaining distortion сидит уже в compiler-side shapes.

## Минимальные repro shapes

Claude выделил три маленьких repro вместо одного огромного `tetris_cpm`.

### 1. Broken: `for + CALL`

Симптом:

- `for` loop с `CALL` в body отрабатывает только одну итерацию или уходит в неверное состояние.

Класс бага:

- loop counter / limit / temp-save оказываются наложены на один и тот же регистр;
- по наблюдениям это тот же family bug, который раньше ломал border/render loops.

Практический вывод:

- критические render/gameplay loops пока лучше держать на `while`, если цель быстро получить рабочий runtime;
- но правильная долгосрочная работа — regression + fix в allocator/codegen.

### 2. Broken: `array index with base-loss`

Симптом:

- форма уровня `board[y*W + x]` компилируется неправильно.

Два независимых класса поломки:

- materialization constant / multiply path может портить index;
- base pointer теряется, и вместо `base + index` фактически получается “index over itself”.

Это очень хорошо согласуется с тем, что мы наблюдали раньше:

- `board_get(0,0)` лез в zero page,
- `can_place()` сразу возвращал false,
- `spawn_piece()` ставил `game_over = 1`.

Практический вывод:

- `board_get/board_set` как direct indexed array shape пока небезопасны;
- pointer-walk здесь остаётся разумным временным safe path.

### 3. Correct: `3-arg LUT`

Симптом:

- форма уровня `LUT[t*4 + r*2 + c]` в минимальном repro собрана корректно.

Почему это важно:

- backend **может** генерировать корректный LUT access;
- когда base попадает в хороший addressing shape, это не фундаментальный запрет на LUT.

Но важная граница:

- safe shape в маленьком repro не оказался drop-in fix для полного `tetris_cpm`;
- попытка прямо заменить `piece_dx/piece_dy` на direct LUT form в полном исходнике снова привела к раннему `GAME OVER`.

Вывод:

- маленький safe repro полезен как ориентир для codegen,
- но не гарантирует, что тот же source rewrite безопасен в большем контексте.

## Что это значит для `tetris_cpm`

`tetris_cpm` сейчас не надо продолжать “чинить” крупными переписываниями.

Правильная стратегия:

1. Оставить исходник максимально близким к осмысленной форме.
2. Добывать из него минимальные repro.
3. Чинить compiler/backend под эти repro.
4. Возвращаться к `tetris_cpm` как к regression target.

## Что чинить первым

Приоритет такой:

1. `for + CALL`
   потому что это ломает не только тетрис, но и широкий класс render/UI loops.
2. `array index + base-loss`
   потому что это убивает board access и любые похожие `base + index` forms.
3. Только потом уже смотреть на более узкие piece/render arithmetic quirks.

## Практический next step

Следующий инженерный шаг:

- перенести два broken minimal repro в нормальный regression form для `minz-vir`;
- добавить focused tests на:
  - broken `for + call` class,
  - broken indexed array/base-loss class;
- не принимать больше source-side rewrites в `tetris_cpm`, если они не подтверждены отдельным minimal repro.

## Итог

Главный результат этой стадии:

- `tetris_cpm under mze` больше не “один большой хаос”;
- проблема разделена на:
  - emulator input bug,
  - compiler loop bug,
  - compiler indexed-array/base-loss bug.

Это уже хорошая точка, из которой можно чинить компилятор последовательно, а не воевать с симптомами.
