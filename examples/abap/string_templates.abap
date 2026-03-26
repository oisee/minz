REPORT zstring_templates.

* ABAP String Templates on Z80
* |Hello { variable }!| — Ruby-style interpolation
*
* mzv: echo "" | mzv -H string_templates.abap
* CP/M: mz string_templates.abap --target=cpm -o out.a80
* ZX:   mz string_templates.abap --target=spectrum -o out.a80

DATA lv_name TYPE string VALUE 'Alice'.
DATA lv_age TYPE i VALUE 30.
DATA lv_city TYPE string VALUE 'Dublin'.

START-OF-SELECTION.

  WRITE '=== String Templates ==='.
  WRITE ' '.
  WRITE |Hello { lv_name }!|.
  WRITE |Age: { lv_age } years|.
  WRITE |City: { lv_city }|.
  WRITE ' '.
  WRITE |{ lv_name } from { lv_city }, age { lv_age }|.
  WRITE ' '.
  WRITE '=== ABAP on Z80 ==='.
  WRITE |Compiler: MinZ v0.23|.
  WRITE |CPU: Zilog Z80 3.5 MHz|.
  WRITE |RAM: 48 KB|.
  WRITE |Status: It works!|.
