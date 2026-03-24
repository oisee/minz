REPORT zselect_loop.

* SELECT ... ENDSELECT loop demo.
* Iterates all rows and WRITEs each one — the classic ABAP pattern.
*
* Run: echo "" | mzv -H examples/abap/select_loop.abap

*!sql CREATE TABLE scarr (carrid TEXT, carrname TEXT, url TEXT)
*!sql INSERT INTO scarr VALUES ('AA', 'American Airlines', 'http://www.aa.com')
*!sql INSERT INTO scarr VALUES ('LH', 'Lufthansa', 'http://www.lufthansa.com')
*!sql INSERT INTO scarr VALUES ('SQ', 'Singapore Airlines', 'http://www.singaporeair.com')

DATA lv_id TYPE string.
DATA lv_name TYPE string.

START-OF-SELECTION.

  WRITE '=== Airlines ==='.

  SELECT carrid carrname FROM scarr INTO (lv_id, lv_name).
    WRITE lv_id.
    WRITE lv_name.
  ENDSELECT.

  WRITE 'Done!'.
