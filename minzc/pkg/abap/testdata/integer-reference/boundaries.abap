REPORT zref.
DATA x TYPE i VALUE -2147483648.
DATA y TYPE i VALUE 2147483647.
DATA seen TYPE i VALUE 65536.
FORM probe.
  CLEAR seen.
  IF x < 0 AND y > 0.
    x = x + 1.
    y = y - 65536.
  ENDIF.
ENDFORM.
