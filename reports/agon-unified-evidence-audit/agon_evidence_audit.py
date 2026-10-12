#!/usr/bin/env python3
"""Offline audit of retained Agon streams/return transcripts; executes no CPU."""
import argparse
import hashlib
import json
from pathlib import Path
import re

from agon_native_vdu_model import expected


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def manifest(root):
    entries = json.loads((root/'manifest.json').read_text())
    actual = {str(p.relative_to(root)) for p in root.rglob('*') if p.is_file() and p != root/'manifest.json'}
    declared = [e['path'] for e in entries]
    if len(declared)!=len(set(declared)) or set(declared)!=actual:
        raise ValueError('manifest coverage mismatch')
    for entry in entries:
        relative = Path(entry['path'])
        if relative.is_absolute() or '..' in relative.parts:
            raise ValueError('unsafe manifest path')
        if sha(root/relative)!=entry['sha256']:
            raise ValueError('artifact digest mismatch: '+entry['path'])
    return len(entries)


def return_snapshot(raw):
    text = re.sub(rb'\x1b\[[0-9;?]*[A-Za-z]',b'',raw).decode('utf-8')
    rows=[]
    for line in text.splitlines():
        if 'PC=' not in line:
            continue
        pairs=dict(re.findall(r'\b(PC|HL|SPL|IX|IY|ADL)[=:]([0-9a-f]+)',line))
        if set(pairs)!={'PC','HL','SPL','IX','IY','ADL'}:
            raise ValueError('incomplete full register row')
        rows.append({key:int(value,16) for key,value in pairs.items()})
    if len(rows)!=3:
        raise ValueError('need entry/pre-RET/post-RET snapshots')
    entry,before,after=rows
    if entry['PC']!=0x40045 or before['PC']!=0x40051:
        raise ValueError('incorrect breakpoint scope')
    for key in ['SPL','IX','IY']:
        if entry[key]!=before[key]:
            raise ValueError('entry/return '+key+' changed')
    if before['HL']!=0 or after['HL']!=0:
        raise ValueError('nonzero status')
    if any(row['ADL']!=1 for row in rows):
        raise ValueError('ADL changed')
    dumps=[]
    for match in re.finditer(r'(?m)([0-9a-f]{6}): ((?:[0-9a-f]{2} )+)\|',text):
        dumps.append((int(match.group(1),16),bytes.fromhex(match.group(2))))
    if len(dumps)!=4:
        raise ValueError('need two complete caller-frame dumps')
    frames=[]
    for left,right in [dumps[:2],dumps[2:]]:
        if left[0]!=entry['SPL'] or right[0]!=left[0]+16 or len(left[1])!=16 or len(right[1])!=8:
            raise ValueError('caller-frame range mismatch')
        frames.append(left[1]+right[1])
    if frames[0]!=frames[1]:
        raise ValueError('caller-frame bytes changed')
    if after['PC']!=int.from_bytes(frames[0][:3],'little') or after['SPL']!=entry['SPL']+3:
        raise ValueError('incorrect RET target/stack adjustment')
    if after['IX']!=entry['IX'] or after['IY']!=entry['IY']:
        raise ValueError('registers changed across RET')
    return {'entry':entry,'before':before,'after':after,'caller_frame_hex':frames[0].hex()}


def stream_snapshot(wire,wanted):
    if wire.count(b'Agon Console8 MOS Version')!=1:
        raise ValueError('MOS boot count')
    marker=b'MINZ_FULL_RETURNED\r\n/ *'
    if wire.count(marker)!=1:
        raise ValueError('MOS continuation count')
    boot=b'\n\r\n\r\x07'
    start=wire.index(boot)+len(boot)
    end=wire.index(marker,start)
    if wire[start:end]!=wanted:
        raise ValueError('exact application VDU mismatch')
    return end-start


def rejected(call):
    try:
        call()
    except ValueError:
        return
    raise ValueError('audit negative control accepted')


CONSOLE_EXPECTED = {
    'hello_agon': b'Hello from MinZ on Agon Light 2!\r\nPress any key: \r\nYou pressed a key!\r\n',
    'hello_world': b'Hello, Agon Light 2!\r\nMinZ running on eZ80 at 18.432 MHz\r\nPress any key to continue...\r\nYou pressed: a\r\nGoodbye!\r\n',
}


