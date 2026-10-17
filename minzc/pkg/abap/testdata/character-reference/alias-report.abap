REPORT zref.
DATA x TYPE c LENGTH 4 VALUE 'test'.
DATA y TYPE c LENGTH 6 VALUE 'before'.
DATA seen TYPE i VALUE 0.
FORM inner USING source TYPE c CHANGING result TYPE c.
  result = source.
ENDFORM.
FORM outer USING source TYPE c CHANGING result TYPE c.
  DATA local TYPE c LENGTH 2.
  local = source.
  PERFORM inner USING local CHANGING result.
  IF local = 'te'.
    seen = seen + 1.
  ENDIF.
ENDFORM.
PERFORM outer USING x CHANGING y.
PERFORM inner USING x CHANGING x.
IF x = 'test'.
  seen = seen + 2.
ENDIF.
PERFORM inner USING 'Z' CHANGING y.
IF y = 'Z'.
  seen = seen + 4.
ENDIF.
