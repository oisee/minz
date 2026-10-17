REPORT zref.
DATA x TYPE i.
DATA y TYPE i.
DATA seen TYPE i.
DO 65537 TIMES.
  x = sy-index.
  y = y + 1.
ENDDO.
seen = sy-index.
