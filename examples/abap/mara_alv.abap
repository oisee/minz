REPORT zmara_alv.

* SAP Material Master with ALV display.
* Uses ABAP OPEN SQL: SELECT ... INTO TABLE @DATA(lt_mara).
* cl_salv_table=>factory() displays the internal table as ALV grid.
*
* Run: echo "" | mzv -H examples/abap/mara_alv.abap

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

  WRITE '=== Material Master Report (ALV) ==='.
  WRITE ' '.

* SELECT with JOIN into internal table (tilde notation for table~field)
  SELECT matnr mtart meins maktx FROM mara INNER JOIN makt ON mara~matnr = makt~matnr WHERE mtart = 'FERT' INTO TABLE @DATA(lt_mara).

* Display as ALV grid
  cl_salv_table=>factory( lt_mara ).

END-OF-SELECTION.
  WRITE ' '.
  WRITE 'Report complete.'.
