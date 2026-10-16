#!/usr/bin/env python3
"""Render the published benchmark JSON as dependency-free SVGs and a winner matrix."""
from __future__ import annotations

import hashlib
import html
import json
import math
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
DATA = ROOT / "reports/c89-full-benchmark/summary.json"
OUT = DATA.parent / "charts"
REPORT = ROOT / "reports/2026-10-16-c89-nanz-full-benchmark.md"
CASES = ("abs_diff", "gcd", "sum_array", "copy_bytes", "fat12_decode", "struct_roundtrip")
LABELS = {"minz": "MinZ C", "sdcc": "SDCC 4.2", "sdcc45": "SDCC 4.5",
          "sccz80": "sccz80", "nanz": "Nanz", "nanz-tuple": "Nanz tuple"}
C_ROWS = tuple((c, p) for c, ps in (("minz", ("default", "size", "grace")),
               ("sdcc", ("size", "speed")), ("sdcc45", ("size", "speed")),
               ("sccz80", ("o3",))) for p in ps)
N_ROWS = tuple((c, p) for c in ("nanz", "nanz-tuple")
               for p in ("default", "size", "grace"))
START = "<!-- BEGIN GENERATED BENCHMARK CHARTS -->"
END = "<!-- END GENERATED BENCHMARK CHARTS -->"


def label(key: tuple[str, str]) -> str:
    return f"{LABELS[key[0]]} {key[1]}"


def color(ratio: float) -> str:
    # Every improvement is visibly green, including ratios just below 100%.
    # Strength still increases logarithmically, saturating at half the reference.
    value = max(-1.0, min(1.0, math.log2(ratio)))
    if value < 0:
        target, strength = (94, 184, 125), 0.60 + 0.40 * abs(value)
    else:
        target, strength = (239, 170, 165), abs(value)
    return "#" + "".join(f"{round(248 + (v - 248) * strength):02x}" for v in target)


def render(rows, title, name, index, digest):
    width, row_h, left, cell_w = 1080, 44, 240, 136
    panel_h = 98 + len(rows) * row_h
    height = 92 + 2 * panel_h + 54
    pieces = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" viewBox="0 0 {width} {height}" role="img" aria-labelledby="title desc">',
              f'<title id="title">{html.escape(title)}</title>',
              f'<desc id="desc">Each cell is a percentage of SDCC 4.5 Size bytes or Speed T-states for the same workload. Lower is better. E means compilation error. A dash means no variant. Source SHA256: {digest}.</desc>',
              '<rect width="100%" height="100%" fill="white"/>',
              '<style>text{font-family:DejaVu Sans,Arial,sans-serif;fill:#172b3a} .cell{font-size:18px;font-weight:600} .small{font-size:14px} .label{font-size:17px}</style>',
              f'<text x="20" y="32" font-size="23" font-weight="600">{html.escape(title)}</text>',
              '<text x="20" y="59" class="small">Lower is better. Every value below 100% is green; red: above; 100%: equal.</text>']
    accounted = set()
    for panel, (metric, profile, heading) in enumerate((
            ("ProgramBytes", "size", "Whole-module bytes / SDCC 4.5 Size = 100%"),
            ("TotalT", "speed", "Workload T-states / SDCC 4.5 Speed = 100%"))):
        top = 92 + panel * panel_h
        pieces.append(f'<text x="20" y="{top}" font-size="20" font-weight="600">{heading}</text>')
        for col, case in enumerate(CASES):
            x = left + col * cell_w
            pieces.append(f'<text x="{x + cell_w / 2}" y="{top + 31}" text-anchor="middle" class="small">{case}</text>')
        for row, key in enumerate(rows):
            y = top + 45 + row * row_h
            pieces.append(f'<text x="20" y="{y + 28}" class="label">{html.escape(label(key))}</text>')
            for col, case in enumerate(CASES):
                x = left + col * cell_w
                item = index.get((*key, case))
                if item is None:
                    text, fill, tip = "—", "#ededed", "Variant not present in this benchmark"
                elif item["Status"] != "PASS":
                    text, fill, tip = "E", "#d9dfe7", item["Status"]
                    accounted.add((*key, case))
                else:
                    ref = index[("sdcc45", profile, case)][metric]
                    ratio = item[metric] / ref
                    text, fill = f"{100 * ratio:.1f}%", color(ratio)
                    tip = f"{item[metric]:,} / {ref:,} = {100 * ratio:.6f}%"
                    accounted.add((*key, case))
                pieces.append(f'<g><title>{html.escape(tip)}</title><rect x="{x}" y="{y}" width="{cell_w - 4}" height="{row_h - 4}" rx="4" fill="{fill}"/><text x="{x + (cell_w - 4) / 2}" y="{y + 27}" text-anchor="middle" class="cell">{text}</text></g>')
    pieces.append(f'<text x="20" y="{height - 22}" class="small">E = compilation error; — = no variant. Bare Z80, fixed inputs; no ULA contention.</text></svg>')
    (OUT / name).write_text("\n".join(pieces) + "\n")
    return accounted


def winner(index, case, metric, rows):
    candidates = [(key, index[(*key, case)]) for key in rows
                  if (*key, case) in index and index[(*key, case)]["Status"] == "PASS"]
    best = min(item[metric] for _, item in candidates)
    groups = {}
    for key, item in candidates:
        if item[metric] == best:
            groups.setdefault(key[0], []).append(key[1])
    names = " / ".join(f"{LABELS[c]} {'/'.join(profiles)}" for c, profiles in groups.items())
    return f"{names}: **{best:,}**".replace(",", " ")


