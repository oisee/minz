#!/usr/bin/env python3
"""Read-only source/fixture inventory; writes only adjacent audit files."""
from pathlib import Path
import collections, hashlib, json, re, subprocess
ROOT = Path(__file__).resolve().parents[2]
OUT = Path(__file__).resolve().parent

def git(*args):
    return subprocess.check_output(['git', *args], cwd=ROOT, text=True)

def rows(paths):
    result=[]
    for path in sorted(set(paths)):
        p=ROOT/path
        result.append({'path':path,'bytes':p.stat().st_size,'sha256':hashlib.sha256(p.read_bytes()).hexdigest()})
    return result

changed=git('diff','--name-only','origin/main...HEAD').splitlines()
source=[p for p in changed if p.startswith(('minzc/','examples/','stdlib/'))]
production=[p for p in source if p.endswith('.go') and not p.endswith('_test.go') and (ROOT/p).is_file()]
tests=[p for p in source if p.endswith('_test.go') and (ROOT/p).is_file()]
deletions=git('diff','--name-only','--diff-filter=D','origin/main...HEAD','--','minzc').splitlines()
references=collections.defaultdict(set)
dynamic_bundles=collections.defaultdict(set)
for f in tests:
    text=(ROOT/f).read_text()
    for literal in re.findall(r'"([^"\n]+)"',text):
        literal=literal.split('::')[0]
        if literal.startswith('../../../'): literal=literal[9:]
        if not literal.startswith(('docs/','reports/','examples/','stdlib/')): continue
        p=ROOT/literal
        if p.is_file(): references[literal].add(f)
        if literal.startswith(('docs/','reports/')):
            bundle='/'.join(Path(literal).parts[:2])
            if (ROOT/bundle).is_dir(): dynamic_bundles[bundle].add(f)
# Conservatively cover variable suffixes (read(root + case + suffix)) without
# copying nested generated output trees. These are candidates, not proven reads.
candidates=set(references)
for bundle in dynamic_bundles:
    base=ROOT/bundle
    for p in base.rglob('*'):
        if not p.is_file(): continue
        rel=p.relative_to(base)
        frozen=any('frozen' in part for part in rel.parts)
        root_input=(len(rel.parts)==1 and p.suffix in {'.json','.bin','.nanz','.pas','.plm','.lizp','.lanz','.c','.img','.csv','.stdout'})
        if frozen or root_input: candidates.add(p.relative_to(ROOT).as_posix())
# Follow existing files referenced by JSON fixtures/manifests: repository paths,
# sibling paths, and paths relative to the nearest docs/reports bundle. Never
# infer missing paths or fetch external resources.
closure=set(candidates); pending=list(candidates); manifest_edges=[]
while pending:
    path=pending.pop(); p=ROOT/path
    if p.suffix!='.json': continue
    try: value=json.loads(p.read_text())
    except (ValueError,UnicodeError): continue
    def strings(v):
        if isinstance(v,str): yield v
        elif isinstance(v,dict):
            for k,x in v.items():
                yield k
                yield from strings(x)
        elif isinstance(v,list):
            for x in v: yield from strings(x)
    for s in strings(value):
        if not s or '\n' in s or len(s)>240 or not re.search(r'\.(json|nanz|pas|plm|lizp|lanz|c|go|bin|a80|mir2?|img|txt|md|py|gz|com|csv|stdout)(::|$)',s): continue
        options=[ROOT/s,p.parent/s]
        if path.startswith(('docs/','reports/')): options.append(ROOT/'/'.join(Path(path).parts[:2])/s)
        for q in options:
            try: q=q.resolve(); rel=q.relative_to(ROOT).as_posix()
            except (ValueError,OSError): continue
            try: exists=q.is_file()
            except OSError: exists=False
            if exists and rel!=path:
                manifest_edges.append({'from':path,'to':rel,'reference':s})
                if rel not in closure:
                    closure.add(rel); pending.append(rel)
                break
embedded=git('ls-files','minzc/pkg/hir/rules').splitlines()
main_only=[line.split('\t',1)[1] for line in git('diff','--name-status','--diff-filter=A','HEAD','origin/main').splitlines() if line.startswith('A\t')]
data={'schema':'minz.main-integration-scope/v1','research_head':git('rev-parse','HEAD').strip(),'main_head':git('rev-parse','origin/main').strip(),'merge_base':git('merge-base','HEAD','origin/main').strip(),'notes':['Static conservative fixture closure includes variable-path candidates, not proof that every file is read.','Keep included JSON manifest references intact; reduce after isolated tests prove omitted candidates unnecessary.','Source paths encode intended LIR retirement; preserve main-only report/prose changes.'], 'first_pr_source_paths':source,'production_go_paths':production,'test_go_paths':tests,'intended_deletions':deletions,'mandatory_embedded_assets':rows(embedded),'direct_literal_fixture_references':[{'path':p,'tests':sorted(t),'bytes':(ROOT/p).stat().st_size} for p,t in sorted(references.items())],'dynamic_fixture_bundles':[{'path':p,'tests':sorted(t)} for p,t in sorted(dynamic_bundles.items())],'conservative_fixture_paths':rows(candidates),'transitive_fixture_closure':rows(closure),'manifest_reference_edges':manifest_edges,'preserve_main_only_paths':[p for p in main_only if not p.startswith('minzc/')], 'runtime_table_assets':[]}
(OUT/'inventory.json').write_text(json.dumps(data,indent=2)+'\n')
for name,paths in [('first-pr-source-paths.txt',source),('fixture-paths.txt',closure),('embedded-assets.txt',embedded)]:
    (OUT/name).write_text('\n'.join(sorted(set(paths)))+'\n')
print(json.dumps({'source_paths':len(source),'production_go':len(production),'tests':len(tests),'direct_fixtures':len(references),'dynamic_bundles':len(dynamic_bundles),'candidate_fixtures':len(candidates),'closure_fixtures':len(closure),'closure_bytes':sum((ROOT/p).stat().st_size for p in closure)}))
