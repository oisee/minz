REPORT zsap_hello.

* SAP Selection Screen on Z80!
* Interactive PARAMETERS with defaults, runs on CP/M and ZX Spectrum.
*
* CP/M:  mz sap_hello_zx.abap -o out.a80 && mza out.a80 -o out.com
*        echo "Alice\n7" | mze out.com -t cpm
*
* MZV:   echo "Alice\n7" | mzv -H sap_hello_zx.abap

PARAMETERS: p_name  TYPE c LENGTH 20 DEFAULT 'SAP User',
            p_count TYPE i DEFAULT 3.

START-OF-SELECTION.

  WRITE '================================'.
  WRITE '  SAP Report on Z80 Processor!'.
  WRITE '================================'.
  WRITE ' '.
  WRITE 'Hello,'.
  WRITE p_name.
  WRITE '!'.
  WRITE ' '.
  WRITE 'You requested:'.
  WRITE p_count.
  WRITE 'repetitions.'.
  WRITE ' '.

* Unrolled "loop" — avoids regalloc bugs with WHILE on Z80.
* Each IF tests p_count against a constant (no loop variable needed).
  IF p_count >= 1.
    WRITE '  [1]'.
    WRITE p_name.
  ENDIF.
  IF p_count >= 2.
    WRITE '  [2]'.
    WRITE p_name.
  ENDIF.
  IF p_count >= 3.
    WRITE '  [3]'.
    WRITE p_name.
  ENDIF.
  IF p_count >= 4.
    WRITE '  [4]'.
    WRITE p_name.
  ENDIF.
  IF p_count >= 5.
    WRITE '  [5]'.
    WRITE p_name.
  ENDIF.
  IF p_count >= 6.
    WRITE '  [6]'.
    WRITE p_name.
  ENDIF.
  IF p_count >= 7.
    WRITE '  [7]'.
    WRITE p_name.
  ENDIF.
  IF p_count >= 8.
    WRITE '  [8]'.
    WRITE p_name.
  ENDIF.

END-OF-SELECTION.
  WRITE ' '.
  WRITE '================================'.
  WRITE '  ABAP/4 on Z80. Yes, really.'.
  WRITE '================================'.
