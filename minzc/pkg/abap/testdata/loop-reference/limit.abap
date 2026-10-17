REPORT zref.
DATA x TYPE i.
DATA y TYPE i VALUE -1.
DATA seen TYPE i.
DO y TIMES.
  x = 77.
ENDDO.
seen = seen + 1.
DO 0 TIMES.
  x = 88.
ENDDO.
seen = seen + 10.
DO 2147483647 TIMES.
  x = sy-index.
  EXIT.
ENDDO.
y = sy-index.
