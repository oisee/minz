REPORT zref.
DATA x TYPE i VALUE 70000.
DATA y TYPE i VALUE -90000.
DATA seen TYPE i VALUE 2147483647.
FORM combine USING a TYPE i CHANGING b TYPE i.
  b = b + a.
ENDFORM.
FORM probe.
  DATA low TYPE i VALUE -2147483648.
  PERFORM combine USING x CHANGING y.
  x = x * 3 + 1.
  IF y < 0 AND low < 0.
    seen = low.
  ENDIF.
ENDFORM.
