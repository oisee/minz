REPORT zshowcase.

* ABAP/4 on ZX Spectrum — the real thing.
*
* Spectrum: mz showcase_zx.abap --lir=false --target=spectrum -o out.a80
* CP/M:    mz showcase_zx.abap --lir=false --target=cpm -o out.a80
*
* On CP/M: interactive PARAMETERS with keyboard input.
* On ZX:   renders with embedded font, uses defaults.

PARAMETERS: p_name  TYPE c LENGTH 20 DEFAULT 'Alice',
            p_count TYPE i DEFAULT 5.

START-OF-SELECTION.

  WRITE '================================'.
  WRITE '  ABAP/4 on Z80 Processor!'.
  WRITE '================================'.
  WRITE ' '.
  WRITE 'Hello,'.
  WRITE p_name.
  WRITE '!'.
  WRITE ' '.
  WRITE 'You requested:'.
  WRITE p_count.
  WRITE 'repetitions.'.
  WRITE ' '.

  IF p_count >= 1.
    WRITE '  [1]'.
    WRITE p_name.
  ENDIF.
  IF p_count >= 2.
    WRITE '  [2]'.
    WRITE p_name.
  ENDIF.
  IF p_count >= 3.
    WRITE '  [3]'.
    WRITE p_name.
  ENDIF.
  IF p_count >= 4.
    WRITE '  [4]'.
    WRITE p_name.
  ENDIF.
  IF p_count >= 5.
    WRITE '  [5]'.
    WRITE p_name.
  ENDIF.

END-OF-SELECTION.
  WRITE ' '.
  WRITE '================================'.
  WRITE '  Compiled by MinZ to Z80'.
  WRITE '  3.5 MHz / 48KB / 2KB binary'.
  WRITE '================================'.
