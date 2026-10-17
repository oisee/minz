REPORT zref.
DATA x TYPE string VALUE `ab  `.
DATA y TYPE string.
DATA c TYPE c LENGTH 5 VALUE 'ABC'.
DATA seen TYPE i VALUE 65536.
FORM inner USING source TYPE string CHANGING result TYPE string.
  result = source.
ENDFORM.
FORM outer USING source TYPE string CHANGING result TYPE string.
  DATA temp TYPE string.
  temp = source.
  CLEAR source.
  PERFORM inner USING temp CHANGING result.
ENDFORM.
PERFORM inner USING x CHANGING x.
PERFORM outer USING x CHANGING y.
IF x = ``.
  seen = seen + 1.
ENDIF.
IF y = `ab  `.
  seen = seen + 2.
ENDIF.
x = c.
IF x = c.
  seen = seen + 4.
ENDIF.
y = `ABC `.
IF y > x.
  seen = seen + 8.
ENDIF.
IF c <> y.
  seen = seen + 16.
ENDIF.
WRITE x.
WRITE y.
