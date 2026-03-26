# Next Session Seed: ABAP main() Fix + LinkedIn Ship

**Birthday session delivered:** SQL on Z80, 3 ZX screenshots, Paper A draft.

---

## One Fix Away: ABAP main() on ZX Spectrum

VIR: 111/112 Z3. Only `main()` fails post-emit validation:
```
main(post-emit validation: 16-bit HL load clobbered before use at: LD H, 0)
```
Zero-extend-to-HL pattern (`LD H, 0`) overwrites 16-bit HL load.
VIR team working on pair aliasing constraint. When fixed → real SQLite ALV on ZX Spectrum.

## Working NOW

| Demo | Platform | Status |
|------|----------|--------|
| ZSQL `Alice\|30, Bob\|25` | CP/M (mze) | **Perfect** — 24/31 Z3 |
| SAP ALV (WRITE-only) | ZX (mzx) | **Perfect** — clean screenshot |
| ABAP PARAMETERS | CP/M (mze) | **Perfect** — interactive |
| Real SQLite data | ZX (mze headless) | **Data flows** — layout garbled |
| MZX SQLite ports | MZX | **Wired** — untested with clean display |

## Commands for Testing

```bash
# Clean ALV on MZX (WRITE-only, works)
mzx --run mara_alv_zx.bin@8000 --frames DI:HALT --screenshot alv.png

# ZSQL on CP/M (real SQL, works)
printf "CREATE TABLE t(x TEXT)\nINSERT INTO t VALUES('hello')\nSELECT * FROM t\n.quit\n" | mze ZSQL.COM -t cpm

# Interactive ABAP on CP/M (works)
echo "Alice\n5" | mze showcase.com -t cpm

# Real SQLite on ZX (data flows, layout garbled)
echo "" | timeout 15 mze mara_real_zx.bin -t spectrum --profile /tmp/zx.json
python3 tools/render_zx_screen.py /tmp/zx.json screenshot.png
```

## Ship

- LinkedIn post: `docs/linkedin-post-abap-zx.md` — ready with screenshots + ASM
- README: ZSQL MARA + ALV screenshots in hero section
- Paper A: `research/paper-a-draft.md` — reviewed by GPT-5.4
