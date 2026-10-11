import json,sys
from pathlib import Path
r=Path(sys.argv[1])
for rec in json.loads((r/'exit-codes.json').read_bytes()):
 p=r/rec['workload']/rec['goal']
 if rec['goal']=='refusal':
  assert rec['exit_code']==1 and not rec['selected_artifact_exists'] and not p.exists()
  log=(p.parent/'refusal-cli.txt').read_text();assert 'Error:' in log
  needle={'unobserved-append':'both exact sites must be observed','source-tail-denial':'undeclared read at 49153','destination-tail-denial':'undeclared write at 36866','read-only-header-denial':'read-only write at 36864','instruction-budget':'gas limit reached'}[rec['workload']];assert needle in log
  print('PASS refusal',rec['workload'],'nonzero/nooutputdir/noacceptedselection',needle)
 else:
  s=json.loads((p/'selection.json').read_bytes());assert rec['exit_code']==0 and s['selected']==(1 if rec['goal']=='speed' else 2)
  assert s['association']['baseline']['compatibility']['context']['compiler_id']=='executable-sha256:1bd95516f7b1ef1ba56c17822104936e2b73fe8bde5fe7368dd128811009f041'
print('PASS11statuses6positive5refused')
