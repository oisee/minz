package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestBytePackSourceCallAndConditionNative(t *testing.T) {
	src := `fun perturb(x:u8)->u8 { return x+1 }
 fun packed_call(lo:u8,hi:u8)->u16 { let low:u16=lo as u16 let high:u16=hi as u16 let q:u8=perturb(lo) let shifted:u16=high<<8 let packed:u16=shifted|low return packed+(q as u16) }
 fun packed_test(lo:u8,hi:u8)->u8 { let low:u16=lo as u16 let high:u16=hi as u16 let shifted:u16=high<<8 let packed:u16=shifted|low if packed!=0 { return 1 } return 0 }`
	s, err := CompileHIRSteps(graceMeasureParse(t, src), Options{ContractOpt: true, AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	bin, err := z80asm.NewAssembler().AssembleString(s.Assembly)
	if err != nil || len(bin.Errors) > 0 {
		t.Fatal(err, bin.Errors)
	}
	vm := mir2.NewVM(s.MIR2Module)
	cases := 0
	for lo := 0; lo < 256; lo++ {
		for _, hi := range []int{0, 1, 127, 128, 255} {
			for _, name := range []string{"packed_call", "packed_test"} {
				f := s.MIR2Module.FuncByName(name)
				want := (hi<<8 | lo) + int(byte(lo+1))
				if name == "packed_test" {
					want = 0
					if hi != 0 || lo != 0 {
						want = 1
					}
				}
				want &= 65535
				v, err := vm.Call(name, []mir2.Value{{I: int64(lo)}, {I: int64(hi)}})
				if err != nil || v[0].I != int64(want) {
					t.Fatal("VM", name, lo, hi, want, v, err)
				}
				address := bin.Symbols[name]
				if address == 0 {
					address = bin.Symbols[strings.ToUpper(name)]
				}
				setup := ""
				// Use stack snapshot to initialize arbitrary byte ABI locations without sequential-copy aliasing.
				for i, value := range []int{lo, hi} {
					loc := s.Allocation.Locs[f.Contract.Params[i].Reg]
					if loc.Kind != mir2.LocReg {
						t.Fatal("unexpected byte ABI", loc)
					}
					setup += fmt.Sprintf("LD %s,%d\n", loc.Name, value)
				}
				boot, err := z80asm.NewAssembler().AssembleString(setup + fmt.Sprintf("CALL %d\nDI\nHALT\n", address))
				if err != nil || len(boot.Errors) > 0 {
					t.Fatal(err, boot.Errors)
				}
				z := emulator.NewRemogattoZ80()
				z.MaxCycles = 10000
				z.LoadMemory(uint16(bin.Origin), bin.Binary)
				z.LoadMemory(0xf100, boot.Binary)
				z.SetPC(0xf100)
				z.SetSP(0xf000)
				if err := z.Run(); err != nil {
					t.Fatal(err)
				}
				r := z.GetRegisters()
				got := int(r.HL)
				if name == "packed_test" {
					switch mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty) {
					case "A":
						got = int(r.A)
					case "C":
						got = int(byte(r.BC))
					default:
						t.Fatal("returnABI")
					}
				}
				if got != want || r.SP != 0xf000 {
					t.Fatalf("native%s lo%d hi%d got%d want%d SP%04x ASM%s", name, lo, hi, got, want, r.SP, s.Assembly)
				}
				cases++
			}
		}
	}
	t.Logf("%d independent source VM/native cases: call-preserved operands and explicit OR comparison PASS", cases)
}
