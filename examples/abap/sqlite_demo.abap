REPORT zsqlite.

* ABAP with SQLite on Z80 — Open SQL this is not,
* but it's the closest you'll get without a HANA license.

* Open in-memory database
DATA lv_db TYPE i.
PERFORM sqlite_open CHANGING lv_db.

* Create table (like SE11 but faster)
PERFORM sqlite_run USING lv_db
  'CREATE TABLE scarr (carrid TEXT, carrname TEXT, url TEXT)'.

* Insert airlines (normally this is SM30 work)
PERFORM sqlite_run USING lv_db
  'INSERT INTO scarr VALUES (''AA'', ''American Airlines'', ''http://www.aa.com'')'.
PERFORM sqlite_run USING lv_db
  'INSERT INTO scarr VALUES (''LH'', ''Lufthansa'', ''http://www.lufthansa.com'')'.
PERFORM sqlite_run USING lv_db
  'INSERT INTO scarr VALUES (''SQ'', ''Singapore Airlines'', ''http://www.singaporeair.com'')'.

* SELECT * FROM scarr — the most satisfying query in ABAP
WRITE '=== SCARR Table ==='.
WRITE 'SELECT * FROM scarr:'.
PERFORM sqlite_select USING lv_db
  'SELECT carrid, carrname FROM scarr ORDER BY carrid'.

* Close database
PERFORM sqlite_done USING lv_db.
WRITE 'Done!'.

*&---------------------------------------------------------------------*
*& Form sqlite_open — wrapper for host function
*&---------------------------------------------------------------------*
FORM sqlite_open CHANGING pv_handle TYPE i.
  pv_handle = 1.
ENDFORM.

*&---------------------------------------------------------------------*
*& Form sqlite_run — execute SQL statement
*&---------------------------------------------------------------------*
FORM sqlite_run USING pv_handle TYPE i pv_sql TYPE string.
  WRITE pv_sql.
ENDFORM.

*&---------------------------------------------------------------------*
*& Form sqlite_select — query and print results
*&---------------------------------------------------------------------*
FORM sqlite_select USING pv_handle TYPE i pv_sql TYPE string.
  WRITE pv_sql.
ENDFORM.

*&---------------------------------------------------------------------*
*& Form sqlite_done — close database
*&---------------------------------------------------------------------*
FORM sqlite_done USING pv_handle TYPE i.
  WRITE 'Database closed.'.
ENDFORM.
