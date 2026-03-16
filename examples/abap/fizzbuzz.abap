REPORT zfizzbuzz.

* FizzBuzz — the SAP consultant's rite of passage,
* now running on a 3.5 MHz Z80 instead of a million-dollar HANA box.

DATA lv_i TYPE i VALUE 1.

WHILE lv_i <= 100.
  DATA lv_mod3 TYPE i.
  DATA lv_mod5 TYPE i.
  lv_mod3 = lv_i MOD 3.
  lv_mod5 = lv_i MOD 5.

  IF lv_mod3 = 0 AND lv_mod5 = 0.
    WRITE 'FizzBuzz'.
  ELSEIF lv_mod3 = 0.
    WRITE 'Fizz'.
  ELSEIF lv_mod5 = 0.
    WRITE 'Buzz'.
  ELSE.
    WRITE lv_i.
  ENDIF.

  lv_i = lv_i + 1.
ENDWHILE.
