package c89_test

import (
	"encoding/binary"
	"fmt"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
)

type volatileEvent struct {
	Write  bool
	Value  byte
	Offset int
}
type volatileMemory struct {
	*emulator.Memory
	start  uint16
	width  int
	events []volatileEvent
}

func (m *volatileMemory) read(a uint16) byte {
	v := m.Memory.Data()[a]
	if int(a) >= int(m.start) && int(a) < int(m.start)+m.width {
		m.events = append(m.events, volatileEvent{Value: v, Offset: int(a - m.start)})
	}
	return v
}
func (m *volatileMemory) write(a uint16, v byte) {
	m.Memory.Data()[a] = v
	if int(a) >= int(m.start) && int(a) < int(m.start)+m.width {
		m.events = append(m.events, volatileEvent{Write: true, Value: v, Offset: int(a - m.start)})
	}
}
func (m *volatileMemory) ReadByte(a uint16) byte             { return m.read(a) }
func (m *volatileMemory) ReadByteInternal(a uint16) byte     { return m.read(a) }
func (m *volatileMemory) WriteByte(a uint16, v byte)         { m.write(a, v) }
func (m *volatileMemory) WriteByteInternal(a uint16, v byte) { m.write(a, v) }

// C99 5.1.2.3 and 6.7.3: each volatile access is observable. Compare exact
// byte events, result and native return PC/SP, including discarded reads.
func TestCVolatileAccessNativeVM(t *testing.T) {
	cases := []struct {
		name, source string
		want         uint16
		events       []volatileEvent
	}{
		{"pointer_branch", `unsigned char peek(volatile unsigned char *p){if(*p)return *p;return 0;} void execute(void){result=peek(&device);}`, 1, []volatileEvent{{Value: 1}, {Value: 1}}},
		{"writes", `void execute(void){device=3;result=device;device=5;result=result*256+device;}`, 773, []volatileEvent{{Write: true, Value: 3}, {Value: 3}, {Write: true, Value: 5}, {Value: 5}}},
		{"discarded", `void execute(void){device;result=7;}`, 7, []volatileEvent{{Value: 1}}},
		{"void_discarded", `void execute(void){(void)device;result=7;}`, 7, []volatileEvent{{Value: 1}}},
		{"field", `struct Reg {volatile unsigned char value;}; void execute(void){struct Reg *p=(struct Reg *)&device;p->value=3;result=p->value;}`, 3, []volatileEvent{{Write: true, Value: 3}, {Value: 3}}},
		{"array", `void execute(void){volatile unsigned char *p=&device;p[0]=3;result=p[0];}`, 3, []volatileEvent{{Write: true, Value: 3}, {Value: 3}}},
		{"bit_test", `void execute(void){result=(device&1)!=0;}`, 1, []volatileEvent{{Value: 1}}},
		{"bit_update", `void execute(void){device|=2;result=device;}`, 3, []volatileEvent{{Value: 1}, {Write: true, Value: 3}, {Value: 3}}},

		{"assignment_value", `void execute(void){result=(device=3);}`, 3, []volatileEvent{{Write: true, Value: 3}}},
		{"compound_value", `void execute(void){result=(device+=2);}`, 3, []volatileEvent{{Value: 1}, {Write: true, Value: 3}}},
		{"preincrement_value", `void execute(void){result=++device;}`, 2, []volatileEvent{{Value: 1}, {Write: true, Value: 2}}},
		{"postincrement_value", `void execute(void){result=device++;}`, 1, []volatileEvent{{Value: 1}, {Write: true, Value: 2}}},
		{"predecrement_value", `void execute(void){result=--device;}`, 0, []volatileEvent{{Value: 1}, {Write: true, Value: 0}}},
		{"postdecrement_value", `void execute(void){result=device--;}`, 1, []volatileEvent{{Value: 1}, {Write: true, Value: 0}}},

		{"aggregate_read", `void execute(void){struct Reg a=device;struct Reg b=device;result=a.a+b.a;}`, 2, []volatileEvent{{Value: 1}, {Value: 3, Offset: 1}, {Value: 2, Offset: 2}, {Value: 4, Offset: 3}, {Value: 1}, {Value: 3, Offset: 1}, {Value: 2, Offset: 2}, {Value: 4, Offset: 3}}},
		{"aggregate_write", `void execute(void){struct Reg a={5,0x0607,8};device=a;device=a;result=7;}`, 7, []volatileEvent{{Write: true, Value: 5}, {Write: true, Value: 7, Offset: 1}, {Write: true, Value: 6, Offset: 2}, {Write: true, Value: 8, Offset: 3}, {Write: true, Value: 5}, {Write: true, Value: 7, Offset: 1}, {Write: true, Value: 6, Offset: 2}, {Write: true, Value: 8, Offset: 3}}},

		{"aggregate_argument", `unsigned char consume(struct Reg v){return v.a;}void execute(void){result=consume(device)+consume(device);}`, 2, []volatileEvent{{Value: 1}, {Value: 3, Offset: 1}, {Value: 2, Offset: 2}, {Value: 4, Offset: 3}, {Value: 1}, {Value: 3, Offset: 1}, {Value: 2, Offset: 2}, {Value: 4, Offset: 3}}},
		{"aggregate_return", `struct Reg get(void){return device;}void execute(void){struct Reg a=get();struct Reg b=get();result=a.a+b.a;}`, 2, []volatileEvent{{Value: 1}, {Value: 3, Offset: 1}, {Value: 2, Offset: 2}, {Value: 4, Offset: 3}, {Value: 1}, {Value: 3, Offset: 1}, {Value: 2, Offset: 2}, {Value: 4, Offset: 3}}},

		{"member_read", `void execute(void){struct Reg a=device;struct Reg b=device;result=7;}`, 7, []volatileEvent{{Value: 1}, {Value: 1}}},
		{"member_write", `void execute(void){struct Reg a={5,0x0607,8};device=a;device=a;result=7;}`, 7, []volatileEvent{{Write: true, Value: 5}, {Write: true, Value: 5}}},
		{"member_array_read", `void execute(void){struct Reg a=device;struct Reg b=device;result=7;}`, 7, []volatileEvent{{Value: 1}, {Value: 2, Offset: 1}, {Value: 1}, {Value: 2, Offset: 1}}},
		{"member_nested_read", `void execute(void){struct Reg a=device;struct Reg b=device;result=7;}`, 7, []volatileEvent{{Value: 1}, {Value: 1}}},
		{"member_union_read", `void execute(void){union Reg a=device;union Reg b=device;result=7;}`, 7, []volatileEvent{{Value: 1}, {Value: 2, Offset: 1}, {Value: 1}, {Value: 2, Offset: 1}}},
		{"decl_assignment", `void execute(void){unsigned char x=(device=3);result=x;}`, 3, []volatileEvent{{Write: true, Value: 3}}},
		{"decl_compound", `void execute(void){unsigned char x=(device+=2);result=x;}`, 3, []volatileEvent{{Value: 1}, {Write: true, Value: 3}}},
		{"decl_preinc", `void execute(void){unsigned char x=++device;result=x;}`, 2, []volatileEvent{{Value: 1}, {Write: true, Value: 2}}},
		{"decl_postinc", `void execute(void){unsigned char x=device++;result=x;}`, 1, []volatileEvent{{Value: 1}, {Write: true, Value: 2}}},
		{"decl_predec", `void execute(void){unsigned char x=--device;result=x;}`, 0, []volatileEvent{{Value: 1}, {Write: true, Value: 0}}},
		{"decl_postdec", `void execute(void){unsigned char x=device--;result=x;}`, 1, []volatileEvent{{Value: 1}, {Write: true, Value: 0}}},

		{"volatile_pointer", `void execute(void){volatile unsigned char *volatile p=&device;result=*p;}`, 1, []volatileEvent{{Value: 1}}},
		{"local_address", `void execute(void){volatile unsigned char x=3;volatile unsigned char *p=&x;*p=5;result=x;}`, 5, nil},

		{"long_read", `void execute(void){result=(unsigned int)device;}`, 0x5678, []volatileEvent{{Value: 0x78}, {Value: 0x56, Offset: 1}, {Value: 0x34, Offset: 2}, {Value: 0x12, Offset: 3}}},
		{"long_write", `void execute(void){device=0xabcdef01UL;result=7;}`, 7, []volatileEvent{{Write: true, Value: 1}, {Write: true, Value: 0xef, Offset: 1}, {Write: true, Value: 0xcd, Offset: 2}, {Write: true, Value: 0xab, Offset: 3}}},

		{"ordinary_bit_test", `void execute(void){result=(device&1)!=0;}`, 1, []volatileEvent{{Value: 0x35}}},

		{"word_read", `void execute(void){result=device;}`, 4660, []volatileEvent{{Value: 0x34}, {Value: 0x12, Offset: 1}}},
		{"word_bit_test", `void execute(void){result=(device&1)!=0;}`, 0, []volatileEvent{{Value: 0x34}, {Value: 0x12, Offset: 1}}},
		{"word_bit_update", `void execute(void){device|=1;result=device;}`, 4661, []volatileEvent{{Value: 0x34}, {Value: 0x12, Offset: 1}, {Write: true, Value: 0x35}, {Write: true, Value: 0x12, Offset: 1}, {Value: 0x35}, {Value: 0x12, Offset: 1}}},

		{"local", `void execute(void){volatile unsigned char x=3;result=x;x=5;result=result*256+x;}`, 773, nil},
		{"parameter", `unsigned int twice(volatile unsigned char x){return (unsigned int)x*256+x;} void execute(void){result=twice(3);}`, 771, nil},
	}
	for _, c := range cases {
		for _, mode := range []string{"default", "grace", "size"} {
			t.Run(c.name+"/"+mode, func(t *testing.T) {
				prefix, width := "volatile unsigned char device=1; unsigned int result;", 1
				if strings.HasPrefix(c.name, "word_") {
					prefix, width = "volatile unsigned int device=0x1234; unsigned int result;", 2
				}
				if strings.HasPrefix(c.name, "long_") {
					prefix, width = "volatile unsigned long device=0x12345678UL; unsigned int result;", 4
				}
				if strings.HasPrefix(c.name, "aggregate_") {
					prefix, width = "struct Reg{unsigned char a;unsigned int b;unsigned char c;};volatile struct Reg device={1,0x0203,4};unsigned int result;", 4
				}

				if strings.HasPrefix(c.name, "member_") {
					prefix, width = "struct Reg{volatile unsigned char a;unsigned int b;unsigned char c;};struct Reg device={1,0x0203,4};unsigned int result;", 1
				}
				if c.name == "member_array_read" {
					prefix, width = "struct Reg{volatile unsigned char a[2];unsigned int b;};struct Reg device={{1,2},0x0304};unsigned int result;", 2
				}
				if c.name == "member_nested_read" {
					prefix, width = "struct Inner{volatile unsigned char a;unsigned int b;};struct Reg{struct Inner inner;unsigned char tail;};struct Reg device={{1,0x0203},4};unsigned int result;", 1
				}
				if c.name == "member_union_read" {
					prefix, width = "union Reg{volatile unsigned int a;unsigned char b[4];};union Reg device={.b={1,2,3,4}};unsigned int result;", 2
				}

				if c.name == "ordinary_bit_test" {
					prefix, width = "unsigned int device=0x1235;unsigned int result;", 2
				}
				source := prefix + c.source
				parse := func() *hir.Module {
					h, e := c89.Compile(source, "volatile.c")
					if e != nil {
						t.Fatal(e)
					}
					return h
				}
				rawmod := hir.LowerModule(parse())
				options := pipeline.Options{ContractOpt: true, AssertMode: "none", UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size", FuseByteWords: true}
				steps, e := pipeline.CompileHIRSteps(parse(), options)
				if e != nil {
					t.Fatal(e)
				}
				for _, m := range []*mir2.Module{rawmod, steps.MIR2Module} {
					vm := mir2.NewVM(m)
					var events []volatileEvent
					addr := vm.GlobalAddr("device")
					vm.ObserveDataAccess = func(a mir2.VMDataAccess) {
						if a.Addr >= addr && a.Addr < addr+int64(width) {
							events = append(events, volatileEvent{Write: a.Op == mir2.OpStore, Value: a.Value, Offset: int(a.Addr - addr)})
						}
					}
					if _, e := vm.Call("execute", nil); e != nil {
						t.Fatal(e)
					}
					got := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("result"), 2))
					vmEvents := c.events
					if c.name == "ordinary_bit_test" {
						vmEvents = []volatileEvent{{Value: 0x35}, {Value: 0x12, Offset: 1}}
					}
					if got != c.want || !reflect.DeepEqual(events, vmEvents) {
						t.Fatalf("VM got %d events%v want%d %v", got, events, c.want, c.events)
					}
					accesses := 0
					for _, f := range m.Funcs {
						for _, b := range f.Blocks {
							for _, i := range b.Insts {
								if i.Volatile {
									accesses++
								}
							}
						}
					}
					if (c.name == "local" && accesses != 4) || (c.name == "parameter" && accesses != 3) {
						t.Fatalf("local volatile access count %d", accesses)
					}
					if accesses == 0 && c.name != "ordinary_bit_test" {
						t.Fatal("volatile qualifier was lost")
					}
				}
				bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
				if e != nil || len(bin.Errors) > 0 {
					t.Fatal(e, bin.Errors, steps.Assembly)
				}
				symbol := func(n string) uint16 {
					v := bin.Symbols[n]
					if v == 0 {
						v = bin.Symbols["v_"+n]
					}
					if v == 0 {
						t.Fatal("missing symbol", n)
					}
					return uint16(v)
				}
				mem := &volatileMemory{Memory: emulator.NewMemory(), start: symbol("device"), width: width}
				copy(mem.Data()[0x8000:], bin.Binary)
				mem.Data()[0xeffe] = 0
				mem.Data()[0xefff] = 0xf1
				cpu := z80.NewZ80(mem, emulator.NewPorts(nil))
				cpu.SetPC(symbol("execute"))
				cpu.SetSP(0xeffe)
				for n := 0; n < 10000 && cpu.PC() != 0xf100; n++ {
					cpu.DoOpcode()
				}
				a := symbol("result")
				got := binary.LittleEndian.Uint16(mem.Data()[a : a+2])
				if got != c.want || cpu.PC() != 0xf100 || cpu.SP() != 0xf000 || !reflect.DeepEqual(mem.events, c.events) {
					t.Fatalf("Z80 got%d PC%04x SP%04x events%v want%d %v\n%s", got, cpu.PC(), cpu.SP(), mem.events, c.want, c.events, steps.Assembly)
				}
				exportVolatile(t, c.name+"-"+mode, "c", source, steps.Assembly, bin.Binary)
				t.Log(fmt.Sprintf("native/raw/final ordered events %v", c.events))
			})
		}
	}
}

