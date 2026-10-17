REPORT zref.
DATA x TYPE i VALUE 5.
DATA y TYPE i VALUE 9.
DATA seen TYPE i.
FORM observe USING p TYPE i. x = 20. seen = p. ENDFORM.
FORM probe.
PERFORM observe USING x.
ENDFORM.
