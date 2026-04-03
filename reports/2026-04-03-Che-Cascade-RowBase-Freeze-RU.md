# `che_cascade.nanz`: заморозка полезного reshaping

Дата: 2026-04-03

## Что заморожено

В [`fun/che_cascade.nanz`](/home/alice/dev/minz-vir/fun/che_cascade.nanz) оставлен только тот reshape, который дал внятный локальный выигрыш и не расплодил новые fallback-узлы:

- `xor_pixel(px, py)` теперь только проверяет `py` и вычисляет `row = zx_row_addr(py)`
- вся работа по адресу строки и маске пикселя вынесена в `xor_pixel_row(row, px)`
- `xor_blk_row` считает `row` один раз на строку и дальше ходит по `x`
- countdown loops (`while left != 0`) сохранены

Сознательно убрано:

- block-size specialization `1/2/4`
- вспомогательные `xor_row1/2/4`
- `xor_blk1/2/4`
- dispatch по `blk == 1/2/4`

Этот слой оказался плохим tradeoff: helper frontier рос быстрее, чем solver pressure падал.

## Что это дало

Базовый trace:

- [`build/che_cascade_e2e.a80`](/home/alice/dev/minz-vir/build/che_cascade_e2e.a80)

Замороженный trace после полезного reshape:

- [`build/che_cascade_work.a80`](/home/alice/dev/minz-vir/build/che_cascade_work.a80)

Ключевой локальный эффект:

- раньше `xor_pixel` был `VIR→PBQP fallback`
- теперь `xor_pixel` идёт через `VIR`
- появился отдельный `xor_pixel_row`, который компилируется как constrained standalone helper

Оставшиеся bottleneck’и после freeze:

- `apply_buf`
- `apply_buf_row`
- `xor_blk`
- `xor_blk_row`
- `fill_buf`
- `main`

То есть выигрыш есть, но он узкий: удалось вынести row-base вычисление из самого нижнего pixel-path, а не “починить весь cascade”.

## ASM before/after

### До: `xor_blk_row` дёргает `xor_pixel`, а `xor_pixel` сам тащит `zx_row_addr`

Из [`build/che_cascade_e2e.a80`](/home/alice/dev/minz-vir/build/che_cascade_e2e.a80):

```asm
; VIR→PBQP fallback for xor_blk_row
xor_blk_row:
    ...
    ; genCall: xor_pixel dst=0
    LD D, A
    PUSH DE
    CALL xor_pixel
    POP DE
    ...

; VIR→PBQP fallback for xor_pixel
xor_pixel:
    ...
    ; genCall: zx_row_addr dst=82
    LD A, C
    PUSH DE
    CALL zx_row_addr
    POP DE
    ...
```

Смысл проблемы: адрес строки ZX вычислялся внутри более горячего pixel helper.

### После: `xor_pixel` только получает row-base и зовёт `xor_pixel_row`

Из [`build/che_cascade_work.a80`](/home/alice/dev/minz-vir/build/che_cascade_work.a80):

```asm
; fun xor_pixel(px: u8 = C, py: u8 = A)
xor_pixel:
    CP 96
    JR C, .xor_pixel_if_join2
    RET
.xor_pixel_if_join2:
    LD B, A
    CALL zx_row_addr
    RET
    LD D, C
    CALL xor_pixel_row
    RET

; fun xor_pixel_row(row: u16 = pointer, px: u8 = general)
xor_pixel_row:
    ...
    ADD HL, BC
    LD DE, bit_masks
    ...
    XOR D
    LD (HL), A
    RET
```

Смысл улучшения: pixel-path отделён от row-base path, и это уже меняет solver shape в правильную сторону.

## Что мы узнали

1. `row-base hoist` — реальный кандидат на будущий Grace transform.
2. `specialize by blk=1/2/4` в лоб — плохой автоматический transform-кандидат.
3. Для `che_cascade` bottleneck сейчас сидит выше:
   - в `apply_buf_row`
   - в `xor_blk_row`
   - в общей call-heavy draw chain
4. Следующий осмысленный automation target — не ещё больше ручных helper’ов, а Shape A/B multi-block pointer-threading и затем row-helper extraction.

## Какие фичи Nanz здесь уже реально работают

Сам `che_cascade.nanz` использует:

- typed global arrays
- LUT-ы как обычные глобалы:
  - `and_masks`
  - `bit_masks`
  - `zx_third_base`
  - `zx_prow_off`
  - `zx_crow_off`
- scalar bit selector:
  - `state.0` в `lfsr16`
- typed pointers:
  - `^u8`
- pointer dereference:
  - `p^`
  - `sp^`
- pointer arithmetic:
  - `row + (px / 8)`
  - `p = p + 1`
- static tables of `u16` and `u8`
- small row-helper style decomposition

Если нужен быстрый набор живых соседних примеров, смотреть:

- [`fun/bit_intent.nanz`](/home/alice/dev/minz-vir/fun/bit_intent.nanz)
- [`fun/pointer_threading.nanz`](/home/alice/dev/minz-vir/fun/pointer_threading.nanz)
- [`fun/tuple_return.nanz`](/home/alice/dev/minz-vir/fun/tuple_return.nanz)
- [`fun/triple_return_skip.nanz`](/home/alice/dev/minz-vir/fun/triple_return_skip.nanz)
- индекс playground: [`fun/README.md`](/home/alice/dev/minz-vir/fun/README.md)

## Итог

`che_cascade` заморожен не в “идеальной” форме, а в честно полезной:

- row-base hoist оставлен
- неудачная specialization-ветка убрана
- файл остаётся хорошим exploratory example для будущей Grace automation, а не набором всё более ручных one-off helper’ов
