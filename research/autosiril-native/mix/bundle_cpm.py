#!/usr/bin/env python3
"""Collect the four verified native upstream COMs plus the measured fifth COM.

This preserves actual measured file links. It does not rerun the preceding four
programs, claim a physical CP/M bootstrap, or replace a full fresh pipeline gate.
"""
from pathlib import Path
import argparse,hashlib,json,shutil

def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['run','upstream','out']:p.add_argument('--'+name,type=Path,required=True)
    a=p.parse_args();proof=json.loads((a.run/'measured/receipt.json').read_text())
    row=next(r for r in proof['cases'] if r['case']=='native_chronos' and r['engine']=='z80');assert row['fault']==0
    a.out.mkdir(parents=True,exist_ok=False)
    previous=a.upstream/'cpm/standalone';native=a.run/'measured/native_chronos/z80'
    for name in ['NOTES.BIN','DELAYED.BIN']:assert (previous/name).read_bytes()==(native/name).read_bytes()
    for f in previous.iterdir():
        if f.is_file():shutil.copyfile(f,a.out/f.name)
    shutil.copyfile(a.run/'measured/program.com',a.out/'MIX.COM')
    shutil.copyfile(native/'MIXED.BIN',a.out/'MIXED.BIN')
    assert sha(a.out/'MIX.COM')==proof['binary_sha256']
    assert sha(a.out/'MIXED.BIN')==row['output_sha256']
    chain=[]
    for program,source,sink in [('PAIR.COM','INPUT.MID','NOTES.BIN'),('FLATTEN.COM','NOTES.BIN','EVENTS.BIN'),('ARRANGE.COM','EVENTS.BIN','ARRANGE.BIN'),('ECHO.COM','ARRANGE.BIN','DELAYED.BIN'),('MIX.COM','DELAYED.BIN','MIXED.BIN')]:
        chain.append(dict(program=program,source=source,sink=sink,program_sha256=sha(a.out/program),source_sha256=sha(a.out/source),sink_sha256=sha(a.out/sink)))
    result=dict(protocol='autosiril-mix-cpm-standalone/v1',scope='five independently compiled COM artifacts with measured native file links; previous four collected from verified echo run, fifth executed in current run; not a fresh five-command replay or physical CP/M bootstrap',stages=chain,mix_notes_companion_sha256=sha(a.out/'NOTES.BIN'),upstream_receipt_sha256=sha(a.upstream/'receipt.json'),mix_receipt_sha256=sha(a.run/'measured/receipt.json'),bundle_script_sha256=sha(Path(__file__)))
    (a.out/'CHAIN.json').write_text(json.dumps(result,indent=2)+'\n');print('PASS collected five COMs and all exact measured native file links')
if __name__=='__main__':main()
