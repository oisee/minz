REPORT zclear.
DATA lv_x TYPE i VALUE 7.
FORM probe CHANGING result TYPE i.
  CLEAR lv_x.
  result = lv_x.
ENDFORM.
