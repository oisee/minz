package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestSMCNativeSourceCalls(t *testing.T) {
	for _, tc := range []struct {
		name, source, slot string
		writes             map[int]byte
		result             byte
	}{
		{"struct", `struct Row2 {b0:u8,b1:u8}
fun draw(@smc dst:u16) {dst^=Row2{b0:195,b1:60}}
fun run()->u8 {draw(u16(&buffer[4])) draw_set_dst(u16(&buffer[12])) draw() return buffer[4]+buffer[12]}`, "draw$dst$imm", map[int]byte{4: 195, 5: 60, 12: 195, 13: 60}, 134},
		{"walk", `fun draw(@smc dst:^u8) {var p:^u8=dst p^=195 p=p+1 p^=60}
fun run()->u8 {draw(&buffer[4]) draw_set_dst(&buffer[12]) draw() return buffer[4]+buffer[12]}`, "draw$dst$imm", map[int]byte{4: 195, 5: 60, 12: 195, 13: 60}, 134},
		{"indexed", `fun draw(@smc dst:^u8) {dst[0]=195 dst[1]=60}
fun run()->u8 {draw(&buffer[4]) draw_set_dst(&buffer[12]) draw() return buffer[4]+buffer[12]}`, "draw$dst$imm", map[int]byte{4: 195, 5: 60, 12: 195, 13: 60}, 134},
		{"nested-mixed", `fun draw(@smc dst:^u8,value:u8)->u8 {dst^=value return value+1}
fun inner()->u8 {return draw(&buffer[8],7)}
fun run()->u8 {let value:u8=draw(&buffer[4],inner()) draw_set_dst(&buffer[12]) return value+draw(9)}`, "draw$dst$imm", map[int]byte{4: 8, 8: 7, 12: 9}, 19},
		{"nested-smc-last", `fun draw(value:u8,@smc dst:^u8)->u8 {dst^=value return value+1}
fun inner()->u8 {return draw(7,&buffer[8])}
fun run()->u8 {let value:u8=draw(inner(),&buffer[4]) draw_set_dst(&buffer[12]) return value+draw(9)}`, "draw$dst$imm", map[int]byte{4: 8, 8: 7, 12: 9}, 19},
	} {
		for mode, opt := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
			t.Run(fmt.Sprintf("%s/%d", tc.name, mode), func(t *testing.T) {
				source := "global buffer:[u8;64]\n" + tc.source
				parse := func() *hir.Module {
					h, e := nanz.Parse(source, "native-smc")
					if e != nil {
						t.Fatal(e)
					}
					return h
				}
				steps, e := CompileHIRSteps(parse(), opt)
				if e != nil {
					t.Fatal(e)
				}
				for _, module := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
					vm := mir2.NewVM(module)
					for repeat := 0; repeat < 3; repeat++ {
						got, e := vm.Call("run", nil)
						if e != nil || len(got) != 1 || byte(got[0].I) != tc.result {
							t.Fatalf("VM: %v %v want%d", got, e, tc.result)
						}
						want := make([]byte, 64)
						for i, v := range tc.writes {
							want[i] = v
						}
						if actual := vm.ReadHeap(vm.GlobalAddr("buffer"), 64); string(actual) != string(want) {
							t.Fatalf("VM buffer: %v want%v", actual, want)
						}
					}
				}
				a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
				if e != nil || len(a.Errors) != 0 {
					t.Fatal(e, a.Errors, steps.Assembly)
				}
				var receipts []map[string]any
				for _, seed := range []int{29, 117} {
					z := emulator.NewRemogattoZ80()
					memory := z.Memory()
					for i := range memory {
						memory[i] = byte(i*73 + seed)
					}
					z.LoadMemory(0x8000, a.Binary)
					entry, base, slot := a.Symbols["run"], a.Symbols["buffer"], a.Symbols[tc.slot]
					memory[0x7000] = 0xcd
					memory[0x7001] = byte(entry)
					memory[0x7002] = byte(entry >> 8)
					for repeat := 0; repeat < 3; repeat++ {
						want := append([]byte(nil), memory...)
						for i, v := range tc.writes {
							want[base+i] = v
						}
						want[slot] = byte(base + 12)
						want[slot+1] = byte((base + 12) >> 8)
						z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, A: 0x5a, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
						start := z.GetCycles()
						minSP := uint16(0xf000)
						done := false
						for step := 0; step < 20000; step++ {
							if z.GetPC() == 0x7003 && z.GetSP() == 0xf000 {
								done = true
								break
							}
							z.Step()
							if z.GetSP() < minSP {
								minSP = z.GetSP()
							}
						}
						if !done || minSP < 0xefe0 {
							t.Fatalf("native PC/SP/minSP: %+v / %x", z.GetRegisters(), minSP)
						}
						if loc := mir2.ReturnLocation(steps.MIR2Module.FuncByName("run").Contract.Returns[0].Class, mir2.TyU8); loc != "A" {
							t.Fatalf("unexpected run return location %s", loc)
						}
						if z.GetRegisters().A != tc.result {
							t.Fatalf("native result %d want%d\n%s", z.GetRegisters().A, tc.result, steps.Assembly)
						}
						for i, v := range want {
							if i >= 0xefe0 && i < 0xf000 {
								continue
							}
							if memory[i] != v {
								t.Fatalf("RAM %04x got%02x want%02x\n%s", i, memory[i], v, steps.Assembly)
							}
						}
						receipts = append(receipts, map[string]any{"seed": seed, "repeat": repeat, "T_states_without_outer_CALL": z.GetCycles() - start - 17, "PC": z.GetPC(), "SP": z.GetSP(), "minSP": minSP})
					}
				}
				if root := os.Getenv("MINZ_SMC_EXPORT"); root != "" {
					dir := filepath.Join(root, tc.name, fmt.Sprint(mode))
					if e := os.MkdirAll(dir, 0755); e != nil {
						t.Fatal(e)
					}
					syms, _ := json.MarshalIndent(a.Symbols, "", "  ")
					proof, _ := json.MarshalIndent(map[string]any{"native": receipts, "VM_calls": 6, "scope": "compiled source calls; full 64K RAM except 32-byte bounded call stack; owned final imm16; PC/SP; repeated image; no ULA/INT/NMI"}, "", "  ")
					for name, data := range map[string][]byte{"source.nanz": []byte(source), "program.a80": []byte(steps.Assembly), "program.bin": a.Binary, "symbols.json": syms, "receipts.json": proof} {
						if e := os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
							t.Fatal(e)
						}
					}
				}
			})
		}
	}
}

