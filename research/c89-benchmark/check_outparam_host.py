#!/usr/bin/env python3
"""Independent host-C execution of the frozen output-contract fixtures."""
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile

root = Path(__file__).resolve().parents[2]
cases = root / 'minzc/pkg/c89/testdata/outparam'
rows = []
for entry in json.loads((cases / 'manifest.json').read_text())['cases']:
    source = (cases / entry['file']).read_bytes()
    assert hashlib.sha256(source).hexdigest() == entry['sha256'], entry['file']
    family = Path(entry['file']).stem
    model = {
        'local_pair': 'unsigned char hi=x+y, n=x+3; unsigned char lo=n>=128?(n^y):(n+7); return ((unsigned short)hi<<8)|lo;',
        'mixed_pair': 'return (unsigned short)((short)x-(short)y) ^ (unsigned short)(x^y);',
        'repeated_pair': 'y++;return ((unsigned short)(unsigned char)(x+y)<<8)|(unsigned char)(y^0xa5);',
    }[family]
    inputs = [(x, y) for x in range(256) for y in [0, 1, 2, 63, 127, 128, 254, 255]]
    if family == 'repeated_pair':
        inputs = []
        state = 0x51de
        for _ in range(73):
            state = (1664525 * state + 1013904223) & 0xffffffff
            x = (state >> 16) & 255
            state = (1664525 * state + 1013904223) & 0xffffffff
            inputs.append((x, (state >> 16) & 255))
    initializer = ','.join('{%d,%d}' % xy for xy in inputs)
    harness = '''
#include <stdio.h>
static unsigned short expected(unsigned char x,unsigned char y){%s}
int main(void){
  static const unsigned char xy[][2]={%s};
  for(unsigned i=0;i<sizeof(xy)/sizeof(xy[0]);i++){
    left=xy[i][0];right=xy[i][1];events=5;result=0x5aa5;
    execute();
    if(result!=expected(xy[i][0],xy[i][1])||left!=xy[i][0]||right!=xy[i][1]||events!=%d)return 1;
  }
  puts("PASS");return 0;
}
''' % (model, initializer, 20 if family == 'repeated_pair' else 16)
    with tempfile.TemporaryDirectory(prefix='minz-outparam-host-') as directory:
        cfile = Path(directory) / entry['file']
        binary = Path(directory) / 'oracle'
        cfile.write_bytes(source + harness.encode())
        subprocess.run(['gcc', '-std=c99', '-O2', '-Wall', '-Wextra', str(cfile), '-o', str(binary)], check=True)
        result = subprocess.check_output([str(binary)], text=True).strip()
        assert result == 'PASS'
    rows.append({'case': family, 'status': result, 'calls': len(inputs), 'source_sha256': entry['sha256']})
print(json.dumps({'compiler': subprocess.check_output(['gcc', '--version'], text=True).splitlines()[0], 'cases': rows}, indent=2))
