"""Geometric means of MinZ/SDCC ratios (T-states, program bytes) per profile pair.
Usage: python3 geomean.py <this directory>
Output tuple: (cases, T ratio, bytes ratio); key: (compiler, profile, sdcc profile)."""
import json,math,collections,sys
S=sys.argv[1]
def load(d,n): return json.load(open('%s/%s-summary.json'%(S,n)))
def gm(xs): return math.exp(sum(math.log(x) for x in xs)/len(xs)) if xs else float('nan')
def table(d):
    rows=load(d,'all')+load(d,'fusion')
    bad=[r for r in rows if r['Status']!='PASS']
    idx={(r['Compiler'],r['Profile'],r['Case']):r for r in rows}
    cps=collections.Counter((r['Compiler'],r['Profile']) for r in rows)
    out={'rows':len(rows),'notpass':len(bad),'profiles':dict(cps)}
    res={}
    for (c,p) in cps:
        if c=='sdcc': continue
        for sp in [k[1] for k in cps if k[0]=='sdcc']:
            t=[];b=[]
            for (cc,pp,case),r in idx.items():
                if (cc,pp)!=(c,p): continue
                s=idx.get(('sdcc',sp,case))
                if not s or not s.get('TotalT') or not r.get('TotalT'): continue
                t.append(r['TotalT']/s['TotalT']); b.append(r['ProgramBytes']/s['ProgramBytes'])
            res[(c,p,sp)]=(len(t),round(gm(t),3),round(gm(b),3))
    return out,res
for d in ['.']:
    o,r=table(d); print(o)
    for k in sorted(r): print('  ',k,r[k])
