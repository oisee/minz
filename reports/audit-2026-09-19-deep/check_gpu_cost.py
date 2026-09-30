#!/usr/bin/env python3
"""Small seeded GPU cost/assignment consistency probe (80 * 7^5 assignments)."""
import argparse,json,random,subprocess
p=argparse.ArgumentParser();p.add_argument('solver');a=p.parse_args()
rng=random.Random(20260919);cases=[];costs=[]
for k in range(80):
 cs=[[rng.randrange(1,30) for _ in range(7)] for _ in range(5)];costs.append(cs)
 cases.append({'nVregs':5,'widths':[8]*5,'interference':[],'ops':[{'dst':i,'src0':-1,'src1':-1,'patterns':[{'dstLocs':[loc],'srcLocs0':[],'srcLocs1':[],'cost':cost} for loc,cost in enumerate(row)]} for i,row in enumerate(cs)]})
r=subprocess.run([a.solver,'--server'],input=''.join(json.dumps(c)+'\n' for c in cases),text=True,capture_output=True,timeout=30)
rows=[json.loads(x) for x in r.stdout.splitlines()];bad=[]
for i,(row,cs) in enumerate(zip(rows,costs)):
 expected=sum(min(x) for x in cs);actual=sum(cs[j][loc] for j,loc in enumerate(row['assignment']))
 if actual!=row['cost'] or row['cost']!=expected:bad.append({'case':i,'expected':expected,'reported':row['cost'],'assignment_cost':actual,'assignment':row['assignment'],'cost_matrix':cs})
print(json.dumps({'seed':20260919,'cases':len(cases),'responses':len(rows),'exit':r.returncode,'mismatches':bad},indent=2))
