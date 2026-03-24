REPORT zsap_zx.

* SAP Report on ZX Spectrum!
* No PARAMETERS (no keyboard input needed).
* Pure WRITE output — renders with embedded ZX font.
*
* Build:
*   mz sap_zx_demo.abap --target=spectrum -o out.a80
*   mza out.a80 -o out.bin
*   mze out.bin -t spectrum --profile /tmp/zx.json
*   python3 /tmp/render_zx_screen.py /tmp/zx.json /tmp/sap.png

START-OF-SELECTION.

  WRITE '================================'.
  WRITE '  SAP Report on ZX Spectrum!'.
  WRITE '================================'.
  WRITE ' '.
  WRITE 'Material Master Report'.
  WRITE ' '.
  WRITE 'MATNR     MTART  MAKTX'.
  WRITE '--------- ------ -----------'.
  WRITE '100-100   FERT   Pump Assy'.
  WRITE '100-300   FERT   Valve Unit'.
  WRITE '200-200   FERT   Ctrl Panel'.
  WRITE ' '.
  WRITE '3 materials found.'.
  WRITE ' '.
  WRITE '================================'.
  WRITE '  ABAP/4 on Zilog Z80'.
  WRITE '  Powered by MinZ Compiler'.
  WRITE '================================'.
