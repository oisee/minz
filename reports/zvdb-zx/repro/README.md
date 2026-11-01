# Task Z4 repros

Three defects the port ran into, each with a minimal program, the command that
shows it and the observed output. All commands run from the repo root; build the
tools first with `cd minzc && make all` plus
`go build -tags mzx_headless -o mzx-headless ./cmd/mzx`.

## 1. `label-home-collision/` — an imported module's variable home aliases a main-module global (silent wrong code)

`collide.nanz` and `control.nanz` are identical except for the name of one
unused global: `top` versus `zvui_top`. `top` also happens to be the name of a
local of `zx.narrow_font.nf_build_packed` (the mask-table pointer), and when the
compiler gives that local a memory home it emits the *bare* label `top` — so the
assembler resolves it to the main module's global. The font is then squeezed with
a mask byte read back from the wrong address, and every glyph on the screen is
wrong. No diagnostic anywhere: the program compiles, runs and draws.

```sh
for v in collide control; do
  cp reports/zvdb-zx/repro/label-home-collision/$v.nanz examples/nanz/zz_$v.nanz
  minzc/mz examples/nanz/zz_$v.nanz -o /tmp/$v.a80
  minzc/mza /tmp/$v.a80 -o /tmp/$v.bin
  rm -rf /tmp/scr-$v
  minzc/mzx-headless --model 48k --run /tmp/$v.bin@8000 --frames 250 --dump-scr /tmp/scr-$v
  rm examples/nanz/zz_$v.nanz
done
python3 - <<'EOF'
import sys; sys.path.insert(0, 'scripts')
from zx_narrow_text_ocr import Screen
for v in ('collide', 'control'):
    s = Screen(f'/tmp/scr-{v}/frame_0250.scr')
    print(v, repr(s.window(x_px=0, y=0, cols=40, rows=1, width=5)[0]))
EOF
awk '/^zx__text__zx__narrow_font__nf_build_packed:/,/RET/' /tmp/collide.a80 | grep 'LD (top)'
```

Observed:

```
collide '??? ?????? ?? ???? ??? ?????? ?????? ???'
control 'The forest is calm and serene during the'
    LD (top), A          <- inside nf_build_packed, an imported module
```

The same defect, with the same symptom, hits `left_w` and `right_x` — locals of
`zx.text.split_vertical_i` — so the shipped demo's right pane would have opened
at a garbage column too. `minzc/pkg/` is off limits for this task, so the fix is
in the demo: every lowercase global lives in the `zvui_` namespace, and
`TestZVDBZXGlobalNamespace` fails if one does not (and if any bare label of the
main module is ever referenced from imported code again).

## 2. `win-open-attr-y/` — `win_open`/`win_attr` painted attribute rows 0..h-1, ignoring the window's y

`win_apply_rows` takes absolute character rows, but `win_apply_attr` passed it
`0` instead of `wn_y[id]`, so opening a window at y = 3 with h = 4 coloured
screen rows 0..3 instead of 3..6. Invisible to every Z3 caller, which only ever
opened windows at y = 0 — and to both Z3 whole-screen tests for the same reason.

```sh
cp reports/zvdb-zx/repro/win-open-attr-y/probe.nanz examples/nanz/zz_probe.nanz
minzc/mz examples/nanz/zz_probe.nanz -o /tmp/probe.a80
minzc/mza /tmp/probe.a80 -o /tmp/probe.bin
rm examples/nanz/zz_probe.nanz
minzc/mzx-headless --model 48k --run /tmp/probe.bin@8000 --frames 60 \
    --save-snapshot /tmp/probe.sna
python3 -c "
sna = open('/tmp/probe.sna','rb').read()
want = (2 << 3) | 7      # white on green
print([r for r in range(24) if sna[27 + (0x5800 - 0x4000) + r*32] == want])"
```

Observed with `stdlib/zx/text.nanz` reverted to the pre-fix line
(`win_apply_rows(id, 0, wn_h[id])`): `[0, 1, 2, 3]`.
Observed with the fix (`win_apply_rows(id, wn_y[id], wn_h[id])`): `[3, 4, 5, 6]`.

The one-line fix is in `stdlib/zx/text.nanz`; both `TestZXNarrowText*` tests
still pass with it, because their windows all start at row 0.

## 3. `keytab-case/` — the keyboard matrix reports uppercase, the Z3 demo compares lowercase

`examples/nanz/zx_narrow_text_demo.nanz` builds `KEYTAB` from the ULA matrix and
compares `zx_getkey()` against `KEY_J = 106`, `KEY_K = 107`, `KEY_Q = 113` —
lowercase. The unshifted matrix reports the *uppercase* ASCII of a letter key,
so those three keys never fire in the Z3 demo; only `1`/`2`/`3` (digits) do, and
those are the only ones its tests press. The Z3 report's claim that `j`/`k` move
the list is therefore not true of the shipped build.

```sh
cp reports/zvdb-zx/repro/keytab-case/probe.nanz examples/nanz/zz_key.nanz
minzc/mz examples/nanz/zz_key.nanz -o /tmp/key.a80 && minzc/mza /tmp/key.a80 -o /tmp/key.bin
rm examples/nanz/zz_key.nanz
python3 -c "
import json
ev, f = [], 120
for row, bit in [(6,3),(6,2),(2,0),(6,4),(1,0)]:
    ev += [{'frame':f,'row':row,'bit':bit,'down':True},
           {'frame':f+20,'row':row,'bit':bit,'down':False}]
    f += 50
json.dump(ev, open('/tmp/kkeys.json','w'))"
minzc/mzx-headless --model 48k --run /tmp/key.bin@8000 --frames 400 \
    --key-script /tmp/kkeys.json --dump-scr /tmp/scr-key
python3 -c "
import sys; sys.path.insert(0, 'scripts')
from zx_narrow_text_ocr import Screen
for f, label in [(130,'J'),(180,'K'),(230,'Q'),(280,'H'),(330,'A')]:
    s = Screen(f'/tmp/scr-key/frame_{f:04d}.scr')
    print(label, repr(s.window(x_px=0, y=0, cols=3, rows=1, width=8)[0]))"
```

Observed:

```
J '074'    K '075'    Q '081'    H '072'    A '065'
```

74/75/81 are 'J', 'K', 'Q'. This port therefore uses `KEY_J = 74`, `KEY_K = 75`,
`KEY_Q = 81` and documents the keys as J, K and Q. The Z3 demo is not touched
here — it is PR #297's file and the fix is a three-constant change for its
author (see "still open" in the report).
