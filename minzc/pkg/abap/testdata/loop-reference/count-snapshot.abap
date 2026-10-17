REPORT zref.
DATA x TYPE i.
DATA y TYPE i VALUE 3.
DATA seen TYPE i.
DO y TIMES.
  x = x * 10 + sy-index.
  y = 0.
ENDDO.
WHILE y < 3.
  y = y + 1.
  x = x * 10 + sy-index.
  IF sy-index = 2.
    CONTINUE.
  ENDIF.
  seen = seen + 1.
ENDWHILE.
