# The Hobbit (1982) — Binary Analysis with mzd

**Date:** 2026-03-15
**Binary:** The Hobbit v1.2 (Melbourne House, 1982, ZX Spectrum 48K)
**Tools:** mzd --regs --dynamic --sym, mze emulator, SkoolKit cross-reference
**Source:** Z80 snapshot extracted via SkoolKit tap2sna.py

---

## Overview

Ran our full mzd analysis pipeline on The Hobbit — a legendary 1982 text adventure with graphics, featuring one of the first natural language parsers ("Inglish") in a video game. The game fits in 48K and contains a complete text parser, graphics engine, AI system, and game world.

```bash
# Extract from TZX → Z80 snapshot → raw memory
tap2sna.py @hobbit.t2s

# Convert SkoolKit labels to mzd .sym format
python3 skool2sym.py > hobbit.sym

# Full analysis
mzd --regs --dynamic --trials 16 -t spectrum -o '$4000' \
    --entry '$6C00' --sym hobbit.sym hobbit_48k.bin
```

---

## Binary Statistics

| Metric | Value |
|--------|-------|
| Total size | 49,152 bytes (48K) |
| Code | 9,515 bytes (19.4%) |
| Data | 92 bytes (0.2%) |
| Strings | 1,483 bytes (3.0%) |
| Undefined | 38,062 bytes (77.4%) — packed data, graphics, word tables |
| **Functions detected** | **154** |
| **Pure functions** | **56** (36%) |
| **Idempotent** | **41** (27%) |
| **Involutions** | **6** |
| **Timeouts** | 92 (mostly due to game loop / LDIR block copies) |
| Spectrum ROM calls | 95 (93 RST $38 interrupt, 2 FP calculator) |
| Detected strings | 185 |

---

## Memory Map

```
$4000-$57FF  Loading screen (pixel data, 6144 bytes)
$5800-$5AFF  Loading screen (attributes, 768 bytes)
$5B00-$5EFF  System variables / stack area
$5F00-$5FFF  Decompressed location data (runtime)
$6000-$67AB  Word Index — Inglish dictionary
             A-Z letter groups, each word tagged with part of speech:
             NOUN, VERB, ADJECTIVE, ADVERB, MOVE_DIR, PREPOSITION
$67AC-$6BFF  Packed game data
$6C00-$A3FF  Game code (main engine, ~14KB)
$A400-$AFFF  Message/text data (compressed)
$B000-$C7FF  Game world data (objects, locations, events)
$C800-$CBFF  AI scripts (Gandalf, Thorin behaviors)
$CC00-$CCFF  Location graphics table
$CC43+       Location graphics data (vector-style)
$F400-$F9FF  Copy of objects table (runtime)
$FA15-$FFFF  Copy of locations table (runtime)
```

---

## Key Findings

### 1. Inglish Natural Language Parser

The text parser lives in the $6E00-$7300 region. Our analysis found:

**sub_6F30 — Punctuation Parser** (pure, 90T)
```
IN: HL (text pointer), A (character)
OUT: HL (advanced), BC (0), A (token type), F

  if A == '.' → token $B0 (end of sentence)
  if A == ',' → token $A0 (comma pause)
  if A == '"' → token $90 (speech mark)
  else → RET NZ (not punctuation)
```

**sub_6F76 — Word Scanner** (IN: HL, OUT: HL, A, B)
Walks text buffer scanning for word boundaries.

**sub_6FBA — Input Reset** (pure, 102T)
Resets the parser state for new input.

**String at $6FF2:** `"> LOOK\r"` followed by 128 spaces — this is the input buffer template (auto-types "LOOK" on game start).

### 2. Word Dictionary ($6040-$67AB)

The Inglish dictionary is organized alphabetically with part-of-speech tags:
```
$6040: A (ADJECTIVE)     — "a", "an", "around", "at", "attack"...
$607C: B (ADJECTIVE)     — "beautiful", "big", "black"...
$60E2: C (NOUN)          — "chest", "coins", "cup"...
$617C: D (MOVE_DIR)      — "down", "drink"...
$61F6: E (MOVE_DIR)      — "east", "eat", "enter"...
$624F: F (VERB)          — "fight", "fill", "find"...
$62BD: G (NOUN)          — "gandalf", "goblin", "gold"...
$6309: H (NOUN)          — "hobbit", "house"...
```

Each entry: first byte = word ID + type, followed by compressed word text (bit-7 terminated).

### 3. Graphics Engine ($7F78-$8251)

**Drawing ($7F78)** — the main graphics renderer. mzd detected it but it timeouts because it writes to screen memory ($4000-$5AFF).

**DrawingPaper ($8251)** — paper/background renderer.

