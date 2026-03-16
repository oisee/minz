REPORT zoop.

* ABAP Objects on Z80!
* Your ZX Spectrum is now an enterprise-grade OOP platform.
* Next step: SAP GUI on a CRT monitor. Oh wait...

INTERFACE lif_shape.
  METHODS get_area RETURNING VALUE(rv_area) TYPE i.
  METHODS describe.
ENDINTERFACE.

CLASS lcl_rectangle DEFINITION.
  PUBLIC SECTION.
    INTERFACES lif_shape.
    DATA: width  TYPE i,
          height TYPE i.
    METHODS constructor IMPORTING iv_w TYPE i iv_h TYPE i.
ENDCLASS.

CLASS lcl_rectangle IMPLEMENTATION.
  METHOD constructor.
    width = iv_w.
    height = iv_h.
  ENDMETHOD.

  METHOD lif_shape~get_area.
    rv_area = width * height.
  ENDMETHOD.

  METHOD lif_shape~describe.
    WRITE 'Rectangle'.
    WRITE width.
    WRITE height.
  ENDMETHOD.
ENDCLASS.

CLASS lcl_square DEFINITION INHERITING FROM lcl_rectangle.
  PUBLIC SECTION.
    METHODS constructor IMPORTING iv_side TYPE i.
ENDCLASS.

CLASS lcl_square IMPLEMENTATION.
  METHOD constructor.
    width = iv_side.
    height = iv_side.
  ENDMETHOD.
ENDCLASS.

DATA lo_rect TYPE REF TO lcl_rectangle.
CREATE OBJECT lo_rect EXPORTING iv_w = 6 iv_h = 7.
lo_rect->lif_shape~describe( ).

DATA lv_area TYPE i.
lv_area = lo_rect->lif_shape~get_area( ).
WRITE 'Area:'.
WRITE lv_area.
