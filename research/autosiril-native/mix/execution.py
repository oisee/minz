"""Shared source/driver composition for fresh and frozen fifth-stage gates."""
from pathlib import Path
import subprocess,sys
sys.dont_write_bytecode=True
from compose_trd import source as trd_source
from trd_native import code as native_code
HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[2]
def compose_cpm(out):
    source='\n'.join((HERE/n).read_text() for n in ['cursors-cpm.nanz','kernel.nanz','folds.nanz','wire.nanz'])
    assert source.count('global fault:u16')==2;source=source.replace('global fault:u16','',1)
    lines=source.splitlines();source='\n'.join([x for x in lines if x.startswith('global ')]+[x for x in lines if not x.startswith('global ')])+'\n'
    (out/'mix.nanz').write_text(source)
    subprocess.run([sys.executable,HERE.parent/'echo/compose.py','--out',out/'generated'],check=True)
    old=(out/'generated/echo-cpm.go').read_text();spills=old[old.index('type dataSpan struct'):old.index('// Research OS adapter:')]
    adapter=old[old.index('// Research OS adapter:'):old.index('type flatEvent struct')]
    needle='name != "NOTES.BIN" && name != "ARRANGE.BIN" && name != "DELAYED.BIN"';assert adapter.count(needle)==1
    adapter=adapter.replace(needle,'name != "NOTES.BIN" && name != "DELAYED.BIN" && name != "MIXED.BIN"')
    directory=old[old.index('// CP/M2.2 may return'):old.index('type echoConfig struct')]
    pre='''package main
import("bytes";"crypto/sha256";"encoding/binary";"encoding/json";"flag";"fmt";"os";"path/filepath";"io";"strings"
"github.com/minz/minzc/pkg/emulator";"github.com/minz/minzc/pkg/hir";"github.com/minz/minzc/pkg/mir2";"github.com/minz/minzc/pkg/nanz";"github.com/minz/minzc/pkg/pipeline";"github.com/minz/minzc/pkg/z80asm")
const origin=0x100
const floor=0xe000
const initialSP=0xeffe
const returnPC=0xf100
func check(e error){if e!=nil{panic(e)}}
func read(p string)[]byte{b,e:=os.ReadFile(p);check(e);return b}
func hash(b []byte)string{return fmt.Sprintf("%x",sha256.Sum256(b))}
'''
    driver=out/'mix.go';driver.write_text(pre+spills+adapter+directory+(HERE/'cpm-driver.go.txt').read_text())
    return driver

def compose_trd(out):
    subprocess.run([sys.executable,HERE.parent/'echo/compose.py','--out',out/'generated'],check=True)
    (out/'mix-trd.nanz').write_text(trd_source(out/'generated'))
    old=(out/'generated/echo-cpm.go').read_text();spills=old[old.index('type dataSpan struct'):old.index('// Research OS adapter:')]
    pre='''package main
import("bytes";"crypto/sha256";"encoding/binary";"encoding/json";"flag";"fmt";"os";"path/filepath";"strings"
"github.com/minz/minzc/pkg/spectrum";"github.com/minz/minzc/pkg/spectrum/formats";"github.com/minz/minzc/pkg/hir";"github.com/minz/minzc/pkg/mir2";"github.com/minz/minzc/pkg/nanz";"github.com/minz/minzc/pkg/pipeline";"github.com/minz/minzc/pkg/z80asm")
const origin=0x4000
const floor=0xb800
const initialSP=0xbffe
const returnPC=0x3f00
func check(e error){if e!=nil{panic(e)}}
func read(p string)[]byte{b,e:=os.ReadFile(p);check(e);return b}
func hash(b []byte)string{return fmt.Sprintf("%x",sha256.Sum256(b))}
'''
    driver=out/'mix-trd.go';driver.write_text(pre+spills+(HERE/'trd-driver.go.txt').read_text()+native_code(out/'generated'))
    return driver

def compose_kernel(out):
    source=(HERE/'kernel.nanz').read_text()+'\n'+(HERE/'folds.nanz').read_text()+'''\n@extern(0x0008) fun read_cell(@z80_c lane:u8,@z80_de destination:u16)->u8
@extern(0x0010) fun write_cell(@z80_c lane:u8,@z80_de source:u16)->u8
'''
    (out/'kernel.nanz').write_text(source)
    subprocess.run([sys.executable,HERE.parent/'echo/compose.py','--out',out/'generated'],check=True)
    old=(out/'generated/echo-cpm.go').read_text();spills=old[old.index('type dataSpan struct'):old.index('// Research OS adapter:')]
    pre='''package main
import("bytes";"crypto/sha256";"encoding/binary";"encoding/json";"flag";"fmt";"os";"path/filepath";"strings"
"github.com/minz/minzc/pkg/emulator";"github.com/minz/minzc/pkg/hir";"github.com/minz/minzc/pkg/mir2";"github.com/minz/minzc/pkg/nanz";"github.com/minz/minzc/pkg/pipeline";"github.com/minz/minzc/pkg/z80asm")
const origin=0x100
const floor=0xe000
const initialSP=0xeffe
const returnPC=0xf100
func check(e error){if e!=nil{panic(e)}}
func read(p string)[]byte{b,e:=os.ReadFile(p);check(e);return b}
func hash(b []byte)string{return fmt.Sprintf("%x",sha256.Sum256(b))}
'''
    driver=out/'kernel.go';driver.write_text(pre+spills+(HERE/'kernel-driver.go.txt').read_text())
    return driver
