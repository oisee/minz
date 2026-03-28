REPORT zoop_assert.

* ABAP OOP with compile-time assertions on Z80
* Classes lowered to ClassName_MethodName functions

CLASS lcl_math DEFINITION.
  PUBLIC SECTION.
    METHODS add IMPORTING a TYPE i b TYPE i RETURNING VALUE(result) TYPE i.
    METHODS mul IMPORTING a TYPE i b TYPE i RETURNING VALUE(result) TYPE i.
    METHODS double IMPORTING x TYPE i RETURNING VALUE(result) TYPE i.
    METHODS triple IMPORTING x TYPE i RETURNING VALUE(result) TYPE i.
    METHODS negate IMPORTING x TYPE i RETURNING VALUE(result) TYPE i.
    METHODS identity IMPORTING x TYPE i RETURNING VALUE(result) TYPE i.
ENDCLASS.

CLASS lcl_math IMPLEMENTATION.
  METHOD add.
    result = a + b.
  ENDMETHOD.
  METHOD mul.
    result = a * b.
  ENDMETHOD.
  METHOD double.
    result = x + x.
  ENDMETHOD.
  METHOD triple.
    result = x * 3.
  ENDMETHOD.
  METHOD negate.
    result = 0 - x.
  ENDMETHOD.
  METHOD identity.
    result = x.
  ENDMETHOD.
ENDCLASS.

* Pure computation — no instance data, no self parameter
* assert lcl_math_add(3, 4) == 7 via mir2
* assert lcl_math_add(0, 0) == 0 via mir2
* assert lcl_math_mul(6, 7) == 42 via mir2
* assert lcl_math_mul(0, 255) == 0 via mir2
* assert lcl_math_double(21) == 42 via mir2
* assert lcl_math_double(0) == 0 via mir2
* assert lcl_math_triple(10) == 30 via mir2
* assert lcl_math_triple(0) == 0 via mir2
* assert lcl_math_negate(0) == 0 via mir2
* assert lcl_math_identity(42) == 42 via mir2
* assert lcl_math_identity(0) == 0 via mir2

CLASS lcl_bits DEFINITION.
  PUBLIC SECTION.
    METHODS shl1 IMPORTING x TYPE i RETURNING VALUE(result) TYPE i.
    METHODS diff IMPORTING a TYPE i b TYPE i RETURNING VALUE(result) TYPE i.
ENDCLASS.

CLASS lcl_bits IMPLEMENTATION.
  METHOD shl1.
    result = x * 2.
  ENDMETHOD.
  METHOD diff.
    result = a - b.
  ENDMETHOD.
ENDCLASS.

* assert lcl_bits_shl1(1) == 2 via mir2
* assert lcl_bits_shl1(64) == 128 via mir2
* assert lcl_bits_diff(10, 3) == 7 via mir2
* assert lcl_bits_diff(100, 100) == 0 via mir2
