REPORT zref.
DATA x TYPE i.
DATA y TYPE i.
DATA seen TYPE i.
DO 2 TIMES.
  x = x * 10 + sy-index.
  DO 2 TIMES.
    y = y * 10 + sy-index.
  ENDDO.
  x = x * 10 + sy-index.
ENDDO.
seen = sy-index.
