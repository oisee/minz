package pipeline

import (
	"encoding/binary"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"strings"
	"testing"
)

func wideCmpOracle(ty, op string, a, b uint32) byte {
	eq := a == b
	lt := a < b
	if ty == "i32" {
		lt = int32(a) < int32(b)
	}
	ok := false
	switch op {
	case "==":
		ok = eq
	case "!=":
		ok = !eq
	case "<":
		ok = lt
	case "<=":
		ok = lt || eq
	case ">":
		ok = !lt && !eq
	case ">=":
		ok = !lt
	}
	if ok {
		return 1
	}
	return 0
}
func TestU32ComparisonSourceNative(t *testing.T) {
	// Book v7 §§3.1,3.2,3.4: typed integer comparisons, independent uint32/int32 oracle.
	vals := []uint32{0, 1, 255, 256, 65535, 65536, 0x1000000, 0x7fffffff, 0x80000000, 0xffffffff, 0x12345678}
	for bit := 0; bit < 32; bit++ {
		vals = append(vals, 1<<bit, ^(uint32(1) << bit))
	}
	for _, helper := range []bool{false, true} {
		for _, ty := range []string{"u32", "i32"} {
			for _, op := range []string{"==", "!=", "<", "<=", ">", ">="} {
				t.Run(fmt.Sprintf("%s%s/helper%v", ty, op, helper), func(t *testing.T) {
					source := fmt.Sprintf("global a:%s\nglobal b:%s\nglobal result:u8\nfun execute(){result=u8(a %s b)}", ty, ty, op)
					if helper {
						source = fmt.Sprintf("global a:%s\nglobal b:%s\nglobal result:u8\nfun compare(x:%s,y:%s)->bool {if x %s y {return true}\nreturn false}\nfun execute(){result=u8(compare(a,b))}", ty, ty, ty, ty, op)
					}
					parse := func() *hir.Module {
						h, e := nanz.Parse(source, "wide_compare.nanz")
						if e != nil {
							t.Fatal(e)
						}
						return h
					}
					raw := mir2.NewVM(hir.LowerModule(parse()))
					s, e := CompileHIRSteps(parse(), Options{ContractOpt: true, AssertMode: "none"})
					if e != nil {
						t.Fatal(e)
					}
					if helper && !strings.Contains(s.Assembly, "CALL compare") {
						t.Fatal("actual helper call missing", s.Assembly)
					}
					final := mir2.NewVM(s.MIR2Module)
					bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
					if e != nil || len(bin.Errors) > 0 {
						t.Fatal(e, bin.Errors)
					}
					if d := os.Getenv("U32_COMPARE_EXPORT"); d != "" {
						os.MkdirAll(d, 0755)
						stem := fmt.Sprintf("%s-%x-helper%v", ty, op, helper)
						for ext, b := range map[string][]byte{"nanz": []byte(source), "a80": []byte(s.Assembly), "bin": bin.Binary, "mir": []byte(s.MIR2Module.Dump())} {
							os.WriteFile(d+"/"+stem+"."+ext, b, 0644)
						}
					}
					sym := func(n string) int {
						v := bin.Symbols[n]
						if v == 0 {
							v = bin.Symbols["v_"+n]
						}
						if v == 0 {
							t.Fatal(n)
						}
						return v
					}
					z := emulator.NewRemogattoZ80()
					z.LoadMemory(0x8000, bin.Binary)
					z.LoadMemory(0xeffe, []byte{0, 0xf1})
					for _, a := range vals {
						for _, b := range vals {
							want := wideCmpOracle(ty, op, a, b)
							for _, vm := range []*mir2.VM{raw, final} {
								var buf [4]byte
								binary.LittleEndian.PutUint32(buf[:], a)
								vm.WriteHeapSlice(vm.GlobalAddr("a"), buf[:])
								binary.LittleEndian.PutUint32(buf[:], b)
								vm.WriteHeapSlice(vm.GlobalAddr("b"), buf[:])
								if _, e := vm.Call("execute", nil); e != nil {
									t.Fatal(e)
								}
								if got := vm.ReadHeap(vm.GlobalAddr("result"), 1)[0]; got != want {
									t.Fatalf("VM a%x b%x got%d want%d", a, b, got, want)
								}
							}
							binary.LittleEndian.PutUint32(z.Memory()[sym("a"):sym("a")+4], a)
							binary.LittleEndian.PutUint32(z.Memory()[sym("b"):sym("b")+4], b)
							z.SetRegisters(emulator.Registers{A: 0x5a, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468, PC: uint16(sym("execute")), SP: 0xeffe})
							for n := 0; n < 10000 && z.GetRegisters().PC != 0xf100; n++ {
								z.Step()
							}
							r := z.GetRegisters()
							if got := z.Memory()[sym("result")]; got != want || r.PC != 0xf100 || r.SP != 0xf000 {
								t.Fatalf("native a%08x b%08x got%d want%d regs%v\n%s", a, b, got, want, r, s.Assembly)
							}
						}
					}
					// Only exact declared globals and generated spill slots may mutate.
					for i, v := range bin.Binary {
						addr := bin.Origin + i
						mutable := false
						for n, start := range bin.Symbols {
							if (strings.HasPrefix(n, "_spill_execute_r") || strings.HasPrefix(n, "_spill_compare_r")) && addr >= start && addr < start+4 {
								mutable = true
							}
						}
						for _, n := range []string{"a", "b", "result"} {
							size := 4
							if n == "result" {
								size = 1
							}
							start := sym(n)
							if addr >= start && addr < start+size {
								mutable = true
							}
						}
						if !mutable && z.Memory()[addr] != v {
							t.Fatalf("code/literal mutation at %x", addr)
						}
					}

				})
			}
		}
	}
}
