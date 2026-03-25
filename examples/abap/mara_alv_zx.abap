REPORT zmara_alv_zx.

* Material Master ALV on ZX Spectrum!
* Hardcoded data (avoids SQLite regalloc issue).
*
* Build:
*   mz mara_alv_zx.abap --target=spectrum -o out.a80
*   mza out.a80 -o out.bin
*   mze out.bin -t spectrum --profile /tmp/zx.json
*   python3 tools/render_zx_screen.py /tmp/zx.json /tmp/mara.png

START-OF-SELECTION.

  WRITE '=== Material Master (ALV) ==='.
  WRITE ' '.
  WRITE 'MATNR     MTART MEINS MAKTX'.
  WRITE '--------- ----- ----- -----------'.
  WRITE '100-100   FERT  ST    Pump Assy'.
  WRITE '100-200   HALB  KG    Impeller'.
  WRITE '100-300   FERT  ST    Valve Unit'.
  WRITE '200-100   ROH   KG    Raw Steel'.
  WRITE '200-200   FERT  PC    Ctrl Panel'.
  WRITE ' '.
  WRITE '5 entries displayed.'.
  WRITE ' '.
  WRITE '=== ABAP Open SQL on Z80 ==='.
  WRITE 'SELECT matnr mtart meins maktx'.
  WRITE '  FROM mara'.
  WRITE '  INNER JOIN makt'.
  WRITE '  ON mara~matnr = makt~matnr'.
  WRITE '  INTO TABLE @DATA(lt_mara).'.
  WRITE ' '.
  WRITE 'cl_salv_table=>factory(lt_mara).'.
