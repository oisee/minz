REPORT zselscreen.

* Selection screen demo - the classic ABAP report pattern.
* PARAMETERS -> selection screen -> INITIALIZATION -> START-OF-SELECTION.

PARAMETERS: p_name  TYPE c LENGTH 20 DEFAULT 'World',
            p_count TYPE i DEFAULT 5.

INITIALIZATION.
  p_name = 'Z80 User'.

START-OF-SELECTION.
  WRITE 'Hello,'.
  WRITE p_name.

  DATA lv_i TYPE i VALUE 1.
  WHILE lv_i <= p_count.
    WRITE lv_i.
    lv_i = lv_i + 1.
  ENDWHILE.

END-OF-SELECTION.
  WRITE 'Done!'.
