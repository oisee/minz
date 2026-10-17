REPORT zref.
DATA x TYPE string VALUE ' init  '.
DATA y TYPE string.
DATA c TYPE c LENGTH 3 VALUE 'A  '.
DATA seen TYPE i VALUE 65536.
FORM copy USING source TYPE string CHANGING result TYPE string.
  result = source.
ENDFORM.
IF x = ` init`.
  seen = seen + 8.
ENDIF.
x = c.
PERFORM copy USING x CHANGING y.
x = `long `.
IF y = `A`.
  seen = seen + 1.
ENDIF.
IF x = `long `.
  seen = seen + 2.
ENDIF.
c = x.
CLEAR x.
IF x = ``.
  seen = seen + 4.
ENDIF.
WRITE y.
WRITE c.
