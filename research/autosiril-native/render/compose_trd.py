#!/usr/bin/env python3
"""Compose the renderer's owned TRD adapter and driver, preserving older stage sources."""
from pathlib import Path
import re, subprocess, sys
sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
MIX = HERE.parent / 'mix'
sys.path.insert(0, str(MIX))
from trd_native import code as mix_native_code  # noqa: E402


def once(s, old, new):
    assert s.count(old) == 1, old
    return s.replace(old, new, 1)


def binding(generated):
    # Same extraction and patches as mix/compose_trd.py: fixed stub at 0x4000,
    # output_open renamed to output_live, open_input keeps the live writer.
    old = (generated / 'echo-trd.nanz').read_text()
    text = old[old.index('// Native append-only TRD binding.'):old.index('fun select_arrangement()')]
    text = once(text, '@extern(0x6000)', '@extern(0x4000)')
    text = text.replace('output_open', 'output_live')
    for line in ['    chunk_capacity = 256\n', '    input_padding = 0\n',
                 '    input_padding_quantum = 0\n', '    output_live = 0\n']:
        first, rest = text.split('fun read_chunk()', 1)
        text = once(first, line, '') + 'fun read_chunk()' + rest
    return text


def cursors():
    text = (MIX / 'cursors-trd.nanz').read_text()
    for name in ['cursor_open', 'cursor_close', 'cursor_get', 'cursor_refill']:
        text = re.sub(r'\b%s\(' % name, 'trd_' + name + '(', text)
    return text


def source(generated):
    kernel = (HERE / 'kernel.nanz').read_text().split('@extern(0x0008)', 1)[0]
    s = '\n'.join([cursors(), kernel, (HERE / 'common.nanz').read_text(),
                   (HERE / 'trd.nanz').read_text(), binding(generated)])
    assert s.count('global fault:u16') == 2
    s = s.replace('global fault:u16', '', 1)
    lines = s.splitlines()
    return '\n'.join([l for l in lines if l.startswith('global ')] +
                     [l for l in lines if not l.startswith('global ')]) + '\n'


def native_code(generated):
    s = mix_native_code(generated)
    s = once(s, 'map[string]int{"delay_one":2,"delay_two":2,"mix_counts_config":3,"mix_policies":3}',
             'map[string]int{}')
    return s


PRE = '''package main
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


def compose(out):
    subprocess.run([sys.executable, HERE.parent / 'echo/compose.py', '--out', out / 'generated'],
                   check=True, capture_output=True)
    generated = out / 'generated'
    (out / 'render-trd.nanz').write_text(source(generated))
    old = (generated / 'echo-cpm.go').read_text()
    spills = old[old.index('type dataSpan struct'):old.index('// Research OS adapter:')]
    driver = out / 'render-trd.go'
    driver.write_text(PRE + spills + (HERE / 'trd-driver.go.txt').read_text() + native_code(generated))
    return driver
