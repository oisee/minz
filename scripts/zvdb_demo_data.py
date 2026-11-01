#!/usr/bin/env python3
"""zvdb demo data builder — TSV embeddings -> Nanz data module + top-K reference.

Reads a `text<TAB>space separated floats` TSV (zvdb sample_embeddings.tsv,
1536-dim float embeddings), quantises every embedding to 256 bits with fixed-seed
random-hyperplane hashing — bit i = sign(dot(v, h_i)) over 256 hyperplanes, as
the ABAP zvdb library does — and emits:

  examples/nanz/zvdb_demo_data.nanz       Nanz globals: N, label offsets,
                                          label blob (NUL-terminated labels),
                                          32-byte quantised vectors
  examples/nanz/zvdb_demo_expected.txt    Python top-K reference for every
                                          record, using exactly the same score
                                          and tie-break as the Z80 code

The full reference table is also printed to stdout (redirect it to the file with
--stdout-ref if you want to inspect it).  Summary progress goes to stderr.

Score = number of equal bits = 256 - popcount(a XOR b).  Ranking is descending
by score, ties broken by ascending record index, so the Z80 and Python orders
are bit-identical.

Stdlib only (no numpy): the 35 x 256 dot products of length 1536 take about a
second.  The hyperplane PRNG is a self-contained xorshift64* + Box-Muller so the
output is reproducible on any Python version.
"""

from __future__ import annotations

import argparse
import math
import sys
from operator import mul
from pathlib import Path

BITS = 256                    # random hyperplanes == quantised bits
VEC_BYTES = BITS // 8         # 32 bytes per record
SEED = 0x5A5A5A5A             # fixed, so the data module is reproducible
MASK64 = (1 << 64) - 1

REPO = Path(__file__).resolve().parent.parent
DEFAULT_TSV = REPO.parent / "zvdb-ref" / "sample_embeddings.tsv"
DEFAULT_DATA = REPO / "examples" / "nanz" / "zvdb_demo_data.nanz"
DEFAULT_REF = REPO / "examples" / "nanz" / "zvdb_demo_expected.txt"


class XorShift64:
    """SplitMix-style seed then xorshift64* — stable across Python versions."""

    def __init__(self, seed: int) -> None:
        self.s = seed & MASK64

    def next_u64(self) -> int:
        x = self.s
        x ^= (x >> 12) & MASK64
        x = (x ^ ((x << 25) & MASK64)) & MASK64
        x ^= (x >> 27) & MASK64
        self.s = x
        return (x * 2685821657736338717) & MASK64

    def unit(self) -> float:
        """Uniform in (0, 1)."""
        return (self.next_u64() + 0.5) / 18446744073709551616.0

    def normal(self) -> float:
        """Standard normal via Box-Muller (no cached state -> no call-order bugs)."""
        return math.sqrt(-2.0 * math.log(self.unit())) * math.cos(2.0 * math.pi * self.unit())


def read_tsv(path: Path) -> list[tuple[str, list[float]]]:
    records: list[tuple[str, list[float]]] = []
    with path.open("r", encoding="utf-8") as fh:
        for lineno, line in enumerate(fh, 1):
            line = line.rstrip("\r\n")
            if not line.strip():
                continue
            text, tab, rest = line.partition("\t")
            if not tab:
                raise SystemExit(f"{path}:{lineno}: expected text<TAB>floats")
            vec = [float(tok) for tok in rest.split()]
            if len(vec) != 1536:
                raise SystemExit(f"{path}:{lineno}: {len(vec)} floats, expected 1536")
            text = text.strip()
            if not text or not all(32 <= ord(c) < 127 for c in text):
                raise SystemExit(f"{path}:{lineno}: label must be printable ASCII")
            records.append((text, vec))
    if not records:
        raise SystemExit(f"{path}: no records")
    if len(records) > 255:
        raise SystemExit(f"{path}: {len(records)} records, the kernel ranks at most 255 "
                         "(zv_rank_cap in zvdb_kernel.nanz, and zv_n is a u8)")
    return records


