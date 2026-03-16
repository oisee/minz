REPORT zfibonacci.

* Fibonacci sequence on a Z80.
* SAP basis admin: "Why is my ZX Spectrum running ABAP?"

DATA: lv_n    TYPE i VALUE 20,
      lv_a    TYPE i VALUE 0,
      lv_b    TYPE i VALUE 1,
      lv_temp TYPE i,
      lv_i    TYPE i VALUE 0.

WHILE lv_i < lv_n.
  WRITE lv_a.
  lv_temp = lv_a + lv_b.
  lv_a = lv_b.
  lv_b = lv_temp.
  lv_i = lv_i + 1.
ENDWHILE.
