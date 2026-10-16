package c89_test

import (
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
)

// The source publishes its local address, then sets ready before the measured
// accesses. Frame setup and parameter initialization precede this boundary:
// they are not confused with source-level volatile traffic or return POPs.
// C90 5.1.2.3/6.5.3 (C99 5.1.2.3/6.7.3) determine the observable
// accesses, not generated MIR counts. Copying a volatile aggregate to an
// ordinary result may use scalar result slots after the source accesses occur.
func TestCLocalVolatileTrafficNativeVM(t *testing.T) {
	read := func(v uint32, width int) []volatileEvent {
		var out []volatileEvent
		for i := 0; i < width; i++ {
			out = append(out, volatileEvent{Value: byte(v >> (8 * i)), Offset: i})
		}
		return out
	}
	write := func(v uint32, width int) []volatileEvent {
		out := read(v, width)
		for i := range out {
			out[i].Write = true
		}
		return out
	}
	join := func(parts ...[]volatileEvent) []volatileEvent {
		var out []volatileEvent
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	cases := []struct {
		name, code string
		width      int
		want       uint32
		events     []volatileEvent
	}{
		{"byte", `void execute(void){volatile unsigned char x;published=(unsigned int)&x;ready=1;x=3;result=x;x=5;result=result*256+x;}`, 1, 773, join(write(3, 1), read(3, 1), write(5, 1), read(5, 1))},
		{"initialized_byte", `void execute(void){volatile unsigned char x=3;published=(unsigned int)&x;ready=1;result=x;x=5;result=result*256+x;}`, 1, 773, join(read(3, 1), write(5, 1), read(5, 1))},
		{"parameter_byte", `unsigned int twice(volatile unsigned char x){published=(unsigned int)&x;ready=1;return (unsigned int)x*256+x;}void execute(void){result=twice(3);}`, 1, 771, join(read(3, 1), read(3, 1))},
		{"word_update", `void execute(void){volatile unsigned int x;published=(unsigned int)&x;ready=1;x=0x1234;result=x;x+=1;result=result*65536UL+x;}`, 2, 0x12341235, join(write(0x1234, 2), read(0x1234, 2), read(0x1234, 2), write(0x1235, 2), read(0x1235, 2))},
		{"parameter_word", `unsigned long twice(volatile unsigned int x){published=(unsigned int)&x;ready=1;return (unsigned long)x*65536UL+x;}void execute(void){result=twice(0x1234);}`, 2, 0x12341234, join(read(0x1234, 2), read(0x1234, 2))},
		{"long_update", `void execute(void){volatile unsigned long x;published=(unsigned int)&x;ready=1;x=0x89abcdefUL;result=x;x++;result^=x;}`, 4, 0x1f, join(write(0x89abcdef, 4), read(0x89abcdef, 4), read(0x89abcdef, 4), write(0x89abcdf0, 4), read(0x89abcdf0, 4))},
		{"aggregate_fields", `struct Reg{unsigned char a;unsigned int b;};void execute(void){volatile struct Reg x;published=(unsigned int)&x;ready=1;x.a=3;x.b=0x1234;result=(unsigned long)x.a*65536UL+x.b;}`, 3, 0x31234, []volatileEvent{{Write: true, Value: 3}, {Write: true, Value: 0x34, Offset: 1}, {Write: true, Value: 0x12, Offset: 2}, {Value: 3}, {Value: 0x34, Offset: 1}, {Value: 0x12, Offset: 2}}},
		// The ordinary return value may be scalar; the published volatile source
		// must retain its storage and exactly one read of each initialized byte.
		{"aggregate_return", `struct Pair{unsigned char a,b;};struct Pair make(void){volatile struct Pair p;published=(unsigned int)&p;ready=1;p.a=3;p.b=5;return p;}void execute(void){struct Pair p=make();result=(unsigned long)p.a*256+p.b;}`, 2, 773, []volatileEvent{{Write: true, Value: 3}, {Write: true, Value: 5, Offset: 1}, {Value: 3}, {Value: 5, Offset: 1}}},
		{"pointer_alias", `void execute(void){volatile unsigned char x=3;volatile unsigned char *p=&x;published=(unsigned int)&x;ready=1;*p=5;result=x;}`, 1, 5, join(write(5, 1), read(5, 1))},
	}
	for _, c := range cases {
		for _, mode := range []string{"default", "grace", "size"} {
			t.Run(c.name+"/"+mode, func(t *testing.T) {
				source := "unsigned int published;unsigned char ready;unsigned long result;" + c.code
				parse := func() *hir.Module {
					h, err := c89.Compile(source, "local-volatile.c")
					if err != nil {
						t.Fatal(err)
					}
					return h
				}
				steps, err := pipeline.CompileHIRSteps(parse(), pipeline.Options{ContractOpt: true, AssertMode: "none", UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
				if err != nil {
					t.Fatal(err)
				}
				for _, m := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
					if c.name == "aggregate_return" {
						f := m.FuncByName("make")
						materialized := false
						if f != nil {
							for _, b := range f.Blocks {
								for _, i := range b.Insts {
									materialized = materialized || i.Op == mir2.OpAlloca
								}
							}
						}
						if !materialized {
							t.Fatal("volatile source object lost its addressable storage")
						}
					}
					vm := mir2.NewVM(m)
					ready, pub := vm.GlobalAddr("ready"), vm.GlobalAddr("published")
					var start int64
					active := false
					var events []volatileEvent
					vm.ObserveDataAccess = func(a mir2.VMDataAccess) {
						if active && a.Addr >= start && a.Addr < start+int64(c.width) {
							events = append(events, volatileEvent{Write: a.Op == mir2.OpStore, Value: a.Value, Offset: int(a.Addr - start)})
						}
						if a.Addr == ready && a.Op == mir2.OpStore && a.Value == 1 {
							start = int64(binary.LittleEndian.Uint16(vm.ReadHeap(pub, 2)))
							active = true
						}
					}
					if _, err := vm.Call("execute", nil); err != nil {
						t.Fatal(err)
					}
					got := binary.LittleEndian.Uint32(vm.ReadHeap(vm.GlobalAddr("result"), 4))
					if !active || got != c.want || !reflect.DeepEqual(events, c.events) {
						t.Fatalf("VM result=%08x events=%v want=%08x %v", got, events, c.want, c.events)
					}
				}
				image, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
				if err != nil {
					t.Fatal(err)
				}
				if len(image.Errors) > 0 {
					t.Fatal(image.Errors)
				}
				symbol := func(name string) uint16 {
					v := image.Symbols[name]
					if v == 0 {
						v = image.Symbols["v_"+name]
					}
					if v == 0 {
						t.Fatal("missing symbol", name)
					}
					return uint16(v)
				}
				for _, interrupts := range []byte{0, 1} {
					mem := &volatileMemory{Memory: emulator.NewMemory()}
					if 0x8000+len(image.Binary) >= 0xe000 {
						t.Fatal("code/data collides with stack reserve")
					}
					copy(mem.Data()[0x8000:], image.Binary)
					mem.Data()[0xefff] = 0xf1
					cpu := z80.NewZ80(mem, emulator.NewPorts(nil))
					cpu.SetSP(0xeffe)
					cpu.SetPC(symbol("execute"))
					cpu.IFF1 = interrupts
					cpu.IFF2 = interrupts
					active := false
					pub, ready := symbol("published"), symbol("ready")
					for n := 0; n < 10000 && cpu.PC() != 0xf100; n++ {
						cpu.DoOpcode()
						if !active && mem.Data()[ready] == 1 {
							mem.start = binary.LittleEndian.Uint16(mem.Data()[pub : pub+2])
							mem.width = c.width
							active = true
							if mem.start < 0xe000 || int(mem.start)+c.width > 0xeffe {
								t.Fatalf("local outside owned stack storage: %04x", mem.start)
							}
						}
						// Ignore later caller spills after the local frame is released.
						if active && mem.width != 0 && int(cpu.SP()) >= int(mem.start)+c.width {
							mem.width = 0
						}
					}
					addr := symbol("result")
					got := binary.LittleEndian.Uint32(mem.Data()[addr : addr+4])
					if !active || got != c.want || cpu.PC() != 0xf100 || cpu.SP() != 0xf000 || cpu.IFF1 != interrupts || cpu.IFF2 != interrupts || !reflect.DeepEqual(mem.events, c.events) {
						t.Fatalf("Z80 result=%08x PC=%04x SP=%04x events=%v want=%08x %v\n%s", got, cpu.PC(), cpu.SP(), mem.events, c.want, c.events, steps.Assembly)
					}
				}
				exportVolatile(t, "local-"+c.name+"-"+mode, "c", source, "ORG 0x8000\n"+steps.Assembly, image.Binary)
			})
		}
	}
}
