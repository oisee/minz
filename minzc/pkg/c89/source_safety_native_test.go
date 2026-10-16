package c89_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestCSourceSafetyRawFinalNative(t *testing.T) {
	// String spelling follows C99 6.4.5; generic selection follows C11 6.5.1.1.
	// In particular the controlling expression and unselected branch are not
	// evaluated, and pointer/aggregate compatibility is not merely their Kind.
	cases := []struct {
		name, body string
		want       uint16
	}{
		{"apostrophe", `void execute(void){result=sizeof("a'b");}`, 4},
		{"bitint-string", `void execute(void){result=sizeof("_BitInt(4)");}`, 11},
		{"attribute-string", `void execute(void){result=sizeof("[[nodiscard]]");}`, 14},
		{"digit-code", `void execute(void){result=1'234;}`, 1234},
		{"stringification", "#define Q(x) #x\nvoid execute(void){result=sizeof(Q(a'b'));}", 5},
		{"attribute-code", `[[deprecated("old")]] int value(void){return 7;} void execute(void){result=value();}`, 7},
		{"pointer-ab", `void execute(void){result=_Generic((int*)0,int*:1,char*:2,default:3);}`, 1},
		{"pointer-ba", `void execute(void){result=_Generic((int*)0,char*:2,int*:1,default:3);}`, 1},
		{"aggregate", `struct A{int x;}; struct B{int x;}; void execute(void){result=_Generic((struct A){0},struct B:2,struct A:1,default:3);}`, 1},
		{"unevaluated", `int calls; int bump(void){calls++;return 9;} void execute(void){result=_Generic(bump(),int:7,default:bump());result=result*16+calls;}`, 112},
		{"selected-effects", `int calls; int bump(void){calls++;return 9;} void execute(void){result=_Generic(0,int:bump(),default:7);result=result*16+calls;}`, 145},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, grace := range []bool{false, true} {
				parse := func() *hir.Module {
					m, err := c89.Compile("unsigned short result;\n"+tc.body, "source-safety.c")
					if err != nil {
						t.Fatal(err)
					}
					return m
				}
				raw := hir.LowerModule(parse())
				steps, err := pipeline.CompileHIRSteps(parse(), pipeline.Options{ContractOpt: true, UseGrace: grace, UseHIRGrace: grace})
				if err != nil {
					t.Fatal(err)
				}
				for _, m := range []*mir2.Module{raw, steps.MIR2Module} {
					vm := mir2.NewVM(m)
					if _, err := vm.Call("execute", nil); err != nil {
						t.Fatal(err)
					}
					if got := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("result"), 2)); got != tc.want {
						t.Fatalf("VM got %d want %d", got, tc.want)
					}
				}
				a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
				if err != nil || len(a.Errors) != 0 {
					t.Fatal(err, a.Errors)
				}
				entry, result := a.Symbols["execute"], a.Symbols["result"]
				if entry == 0 || result == 0 {
					t.Fatal("missing native symbols")
				}
				z := emulator.NewRemogattoZ80()
				mem := z.Memory()
				copy(mem[0x8000:], a.Binary)
				mem[result], mem[result+1] = 0xef, 0xbe
				mem[0xeffe], mem[0xefff] = 0, 0xf1
				z.SetRegisters(emulator.Registers{PC: uint16(entry), SP: 0xeffe})
				for i := 0; i < 10000 && z.GetPC() != 0xf100; i++ {
					z.Step()
				}
				regs := z.GetRegisters()
				got := binary.LittleEndian.Uint16(mem[result : result+2])
				if got != tc.want || regs.PC != 0xf100 || regs.SP != 0xf000 {
					t.Fatalf("grace=%v got=%d want=%d PC/SP=%04x/%04x", grace, got, tc.want, regs.PC, regs.SP)
				}
			}
		})
	}
}

func TestCEmbedDiagnosticsAndInactiveBranches(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "data.bin"), []byte{195, 60}, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, src string
		bad       bool
	}{
		{"missing", "unsigned char data[1]={\n#embed \"missing.bin\"\n};", true},
		{"inactive", "#if 0\nunsigned char data[1]={\n#embed \"missing.bin\"\n};\n#endif\nint answer(void){return 7;}", false},
		{"comment", "/*\n#embed \"missing.bin\"\n*/\nint answer(void){return 7;}", false},
		{"present", "unsigned char data[2]={\n#embed \"data.bin\"\n}; int answer(void){return data[0]+data[1];}", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := c89.CompileWithOpts(tc.src, "embed-safety.c", c89.CompileOpts{BaseDir: dir})
			if tc.bad {
				if err == nil || !strings.Contains(err.Error(), "#embed") || !strings.Contains(err.Error(), "missing.bin") {
					t.Fatalf("missing asset must give an explicit #embed file error: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			vm := mir2.NewVM(hir.LowerModule(m))
			got, err := vm.Call("answer", nil)
			want := int64(7)
			if tc.name == "present" {
				want = 255
			}
			if err != nil || len(got) != 1 || got[0].I != want {
				t.Fatal(got, err)
			}
		})
	}
}
