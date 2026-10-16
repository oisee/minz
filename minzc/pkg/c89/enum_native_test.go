package c89_test

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

// C99 6.7.2.2 uses a compatible integer representation for enum storage.
// This frontend also accepts wider constants (GNU/C23 extension); their
// representation follows the target ABI rather than host int assumptions.
func TestCEnumWidthsNativeVM(t *testing.T) {
	cases := []struct {
		name, expr string
		want       func(uint32) uint32
	}{
		{"constant", "Wmax", func(v uint32) uint32 { return 65536 }},
		{"global", "wide", func(v uint32) uint32 { return 65536 }},
		{"negative", "Wmin", func(v uint32) uint32 { return 0xffff0000 }},
		{"unsigned_max", "Umax", func(v uint32) uint32 { return 0xffffffff }},
		{"local", "local_value()", func(v uint32) uint32 { return v }},
		{"parameter", "parameter((enum Wide)(long)input)", func(v uint32) uint32 { return v }},
		{"return", "enum_return((long)input)", func(v uint32) uint32 { return v }},
		{"static", "static_value()", func(v uint32) uint32 { return 0xffff0000 }},
		{"layout", "sizeof(enum Small)+sizeof(enum Positive)+sizeof(enum Wide)+sizeof(enum Unsigned)+sizeof(struct Record)", func(v uint32) uint32 { return 19 }},
		{"array", "array_values[0]+array_values[1]", func(v uint32) uint32 { return 0 }},
		{"signed16", "(long)(enum Small)(int)input", func(v uint32) uint32 { return uint32(int32(int16(v))) }},
		{"unsigned16", "(enum Positive)(unsigned int)input", func(v uint32) uint32 { return uint32(uint16(v)) }},
		{"unsigned_arithmetic", "unsigned_arithmetic()", func(v uint32) uint32 { return v + 1 }},
		{"signed_compare", "((enum Small)(int)input)<0", func(v uint32) uint32 {
			if int16(v) < 0 {
				return 1
			}
			return 0
		}},
	}
	const prefix = `unsigned long input,result;
enum Small{Smin=-1,Smax=32767};
enum Positive{Pmin=32768,Pmax=65535};
enum Wide{Wmin=-65536L,Wmax=65536L};
enum Unsigned{Umin=0x80000000UL,Umax=0xffffffffUL};
enum Wide wide=Wmax;
enum Wide array_values[2]={Wmax,Wmin};
struct Record{unsigned char tag;enum Wide value;unsigned int tail;};
unsigned long local_value(void){enum Wide value=(enum Wide)(long)input;return value;}
unsigned long parameter(enum Wide value){return value;}
enum Wide enum_return(long value){return (enum Wide)value;}
unsigned long static_value(void){static enum Wide value=Wmin;return value;}
unsigned long unsigned_arithmetic(void){enum Unsigned value=(enum Unsigned)input;return value+1UL;}
`
	values := []uint32{0, 1, 2, 127, 128, 255, 256, 32767, 32768, 65535, 65536, 0x12345678, 0x7fffffff, 0x80000000, 0xffffffff}
	for _, c := range cases {
		for _, mode := range []string{"default", "grace", "size"} {
			t.Run(c.name+"/"+mode, func(t *testing.T) {
				parse := func() *hir.Module {
					h, e := c89.Compile(prefix+fmt.Sprintf("void execute(void){result=%s;}", c.expr), "enum-widths.c")
					if e != nil {
						t.Fatal(e)
					}

					return h
				}
				raw := mir2.NewVM(hir.LowerModule(parse()))
				steps, e := pipeline.CompileHIRSteps(parse(), pipeline.Options{ContractOpt: true, AssertMode: "none", UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
				if e != nil {
					t.Fatal(e)
				}
				final := mir2.NewVM(steps.MIR2Module)
				bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
				if e != nil {
					t.Fatal(e)
				}
				if len(bin.Errors) > 0 {
					t.Fatal(bin.Errors)
				}
				if root := os.Getenv("SYSTEM_C_ENUM_EXPORT"); root != "" {
					dir := filepath.Join(root, c.name, mode)
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					for name, data := range map[string][]byte{"source.c": []byte(prefix + fmt.Sprintf("void execute(void){result=%s;}", c.expr)), "output.a80": []byte("ORG 0x8000\n" + steps.Assembly), "output.bin": bin.Binary, "source.hir": []byte(parse().Dump())} {
						if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
							t.Fatal(err)
						}
					}
				}
				symbol := func(n string) int {
					p := bin.Symbols[n]
					if p == 0 {
						p = bin.Symbols["v_"+n]
					}
					if p == 0 {
						t.Fatalf("missing %s", n)
					}
					return p
				}
				z := emulator.NewRemogattoZ80()
				z.LoadMemory(0x8000, bin.Binary)
				z.LoadMemory(0xeffe, []byte{0, 0xf1})
				for _, v := range values {
					want := c.want(v)
					var data [4]byte
					binary.LittleEndian.PutUint32(data[:], v)
					for _, vm := range []*mir2.VM{raw, final} {
						vm.WriteHeapSlice(vm.GlobalAddr("input"), data[:])
						if _, e := vm.Call("execute", nil); e != nil {
							t.Fatal(e)
						}
						if got := binary.LittleEndian.Uint32(vm.ReadHeap(vm.GlobalAddr("input"), 4)); got != v {
							t.Fatalf("VM corrupted input: got %08x want %08x", got, v)
						}
						if got := binary.LittleEndian.Uint32(vm.ReadHeap(vm.GlobalAddr("result"), 4)); got != want {
							t.Fatalf("VM input=%08x got=%08x want=%08x", v, got, want)
						}
					}
					for i := 0; i < 32; i++ {
						z.Memory()[i] = byte(0xA5 ^ i)
					}
					copy(z.Memory()[symbol("input"):], data[:])
					z.SetRegisters(emulator.Registers{PC: uint16(symbol("execute")), SP: 0xeffe})
					for n := 0; n < 20000 && z.GetRegisters().PC != 0xf100; n++ {
						z.Step()
					}
					for i := 0; i < 32; i++ {
						if z.Memory()[i] != byte(0xA5^i) {
							t.Fatalf("Z80 wrote to protected low memory at %d", i)
						}
					}
					r := z.GetRegisters()
					got := binary.LittleEndian.Uint32(z.Memory()[symbol("result") : symbol("result")+4])
					if got != want || r.PC != 0xf100 || r.SP != 0xf000 {
						t.Fatalf("Z80 input=%08x got=%08x want=%08x regs=%v\n%s", v, got, want, r, steps.Assembly)
					}
				}
			})
		}
	}
}
