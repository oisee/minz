REPORT zref.
DATA x TYPE i.
DATA y TYPE i.
DATA seen TYPE i.
FORM measure.
  DO 2 TIMES.
    DATA c TYPE c LENGTH 3 VALUE 'ab'.
    DATA s TYPE string VALUE `xy`.
    IF sy-index = 1.
      c = 'cd'.
      s = `zz`.
    ENDIF.
    IF c = 'cd' AND s = `zz`.
      x = x * 3 + 1.
    ELSE.
      x = x * 3 + 2.
    ENDIF.
  ENDDO.
  IF c = 'cd'.
    y = y + 1.
  ENDIF.
  IF s = `zz`.
    seen = seen + 1.
  ENDIF.
ENDFORM.
PERFORM measure.
PERFORM measure.
