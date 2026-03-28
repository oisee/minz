REPORT zfunc_test.

* ABAP Function Modules on Z80
* FUNCTION/ENDFUNCTION with IMPORTING/EXPORTING

FUNCTION z_add.
  IMPORTING a TYPE i b TYPE i
  EXPORTING result TYPE i.
  result = a + b.
ENDFUNCTION.

FUNCTION z_double.
  IMPORTING x TYPE i
  EXPORTING result TYPE i.
  result = x + x.
ENDFUNCTION.

FUNCTION z_mul.
  IMPORTING a TYPE i b TYPE i
  EXPORTING result TYPE i.
  result = a * b.
ENDFUNCTION.

* FUNCTION names lowercased: z_add, z_double, z_mul
* assert z_add(3, 4) == 7 via mir2
* assert z_add(0, 0) == 0 via mir2
* assert z_add(100, 55) == 155 via mir2
* assert z_double(21) == 42 via mir2
* assert z_double(0) == 0 via mir2
* assert z_mul(6, 7) == 42 via mir2
* assert z_mul(0, 255) == 0 via mir2
