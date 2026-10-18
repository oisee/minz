# ABAP CSV → ALV on ZX Spectrum, without a DOS ROM

The unchanged [zfleet_csv_alv.abap](../zfleet_csv_alv.abap) reads a real TRD
catalogue, chooses a CSV and displays its typed rows using compiled Z80 code.
Disk I/O uses MZX's existing sector traps. Directory parsing, name matching,
CSV decoding, table operations and ULA rendering all execute in the guest.

Build `mz`, `mza` and `mzx` from `minzc/` (Go on PATH):

```sh
go build -o /tmp/mz ./cmd/minzc
go build -o /tmp/mza ./cmd/mza
go build -tags mzx_headless -o /tmp/mzx ./cmd/mzx
```

From the repository root, prepare the image, native launcher and data disk:

```sh
python3 examples/abap/fleet-zx/build.py --out /tmp/fleet-zx --mz /tmp/mz --mza /tmp/mza
/tmp/mzx --model pentagon --rom /tmp/fleet-zx/EMPTY.ROM \
  --snapshot /tmp/fleet-zx/FLEET.sna --trd /tmp/fleet-zx/FLEET.trd \
  --key-script /tmp/fleet-zx/keys.json --frames 500 \
  --dump-frames /tmp/fleet-zx/frames --dump-scr /tmp/fleet-zx/scr \
  --save-snapshot /tmp/fleet-zx/after.sna
```

`EMPTY.ROM` is 16 KiB of zeros, generated locally. This demo executes no system
or DOS ROM. The `.sna` contains a native launcher below the application's
0x6000 origin. On return it stores status at 0x5B20, changes the border to white on success (red on fault),
and halts at 0x5B1B. This is a launcher return, not a DOS command prompt.

For interactive use, open the same snapshot and disk with the GUI build of
MZX. Native **F** invokes the report's value-request event; **2** chooses
FLEET.CSV, **Enter** accepts it, the next **Enter** runs the report, and **Q**
returns. PC arrow keys map to Spectrum Caps Shift+6/7 for row navigation. F3/F4
retain the emulator's turbo controls. The supplied key script drives these
same native keyboard-matrix transitions; it supplies no file or table results.

Expected rows: Atlas/North/40 and Beacon/South/32; total seats 72. Character
columns display their first eight characters on the 32×24 screen; the full
20-character values remain in memory. Signed integer columns retain eleven
cells. This bounded picker supports native menu keys, not a general filename
editor or arbitrary SAP GUI.

The disk uses the established three-character extension convention: descriptor
bytes 8..10 contain `CSV`, repurposing the former load-address bytes. File data
is read into a compiler-owned buffer, never at that descriptor address. Standard
TRD logical tracks interleave heads; the adapter translates each sector to
MZX's legacy head-grouped trap coordinates and validates the full file extent.

[Technical report and genuine capture](../../../reports/native-abap/2026-10-18-zx-trdos-csv-alv.md).
