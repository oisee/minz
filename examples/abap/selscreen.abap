REPORT zselscreen.

* Selection screen demo — the classic ABAP report pattern.
* PARAMETERS → selection screen → INITIALIZATION → START-OF-SELECTION.
* Now on Z80!

PARAMETERS: p_name  TYPE c LENGTH 20 DEFAULT 'Z80 User',
            p_count TYPE i DEFAULT 5,
            p_flag  TYPE c DEFAULT 'X'.

INITIALIZATION.
  WRITE 'Initializing report...'.
  p_name = 'Retro Dev'.

AT-SELECTION-SCREEN.
  WRITE 'Selection screen processed.'.
  WRITE 'SY-UCOMM:'.
  WRITE SY-UCOMM.

START-OF-SELECTION.
  WRITE '=== Report Output ==='.
  WRITE 'Hello,'.
  WRITE p_name.

  DATA lv_i TYPE i VALUE 1.
  WHILE lv_i <= p_count.
    WRITE lv_i.
    lv_i = lv_i + 1.
  ENDWHILE.

  IF p_flag = 'X'.
    WRITE 'Flag is set!'.
  ELSE.
    WRITE 'Flag is not set.'.
  ENDIF.

END-OF-SELECTION.
  WRITE '=== End of Report ==='.
  WRITE 'SY-SUBRC:'.
  WRITE SY-SUBRC.