def main():
    data = json.loads(DATA.read_text())
    index = {(r["Compiler"], r["Profile"], r["Case"]): r for r in data}
    assert len(index) == len(data) == 69, "Unexpected or duplicate benchmark records"
    assert sum(r["Status"] == "PASS" for r in data) == 66
    assert sum(r["Status"] == "COMPILE_ERROR" for r in data) == 3
    for case in CASES:
        same_c = [r for r in data if r["Case"] == case and r["Compiler"] in
                  ("minz", "sdcc", "sdcc45", "sccz80")]
        assert len({r["SourceSHA256"] for r in same_c}) == 1
        all_case = [r for r in data if r["Case"] == case and r["Status"] == "PASS"]
        assert len({r["WorkloadSHA256"] for r in all_case}) == 1
        assert len({r["Calls"] for r in all_case if r["Status"] == "PASS"}) == 1
    digest = hashlib.sha256(DATA.read_bytes()).hexdigest()
    OUT.mkdir(exist_ok=True)
    covered = render(C_ROWS, "Identical C: every compiler/profile", "identical-c.svg", index, digest)
    covered |= render(N_ROWS, "Idiomatic Nanz / tuple: separate sources", "idiomatic-nanz.svg", index, digest)
    assert covered == set(index), "Charts must account for every recorded result"
    manifest_path = DATA.parent / "manifest.json"
    manifest = [entry for entry in json.loads(manifest_path.read_text())
                if not entry["path"].startswith("charts/")]
    assert next(entry["sha256"] for entry in manifest if entry["path"] == "summary.json") == digest
    for chart in sorted(OUT.glob("*.svg")):
        raw = chart.read_bytes()
        manifest.append({"path": str(chart.relative_to(DATA.parent)),
                         "sha256": hashlib.sha256(raw).hexdigest(), "bytes": len(raw)})
    manifest_path.write_text(json.dumps(manifest, indent=2) + "\n")
    lines = [START, "## Кто где выигрывает: диаграммы всей матрицы", "",
             "Каждая колонка — одна замороженная нагрузка. **Меньше — лучше**: 80% означает",
             "на 20% меньше, 120% — на 20% больше. Верхняя панель показывает байты полного",
             "модуля относительно **SDCC 4.5 Size**, нижняя — такты относительно **SDCC 4.5 Speed**.",
             "Эти два эталона выбраны по критерию, поэтому их строки не обязаны быть лучшими",
             "даже среди профилей самого SDCC. Проценты между разными программами не суммируем.", "",
             "**Одинаковый C.** Все восемь compiler/profile-вариантов для каждого из шести",
             "неизменных C-исходников; три отказа aggregate-сборки показаны как `E`.", "",
             "![Матрица одинакового C: размер и такты](c89-full-benchmark/charts/identical-c.svg)", "",
             "**Идиоматичный Nanz — отдельные исходники.** Ожидаемые результаты и нагрузки те же,",
             "но типы и способ выразить алгоритм отличаются от C. Nanz tuple измерен только для",
             "struct_roundtrip; `—` означает отсутствующий вариант, а не проигрыш или ошибку.", "",
             "![Матрица отдельного Nanz и tuple: размер и такты](c89-full-benchmark/charts/idiomatic-nanz.svg)", "",
             "Ниже — минимум **среди показанных успешных сборок**, отдельно по размеру и времени.",
             "Все ничьи сохранены: разные критерии могут выбирать разные варианты.", "",
             "| Нагрузка | Одинаковый C: меньше байтов | Одинаковый C: меньше тактов | Отдельный Nanz/tuple: меньше байтов | Отдельный Nanz/tuple: меньше тактов |",
             "|---|---|---|---|---|"]
    for case in CASES:
        lines.append("| " + " | ".join((case, winner(index, case, "ProgramBytes", C_ROWS),
                     winner(index, case, "TotalT", C_ROWS), winner(index, case, "ProgramBytes", N_ROWS),
                     winner(index, case, "TotalT", N_ROWS))) + " |")
    lines += ["", "Таким образом, MinZ C берёт оба минимума на abs_diff и struct_roundtrip;",
              "на gcd Grace выигрывает по тактам, SDCC 4.5 Size — по байтам. На sum_array",
              "MinZ C меньше, SDCC 4.5 Speed быстрее; такая же развилка у copy. FAT12 здесь",
              "выигрывает SDCC 4.5 по обоим критериям, но с разными профилями. Это результаты",
              "этих шести программ.", "",
              "Все значения ниже 100% выделены заметным зелёным, включая небольшие выигрыши.",
              "Ничьи нейтральны; цвет насыщается при 50%/200%. Минимальная зелёная",
              "подсветка не означает одинаковый выигрыш: точное округлённое отношение — в каждой",
              "ячейке. Абсолютные цифры, ASM и ошибки сборки находятся в таблицах выше.",
              "Статические SVG и эта таблица генерируются из [summary.json](c89-full-benchmark/summary.json)",
              "командой `python3 research/c89-benchmark/plot_summary.py`. Генератор проверяет все",
              "69 записей, SHA одинакового C, SHA общих нагрузок и число вызовов, прежде чем",
              "строить картинки: [исходник генератора](../research/c89-benchmark/plot_summary.py).",
              "Измерения — bare Z80 без ULA contention; размер — полный измеряемый модуль,",
              "не только тело функции и не общий объём RAM/стека.", END]
    block = "\n".join(lines) + "\n"
    content = REPORT.read_text()
    if START in content:
        assert content.count(START) == content.count(END) == 1
        content = content[:content.index(START)] + block + content[content.index(END) + len(END):].lstrip("\n")
    else:
        content = content.rstrip() + "\n\n" + block
    REPORT.write_text(content)
    print(f"Verified {len(index)} records: 66 PASS, 3 COMPILE_ERROR; all records charted")
    print(f"summary SHA256: {digest}")


if __name__ == "__main__":
    main()
