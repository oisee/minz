package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestZMachineStateValidateNative(t *testing.T) {
	for _, storySize := range []int{1024, 65536, 65537, 86928, 98304} {
		command := exec.Command("python3", "-c", `
import json,sys
from restart_rle import pack
from save_format import check_state
from native_state_validate import emit
from native_shared_reader import emit as reader
story=bytearray(int(sys.argv[1]));story[0]=3;story[2:4]=b'\x00\x01';story[14:16]=b'\x01\x00';story[18:24]=b'260101';story[28:30]=b'\x12\x34'
state=bytearray(1786);state[:16]=b'MZ3S\x00\x01260101\x12\x34\x01\x00';state[16]=128;state[19]=2;state[20]=8;state[24]=1;state[25]=1;state[26]=64
base=1082
for i in [1,2]:state[base+i]=128;state[base+128+i]=i
cases=[]
def add(data):
 try:check_state(data,story,32,256);want=1
 except ValueError:want=0
 cases.append({'payload':list(pack(data)),'want':want})
add(state)
for offset in list(range(26))+[26,42,base+1,base+33,base+65,base+129,base+161,base+130,base+162]:
 for value in [0,1,16,31,32,63,64,127,128,255]:
  d=bytearray(state);d[offset]=value;add(d)
for pc in [63,64,65535,65536,len(story)-1,len(story),98303,98304,16777215]:
 d=bytearray(state);d[16:19]=pc.to_bytes(3,'little');add(d)
 d=bytearray(state)
 for j in range(3):d[base+1+j*32]=(pc>>(j*8))&255
 add(d)
print(json.dumps({'source':reader(0x17,0)+emit(story),'cases':cases}))
`, strconv.Itoa(storySize))
		command.Dir = "../../../scripts/zmachine"
		output, err := command.Output()
		if err != nil {
			t.Fatal(err)
		}
		var f struct {
			Source string
			Cases  []struct {
				Payload []byte
				Want    byte
			}
		}
		if err = json.Unmarshal(output, &f); err != nil {
			t.Fatal(err)
		}
		h, err := nanz.Parse(f.Source+"\nfun main()->u8 {return native_state_validate()}", "validate-state.nanz")
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
		for cindex, c := range f.Cases {
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
			offset := 16384 - len(c.Payload)
			copy(m.Memory.RAM[6][offset:], c.Payload)
			m.Memory.Write(0xbf00, 0, false)
			m.Memory.Write(0xbf01, 0x70, false)
			before := m.Memory.RAM
			m.Memory.SetPagingForce(0x10)
			m.CPU.SetPC(uint16(a.Symbols["native_saved_begin"]))
			m.CPU.SetSP(0xbf00)
			m.CPU.SetHL(uint16(offset))
			m.CPU.SetAF(0x1600)
			m.CPU.SetIFF1(false)
			m.CPU.SetIFF2(false)
			for i := 0; i < 100; i++ {
				if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
					break
				}
				m.CPU.DoOpcode()
			}
			m.CPU.SetPC(uint16(a.Symbols["native_state_validate"]))
			m.CPU.SetSP(0xbf00)
			for i := 0; i < 3000000; i++ {
				if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
					break
				}
				m.CPU.DoOpcode()
			}
			if byte(m.CPU.AF()>>8) != c.Want || m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || m.CPU.IFF2() {
				t.Fatalf("case%d got%d want%d PC/SP%04x/%04x", cindex, m.CPU.AF()>>8, c.Want, m.CPU.PC(), m.CPU.SP())
			}
			cursor := a.Symbols["saved_cursor"]
			scratch := map[int]bool{}
			for name, size := range map[string]int{"state_check_frame": 1, "state_check_depth": 2, "state_check_last": 2} {
				for n := 0; n < size; n++ {
					scratch[a.Symbols[name]+n] = true
				}
			}
			spills := map[int]bool{}
			for name, address := range a.Symbols {
				if strings.HasPrefix(strings.ToLower(name), "_spill_native_saved_") || strings.HasPrefix(strings.ToLower(name), "_spill_native_state_validate_") {
					for n := 0; n < 4; n++ {
						spills[address+n] = true
					}
				}
			}
			for p := 0; p < 8; p++ {
				for i, b := range before[p] {
					address := 0x8000 + i
					if p == 2 && (i >= 0x3e00 && i < 0x3f00 || address >= cursor && address < cursor+12 || spills[address] || scratch[address]) {
						continue
					}
					if m.Memory.RAM[p][i] != b {
						t.Fatalf("validator wrote bank%d offset%x", p, i)
					}
				}
			}
		}
		t.Logf("story %d: %d semantic states match reference validation without changing game banks", storySize, len(f.Cases))
	}
}
