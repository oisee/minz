REPORT zselect_demo.

* Open SQL -> SQLite bridge demo.
* *!sql pragmas seed the database, SELECT queries it.
*
* Run: echo "" | mzv -H examples/abap/select_demo.abap

*!sql CREATE TABLE IF NOT EXISTS scarr (carrid TEXT, carrname TEXT, url TEXT)
*!sql INSERT INTO scarr VALUES ('AA', 'American Airlines', 'http://www.aa.com')
*!sql INSERT INTO scarr VALUES ('LH', 'Lufthansa', 'http://www.lufthansa.com')
*!sql INSERT INTO scarr VALUES ('SQ', 'Singapore Airlines', 'http://www.singaporeair.com')

DATA lv_count TYPE i.
DATA lv_name TYPE string.
DATA lv_url TYPE string.

START-OF-SELECTION.

  WRITE '=== Open SQL Demo ==='.
  WRITE ' '.

* SELECT SINGLE — count
  SELECT SINGLE COUNT(*) FROM scarr INTO lv_count.
  WRITE 'Total airlines:'.
  WRITE lv_count.
  WRITE ' '.

* SELECT SINGLE — lookup by key
  SELECT SINGLE carrname url FROM scarr INTO (lv_name, lv_url) WHERE carrid = 'LH'.
  WRITE 'Lufthansa:'.
  WRITE lv_name.
  WRITE lv_url.
  WRITE ' '.

* SELECT SINGLE — another lookup
  SELECT SINGLE carrname FROM scarr INTO lv_name WHERE carrid = 'SQ'.
  WRITE 'Singapore:'.
  WRITE lv_name.

END-OF-SELECTION.
  WRITE ' '.
  WRITE 'Done!'.