**LocGFXTable ($CC00)** — lookup table mapping location IDs to graphics data at $CC43+. Graphics appear to be vector-style (line drawing commands, not bitmaps).

### 4. Action System ($8C4B-$9FFF)

Named action handlers found via SkoolKit cross-reference:

| Address | Label | mzd Analysis |
|---------|-------|-------------|
| $8C4B | Action_Look | IN: complex, TIMEOUT |
| $8CA6 | Action_Putdown | TIMEOUT (writes objects) |
| $8D33 | Action_Pickup | TIMEOUT |
| $8D9D | Action_Dir | Movement handler |
| $8FAD | Action_Run | TIMEOUT |
| $90D2 | Action_Open | TIMEOUT |
| $96B3 | Action_Eat | TIMEOUT |
| $9918 | Action_Break | pure, involution! |
| $9DD9 | Action_Climb | TIMEOUT |
| $9EA0 | Action_Attack | TIMEOUT |

**Interesting:** `Action_Break ($9918)` is detected as **pure + involution** — `break(break(x)) == x`. This makes sense if "break" toggles an object state.

### 5. AI / Event Scripts ($C700-$C913)

```
$C7B9  EventDeepBog        — Bog event handler
$C7C0  EventElvenkingsCellar — Elvenking's cellar event
$C7DD  EventForest          — Forest random events
$C7EA  EventForestriver     — River events
$C8E2  ScriptGandalf_5      — Gandalf AI behavior
$C913  ScriptThorin_1       — Thorin AI behavior
```

These are the famous "animaction" scripts — Melbourne House's AI system that gives NPCs independent behavior. Thorin famously "sits down and starts singing about gold."

### 6. Pure Functions — Superoptimizer Candidates

56 functions detected as pure (no memory writes, deterministic). Top candidates for z80-optimizer whole-function replacement:

| Function | IN | OUT | Cycles | Property |
|----------|-----|------|--------|----------|
| Blanker ($70E2) | HL, A, B | — | 30T | pure, idempotent, involution |
| IndexAction ($70E8) | A | HL, DE, F | 47T | pure, idempotent |
| sub_785F ($785F) | A | A, F | 26T | pure, idempotent, involution |
| sub_75F1 ($75F1) | BC, DE | A, F | 26T | pure, idempotent, involution |
| sub_6F30 ($6F30) | HL, A | HL, BC, A, F | 90T | pure |
| sub_6FBA ($6FBA) | — | HL, DE, A, F | 102T | pure |

These could be fed to `z80opt stoke` to search for shorter equivalent implementations — proving that 1982 Melbourne House code could be tightened by modern brute-force search.

---

## Data Formats Identified

### Strings
- **Null-terminated ($00):** Standard C-style, used in input buffer
- **Bit-7 terminated:** High bit set on last character — compact, used for dictionary words
- **$-terminated ($24):** Not found (CP/M style, not used)
- **CR-terminated ($0D):** Used in display output

### Compressed Text
Messages at $A400+ appear to use token compression — common words replaced by single-byte codes, expanded at runtime by `PrintMsg` ($72DD).

### Graphics
Location graphics at $CC43+ use a compact vector format — drawing commands rather than pixel bitmaps. This is how Melbourne House fit 30+ illustrated locations into ~3KB.

---

## Symbol File

Generated 95-label `.sym` file from SkoolKit disassembly:
```
; The Hobbit (1982) symbols from SkoolKit disassembly
6C00 Start
6CCD SquiggleLine
6FD3 ClearScreen
70E2 Blanker
70E8 IndexAction
72CE ICannotDoThat
72DD PrintMsg
7F78 Drawing
8C4B Action_Look
...
```

Loadable via `mzd --sym hobbit.sym`.

---

## What This Demonstrates

1. **mzd scales to real-world 48K binaries** — 154 functions detected, 185 strings, 95 ROM call annotations
2. **Dynamic analysis works on 40-year-old code** — 56 pure functions identified, including parts of the Inglish parser
3. **Cross-referencing with existing disassemblies** — .sym import connects human-authored labels with automated analysis
4. **The pipeline is ready for serious reverse engineering** — static regs + dynamic properties + ABI verification + symbol loading

### Formats mzd supports for saving/loading analysis

| Format | Flag | Content |
|--------|------|---------|
| `.sym` | `--sym` / `--export-sym` | Label → address mapping |
| `.mzp` | `--project` | Full analysis state (JSON) |
| `.abi` | `--abi` / `--export-abi` | Platform syscall definitions |
| `.a80` | `--verify-abi` | Compiler ABI comments (MinZ) |

For external tools: SkoolKit `.skool` → convert to `.sym` with simple script.
