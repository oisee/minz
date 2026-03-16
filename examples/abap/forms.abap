REPORT zforms.

* Modular ABAP with FORMs — the OG subroutines
* before everyone went OOP-crazy with ABAP Objects.
* Now running on hardware from 1976.

DATA: lv_result TYPE i,
      lv_gcd    TYPE i.

PERFORM factorial USING 6 CHANGING lv_result.
WRITE 'Factorial of 6:'.
WRITE lv_result.

PERFORM gcd USING 48 18 CHANGING lv_gcd.
WRITE 'GCD of 48 and 18:'.
WRITE lv_gcd.

*&---------------------------------------------------------------------*
*& Form factorial
*&---------------------------------------------------------------------*
FORM factorial USING pv_n TYPE i CHANGING pv_result TYPE i.
  DATA lv_i TYPE i VALUE 1.
  pv_result = 1.
  WHILE lv_i <= pv_n.
    pv_result = pv_result * lv_i.
    lv_i = lv_i + 1.
  ENDWHILE.
ENDFORM.

*&---------------------------------------------------------------------*
*& Form gcd — Euclid's algorithm, older than both ABAP and Z80
*&---------------------------------------------------------------------*
FORM gcd USING pv_a TYPE i pv_b TYPE i CHANGING pv_result TYPE i.
  DATA: lv_a    TYPE i,
        lv_b    TYPE i,
        lv_temp TYPE i.
  lv_a = pv_a.
  lv_b = pv_b.
  WHILE lv_b > 0.
    lv_temp = lv_a MOD lv_b.
    lv_a = lv_b.
    lv_b = lv_temp.
  ENDWHILE.
  pv_result = lv_a.
ENDFORM.
