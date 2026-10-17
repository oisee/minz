REPORT zref.
DATA x TYPE i.
DATA y TYPE i.
DATA seen TYPE i.
DO.
  x = x * 10 + sy-index.
  IF sy-index = 3.
    EXIT.
  ENDIF.
ENDDO.
WHILE 1 = 1.
  y = y * 10 + sy-index.
  IF sy-index = 3.
    EXIT.
  ENDIF.
ENDWHILE.
seen = sy-index.