def hyperplanes(dim: int, count: int, seed: int) -> list[list[float]]:
    rng = XorShift64(seed)
    return [[rng.normal() for _ in range(dim)] for _ in range(count)]


def quantise(vec: list[float], planes: list[list[float]]) -> bytes:
    """bit i (byte i>>3, bit i&7) = 1 iff dot(vec, h_i) > 0."""
    out = bytearray(VEC_BYTES)
    for i, plane in enumerate(planes):
        if sum(map(mul, vec, plane)) > 0.0:
            out[i >> 3] |= 1 << (i & 7)
    return bytes(out)


def popcount(b: int) -> int:
    return bin(b).count("1")


def score(a: bytes, b: bytes) -> int:
    """Equal bits = 256 - hamming distance. Same value the Z80 computes."""
    return BITS - sum(popcount(x ^ y) for x, y in zip(a, b))


def rank_all(vectors: list[bytes]) -> list[list[tuple[int, int]]]:
    """Full ranking of the *other* records for every record.

    Descending score, ties by ascending index — identical to the Z80 top_k.
    """
    out = []
    for i, vec in enumerate(vectors):
        rows = [(score(vec, vectors[j]), j) for j in range(len(vectors)) if j != i]
        rows.sort(key=lambda r: (-r[0], r[1]))
        out.append(rows)
    return out


def nanz_literal(values, per_line: int = 16) -> str:
    lines = []
    for start in range(0, len(values), per_line):
        lines.append("    " + ", ".join(str(v) for v in values[start:start + per_line]) + ",")
    return "\n".join(lines)


def emit_data_module(records, vectors) -> str:
    offsets = [0]
    labels: list[int] = []
    for text, _ in records:
        labels.extend(ord(c) for c in text)
        labels.append(0)
        offsets.append(len(labels))
    n = len(records)
    flat = [b for vec in vectors for b in vec]
    assert len(flat) == n * VEC_BYTES

    out = []
    out.append("// zvdb_demo_data.nanz — zvdb demo data module (GENERATED, do not edit)")
    out.append("//")
    out.append("// Written by scripts/zvdb_demo_data.py from zvdb-ref/sample_embeddings.tsv.")
    out.append("// Quantisation: 256 random hyperplanes, bit i = sign(dot(v, h_i)),")
    out.append("//               packed LSB-first into 32 bytes per record (byte i>>3, bit i&7).")
    out.append(f"// Seed: {SEED} (0x{SEED:08X}).  Regenerate with:")
    out.append("//   python3 scripts/zvdb_demo_data.py")
    out.append("")
    out.append(f"const zv_n: u8 = {n}                        // record count")
    out.append(f"const zv_vec_bytes: u8 = {VEC_BYTES}                  // 256 bits per vector")
    out.append("")
    out.append("// label i is zv_labels[zv_label_off[i] .. zv_label_off[i+1]-1], NUL terminated")
    out.append(f"global zv_label_off: [u16; {n + 1}] = [")
    out.append(nanz_literal(offsets))
    out.append("]")
    out.append("")
    out.append(f"global zv_labels: [u8; {len(labels)}] = [")
    out.append(nanz_literal(labels, 20))
    out.append("]")
    out.append("")
    out.append(f"global zv_vecs: [u8; {len(flat)}] = [")
    out.append(nanz_literal(flat, 16))
    out.append("]")
    out.append("")
    return "\n".join(out)


