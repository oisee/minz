REPORT zref.
DATA x TYPE c LENGTH 5 VALUE 'ABCDE'.
DATA y TYPE c LENGTH 3 VALUE 'xyz'.
DATA seen TYPE i VALUE 0.
FORM copy USING source TYPE c CHANGING result TYPE c.
  result = source.
ENDFORM.
PERFORM copy USING x CHANGING y.
IF y = 'ABC'.
  seen = seen + 1.
ENDIF.
x = 'Q'.
PERFORM copy USING y CHANGING x.
IF x = 'ABC'.
  seen = seen + 2.
ENDIF.
CLEAR y.
IF y = ' '.
  seen = seen + 4.
ENDIF.
