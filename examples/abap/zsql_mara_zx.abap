REPORT zsql_mara_zx.

* ZSQL + SAP MARA on ZX Spectrum
* Now matching real ZSQL output format!

START-OF-SELECTION.

  WRITE 'ZSQL - Z80 SQLite on Spectrum'.
  WRITE '=============================='.
  WRITE ' '.
  WRITE 'zsql> CREATE TABLE mara'.
  WRITE '  (matnr TEXT, mtart TEXT,'.
  WRITE '   meins TEXT, maktx TEXT)'.
  WRITE 'OK'.
  WRITE ' '.
  WRITE 'zsql> INSERT INTO mara VALUES'.
  WRITE 'OK (5 rows)'.
  WRITE ' '.
  WRITE 'zsql> SELECT * FROM mara'.
  WRITE ' '.
  WRITE '100-100 | FERT | ST | Pump Assy'.
  WRITE '100-200 | HALB | KG | Impeller'.
  WRITE '100-300 | FERT | ST | Valve Unit'.
  WRITE '200-100 | ROH  | KG | Raw Steel'.
  WRITE '200-200 | FERT | PC | Ctrl Panel'.
  WRITE ' '.
  WRITE '5 rows'.
  WRITE '=============================='.
