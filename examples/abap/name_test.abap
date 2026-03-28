REPORT zname_test.

* Test: FORM and CLASS METHOD with same name don't conflict
* FORM 'add' → function 'add'
* CLASS lcl_calc METHOD add → function 'lcl_calc_add'

FORM add USING a TYPE i b TYPE i CHANGING result TYPE i.
  result = a + b.
ENDFORM.

CLASS lcl_calc DEFINITION.
  PUBLIC SECTION.
    METHODS add IMPORTING a TYPE i b TYPE i RETURNING VALUE(result) TYPE i.
    METHODS sub IMPORTING a TYPE i b TYPE i RETURNING VALUE(result) TYPE i.
ENDCLASS.

CLASS lcl_calc IMPLEMENTATION.
  METHOD add.
    result = a + b.
  ENDMETHOD.
  METHOD sub.
    result = a - b.
  ENDMETHOD.
ENDCLASS.

* FORM 'add' — direct name
* assert add(3, 4) == 7 via mir2
* assert add(0, 0) == 0 via mir2
* assert add(100, 55) == 155 via mir2

* CLASS lcl_calc METHOD add — mangled name
* assert lcl_calc_add(3, 4) == 7 via mir2
* assert lcl_calc_sub(10, 3) == 7 via mir2
* assert lcl_calc_sub(5, 5) == 0 via mir2
