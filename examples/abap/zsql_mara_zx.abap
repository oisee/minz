REPORT zsql_mara_zx.

* ZSQL + SAP MARA combined demo on ZX Spectrum
* Enterprise SQL meets 1982 retro hardware.

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
  WRITE '  100-100 FERT ST Pump Assy'.
  WRITE '  100-300 FERT ST Valve Unit'.
  WRITE '  200-100 ROH  KG Raw Steel'.
  WRITE '  200-200 HALB KG Steel Plate'.
  WRITE 'OK (4 rows)'.
  WRITE ' '.
  WRITE 'zsql> SELECT * FROM mara'.
  WRITE '  WHERE mtart = FERT'.
  WRITE ' '.
  WRITE 'matnr   |mtart|meins|maktx'.
  WRITE '--------|-----|-----|----------'.
  WRITE '100-100 |FERT |ST   |Pump Assy'.
  WRITE '100-300 |FERT |ST   |Valve Unit'.
  WRITE ' '.
  WRITE '2 rows. SQL on Z80!'.
