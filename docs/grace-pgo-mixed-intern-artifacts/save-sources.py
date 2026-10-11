from pathlib import Path
import gzip, shutil
root=Path('/tmp/minz-grace-pgo-resume/docs/grace-pgo-mixed-intern-artifacts');root.mkdir(exist_ok=True)
def copytree(src,dst):
 for p in Path(src).rglob('*'):
  if p.is_file():
   q=root/dst/p.relative_to(src);q.parent.mkdir(parents=True,exist_ok=True)
   if p.suffix=='.json' and p.stat().st_size>100000:q=q.with_suffix('.json.gz');q.write_bytes(gzip.compress(p.read_bytes(),mtime=0))
   else:shutil.copyfile(p,q)
copytree('/tmp/minz-mixed-intern-artifacts','selector')
copytree('/home/alice/dev/minz/artifacts/grace-reports/2026-10-10-resume/pending-intern-final','intern-baseline-history')
for name in ['minz-mixed-intern-final.txt','minz-mixed-final.txt','minz-mixed-caller.txt','minz-mixed-mutants.txt','minz-mixed-attribution-fixed.txt','minz-mixed-packages-parent-final.txt','minz-mixed-cli-parent-final.txt','minz-mixed-lineage-review-before.txt','minz-mixed-lineage-review-after.txt','minz-mixed-copy-weighted-final-review.md']:
 p=Path('/tmp')/name
 if p.exists():shutil.copyfile(p,root/name)
