REPORT zref.
DATA x TYPE i VALUE 0.
DATA y TYPE i VALUE 1.
DATA seen TYPE i VALUE 31.
FORM probe.
  DATA a TYPE i.
  DATA b TYPE i.
  a = x.
  b = y.
  x = a / b.
  y = a DIV b.
  seen = a MOD b.
ENDFORM.
FORM plus. x = x + y. seen = 99. ENDFORM.
FORM minus. x = x - y. seen = 99. ENDFORM.
FORM product. x = x * y. seen = 99. ENDFORM.
FORM negate. x = - x. seen = 99. ENDFORM.
