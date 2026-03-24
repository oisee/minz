REPORT zsap_calc.

* SAP-style Calculator on Z80 CP/M!
* Interactive selection screen with computation.
*
* CP/M:  echo "42\n7\n1" | mze out.com -t cpm
* MZV:   echo "42\n7\n1" | mzv -H sap_calculator.abap

PARAMETERS: p_val1 TYPE i DEFAULT 42,
            p_val2 TYPE i DEFAULT 7,
            p_op   TYPE i DEFAULT 1.

DATA lv_result TYPE i.
DATA lv_done TYPE i VALUE 0.

START-OF-SELECTION.

  WRITE '================================'.
  WRITE '  SAP Calculator on Z80!'.
  WRITE '  1=Add 2=Sub 3=Mul'.
  WRITE '================================'.
  WRITE ' '.
  WRITE 'Value 1:'.
  WRITE p_val1.
  WRITE 'Value 2:'.
  WRITE p_val2.
  WRITE ' '.

  IF p_op = 1.
    lv_result = p_val1 + p_val2.
    WRITE 'ADD =>'.
    WRITE lv_result.
    lv_done = 1.
  ENDIF.
  IF p_op = 2.
    lv_result = p_val1 - p_val2.
    WRITE 'SUB =>'.
    WRITE lv_result.
    lv_done = 1.
  ENDIF.
  IF p_op = 3.
    lv_result = p_val1 * p_val2.
    WRITE 'MUL =>'.
    WRITE lv_result.
    lv_done = 1.
  ENDIF.
  IF lv_done = 0.
    WRITE 'Unknown operation!'.
  ENDIF.

END-OF-SELECTION.
  WRITE ' '.
  WRITE '================================'.
  WRITE '  Powered by ABAP/4 on Z80'.
  WRITE '================================'.
