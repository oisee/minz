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

// C99 6.9.1: by-value aggregate parameters designate independent objects.
// C99 6.5.2.2: direct and indirect calls use the same parameter semantics.
func TestCStructParametersNativeVM(t *testing.T) {
	cases := []struct {
		name, expr string
		want       func(uint32) uint32
	}{
		{"word_copy", "word_copy()", func(v uint32) uint32 { return uint32(uint16(v)) }},
		{"two_aliases", "two_aliases()", func(v uint32) uint32 { return uint32(uint16(v)) }},
		{"wide_copy", "wide_copy()", func(v uint32) uint32 { return v }},
		{"array_copy", "array_copy()", func(v uint32) uint32 { return uint32(uint16(v)) }},
		{"union_copy", "union_copy()", func(v uint32) uint32 { return uint32(uint16(v)) }},
		{"nested_field", "nested_field()", func(v uint32) uint32 { return uint32(uint16(v)) }},
		{"dereferenced", "dereferenced()", func(v uint32) uint32 { return uint32(uint16(v)) }},
		{"indirect", "indirect_call()", func(v uint32) uint32 { return uint32(uint16(v)) }},
		{"compound_literal", "compound_literal()", func(v uint32) uint32 { return uint32(uint16(v + 3)) }},
		{"recursive", "recursive_copy()", func(v uint32) uint32 { n := v % 5; return uint32(uint16((n+2)*uint32(uint16(v)) + n*(n+1)/2)) }},
	}
	const prefix = `unsigned long input,result;
struct P{unsigned char x;unsigned int y;};
struct Wide{unsigned long value;unsigned char tag;};
struct Array{unsigned int words[3];unsigned char tag;};
union Value{unsigned int word;unsigned char bytes[2];};
struct Outer{unsigned char head;struct P inner;};
struct P original;
struct Wide wide;
struct Array array;
union Value shared;
struct Outer outer;
unsigned int update(struct P p){p.y=9;return p.y;}
unsigned int pair(struct P p,struct P q){p.y=9;return q.y;}
unsigned long update_wide(struct Wide p){p.value=7;return p.value;}
unsigned int update_array(struct Array p){p.words[1]=7;return p.words[1];}
unsigned int update_union(union Value p){p.word=7;return p.word;}
unsigned int word_copy(void){original.y=(unsigned int)input;update(original);return original.y;}
unsigned int two_aliases(void){original.y=(unsigned int)input;return pair(original,original);}
unsigned long wide_copy(void){wide.value=input;update_wide(wide);return wide.value;}
unsigned int array_copy(void){array.words[1]=(unsigned int)input;update_array(array);return array.words[1];}
unsigned int union_copy(void){shared.word=(unsigned int)input;update_union(shared);return shared.word;}
unsigned int nested_field(void){outer.inner.y=(unsigned int)input;update(outer.inner);return outer.inner.y;}
unsigned int dereferenced(void){struct P *p=&original;original.y=(unsigned int)input;update(*p);return original.y;}
unsigned int indirect_call(void){unsigned int (*fn)(struct P)=update;original.y=(unsigned int)input;fn(original);return original.y;}
unsigned int inspect(struct P p){return p.x+p.y;}
unsigned int compound_literal(void){return inspect((struct P){3,(unsigned int)input});}
unsigned int recursive(struct P p,unsigned int n){unsigned int saved=p.y;if(n){unsigned int child;p.y=p.y+1;child=recursive(p,n-1);return child+saved;}return saved;}
unsigned int recursive_copy(void){original.y=(unsigned int)input;return recursive(original,(unsigned int)(input%5))+original.y;}
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
				if root := os.Getenv("SYSTEM_C_STRUCT_PARAMETER_EXPORT"); root != "" {
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
