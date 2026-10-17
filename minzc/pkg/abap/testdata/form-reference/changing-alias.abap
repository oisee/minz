REPORT zref.
DATA x TYPE i VALUE 5.
DATA y TYPE i VALUE 9.
DATA seen TYPE i.
FORM mix CHANGING a TYPE i b TYPE i. a = 7. b = b + 1. seen = a. ENDFORM.
FORM probe.
PERFORM mix CHANGING x x.
ENDFORM.