def matrix_records(summary, names):
    required = {(name, mode) for name in names for mode in ['ordinary', 'size', 'speed']}
    records = {}
    for row in summary['results']:
        key = (row['source'].split('/')[-1].removesuffix('.nanz'), row['mode'])
        if key in records:
            raise ValueError('duplicate matrix case')
        records[key] = row
    if set(records) != required:
        raise ValueError('matrix labels are incomplete or unexpected')
    return records


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--wire',type=Path,required=True)
    parser.add_argument('--returns',type=Path,required=True)
    parser.add_argument('--output',type=Path,required=True)
    parser.add_argument('--programs', choices=['graphics', 'console'], default='graphics')
    args=parser.parse_args();wire=args.wire.resolve();returns=args.returns.resolve()
    counts={'wire':manifest(wire),'returns':manifest(returns)}
    ws=json.loads((wire/'summary.json').read_text());rs=json.loads((returns/'summary.json').read_text())
    names = list(CONSOLE_EXPECTED) if args.programs == 'console' else ['tui_hello', 'tui_demo', 'nc_agon']
    wm = matrix_records(ws, names); rm = matrix_records(rs, names)
    for name in ['emulator','mos','vdp']:
        if ws['tools'][name]['sha256']!=rs['tools'][name]['sha256']:
            raise ValueError('different firmware/tool identity')
    repo=Path(__file__).resolve().parents[1]
    for filename in ['agon_native_vdu_model.py','agon_tui_stream_model.py']:
        if (wire/filename).read_bytes()!=(repo/'scripts'/filename).read_bytes():
            raise ValueError('retained model differs from audited model')
    rows=[]
    for name in names:
        for mode in ['ordinary','size','speed']:
            w=wire/name/mode;r=returns/name/mode
            if (w/'source.nanz').read_bytes()!=(r/'source.nanz').read_bytes() or (w/'program.bin').read_bytes()!=(r/'program.bin').read_bytes():
                raise ValueError('source/BIN differs between evidence runs')
            keys=(10,)*9+(13,11,13,27) if name=='nc_agon' else ()
            wanted=CONSOLE_EXPECTED[name] if name in CONSOLE_EXPECTED else expected(name,keys)
            if (w/'expected.bin').read_bytes()!=wanted:
                raise ValueError('saved oracle differs from source-level model')
            for saved in [wm[(name, mode)], rm[(name, mode)]]:
                if saved['bin_sha256'] != sha(w/'program.bin'):
                    raise ValueError('matrix BIN identity differs from retained image')
            if wm[(name, mode)]['source_sha256'] != sha(w/'source.nanz'):
                raise ValueError('matrix source identity differs')
            stream=stream_snapshot((w/'wire.bin').read_bytes(),wanted)
            snapshot=return_snapshot((r/'debugger.raw').read_bytes())
            record=json.loads((r/'result.json').read_text())
            for old,new in [('entry','entry'),('before_ret','before'),('after_ret','after')]:
                saved=record[old]
                for skey,key in [('pc','PC'),('sp','SPL'),('iy','IY'),('ix','IX'),('hl','HL'),('adl','ADL')]:
                    if int(saved[skey],16)!=snapshot[new][key]:
                        raise ValueError('summary does not match raw debugger evidence')
            if record['caller_frame_hex']!=snapshot['caller_frame_hex']:
                raise ValueError('saved caller frame differs')
            if record['bin_sha256']!=sha(r/'program.bin'):
                raise ValueError('saved BIN identity differs')
            rows.append({'source':name,'mode':mode,'bin_sha256':sha(w/'program.bin'),'source_sha256':sha(w/'source.nanz'),'application_bytes':stream,'return_snapshot':snapshot})
    # Mutations exercise the independent audit, not the producer's passed flags.
    name=names[0]
    r=returns/name/'ordinary';raw=(r/'debugger.raw').read_bytes()
    rejected(lambda:return_snapshot(raw.replace(b'PC=000874',b'PC=000875')))
    rejected(lambda:return_snapshot(raw.replace(b'SPL:0bfe69',b'SPL:0bfe68')))
    rejected(lambda:return_snapshot(raw.replace(b'HL:000000',b'HL:000001')))
    rejected(lambda:return_snapshot(raw.replace(b'74 08 00',b'75 08 00',1)))
    w=wire/name/'ordinary';data=(w/'wire.bin').read_bytes();want=CONSOLE_EXPECTED[name] if name in CONSOLE_EXPECTED else expected(name)
    rejected(lambda:stream_snapshot(data.replace(b'Hello from MinZ',b'Hallo from MinZ'),want))
    rejected(lambda:stream_snapshot(data.replace(b'MINZ_FULL_RETURNED',b'NO_RETURN'),want))
    output={'scope':f'offline independent reparse of {len(rows)} {args.programs} streams and {len(rows)} debugger return runs; no CPU execution by auditor; bounded command/text and caller-frame proofs, no global RAM/pixel/hardware proof','manifest_files':counts,'rows':rows,'negative_controls':6,'verified':True}
    args.output.write_text(json.dumps(output,indent=2)+'\n')
    print(f'Independent retained-evidence audit PASS: {2*len(rows)} runs, 6 rejection controls')


if __name__=='__main__':main()
