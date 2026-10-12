#!/usr/bin/env python3
"""Research eZ80 debugger return contract replay using Python stdlib PTY."""
import argparse
import errno
import fcntl
import hashlib
import json
import os
from pathlib import Path
import pty
import re
import select
import shutil
import subprocess
import termios
import time


def clean(data):
    return re.sub(rb'\x1b\[[0-9;?]*[A-Za-z]', b'', data).decode('utf-8',errors='replace')


class Debugger:
    def __init__(self, command, cwd, env):
        self.master, slave = pty.openpty()
        def terminal():
            os.setsid()
            fcntl.ioctl(slave, termios.TIOCSCTTY, 0)
        self.proc = subprocess.Popen(command,cwd=cwd,env=env,stdin=slave,stdout=slave,stderr=slave,preexec_fn=terminal)
        os.close(slave)
        self.log = bytearray()

    def prompt(self, timeout=20):
        chunk = bytearray()
        deadline = time.monotonic()+timeout
        while time.monotonic()<deadline:
            if select.select([self.master],[],[],.2)[0]:
                try:
                    data = os.read(self.master,65536)
                except OSError as exc:
                    if exc.errno==errno.EIO:
                        break
                    raise
                if not data:
                    break
                self.log.extend(data);chunk.extend(data)
                if clean(chunk).endswith('>> '):
                    return clean(chunk)
        raise RuntimeError('debugger prompt missing: '+clean(chunk)[-500:])

    def command(self, text):
        os.write(self.master,(text+'\n').encode())
        return self.prompt()

    def close(self):
        if self.proc.poll() is None:
            os.write(self.master,b'exit\n')
            try:
                self.proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.proc.terminate();self.proc.wait(timeout=5)
        os.close(self.master)


def registers(text):
    match = re.search(r'PC=([0-9a-f]+).*?HL:([0-9a-f]+).*?SPL:([0-9a-f]+).*?IX:([0-9a-f]+).*?IY:([0-9a-f]+).*?ADL:([01])',text)
    if not match:
        raise ValueError('missing full register snapshot')
    return dict(zip(['pc','hl','sp','ix','iy','adl'],match.groups()))


def memory(text,address):
    data = bytearray()
    for match in re.finditer(r'([0-9a-f]{6}): ([0-9a-f ]+) \|',text):
        if int(match.group(1),16) != address+len(data):
            raise ValueError('noncontiguous memory dump')
        data.extend(bytes.fromhex(match.group(2)))
    if len(data)!=24:
        raise ValueError('incomplete caller frame')
    return bytes(data)


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    for name in ['emulator','mos','vdp','sdl','matrix','output']:
        parser.add_argument('--'+name,type=Path,required=True)
    args=parser.parse_args();out=args.output.resolve();out.mkdir(parents=True,exist_ok=False)
    repo=Path(__file__).resolve().parents[1]
    capture=repo/'reports/agon-debug-return-hello/capture.c';shutil.copy2(capture,out/'capture.c');shutil.copy2(__file__,out/'agon_debug_return_replay.py')
    subprocess.run(['cc','-shared','-fPIC','-I',str(args.sdl.resolve()/'include'),str(out/'capture.c'),'-ldl','-o',str(out/'capture.so')],check=True,capture_output=True)
    results=[]
    try:
        for name in ['tui_hello','tui_demo','nc_agon']:
            for mode in ['ordinary','size','speed']:
                source=args.matrix.resolve()/name/mode;case=out/name/mode;case.mkdir(parents=True)
                shutil.copy2(source/'program.bin',case/'program.bin');shutil.copy2(source/'source.nanz',case/'source.nanz')
                sd=case/'sdcard';sd.mkdir();shutil.copy2(case/'program.bin',sd/'program.bin');(sd/'autoexec.txt').write_text('load program.bin\nrun\necho MINZ_FULL_RETURNED\n')
                image=(case/'program.bin').read_bytes()
                if image[64:69]!=b'MOS\x00\x01' or image[69:71]!=b'\xfd\xe5' or image[81]!=0xc9:
                    raise ValueError('unexpected entry wrapper; breakpoint addresses require audit')
                env=os.environ.copy();env.update({'LD_LIBRARY_PATH':str(args.sdl.resolve()/'lib'),'LD_PRELOAD':str(out/'capture.so'),'SDL_VIDEODRIVER':'dummy','SDL_AUDIODRIVER':'dummy','MINZ_AGON_SEND_KEY':'1'})
                if name=='nc_agon':env['MINZ_AGON_ESCAPE']='1'
                else:env.pop('MINZ_AGON_ESCAPE',None)
                command=[str(args.emulator.resolve()),'--renderer','sw','--firmware','console8','--mos',str(args.mos.resolve()),'--vdp',str(args.vdp.resolve()),'--sdcard',str(sd),'--zero','--debugger','--breakpoint','0x40045','--breakpoint','0x40051']
                dbg=Debugger(command,args.emulator.resolve().parent,env)
                try:
                    dbg.prompt();entry=registers(dbg.command('registers'));sp=int(entry['sp'],16)
                    frame=memory(dbg.command('mem SP 24'),sp)
                    dbg.command('c');before=registers(dbg.command('registers'));endframe=memory(dbg.command('mem SP 24'),sp)
                    dbg.command('s');after=registers(dbg.command('registers'))
                    assert entry['pc']=='040045' and before['pc']=='040051'
                    assert entry['sp']==before['sp'] and entry['iy']==before['iy'] and entry['ix']==before['ix']
                    assert frame==endframe
                    assert before['hl']==after['hl']=='000000'
                    assert int(after['pc'],16)==int.from_bytes(frame[:3],'little')
                    assert int(after['sp'],16)==sp+3 and after['iy']==entry['iy'] and after['ix']==entry['ix']
                    assert entry['adl']==before['adl']==after['adl']=='1'
                    record={'source':name,'mode':mode,'bin_sha256':digest(case/'program.bin'),'entry':entry,'before_ret':before,'after_ret':after,'caller_frame_hex':frame.hex(),'caller_frame_24_bytes_unchanged':True,'exact_return_contract':True,'keys':'repeated Escape' if name=='nc_agon' else 'repeated A','command':command}
                    results.append(record);(case/'result.json').write_text(json.dumps(record,indent=2)+'\n');print(f'{name}/{mode}: return/SP/IY/IX/caller-frame PASS',flush=True)
                finally:
                    dbg.close();(case/'debugger.raw').write_bytes(dbg.log);(case/'debugger.txt').write_text(clean(dbg.log))
    finally:
        pins={name:{'path':str(getattr(args,name).resolve()),'sha256':digest(getattr(args,name))} for name in ['emulator','mos','vdp']}
        summary={'scope':'fixed early-exit scenarios, three examples x three modes; exact return/SP/IY/IX/24-byte caller-frame, no full RAM/pixel/hardware proof','tools':pins,'results':results}
        (out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
        (out/'manifest.json').write_text(json.dumps([{'path':str(p.relative_to(out)),'sha256':digest(p)} for p in sorted(out.rglob('*')) if p.is_file() and p.name!='manifest.json'],indent=2)+'\n')


if __name__=='__main__':main()
