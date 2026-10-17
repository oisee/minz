REPORT zref.
DATA x TYPE i.
DATA y TYPE i.
DATA seen TYPE i.
FORM measure.
  DO 3 TIMES.
    IF sy-index = 1.
      DATA n TYPE i VALUE 10.
    ENDIF.
    n = n + 1.
    x = x * 3 + n.
  ENDDO.
  y = n.
  seen = seen + 1.
ENDFORM.
PERFORM measure.
PERFORM measure.
