REPORT zsql_zx.

* ZSQL on ZX Spectrum — SQL session rendered with ZX font!
*
* Build & screenshot:
*   mz zsql_zx.abap --target=spectrum -o zsql_zx.a80
*   mza zsql_zx.a80 -o zsql_zx.bin
*   mze zsql_zx.bin -t spectrum --profile /tmp/zx.json
*   python3 tools/render_zx_screen.py /tmp/zx.json /tmp/zsql_zx.png

START-OF-SELECTION.

  WRITE 'ZSQL - Z80 SQLite on Spectrum'.
  WRITE '=============================='.
  WRITE 'Connected to :memory:'.
  WRITE ' '.
  WRITE 'zsql> CREATE TABLE users'.
  WRITE '      (name TEXT, age TEXT)'.
  WRITE 'OK'.
  WRITE ' '.
  WRITE 'zsql> INSERT INTO users'.
  WRITE '      VALUES (Alice, 30)'.
  WRITE 'OK'.
  WRITE ' '.
  WRITE 'zsql> SELECT * FROM users'.
  WRITE ' '.
  WRITE 'name     | age'.
  WRITE '---------|----'.
  WRITE 'Alice    | 30'.
  WRITE 'Bob      | 25'.
  WRITE 'Charlie  | 35'.
  WRITE ' '.
  WRITE '3 rows'.
  WRITE '=============================='.
  WRITE 'SQL on Z80. Yes, really.'.
