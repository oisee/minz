REPORT zmara_report.

* Classic SAP material master report.
* SELECT from MARA/MAKT with selection screen.
* Backed by SQLite via MZV host functions.
*
* This is probably the first time MARA has been queried
* on a Z80 processor. SAP would be proud. Or horrified.

PARAMETERS p_matnr TYPE c LENGTH 18 DEFAULT '%'.

START-OF-SELECTION.

  WRITE '=== Material Master Report ==='.
  WRITE ' '.

* Open database (host function)
  DATA lv_db TYPE i.
  PERFORM open_db CHANGING lv_db.

* Query MARA with LIKE filter
  DATA lv_stmt TYPE i.
  PERFORM query_mara USING lv_db p_matnr CHANGING lv_stmt.

* Output header
  WRITE 'MATNR            MTART  MBRSH MEINS'.
  WRITE '-----------------------------------------'.

* Fetch rows
  DATA: lv_has_row TYPE i,
        lv_count   TYPE i VALUE 0.

  PERFORM step_query USING lv_stmt CHANGING lv_has_row.
  WHILE lv_has_row = 1.
    PERFORM print_mara_row USING lv_stmt.
    lv_count = lv_count + 1.
    PERFORM step_query USING lv_stmt CHANGING lv_has_row.
  ENDWHILE.

  WRITE '-----------------------------------------'.
  WRITE lv_count.
  WRITE 'materials found.'.

* Now get texts from MAKT
  WRITE ' '.
  WRITE '=== Material Texts (MAKT) ==='.
  DATA lv_stmt2 TYPE i.
  PERFORM query_makt USING lv_db CHANGING lv_stmt2.

  PERFORM step_query USING lv_stmt2 CHANGING lv_has_row.
  WHILE lv_has_row = 1.
    PERFORM print_makt_row USING lv_stmt2.
    PERFORM step_query USING lv_stmt2 CHANGING lv_has_row.
  ENDWHILE.

* Cleanup
  PERFORM close_db USING lv_db.

END-OF-SELECTION.
  WRITE ' '.
  WRITE 'Report complete. SY-SUBRC:'.
  WRITE SY-SUBRC.

*&---------------------------------------------------------------------*
*& FORM stubs - in MZV these map to SQLite host functions
*& On Z80 they would use I/O port protocol to a database server
*&---------------------------------------------------------------------*
FORM open_db CHANGING pv_handle TYPE i.
  pv_handle = 1.
ENDFORM.

FORM query_mara USING pv_db TYPE i pv_filter TYPE c CHANGING pv_stmt TYPE i.
  pv_stmt = 1.
ENDFORM.

FORM query_makt USING pv_db TYPE i CHANGING pv_stmt TYPE i.
  pv_stmt = 1.
ENDFORM.

FORM step_query USING pv_stmt TYPE i CHANGING pv_has_row TYPE i.
  pv_has_row = 0.
ENDFORM.

FORM print_mara_row USING pv_stmt TYPE i.
  WRITE 'row'.
ENDFORM.

FORM print_makt_row USING pv_stmt TYPE i.
  WRITE 'row'.
ENDFORM.

FORM close_db USING pv_handle TYPE i.
  WRITE 'DB closed.'.
ENDFORM.
