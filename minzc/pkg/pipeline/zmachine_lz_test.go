package pipeline

import (
	"bytes"
	"encoding/json"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"os/exec"
	"testing"
)

func TestZMachineLZDecodeNative(t *testing.T) {
	script := `import sys,json,base64,os
from pathlib import Path
sys.path.insert(0,'../../../scripts/zmachine')
from restart_lz import pack,unpack
rows=[]
for size in [0,1,2,3,8,9,18,19,127,128,1024,2048,4097,8192]:
 for mode in range(3):
  data=bytes((0 if mode==0 else i%251 if mode==1 else (i*37+i//31)&255) for i in range(size))
  encoded=pack(data)
  assert unpack(encoded,size)==data
  rows.append({'Data':base64.b64encode(data).decode(),'Encoded':base64.b64encode(encoded).decode()})
# Maximum distance4096, length18; literals control bits then match.
data=bytes((i*13+i//17)&255 for i in range(4096))
encoded=b''.join(bytes([255])+data[i:i+8] for i in range(0,4096,8))+bytes([0,255,255])
data+=data[:18]
assert unpack(encoded,len(data))==data
rows.append({'Data':base64.b64encode(data).decode(),'Encoded':base64.b64encode(encoded).decode()})
path=os.environ.get('MINZ_Z3_LZ_STORY')
if path:
 story=Path(path).read_bytes();boundary=int.from_bytes(story[14:16],'big')
 for begin in [0,boundary,len(story)-2048]:
  data=story[begin:begin+2048];encoded=pack(data)
  assert unpack(encoded,len(data))==data
  rows.append({'Data':base64.b64encode(data).decode(),'Encoded':base64.b64encode(encoded).decode()})
print(json.dumps(rows))`
	out, err := exec.Command("python3", "-c", script).Output()
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct{ Data, Encoded []byte }
	if err := json.Unmarshal(out, &rows); err != nil {
		t.Fatal(err)
	}
	h, err := nanz.ParseWithOpts("import zx.lz\nfun main() { bank_unpack_lz(7,0,ptr(0x6000),0) }", "lz.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x9000\n" + s.Assembly)
	if err != nil || len(a.Errors) > 0 {
		t.Fatalf("%v %v\n%s", err, a.Errors, s.Assembly)
	}
	cases := 0
	for _, row := range rows {
		for _, bank := range []int{0, 1, 3, 4, 6, 7} {
			for _, offset := range []int{0, 16384 - len(row.Encoded)} {
				m, err := spectrum.NewPentagon128(nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				for p := 0; p < 8; p++ {
					for i := range m.Memory.RAM[p] {
						m.Memory.RAM[p][i] = byte(i*37 + p*17)
					}
				}
				for i, b := range a.Binary {
					m.Memory.Write(0x9000+uint16(i), b, false)
				}
				copy(m.Memory.RAM[bank][offset:], row.Encoded)
				m.Memory.Write(0xbf00, 0, false)
				m.Memory.Write(0xbf01, 0x70, false)
				before := m.Memory.RAM
				m.Memory.SetPagingForce(0x10)
				m.CPU.SetPC(uint16(a.Symbols["zx__lz__bank_unpack_lz"]))
				m.CPU.SetSP(0xbf00)
				m.CPU.SetAF(uint16(bank+0x10) << 8)
				m.CPU.SetHL(uint16(offset))
				m.CPU.SetDE(0x6000)
				m.CPU.SetIX(uint16(len(row.Data)))
				m.CPU.SetIFF1(false)
				m.CPU.SetIFF2(false)
				for i := 0; i < 1000000; i++ {
					if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
						break
					}
					m.CPU.DoOpcode()
				}
				if m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || m.CPU.IFF2() {
					t.Fatalf("boundary size%d bank%d offset%d", len(row.Data), bank, offset)
				}
				if !bytes.Equal(m.Memory.RAM[5][0x2000:0x2000+len(row.Data)], row.Data) {
					t.Fatalf("decode size%d bank%d offset%d", len(row.Data), bank, offset)
				}
				for p := 0; p < 8; p++ {
					for i, b := range before[p] {
						if p == 5 && i >= 0x2000 && i < 0x2000+len(row.Data) {
							continue
						}
						if p == 2 && i >= 0x3ed0 && i < 0x3f00 {
							continue
						}
						adr := uint16(0x8000 + i)
						if p == 2 && (adr == uint16(a.Symbols["zx__lz__lz_control"]) || adr == uint16(a.Symbols["zx__lz__lz_bits"])) {
							continue
						}
						if m.Memory.RAM[p][i] != b {
							t.Fatalf("unexpected write bank%d offset%04x", p, i)
						}
					}
				}
				cases++
			}
		}
	}
	t.Logf("%d physical-bank/bounds cases; module %d bytes", cases, len(a.Binary))
}
