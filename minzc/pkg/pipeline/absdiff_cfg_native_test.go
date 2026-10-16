package pipeline

import (
	"encoding/binary"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

// C90 6.2.1.1/6.2.1.2: promoted differences fit int16; unsigned-char
// conversion is modulo 256. Nanz declares its byte arithmetic explicitly.
func TestByteAbsDiffCFGNativeCAndNanz(t *testing.T) {
	for _, language := range []string{"c", "nanz"} {
		for _, mode := range []string{"default", "grace", "size"} {
			t.Run(language+"/"+mode, func(t *testing.T) {
				var h *hir.Module
				var err error
				if language == "c" {
					h, err = c89.CompileWithOpts("unsigned char a,b;unsigned short result;unsigned char distance(unsigned char x,unsigned char y){if(x>y)return x-y;return y-x;}void execute(void){result=distance(a,b);}", "distance.c", c89.CompileOpts{NoLibc: true})
				} else {
					h, err = nanz.Parse("global a:u8=0\nglobal b:u8=0\nglobal result:u16=0\nfun distance(x:u8,y:u8)->u8 {if x>y {return x-y} return y-x}\nfun execute(){result=u16(distance(a,b))}", "distance.nanz")
				}
				if err != nil {
					t.Fatal(err)
				}
				raw := hir.LowerModule(h)
				s, err := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
				if err != nil {
					t.Fatal(err)
				}
				assembled, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
				if err != nil {
					t.Fatal(err)
				}
				if len(assembled.Errors) > 0 {
					t.Fatal(assembled.Errors)
				}
				if assembled.Symbols["execute"]-assembled.Symbols["distance"] != 5 {
					t.Fatal("five-byte CFG result missing", s.Assembly)
				}
				vms := []*mir2.VM{mir2.NewVM(raw), mir2.NewVM(s.MIR2Module)}
				z := emulator.NewRemogattoZ80()
				mem := z.Memory()
				copy(mem[0x8000:], assembled.Binary)
				addr := func(name string) int {
					p := assembled.Symbols[name]
					if p == 0 {
						p = assembled.Symbols["v_"+name]
					}
					if p == 0 {
						t.Fatal(name)
					}
					return p
				}
				a, b, out, entry := addr("a"), addr("b"), addr("result"), addr("execute")
				for x := 0; x < 256; x++ {
					for y := 0; y < 256; y++ {
						want := x - y
						if want < 0 {
							want = -want
						}
						for _, v := range vms {
							v.WriteHeapSlice(v.GlobalAddr("a"), []byte{byte(x)})
							v.WriteHeapSlice(v.GlobalAddr("b"), []byte{byte(y)})
							if _, err := v.Call("execute", nil); err != nil {
								t.Fatal(err)
							}
							if int(binary.LittleEndian.Uint16(v.ReadHeap(v.GlobalAddr("result"), 2))) != want || v.ReadHeap(v.GlobalAddr("a"), 1)[0] != byte(x) || v.ReadHeap(v.GlobalAddr("b"), 1)[0] != byte(y) {
								t.Fatal("VM", x, y)
							}
						}
						mem[a], mem[b] = byte(x), byte(y)
						mem[out], mem[out+1] = 0xa5, 0x5a
						mem[0xeffe], mem[0xefff] = 0, 0xf1
						z.SetRegisters(emulator.Registers{F: byte(x ^ y), PC: uint16(entry), SP: 0xeffe})
						for step := 0; step < 1000 && z.GetPC() != 0xf100; step++ {
							z.Step()
						}
						r := z.GetRegisters()
						got := binary.LittleEndian.Uint16(mem[out : out+2])
						if int(got) != want || r.PC != 0xf100 || r.SP != 0xf000 || mem[a] != byte(x) || mem[b] != byte(y) {
							t.Fatal(language, mode, x, y, got, want, r)
						}
					}
				}
			})
		}
	}
}
