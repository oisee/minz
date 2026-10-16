package c89_test

import (
	"encoding/binary"
	"fmt"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

// C99 6.7.2.1: union fields overlap; their enclosing struct and arrays
// retain the declared byte layout. Typed initializer offsets preserve data.
func TestCNestedUnionLayoutNativeVM(t *testing.T) {
	cases := []struct {
		name, expr string
		want       func(uint32) uint32
	}{
		{"global", "envelope.tail+envelope.inner.bytes[2]", func(v uint32) uint32 { return 9 }},
		{"array_stride", "records[1].tail+records[1].inner.bytes[2]", func(v uint32) uint32 { return 300 }},
		{"local", "local_record()", func(v uint32) uint32 { return 34 }},
		{"overlap", "overlap()", func(v uint32) uint32 { return 70 }},
		{"tail", "tail_write()", func(v uint32) uint32 { return uint32(uint8(v)) + 770 }},
		{"by_value", "by_value()", func(v uint32) uint32 { return 5 }},
		{"anonymous", "anonymous_union()", func(v uint32) uint32 { return uint32(uint16(v + 6)) }},
		{"deep", "deep_value()", func(v uint32) uint32 { return 17 }},
		{"anonymous_types", "anonymous_types()", func(v uint32) uint32 { return 515 }},
	}
	const prefix = `unsigned long input,result;
union U{unsigned char bytes[3];unsigned int word;};
struct Envelope{unsigned char head;union U inner;unsigned char tail;};
struct Envelope envelope={1,{{2,3,4}},5};
struct Envelope records[2]={{1,{{2,3,4}},5},{10,{{20,30,200}},100}};
struct Anonymous{unsigned char head;union{unsigned char byte;unsigned int word;};unsigned char tail;};
struct Anonymous anonymous={1,{2},5};
struct Deep{unsigned char first;struct Envelope embedded;unsigned char last;};
struct Deep deep={7,{1,{{2,3,4}},5},11};
unsigned int local_record(void){struct Envelope value={11,{{4,5,6}},17};return value.head+value.inner.bytes[2]+value.tail;}
unsigned int overlap(void){union U value;value.word=0x1234;return value.bytes[0]+value.bytes[1];}
unsigned int tail_write(void){envelope.tail=(unsigned char)input;return envelope.tail+envelope.inner.word;}
unsigned int mutate(struct Envelope value){value.tail=8;value.inner.word=7;return value.tail;}
unsigned int by_value(void){envelope.tail=5;mutate(envelope);return envelope.tail;}
unsigned int anonymous_union(void){anonymous.word=(unsigned int)input;return anonymous.head+anonymous.tail+anonymous.word;}
unsigned int deep_value(void){return deep.last+deep.embedded.tail+deep.embedded.head;}
unsigned int anonymous_types(void){struct{unsigned char x;unsigned char y;}first={1,2};struct{unsigned char x;unsigned int y;}second={3,513};return first.y+second.y;}
`
	values := []uint32{0, 1, 2, 127, 128, 255, 256, 32767, 32768, 65535, 65536, 0x12345678, 0x7fffffff, 0x80000000, 0xffffffff}
	for _, c := range cases {
		for _, mode := range []string{"default", "grace", "size"} {
			t.Run(c.name+"/"+mode, func(t *testing.T) {
				parse := func() *hir.Module {
					h, e := c89.Compile(prefix+fmt.Sprintf("void execute(void){result=%s;}", c.expr), "struct-parameter.c")
					if e != nil {
						t.Fatal(e)
					}

					for name, want := range map[string]int{"envelope": 5, "records": 10, "anonymous": 4, "deep": 7} {
						found := false
						for _, g := range h.Globals {
							if g.Name == name {
								found = true
								if mir2.ByteWidth(g.Ty) != want {
									t.Fatalf("%s layout=%d want=%d", name, mir2.ByteWidth(g.Ty), want)
								}
							}
						}
						if !found {
							t.Fatal("missing global", name)
						}
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
				if root := os.Getenv("SYSTEM_C_UNION_LAYOUT_EXPORT"); root != "" {
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
				for name, offset := range map[string]int{"envelope__tail": 4, "anonymous__tail": 3, "deep__last": 6} {
					base := "envelope"
					if name == "anonymous__tail" {
						base = "anonymous"
					}
					if name == "deep__last" {
						base = "deep"
					}
					if got := symbol(name) - symbol(base); got != offset {
						t.Fatalf("exported field symbol %s: offset %d want %d", name, got, offset)
					}
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
