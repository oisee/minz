"""Build the offline report from the checked-in experiment receipts."""
from pathlib import Path
import json, html
root = Path(__file__).resolve().parents[2]
evidence = root / 'research/crystal-hir/evidence'
receipts = {str(i): json.loads((evidence / f'iteration{i}-results.json').read_text()) for i in range(4)}
template = Path(__file__).with_name('template.html').read_text()
output = template.replace('@@RECEIPTS@@', html.escape(json.dumps(receipts, ensure_ascii=False, indent=2)))
output = output.replace('@@ARENA_CASES@@', str(receipts['3']['cases_per_engine']))
Path(__file__).with_name('index.html').write_text(output)
