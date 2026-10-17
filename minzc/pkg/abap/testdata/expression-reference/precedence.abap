REPORT zref.
DATA x TYPE i VALUE 2.
DATA y TYPE i VALUE 3.
DATA seen TYPE i VALUE 0.
FORM probe.
  x = x + y * 4 + 5.
  y = ( x - 1 ) / 3.
  IF NOT x = 19 AND y = 6 OR x = 19 AND y = 6.
    seen = 101.
  ELSE.
    seen = 202.
  ENDIF.
ENDFORM.