func TestSMCSourceAdmission(t *testing.T) {
	// Name resolution must return the lexical value, not the function address.
	for _, tc := range []struct {
		source string
		args   []int64
		want   int64
	}{
		{`fun draw(@smc dst:u16){} fun use(draw:u8)->u8{return draw}`, []int64{9}, 9},
		{`fun draw(@smc dst:u16){} fun use()->u8{let draw:u8=7 return draw}`, nil, 7},
	} {
		parse := func() *hir.Module {
			h, err := nanz.Parse(tc.source, "smc-shadowing")
			if err != nil {
				t.Fatal(err)
			}
			return h
		}
		steps, err := CompileHIRSteps(parse())
		if err != nil {
			t.Fatal(err)
		}
		args := make([]mir2.Value, len(tc.args))
		for i, v := range tc.args {
			args[i] = mir2.Value{I: v}
		}
		for _, module := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
			got, err := mir2.NewVM(module).Call("use", args)
			if err != nil || len(got) != 1 || got[0].I != tc.want {
				t.Fatalf("lexical value: %v %v want %d", got, err, tc.want)
			}
		}
		measureGraceZ80HIR(t, parse(), "use", Options{AssertMode: "none"}, tc.args, nil, tc.want)
	}
	for _, tc := range []struct{ source, diagnostic string }{
		{`fun draw(@smc dst:u8){}`, "pointer or u16"},
		{`fun draw(@smc x:u16,@smc y:u16){}`, "one address slot"},
		{`fun draw(@smc dst:u16){draw(dst)}`, "recursive SMC"},
		{`fun draw(@smc dst:u16){other()} fun other(){draw(0)}`, "recursive SMC"},
		{`fun draw(@smc dst:u16){} fun caller()->u16{return draw}`, "address escape"},
		{`fun draw(@smc dst:u16){} fun draw_set_dst(value:u16){}`, "reserved SMC setter"},
		{`fun draw(@smc dst:^u8){} fun caller(){draw_set_dst(true)}`, "argument"},
	} {
		h, e := nanz.Parse(tc.source, "smc-rejection")
		if e == nil {
			_, e = CompileHIRSteps(h)
		}
		if e == nil || !strings.Contains(e.Error(), tc.diagnostic) {
			t.Fatalf("%s: %v want %q", tc.source, e, tc.diagnostic)
		}
	}
}
