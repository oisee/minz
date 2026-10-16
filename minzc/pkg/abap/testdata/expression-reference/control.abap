REPORT zref.
DATA x TYPE i VALUE 1.
DATA y TYPE i VALUE 2.
DATA seen TYPE i VALUE 0.
FORM probe.
  DO ( y + 1 ) TIMES.
    x = x + sy-index.
  ENDDO.
  IF x = 5.
    seen = 11.
  ELSEIF x = 6.
    seen = 12.
  ELSEIF x = 7 AND NOT y = 3.
    seen = 22.
  ELSEIF x = 8.
    seen = 23.
  ELSE.
    seen = 33.
  ENDIF.
  seen = seen - 22.
  IF x = 7 OR 1 / seen = 3.
    y = 9.
  ENDIF.
ENDFORM.
