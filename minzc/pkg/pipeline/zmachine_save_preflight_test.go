package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"os/exec"
	"testing"
)

func TestZMachineSavePreflightNative(t *testing.T) {
	command := exec.Command("python3", "-c", `
import json
from save_format import encode,crc16
from native_save_preflight import emit
story=bytearray(1024);story[0]=3;story[2:4]=b'\x00\x01';story[14:16]=b'\x01\x00';story[18:24]=b'260101';story[28:30]=b'\x12\x34'
state=bytearray(1786);state[:16]=b'MZ3S\x00\x01260101\x12\x34\x01\x00';state[16]=128;state[24]=1
record=encode(story,state,story[:256])
cases=[{'record':list(record),'want':1}]
for i in range(len(record)):
 d=bytearray(record);d[i]^=1;cases.append({'record':list(d),'want':0})
for cut in range(len(record)):cases.append({'record':list(record[:cut]),'want':0})
cases.append({'record':list(record+b'\0'),'want':0})
for i in [0,4,5,6,8,10,12,14,16,18,26,32]:
 d=bytearray(record);d[i]^=255;d[30:32]=crc16(d[:30]+d[32:]).to_bytes(2,'little');cases.append({'record':list(d),'want':0})
print(json.dumps({'source':emit(story),'cases':cases}))
`)
	command.Dir = "../../../scripts/zmachine"
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Source string
		Cases  []struct {
			Record []byte
			Want   byte
		}
	}
	if err = json.Unmarshal(output, &fixture); err != nil {
		t.Fatal(err)
	}
	h, err := nanz.ParseWithOpts(fixture.Source+"\nfun main()->u8 {return native_save_preflight(7,0,0)}", "preflight.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x9000\n" + s.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Errors) > 0 {
		t.Fatal(a.Errors)
	}
	count := 0
	for _, bank := range []int{0, 1, 3, 4, 6, 7} {
		for _, c := range fixture.Cases {
			for _, offset := range []int{0, 16384 - len(c.Record)} {
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
				copy(m.Memory.RAM[bank][offset:], c.Record)
				m.Memory.Write(0xbf00, 0, false)
				m.Memory.Write(0xbf01, 0x70, false)
				before := m.Memory.RAM
				m.Memory.SetPagingForce(0x10)
				m.CPU.SetPC(uint16(a.Symbols["native_save_preflight"]))
				m.CPU.SetSP(0xbf00)
				m.CPU.SetAF(uint16(0x10|bank) << 8)
				m.CPU.SetHL(uint16(offset))
				m.CPU.SetDE(uint16(len(c.Record)))
				m.CPU.SetIFF1(false)
				m.CPU.SetIFF2(false)
				for i := 0; i < 200000; i++ {
					if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
						break
					}
					m.CPU.DoOpcode()
				}
				if byte(m.CPU.AF()>>8) != c.Want || m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || m.CPU.IFF2() {
					t.Fatalf("bank%d offset%d len%d got%d want%d PC/SP%04x/%04x", bank, offset, len(c.Record), m.CPU.AF()>>8, c.Want, m.CPU.PC(), m.CPU.SP())
				}
				scratch := map[uint16]bool{}
				for name, size := range map[string]int{"save_check_bank": 1, "save_check_offset": 2, "save_check_length": 2, "save_check_first": 2, "save_check_second": 2, "save_check_crc": 2} {
					for i := 0; i < size; i++ {
						scratch[uint16(a.Symbols[name]+i)] = true
					}
				}
				for p := 0; p < 8; p++ {
					for i, b := range before[p] {
						if p == 2 && (i >= 0x3ec0 && i < 0x3f00 || scratch[uint16(0x8000+i)]) {
							continue
						}
						if m.Memory.RAM[p][i] != b {
							t.Fatalf("preflight wrote bank%d offset%x", p, i)
						}
					}
				}
				count++
			}
		}
	}
	t.Logf("%d structural record preflights, all banks unchanged outside scratch/stack", count)
}
