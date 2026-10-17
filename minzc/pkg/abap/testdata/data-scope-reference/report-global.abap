REPORT zref.
DATA x TYPE i.
DATA y TYPE i.
DATA seen TYPE i.
START-OF-SELECTION.
  DO 3 TIMES.
    DATA n TYPE i VALUE 10.
    n = n + 1.
    x = x * 3 + n.
  ENDDO.
  PERFORM observe.
FORM observe.
  y = n.
  seen = seen + 1.
ENDFORM.
