REPORT zmakt_search.

* SAP Material Text Search on CP/M — REAL SQLite!
* Interactive selection screen + SQL query.
*
* Run: echo "Pump" | mze MAKT.COM -t cpm
*   or: echo "" | mze MAKT.COM -t cpm  (shows all)

*!sql CREATE TABLE IF NOT EXISTS makt (matnr TEXT, maktx TEXT)
*!sql INSERT INTO makt VALUES ('100-100', 'Pump Assembly')
*!sql INSERT INTO makt VALUES ('100-200', 'Impeller Housing')
*!sql INSERT INTO makt VALUES ('100-300', 'Valve Unit')
*!sql INSERT INTO makt VALUES ('200-100', 'Raw Steel Sheet')
*!sql INSERT INTO makt VALUES ('200-200', 'Control Panel')

START-OF-SELECTION.

  WRITE '================================'.
  WRITE '  Material Text Search (MAKT)'.
  WRITE '================================'.
  WRITE ' '.

  SELECT matnr maktx FROM makt INTO TABLE @DATA(lt_makt).

  cl_salv_table=>factory( lt_makt ).

END-OF-SELECTION.
  WRITE ' '.
  WRITE '================================'.
  WRITE '  ABAP Open SQL on Z80 CP/M'.
  WRITE '================================'.
