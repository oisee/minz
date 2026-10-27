#!/usr/bin/env python3
"""Prove a genuinely composed host iterator graph, with no intermediate lists.

Inputs and final results remain host fixtures. This is not target execution.
The first source is an identity stream; at most two four-slot folds compose.
Each fold additionally owns one earlier-stream lookahead cell. Those heads are
separate from pending slots; neither eight total notes nor four global slots
is a valid whole-program RAM claim.
"""
from pathlib import Path
from copy import deepcopy
import argparse,hashlib,json,sys
sys.dont_write_bytecode=True
from model import fold
HERE=Path(__file__).resolve().parent

def identity(cells):
    previous=-1
    for c in cells:
        assert c['row']>previous
        previous=c['row']
        yield deepcopy(c)

def downmix(case):
    source=0;result=[];units=[]
    for count in case['config']['counts']:
        stream=iter(())
        if count:
            stream=identity(case['channels'][source]);source+=1
        for _ in range(max(0,count-1)):
            stats=dict(peak=0)
            stream=fold(stream,case['channels'][source],case['config']['policies'][source],stats)
            units.append(stats);source+=1
        # Only the final sink is collected, never the output between two folds.
        result.append(list(stream))
    assert source==3 and len(units)<=2
    return dict(channels=result,lengths=[ch[-1]['row']+1 if ch else 0 for ch in result],fold_peaks=[u['peak'] for u in units])

def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--planning',type=Path,required=True)
    p.add_argument('--out',type=Path,required=True)
    a=p.parse_args();a.out.mkdir(parents=True,exist_ok=False)
    inputs=json.loads((a.planning/'inputs.json').read_text())['cases']
    refs={c['name']:c for c in json.loads((a.planning/'ruby.json').read_text())['cases']}
    rows=[]
    for c in inputs:
        before=deepcopy(c);actual=downmix(c);assert c==before
        for k in ['channels','lengths']:assert actual[k]==refs[c['name']][k],(c['name'],k)
        assert all(p<=4 for p in actual['fold_peaks'])
        rows.append(dict(name=c['name'],fold_units=len(actual['fold_peaks']),fold_peaks=actual['fold_peaks'],pending_slot_bound=4*len(actual['fold_peaks']),earlier_head_bound=len(actual['fold_peaks'])))
    assert any(r['fold_units']==2 for r in rows)
    paths=[Path(__file__).resolve(),HERE/'model.py',a.planning/'inputs.json',a.planning/'ruby.json',a.planning/'receipt.json']
    proof=dict(protocol='autosiril-mix-lazy-host/v1',scope='actual Ruby results versus composed host fold iterators, no target/memory-layout proof',cases=rows,source_sha256={str(p.relative_to(HERE.parents[2]) if p.is_relative_to(HERE.parents[2]) else Path('planning')/p.name):hashlib.sha256(p.read_bytes()).hexdigest() for p in paths},max_fold_units=max(r['fold_units'] for r in rows),pending_slots_bound=8,earlier_heads_bound=2)
    (a.out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n')
    print('PASS',len(rows),'composed lazy iterator comparisons; at most two four-slot folds plus two earlier heads; target lowering pending')
if __name__=='__main__':main()
