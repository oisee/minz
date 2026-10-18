from pathlib import Path
import importlib.util,json
r=Path(__file__).resolve().parent
s=importlib.util.spec_from_file_location('grid',r/'grid_model.py');m=importlib.util.module_from_spec(s);s.loader.exec_module(m)
case={'name':'all256-bytes','scope':'typed-storage renderer fixture; not an ABAP ASCII source-literal admission claim','columns':[{'name':'offset','kind':'i'},{'name':'bytes','kind':'c','length':16}],'rows':[[n,list(range(n,n+16))] for n in range(0,256,16)],'top':0}
case['screen']=m.screen(case['columns'],case['rows'],0)
(r/'all256-bytes.json').write_text(json.dumps(case,indent=2)+'\n')
(r/'all256-bytes.txt').write_text('\n'.join(case['screen'])+'\n')
print('Independent all256-byte screen:16 rows, 24x80, every source byte exactly once')
