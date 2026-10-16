package pipeline

import (
	"bytes"
	"encoding/binary"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

// C90 6.3.6/6.3.2.1: unsigned-byte index promotions preserve 0..254;
// the pointers remain within buffer[512]. Copy order is independently modeled.
// Nanz uses explicit byte arithmetic. This also retains a returned loop index.
func TestCountedPointerWalkNativeOverlapCAndNanz(t *testing.T) {
	for _, language := range []string{"c", "nanz"} {
		for _, mode := range []string{"default", "size", "grace"} {
			t.Run(language+"/"+mode, func(t *testing.T) {
				var h *hir.Module
				var err error
				if language == "c" {
					h, err = c89.CompileWithOpts(`unsigned char buffer[512],n;unsigned short src_off,dst_off,result;
unsigned char transfer(unsigned char *dst,unsigned char *src,unsigned char count){unsigned char i;for(i=0;i<count;i++)dst[i]=src[i];return i;}
void execute(void){result=transfer(buffer+dst_off,buffer+src_off,n);}`, "overlap.c", c89.CompileOpts{NoLibc: true})
				} else {
					h, err = nanz.Parse("global buffer:[u8;512]\nglobal n:u8\nglobal src_off:u16\nglobal dst_off:u16\nglobal result:u16\nfun transfer(dst:^u8,src:^u8,count:u8)->u8 {var i:u8=0\nwhile i<count {dst[i]=src[i]\ni=i+1}\nreturn i}\nfun execute(){result=u16(transfer(&buffer+dst_off,&buffer+src_off,n))}", "overlap.nanz")
				}
				if err != nil {
					t.Fatal(err)
				}
				raw := hir.LowerModule(h)
				s, err := CompileHIRSteps(h, Options{ContractOpt: true, OptSize: mode == "size", UseGrace: mode == "grace", UseHIRGrace: mode == "grace"})
				if err != nil {
					t.Fatal(err)
				}
				a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
				if err != nil || len(a.Errors) != 0 {
					t.Fatal(err, a.Errors)
				}
				addr := func(name string) int {
					p := a.Symbols[name]
					if p == 0 {
						p = a.Symbols["v_"+name]
					}
					if p == 0 {
						t.Fatal(name)
					}
					return p
				}
				z := emulator.NewRemogattoZ80()
				mem := z.Memory()
				copy(mem[0x8000:], a.Binary)
				vms := []*mir2.VM{mir2.NewVM(raw), mir2.NewVM(s.MIR2Module)}
				for _, delta := range []int{-17, -1, 0, 1, 17} {
					for count := 0; count < 256; count++ {
						initial := make([]byte, 512)
						for i := range initial {
							initial[i] = byte((i * 37) ^ (i >> 3) ^ 0x5a)
						}
						want := bytes.Clone(initial)
						src, dst := 32, 32+delta
						for i := 0; i < count; i++ {
							want[dst+i] = want[src+i]
						}
						inputs := map[string][]byte{"buffer": initial, "n": {byte(count)}, "src_off": {byte(src), 0}, "dst_off": {byte(dst), 0}, "result": {0xa5, 0x5a}}
						for _, vm := range vms {
							for name, b := range inputs {
								vm.WriteHeapSlice(vm.GlobalAddr(name), b)
							}
							if _, err := vm.Call("execute", nil); err != nil {
								t.Fatal(delta, count, err)
							}
							if !bytes.Equal(vm.ReadHeap(vm.GlobalAddr("buffer"), 512), want) || int(binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("result"), 2))) != count {
								t.Fatal("VM", delta, count)
							}
						}
						for name, b := range inputs {
							copy(mem[addr(name):], b)
						}
						mem[0xeffe], mem[0xefff] = 0, 0xf1
						z.SetRegisters(emulator.Registers{F: byte(count), PC: uint16(addr("execute")), SP: 0xeffe})
						for step := 0; step < 20000 && z.GetPC() != 0xf100; step++ {
							z.Step()
						}
						r := z.GetRegisters()
						if r.PC != 0xf100 || r.SP != 0xf000 || !bytes.Equal(mem[addr("buffer"):addr("buffer")+512], want) || int(binary.LittleEndian.Uint16(mem[addr("result"):addr("result")+2])) != count {
							t.Fatal("Z80", delta, count, r)
						}
						for _, name := range []string{"n", "src_off", "dst_off"} {
							if !bytes.Equal(mem[addr(name):addr(name)+len(inputs[name])], inputs[name]) {
								t.Fatal("input clobbered", name)
							}
						}
					}
				}
			})
		}
	}
}
