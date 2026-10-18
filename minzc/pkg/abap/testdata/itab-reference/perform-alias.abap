REPORT zcomponents.
TYPES: BEGIN OF ty, tag TYPE c LENGTH 3, n TYPE i, END OF ty.
DATA ls TYPE ty.
DATA rows TYPE STANDARD TABLE OF ty WITH EMPTY KEY INITIAL SIZE 2.
ls-tag = 'abc'.
ls-n = 41.
PERFORM relay USING ls-n ls-tag CHANGING ls-n ls-tag.
APPEND ls TO rows.
WRITE |{ ls-n }:{ ls-tag }|.
FORM relay USING a TYPE i text TYPE c CHANGING b TYPE i other TYPE c.
 PERFORM mutate USING a text CHANGING b other.
ENDFORM.
FORM mutate USING a TYPE i text TYPE c CHANGING b TYPE i other TYPE c.
 a = a + 1.
 b = b + a.
 text = 'X'.
 IF other = 'X'.
  other = 'XY'.
 ENDIF.
ENDFORM.
