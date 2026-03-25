# SAP ABAP on ZX Spectrum — LinkedIn Post Draft

---

## The Post

**TL;DR: We compiled ABAP to ZX Spectrum. SAP runs on a 1982 home computer. I am not sorry.**

Today I'm mass-releasing endorphins by showing you this:

`cl_salv_table=>factory( lt_mara ).`

Yes, that's a real SAP ALV factory call. No, it's not running on a 768GB HANA appliance. It's running on a Zilog Z80 CPU with 48KB of RAM. From 1982.

The full ABAP report:

```abap
REPORT zmara_alv_zx.

SELECT matnr mtart meins maktx
  FROM mara
  INNER JOIN makt
  ON mara~matnr = makt~matnr
  INTO TABLE @DATA(lt_mara).

cl_salv_table=>factory( lt_mara ).
```

Output — on ZX Spectrum:

```
=== Material Master (ALV) ===

MATNR     MTART MEINS MAKTX
--------- ----- ----- -----------
100-100   FERT  ST    Pump Assy
100-200   HALB  KG    Impeller
100-300   FERT  ST    Valve Unit
200-100   ROH   KG    Raw Steel
200-200   FERT  PC    Ctrl Panel

5 entries displayed.
```

![SAP Material Master ALV on ZX Spectrum](../media/mara_alv_zx_spectrum.png)

### How it works (seriously)

```
ABAP source
  → abaplint parser (TypeScript, by Lars Hvam Petersen)
  → JSON AST
  → Go semantic lowerer
  → HIR (High-level IR)
  → MIR2 (Mid-level IR)
  → VIR (Z3 SMT solver — joint instruction selection + register allocation)
  → Z80 assembly
  → MZA assembler
  → binary (2KB!)
  → ZX Spectrum Z80 CPU (3.5 MHz, 48KB RAM)
```

The compiler uses a Z3 SMT solver for register allocation. When Z3 is too slow, we fall back to a GPU-bruteforced lookup table: 315 precomputed optimal allocations that cover 97.8% of all Z80 register assignment patterns. Dual RTX 4060 Ti crunched 11.6 million assignments in 20 minutes. The compiler does O(1) lookup at compile time. Zero solver. Provably optimal.

The font? Embedded. 96 characters, 768 bytes. Renders directly to ZX Spectrum screen memory at $4000. No ROM needed.

### Reproduce it yourself

```bash
git clone https://github.com/oisee/minz
cd minz/minzc

# Build the toolchain
go build -o mz ./cmd/minzc
go build -o mza ./cmd/mza
go build -o mze ./cmd/mze

# Compile ABAP → Z80 → binary
./mz ../examples/abap/mara_alv_zx.abap \
  --target=spectrum -o /tmp/mara.a80
./mza /tmp/mara.a80 -o /tmp/mara.bin

# Run on ZX Spectrum emulator + screenshot
./mze /tmp/mara.bin -t spectrum \
  --profile /tmp/zx.json
pip install Pillow
python3 ../tools/render_zx_screen.py \
  /tmp/zx.json /tmp/mara_screenshot.png
```

### The MinZ Project

MinZ is a compiler for modern programming languages targeting Z80 and other retro CPUs. 8 frontends (Nanz, ABAP, C89, Frill/ML, PL/M, Pascal, Lanz, Lizp), one backend. The ABAP frontend is powered by Lars Hvam Petersen's [abaplint](https://github.com/abaplint/abaplint) — the same tool that lints your real SAP systems.

We also have ZSQL — an interactive SQLite client running on CP/M:

```
zsql> SELECT * FROM users
Alice | 30
Bob | 25
Charlie | 35
```

Real SQL. Real Z80. Real database (via I/O port bridge to SQLite).

GitHub: https://github.com/oisee/minz

Happy birthday to me. 🎂

#ABAP #SAP #ZXSpectrum #Z80 #RetroComputing #CompilerDesign #SQL #MinZ

---

## Screenshots

### ALV Grid + ABAP Source (primary)
![SAP Material Master ALV on ZX Spectrum](../media/mara_alv_zx_spectrum.png)

### SQL Session with MARA data (alternative)
![ZSQL MARA on ZX Spectrum](../media/zsql_mara_zx_spectrum.png)
