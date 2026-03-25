REPORT zmara_alv_real.

* REAL SQLite → ALV on ZX Spectrum!
* SELECT with INNER JOIN from actual database.
*
* mz mara_alv_real_zx.abap --lir=false --target=spectrum -o out.a80
* mza out.a80 -o out.bin
* mzx --run out.bin@8000

*!sql CREATE TABLE IF NOT EXISTS mara (matnr TEXT, mtart TEXT, meins TEXT)
*!sql INSERT INTO mara VALUES ('100-100', 'FERT', 'ST')
*!sql INSERT INTO mara VALUES ('100-200', 'HALB', 'KG')
*!sql INSERT INTO mara VALUES ('100-300', 'FERT', 'ST')
*!sql INSERT INTO mara VALUES ('200-100', 'ROH', 'KG')
*!sql INSERT INTO mara VALUES ('200-200', 'FERT', 'PC')

*!sql CREATE TABLE IF NOT EXISTS makt (matnr TEXT, maktx TEXT)
*!sql INSERT INTO makt VALUES ('100-100', 'Pump Assembly')
*!sql INSERT INTO makt VALUES ('100-200', 'Impeller Housing')
*!sql INSERT INTO makt VALUES ('100-300', 'Valve Unit')
*!sql INSERT INTO makt VALUES ('200-100', 'Raw Steel Sheet')
*!sql INSERT INTO makt VALUES ('200-200', 'Control Panel')

START-OF-SELECTION.

  WRITE '=== Material Master (ALV) ==='.
  WRITE ' '.

  SELECT matnr mtart meins maktx FROM mara INNER JOIN makt ON mara~matnr = makt~matnr INTO TABLE @DATA(lt_mara).

  cl_salv_table=>factory( lt_mara ).

END-OF-SELECTION.
  WRITE ' '.
  WRITE 'cl_salv_table on ZX Spectrum!'.