func TestNanzImportedVolatileStorage(t *testing.T) {
	dir := t.TempDir()
	if e := os.WriteFile(filepath.Join(dir, "driver.c"), []byte("volatile unsigned char device=1; volatile unsigned char bank[2]={1,2};"), 0644); e != nil {
		t.Fatal(e)
	}
	source := `import driver {device,bank}
 global result:u16
 fun execute(){device=3
 result=device
 bank[1]=5
 asm { NOP }
 result=result*256+bank[1]
 }`
	parse := func() *hir.Module {
		h, e := nanz.ParseWithOpts(source, filepath.Join(dir, "app.nanz"), nanz.ParseOpts{BaseDir: dir})
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	steps, e := pipeline.CompileHIRSteps(parse(), pipeline.Options{ContractOpt: true, UseHIRGrace: true, UseGrace: true, OptSize: true, AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	for _, m := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
		vm := mir2.NewVM(m)
		var events []volatileEvent
		addr := vm.GlobalAddr("driver__device")
		vm.ObserveDataAccess = func(a mir2.VMDataAccess) {
			if a.Addr == addr {
				events = append(events, volatileEvent{Write: a.Op == mir2.OpStore, Value: a.Value})
			}
		}
		if _, e := vm.Call("execute", nil); e != nil {
			t.Fatal(e)
		}
		got := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("result"), 2))
		if got != 773 || !reflect.DeepEqual(events, []volatileEvent{{Write: true, Value: 3}, {Value: 3}}) {
			t.Fatal(got, events)
		}
		count := 0
		for _, f := range m.Funcs {
			for _, b := range f.Blocks {
				for _, i := range b.Insts {
					if i.Volatile {
						count++
					}
				}
			}
		}
		if count != 4 {
			t.Fatalf("import lost volatile scalar/array contract: %d", count)
		}
	}
	bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if e != nil || len(bin.Errors) > 0 {
		t.Fatal(e, bin.Errors)
	}
	symbol := func(n string) uint16 {
		v := bin.Symbols[n]
		if v == 0 {
			v = bin.Symbols["v_"+n]
		}
		if v == 0 {
			t.Fatal("missing symbol", n)
		}
		return uint16(v)
	}
	mem := &volatileMemory{Memory: emulator.NewMemory(), start: symbol("driver__device"), width: 1}
	copy(mem.Data()[0x8000:], bin.Binary)
	mem.Data()[0xeffe] = 0
	mem.Data()[0xefff] = 0xf1
	cpu := z80.NewZ80(mem, emulator.NewPorts(nil))
	cpu.SetPC(symbol("execute"))
	cpu.SetSP(0xeffe)
	for n := 0; n < 10000 && cpu.PC() != 0xf100; n++ {
		cpu.DoOpcode()
	}
	exportVolatile(t, "nanz-import", "nanz", source, steps.Assembly, bin.Binary)
	a := symbol("result")
	if got := binary.LittleEndian.Uint16(mem.Data()[a : a+2]); got != 773 || cpu.PC() != 0xf100 || cpu.SP() != 0xf000 || !reflect.DeepEqual(mem.events, []volatileEvent{{Write: true, Value: 3}, {Value: 3}}) {
		t.Fatal(got, cpu.PC(), cpu.SP(), mem.events)
	}
}

func exportVolatile(t *testing.T, name, ext, source, asm string, binary []byte) {
	t.Helper()
	dir := os.Getenv("SYSTEM_C_VOLATILE_EXPORT")
	if dir == "" {
		return
	}
	if e := os.MkdirAll(dir, 0755); e != nil {
		t.Fatal(e)
	}
	for file, data := range map[string][]byte{name + "." + ext: []byte(source), name + ".a80": []byte(asm), name + ".bin": binary} {
		if e := os.WriteFile(filepath.Join(dir, file), data, 0644); e != nil {
			t.Fatal(e)
		}
	}
}