def emit_reference(records, ranked) -> str:
    out = []
    out.append("# zvdb top-K reference — generated by scripts/zvdb_demo_data.py")
    out.append(f"# seed={SEED} bits={BITS} records={len(records)}")
    out.append("# score(i,j) = 256 - popcount(vec_i XOR vec_j);")
    out.append("# ranking: descending score, ties by ascending record index")
    out.append("# columns: sel <TAB> rank <TAB> idx <TAB> score <TAB> label")
    for i, (text, _) in enumerate(records):
        for rank, (s, j) in enumerate(ranked[i]):
            out.append(f"{i}\t{rank}\t{j}\t{s}\t{records[j][0]}")
        if not ranked[i]:
            out.append(f"{i}\t-\t-\t-\t{text}")
    return "\n".join(out) + "\n"


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--tsv", type=Path, default=DEFAULT_TSV, help="input TSV (default: %(default)s)")
    ap.add_argument("--out-data", type=Path, default=DEFAULT_DATA, help="Nanz data module to write")
    ap.add_argument("--out-ref", type=Path, default=DEFAULT_REF, help="reference table to write")
    ap.add_argument("--stdout-ref", action="store_true",
                    help="also print the full reference table to stdout")
    ap.add_argument("--suggest-asserts", action="store_true",
                    help="print suggested Nanz assert lines for the demo (stdout)")
    args = ap.parse_args()

    records = read_tsv(args.tsv)
    print(f"{args.tsv}: {len(records)} records, 1536 floats each", file=sys.stderr)

    planes = hyperplanes(1536, BITS, SEED)
    vectors = [quantise(vec, planes) for _, vec in records]
    bits_set = sum(popcount(b) for v in vectors for b in v)
    print(f"quantised to {BITS} bits (seed {SEED}): mean {bits_set / (len(records) * BITS):.1f} bits set per record",
          file=sys.stderr)

    ranked = rank_all(vectors)

    data = emit_data_module(records, vectors)
    args.out_data.write_text(data, encoding="utf-8")
    print(f"wrote {args.out_data} ({len(data)} bytes)", file=sys.stderr)

    ref = emit_reference(records, ranked)
    args.out_ref.write_text(ref, encoding="utf-8")
    print(f"wrote {args.out_ref} ({len(ref.splitlines())} lines)", file=sys.stderr)

    if args.stdout_ref:
        sys.stdout.write(ref)

    for i, (text, _) in enumerate(records):
        s, j = ranked[i][0]
        print(f"  [{i:2d}] best -> [{j:2d}] score {s:3d}  {records[j][0][:52]}", file=sys.stderr)

    if args.suggest_asserts:
        i = 0
        s0, j0 = ranked[i][0]
        s1, j1 = ranked[i][1]
        last = ranked[i][-1]
        print("// paste into examples/nanz/zvdb_demo_cpm.nanz (values from the real data)")
        print(f"assert zv_score(0, 1) == {score(vectors[0], vectors[1])} via mir2")
        print(f"assert zv_score(0, 1) == {score(vectors[0], vectors[1])} via z80")
        print(f"assert zv_score(0, {j0}) == {s0} via mir2")
        print(f"assert zv_score(0, {j0}) == {s0} via z80")
        print(f"assert zv_top_count(0) == {len(records) - 1} via mir2")
        print(f"assert zv_top_count(0) == {len(records) - 1} via z80")
        print(f"assert zv_top_index(0, 0) == {j0} via mir2")
        print(f"assert zv_top_index(0, 0) == {j0} via z80")
        print(f"assert zv_top_score(0, 0) == {s0} via mir2")
        print(f"assert zv_top_score(0, 0) == {s0} via z80")
        print(f"assert zv_top_index(0, 1) == {j1} via mir2")
        print(f"assert zv_top_index(0, 1) == {j1} via z80")
        print(f"assert zv_top_score(0, {len(ranked[i]) - 1}) == {last[0]} via mir2")
        print(f"assert zv_top_score(0, {len(ranked[i]) - 1}) == {last[0]} via z80")
        print(f"assert zv_top_index(0, {len(ranked[i]) - 1}) == {last[1]} via mir2")
        print(f"assert zv_top_index(0, {len(ranked[i]) - 1}) == {last[1]} via z80")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
