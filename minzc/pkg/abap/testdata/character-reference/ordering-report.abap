REPORT zref.
DATA x TYPE c LENGTH 3 VALUE 'ABC'.
DATA y TYPE c LENGTH 5 VALUE 'ABD'.
DATA seen TYPE i VALUE 0.
IF x < y.
  seen = seen + 1.
ENDIF.
IF y > x.
  seen = seen + 2.
ENDIF.
IF x <= y.
  seen = seen + 4.
ENDIF.
IF y >= x.
  seen = seen + 8.
ENDIF.
IF x <> y.
  seen = seen + 16.
ENDIF.
IF x = 'ABC  '.
  seen = seen + 32.
ENDIF.
CLEAR x.
IF x < 'A'.
  seen = seen + 64.
ENDIF.
CLEAR y.
IF x = y.
  seen = seen + 128.
ENDIF.
x = 'A'.
y = 'A'.
IF x >= y.
  seen = seen + 256.
ENDIF.
IF x <= y.
  seen = seen + 512.
ENDIF.
WRITE x.
WRITE y.
