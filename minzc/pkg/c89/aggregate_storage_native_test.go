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

// C99 6.2.4, 6.3.2.1, 6.5.2.1 and 6.7.8: inline array layout,
// pointer decay, automatic lifetime and typed initializer offsets.
// Oracles below are independent arithmetic on source-level values.
func TestCAggregateStorageNativeVM(t *testing.T) {
	cases := []struct {
		name, expr string
		want       func(uint32) uint32
	}{
		{"field_array", "packet.data[2]+packet.tag", func(v uint32) uint32 { return 516 }},
		{"field_write", "field_write()", func(v uint32) uint32 { return uint32(uint8(v)) + 513 }},
		{"global_matrix", "global_matrix()", func(v uint32) uint32 { return uint32(uint16(v + 4)) }},
		{"array_of_structs", "records[1].data[2]+records[1].tag", func(v uint32) uint32 { return 1040 }},
		{"union_array", "union_array()", func(v uint32) uint32 { return uint32(uint8(v)) }},
		{"auto_bytes", "auto_bytes()", func(v uint32) uint32 { return uint32(uint8(v)) + 5 }},
		{"auto_longs", "auto_longs()", func(v uint32) uint32 { return v + 0x12345678 }},
		{"auto_matrix", "auto_matrix()", func(v uint32) uint32 { return uint32(uint16(v + 4)) }},
		{"auto_sparse", "auto_sparse()", func(v uint32) uint32 { return uint32(uint16(v + 9)) }},
		{"auto_packet", "auto_packet()", func(v uint32) uint32 { return uint32(uint16(v + 3)) }},
		{"flexible_array", "flexible_array()", func(v uint32) uint32 { return uint32(uint16(v + 2)) }},
		{"func_name", "func_name()", func(v uint32) uint32 { return 102 }},
		{"auto_string", "auto_string()", func(v uint32) uint32 { return 131 }},
		{"auto_structs", "auto_structs()", func(v uint32) uint32 { return uint32(uint16(v + 7)) }},
		{"recursive_array", "recursive_array((unsigned int)(input%5))", func(v uint32) uint32 { n := v % 5; return n * (n + 1) / 2 }},
		{"pointer_cell_field", "pointer_cell_field()", func(v uint32) uint32 { return 513 }},
		{"reversed_index", "2[packet.data]", func(v uint32) uint32 { return 3 }},
	}
	const prefix = `unsigned long input,result;
struct Packet{unsigned char data[3];unsigned int tag;};
struct Packet packet={{1,2,3},513};
struct Packet records[2]={{{4,5,6},257},{{7,8,9},1031}};
unsigned int matrix[2][3]={{1,2,3},{4,5,6}};
union Buffer{unsigned char bytes[5];unsigned int word;};
union Buffer buffer;
struct Flex{unsigned int len;unsigned int data[];};
unsigned char backing[8];
unsigned int flexible_array(void){struct Flex *p=(struct Flex *)backing;p->len=2;p->data[0]=(unsigned int)input;return p->len+p->data[0];}
unsigned int field_write(void){packet.data[1]=(unsigned char)input;return packet.data[1]+packet.tag;}
unsigned int global_matrix(void){matrix[1][2]=(unsigned int)input;return matrix[1][2]+matrix[1][0];}
unsigned int union_array(void){buffer.bytes[4]=(unsigned char)input;return buffer.bytes[4];}
unsigned int auto_bytes(void){unsigned char bytes[4]={2,(unsigned char)input,3};return bytes[0]+bytes[1]+bytes[2]+bytes[3];}
unsigned long auto_longs(void){unsigned long values[2]={0x12345678UL,input};return values[0]+values[1];}
unsigned int auto_matrix(void){unsigned int values[2][2]={{1,2},{3,4}};values[0][1]=(unsigned int)input;return values[0][1]+values[1][1];}
unsigned int auto_sparse(void){unsigned int values[5]={[3]=(unsigned int)input,[1]=9};return values[0]+values[1]+values[2]+values[3]+values[4];}
unsigned int auto_packet(void){struct Packet value={{1,2,3},(unsigned int)input};return value.data[2]+value.tag;}
unsigned int func_name(void){return __func__[0]+__func__[9];}
unsigned int auto_string(void){unsigned char text[4]="AB";return text[0]+text[1]+text[2]+text[3];}
unsigned int auto_structs(void){struct Packet values[2]={{{1,2,3},5},{{4,5,6},7}};values[1].tag=(unsigned int)input;return values[1].tag+values[0].tag+values[0].data[1];}
unsigned int recursive_array(unsigned int n){unsigned int values[2];values[0]=n;values[1]=0;if(n){values[1]=recursive_array(n-1);}return values[0]+values[1];}
unsigned int pointer_cell_field(void){struct Packet *p=&packet;struct Packet **pp=&p;return (*pp)->tag;}
`
	values := []uint32{0, 1, 2, 127, 128, 255, 256, 32767, 32768, 65535, 65536, 0x12345678, 0x7fffffff, 0x80000000, 0xffffffff}
	for _, c := range cases {
		for _, mode := range []string{"default", "grace", "size"} {
			t.Run(c.name+"/"+mode, func(t *testing.T) {
				parse := func() *hir.Module {
					h, e := c89.Compile(prefix+fmt.Sprintf("void execute(void){result=%s;}", c.expr), "aggregate.c")
					if e != nil {
						t.Fatal(e)
					}
					for name, want := range map[string]int{"packet": 5, "records": 10, "matrix": 12, "buffer": 5, "backing": 8} {
						found := false
						for _, global := range h.Globals {
							if global.Name == name {
								found = true
								if got := mir2.ByteWidth(global.Ty); got != want {
									t.Fatalf("%s storage: got %d bytes want %d", name, got, want)
								}
							}
						}
						if !found {
							t.Fatalf("missing storage %s", name)
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
				if root := os.Getenv("SYSTEM_C_AGGREGATE_EXPORT"); root != "" {
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

func TestCRejectsVariableLengthArrayStorage(t *testing.T) {
	for _, source := range []string{
		"int f(int n){int values[n];values[0]=7;return values[0];}",
		"int f(int n){int values[2][n];values[1][0]=7;return values[1][0];}",
	} {
		if _, err := c89.Compile(source, "vla.c"); err == nil {
			t.Fatal("expected unsupported VLA diagnostic")
		}
	}
}
