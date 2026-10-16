package c89_test

import (
	"encoding/binary"
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

// C99 6.5.16.2 and 6.5.2.4: evaluate the lvalue once; assignment
// expressions yield the converted assigned value, not another object read.
func TestCLvalueSingleEvaluationNativeVM(t *testing.T) {
	byteUpdate := func(v uint32) uint32 { return uint32(uint8(v) + 2) }
	byteNext := func(v uint32) uint32 { return uint32(uint8(v) + 1) }
	bytePrev := func(v uint32) uint32 { return uint32(uint8(v) - 1) }
	wordUpdate := func(v uint32) uint32 { return uint32(uint16(v) + 5) }
	cases := []struct {
		name, expr string
		want       func(uint32) uint32
	}{
		{"compound", "*pick()+=2;result=calls*1000UL+bytes[0]", func(v uint32) uint32 { return 1000 + byteUpdate(v) }},
		{"preinc", "unsigned char n=++*pick();result=calls*1000UL+bytes[0]*10UL+n", func(v uint32) uint32 { return 1000 + 11*byteNext(v) }},
		{"postinc", "unsigned char n=(*pick())++;result=calls*1000UL+bytes[0]*10UL+n", func(v uint32) uint32 { return 1000 + 10*byteNext(v) + uint32(uint8(v)) }},
		{"predec", "unsigned char n=--*pick();result=calls*1000UL+bytes[0]*10UL+n", func(v uint32) uint32 { return 1000 + 11*bytePrev(v) }},
		{"postdec", "unsigned char n=(*pick())--;result=calls*1000UL+bytes[0]*10UL+n", func(v uint32) uint32 { return 1000 + 10*bytePrev(v) + uint32(uint8(v)) }},
		{"assign_expr", "unsigned char n=(*pick()=263);result=calls*1000UL+bytes[0]*10UL+n", func(v uint32) uint32 { return 1077 }},
		{"field", "record()->value+=5;result=calls*100000UL+item.value", func(v uint32) uint32 { return 100000 + wordUpdate(v) }},
		{"index", "words[index()]+=5;result=calls*100000UL+words[1]", func(v uint32) uint32 { return 100000 + wordUpdate(v) }},
		{"pointer_post", "unsigned char *p=bytes;*p+++=2;result=(p-bytes)*1000UL+bytes[0]", func(v uint32) uint32 { return 1000 + byteUpdate(v) }},
		{"compound_expr", "unsigned char n=(*pick()+=2);result=calls*1000UL+bytes[0]*10UL+n", func(v uint32) uint32 { return 1000 + 11*byteUpdate(v) }},
		{"aggregate_assign_expr", "struct Record copy=(item=(struct Record){2,(unsigned int)input,9});result=copy.value+0UL+item.value+copy.head+copy.tail", func(v uint32) uint32 { return 2*uint32(uint16(v)) + 11 }},
		{"nested_assign", "*pick()=(*pick()=7);result=calls*1000UL+bytes[0]", func(v uint32) uint32 { return 2007 }},
	}
	const prefix = `unsigned long input,result;
unsigned int calls;
unsigned char bytes[3];
unsigned int words[3];
struct Record{unsigned char head;unsigned int value;unsigned char tail;};
struct Record item;
unsigned char *pick(void){calls++;return &bytes[0];}
struct Record *record(void){calls++;return &item;}
unsigned int index(void){calls++;return 1;}
`
	makeSource := func(body string) string {
		return prefix + "void execute(void){calls=0;bytes[0]=(unsigned char)input;bytes[1]=99;words[1]=(unsigned int)input;item.value=(unsigned int)input;" + body + ";}"
	}
	values := []uint32{0, 1, 2, 127, 128, 255, 256, 32767, 32768, 65535, 65536, 0x12345678, 0x7fffffff, 0x80000000, 0xffffffff}
	for _, c := range cases {
		for _, mode := range []string{"default", "grace", "size"} {
			t.Run(c.name+"/"+mode, func(t *testing.T) {
				parse := func() *hir.Module {
					h, e := c89.Compile(makeSource(c.expr), "lvalue-evaluation.c")
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
				if root := os.Getenv("SYSTEM_C_LVALUE_EXPORT"); root != "" {
					dir := filepath.Join(root, c.name, mode)
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					for name, data := range map[string][]byte{"source.c": []byte(makeSource(c.expr)), "output.a80": []byte("ORG 0x8000\n" + steps.Assembly), "output.bin": bin.Binary, "source.hir": []byte(parse().Dump())} {
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
				if err := z.LoadMemory(0x8000, bin.Binary); err != nil {
					t.Fatal(err)
				}
				if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
					t.Fatal(err)
				}
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
