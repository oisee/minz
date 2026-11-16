#!/usr/bin/env python3
"""Check the data generator's record cap end to end at the kernel's limit (255).

Synthesises a TSV of N records by rotating the real embeddings, runs
scripts/zvdb_demo_data.py on it into a scratch dir, and compiles a sampling dump
driver against the *generated* module so the Z80 kernel's top-K can be compared
with an independent Python scorer that reads the generated .nanz back. This
exercises the generator, not just the kernel.

  python3 reports/zvdb-demo-cpm/repro/check-cap-255.py 255

Needs the zvdb-ref sample TSV next to the repo (the same input
scripts/zvdb_demo_data.py reads) and minzc/{mz,mze} built.
"""
#!/usr/bin/env python3

import pathlib
import subprocess
import sys

REPO = pathlib.Path(__file__).resolve().parents[3]
SRC_TSV = REPO.parent / "zvdb-ref" / "sample_embeddings.tsv"   # outside the repo
WORK = pathlib.Path("/tmp/zvdb-cap-check")
POP = [bin(i).count("1") for i in range(256)]


def synth(n, path):
    base = [l for l in SRC_TSV.read_text().splitlines() if l.strip()]
    text, vec = base[0].split("\t")
    vals = vec.split()
    with path.open("w") as fh:
        for i in range(n):
            # rotate the embedding so each record differs but stays well-formed
            k = (i * 37) % len(vals)
            fh.write(f"rec {i}\t" + " ".join(vals[k:] + vals[:k]) + "\n")


def parse_module(path):
    src = path.read_text()
    def block(name):
        i = src.index(f"global {name}:")
        i = src.index("= [", i) + 3
        j = src.index("]", i)
        return [int(x) for x in src[i:j].replace("\n", " ").split(",") if x.strip()]
    n = int(src.split("const zv_n: u8 = ")[1].split()[0])
    return n, block("zv_labels"), block("zv_vecs")


def reference(vecs, n, sel, ranks):
    rows = []
    for j in range(n):
        if j == sel:
            continue
        ham = sum(POP[a ^ b] for a, b in zip(vecs[sel * 32:(sel + 1) * 32], vecs[j * 32:(j + 1) * 32]))
        rows.append((256 - ham, j))
    rows.sort(key=lambda r: (-r[0], r[1]))
    return rows[:ranks]


DRIVER = """import tui.render
import zvdb_demo_data
import zvdb_kernel

global numbuf: [u8; 6]

fun emit_num(v: u16) -> void {
    var k: u8 = zv_fmt_u16(v, &numbuf)
    var i: u8 = 0
    while i < k {
        tui_putch(numbuf[i])
        i = i + 1
    }
}

fun emit(sel: u8, rank: u8) -> void {
    emit_num(u16(sel))
    tui_putch(9)
    emit_num(u16(rank))
    tui_putch(9)
    emit_num(u16(zv_rank_index(rank)))
    tui_putch(9)
    emit_num(zv_rank_score(rank))
    tui_putch(13)
    tui_putch(10)
}

fun main() -> void {
%s
}
"""

SELECTIONS = [0, 64, 128, 200, 254]
RANKS = 5


def main():
    WORK.mkdir(parents=True, exist_ok=True)
    n = int(sys.argv[1]) if len(sys.argv) > 1 else 255
    tsv = WORK / f"synth-{n}.tsv"
    synth(n, tsv)

    r = subprocess.run(["python3", str(REPO / "scripts/zvdb_demo_data.py"),
                        "--tsv", str(tsv), "--out-data", str(WORK / "zvdb_demo_data.nanz"),
                        "--out-ref", str(WORK / "ref.txt")],
                       capture_output=True, text=True)
    if r.returncode:
        sys.exit(r.stderr[-1500:])
    print(f"[cap] generator accepted {n} records")

    got_n, labels, vecs = parse_module(WORK / "zvdb_demo_data.nanz")
    if got_n != n:
        sys.exit(f"module declares zv_n = {got_n}, want {n}")

    # the reference file must have n*(n-1) rows
    rows = [l for l in (WORK / "ref.txt").read_text().splitlines() if l and not l.startswith("#")]
    print(f"[cap] reference rows: {len(rows)} (want {n * (n - 1)})")
    if len(rows) != n * (n - 1):
        sys.exit("reference row count mismatch")

    for f in ("zvdb_kernel.nanz",):
        (WORK / f).write_text((REPO / "examples/nanz" / f).read_text())
    body = []
    for sel in SELECTIONS:
        if sel >= n:
            continue
        body.append(f"    zv_top_k({sel})")
        for rank in range(RANKS):
            body.append(f"    emit({sel}, {rank})")
    (WORK / "driver.nanz").write_text(DRIVER % "\n".join(body))

    for tool, args in ((REPO / "minzc/mz", [str(WORK / "driver.nanz"), "-t", "cpm", "-o", str(WORK / "d.com")]),):
        p = subprocess.run([str(tool)] + args, capture_output=True, text=True)
        if p.returncode:
            sys.exit(p.stderr[-1500:])
    p = subprocess.run([str(REPO / "minzc/mze"), "-t", "cpm", "--cpm-no-echo", str(WORK / "d.com")],
                       capture_output=True)
    lines = [l for l in p.stdout.decode("latin1").replace("\r\n", "\n").split("\n") if l]
    bad = 0
    for i, sel in enumerate(s for s in SELECTIONS if s < n):
        want = reference(vecs, n, sel, RANKS)
        for rank in range(RANKS):
            got = lines[i * RANKS + rank].split("\t")
            exp = [str(sel), str(rank), str(want[rank][1]), str(want[rank][0])]
            if got != exp:
                print(f"  MISMATCH sel {sel} rank {rank}: z80 {got} want {exp}")
                bad += 1
    print(f"[cap] Z80 top-K vs independent scorer at n={n}: "
          f"{len([s for s in SELECTIONS if s < n])} selections x {RANKS} ranks, {bad} mismatches")
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
