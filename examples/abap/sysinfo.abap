REPORT zsysinfo.

* System info report — every ABAPer's first transaction.
* SM50 eat your heart out — we have 3.5 MHz and 48KB.

WRITE '=== Z80 System Report ==='.
WRITE ' '.

DATA: lv_cpu_mhz  TYPE i VALUE 3,
      lv_ram_kb   TYPE i VALUE 48,
      lv_rom_kb   TYPE i VALUE 16,
      lv_total_kb TYPE i.

lv_total_kb = lv_ram_kb + lv_rom_kb.

WRITE 'CPU: Z80A'.
WRITE 'Clock (MHz):'.
WRITE lv_cpu_mhz.
WRITE 'RAM (KB):'.
WRITE lv_ram_kb.
WRITE 'ROM (KB):'.
WRITE lv_rom_kb.
WRITE 'Total (KB):'.
WRITE lv_total_kb.
WRITE ' '.
WRITE 'SAP Basis: N/A'.
WRITE 'Kernel: CP/M 2.2'.
WRITE 'ABAP Runtime: MinZ v0.19.6'.
WRITE '========================='.
WRITE 'STATUS: ALL SYSTEMS GO'.
