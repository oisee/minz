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
		{"forward-function-pointer", `int target(int,int,int); typedef int (*Fn)(int,int,int); int invoke(Fn fn){return fn(2,3,4);} void execute(void){result=invoke((Fn)target);} int target(int a,int b,int c){return a+2*b+3*c;}`, 20},
		{"four-pointer-callback", `typedef int (*Fn)(int*,int*,int*,int*); int a=1,b=2,c=3,d=4; int action(int*p,int*q,int*r,int*s){*s+=1; return *p+2**q+3**r+4**s;} int invoke(Fn fn,int*p,int*q,int*r,int*s){int keep=*p; int x=fn(p,q,r,s); return x+keep;} void execute(void){result=invoke(action,&a,&b,&c,&d)+a+b+c+d;}`, 46},
		{"backslash-data", `void execute(void){const char*s="\\n";result=(unsigned char)s[0]+256u*(unsigned char)s[1];}`, 28252},
		{"semicolon-data", `void execute(void){const char*s=";#";result=(unsigned char)s[0]+256u*(unsigned char)s[1];}`, 9019},
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
		{"limit-overflow", "unsigned char data[1]={\n#embed \"data.bin\" limit(999999999999999999999999999999999999999999)\n};", true},
		{"offset-overflow", "unsigned char data[1]={\n#embed \"data.bin\" offset(999999999999999999999999999999999999999999)\n};", true},
		{"inactive", "#if 0\nunsigned char data[1]={\n#embed \"missing.bin\"\n};\n#endif\nint answer(void){return 7;}", false},
		{"comment", "/*\n#embed \"missing.bin\"\n*/\nint answer(void){return 7;}", false},
		{"present", "unsigned char data[2]={\n#embed \"data.bin\"\n}; int answer(void){return data[0]+data[1];}", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := c89.CompileWithOpts(tc.src, "embed-safety.c", c89.CompileOpts{BaseDir: dir})
			if tc.bad {
				if err == nil || !strings.Contains(err.Error(), "#embed") || tc.name == "missing" && !strings.Contains(err.Error(), "missing.bin") {
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

// C23 6.2.5 / 6.7.1: bit-precise integer and constexpr declarations
// cannot be implemented by replacing their spelling with a native type/const.
func TestUnsupportedC23DiagnosticsAfterPreprocessing(t *testing.T) {
	for _, construct := range []struct{ name, source string }{
		{"_BitInt", "signed _BitInt(4) x=-1;"},
		{"_BitInt", "unsigned _BitInt(33) x;"},
		{"constexpr", "int make(void){return 3;} constexpr int x=make();"},
		{"nullptr", "int answer(void){return _Generic(nullptr, void*:1, default:2);}"},
	} {
		if _, err := c89.Compile(construct.source, "unsupported.c"); err == nil || !strings.Contains(err.Error(), "unsupported "+construct.name) {
			t.Fatalf("expected %s diagnostic: %v", construct.name, err)
		}
		if _, err := c89.Compile("#if 0\n"+construct.source+"\n#endif\nint answer(void){return 7;}", "inactive.c"); err != nil {
			t.Fatalf("inactive %s: %v", construct.name, err)
		}
	}
	for _, src := range []string{
		"#define UNUSED _BitInt(33)\nint answer(void){return 7;}",
		"#define SPELL(x) #x\nint answer(void){return sizeof(SPELL(_BitInt(4)));}",
		"#define constexpr const\nconstexpr int x=3; int answer(void){return x;}",
	} {
		if _, err := c89.Compile(src, "macro.c"); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "unsupported.h"), []byte("#define TYPE _BitInt(4)\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := c89.CompileWithOpts("#include \"unsupported.h\"\nTYPE x;", "header.c", c89.CompileOpts{IncludePaths: []string{dir}})
	if err == nil || !strings.Contains(err.Error(), "unsupported _BitInt") {
		t.Fatal(err)
	}
}
