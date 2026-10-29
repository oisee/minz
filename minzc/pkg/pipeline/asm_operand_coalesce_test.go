package pipeline

import (
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// asmOperandFixture is one Nanz source compiled with the production options,
// together with the globals it reads.
type asmOperandFixture struct {
	src     string
	globals map[string][]byte
}

// asmOperandSources covers the shapes whose block-arg/param pairs
// PreallocCoalesce merges: a while loop carrying a value across the back edge,
// an if/else diamond, and a value selected by an inlined compare-and-select
// inside a loop. Every asm block names its input explicitly with (in …).
var asmOperandSources = map[string]asmOperandFixture{
	"loop_carried": {
		src: `global n: u16
global result: u16
fun execute() {
    let acc: u8 = 5
    let i: u16 = 0
    while i < n {
        asm z80 (in acc) { ADD A, 1 }
        acc = acc + 1
        i = i + 1
    }
    result = u16(acc)
}
`,
		globals: map[string][]byte{"n": {3, 0}, "result": {0xef, 0xbe}},
	},
	"loop_carried_two": {
		src: `global n: u16
global result: u16
fun execute() {
    let acc: u8 = 5
    let other: u8 = 9
    let i: u16 = 0
    while i < n {
        asm z80 (in acc, other) { ADD A, 1 / ADD A, C }
        acc = acc + 1
        other = other + 2
        i = i + 1
    }
    result = u16(acc) + u16(other)
}
`,
		globals: map[string][]byte{"n": {3, 0}, "result": {0xef, 0xbe}},
	},
	"if_diamond": {
		src: `global c: u8
global result: u16
fun execute() {
    let v: u8 = c
    if c > 3 {
        v = c + 1
    } else {
        v = c + 2
    }
    asm z80 (in v) { ADD A, 1 }
    result = u16(v)
}
`,
		globals: map[string][]byte{"c": {10}, "result": {0xef, 0xbe}},
	},
	"block_param_after_call": {
		src: `global data: [u8; 8]
global n: u16
global result: u16
fun pick(a: u8, x: u8) -> u8 { if x > a { return x } return a }
fun execute() {
    let acc: u8 = 5
    let i: u16 = 0
    while i < n {
        acc = pick(acc, data[i])
        asm z80 (in acc) { ADD A, 1 }
        i = i + 1
    }
    result = u16(acc)
}
`,
		globals: map[string][]byte{"n": {4, 0}, "result": {0xef, 0xbe}, "data": {1, 200, 3, 100}},
	},
}

// asmOperandModule compiles src with the production options and returns the
// lowered-but-unoptimised module and the final module.  The final module is the
// same object the allocator and Z80 codegen see, so it already carries whatever
// PreallocCoalesce rewrote in place.
func asmOperandModule(t *testing.T, src string) (raw, final *mir2.Module) {
	t.Helper()
	hm, err := nanz.Parse(src, "asm_operand.nanz")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	raw = hir.LowerModule(hm)
	steps, err := CompileHIRSteps(hm, DefaultOptions())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return raw, steps.MIR2Module
}

// danglingAsmOperands reports every Asm.Ins / Asm.Outs register in m that has
// no definition anywhere in its function (contract param, block param or
// instruction Dst).  A dangling operand means the allocator no longer knows the
// template reads or writes that value, so it may move or spill it freely.
func danglingAsmOperands(m *mir2.Module) []string {
	var bad []string
	for _, f := range m.Funcs {
		defs := map[mir2.Reg]bool{}
		for _, cp := range f.Contract.Params {
			defs[cp.Reg] = true
		}
		for _, b := range f.Blocks {
			for _, p := range b.Params {
				defs[p.Dst] = true
			}
			for _, in := range b.Insts {
				if in.Dst != mir2.NoReg {
					defs[in.Dst] = true
				}
				if in.Asm == nil {
					continue
				}
				for _, r := range in.Asm.Ins {
					if r != mir2.NoReg && !defs[r] {
						bad = append(bad, fmt.Sprintf("%s: asm Ins=%%r%d undefined (in %s)", f.Name, r, b.Label))
					}
				}
				for _, r := range in.Asm.Outs {
					if r != mir2.NoReg && !defs[r] {
						bad = append(bad, fmt.Sprintf("%s: asm Outs=%%r%d undefined (in %s)", f.Name, r, b.Label))
					}
				}
			}
		}
	}
	return bad
}

// TestAsmOperandsDefinedAfterCoalesce is the failing-first check for the
// PreallocCoalesce asm-operand bug: the pass rewrote Params/Dst/Src/Args/
// ExtraRets and terminators to union-find roots but left Asm.Ins/Asm.Outs
// naming the pre-merge virtual registers, which no longer have a definition.
// On origin/main it reported:
//
//	loop_carried:          execute: asm Ins=%r3 undefined (in loop_body2)
//	loop_carried_two:      execute: asm Ins=%r4 and %r6 undefined (in loop_body2)
//	block_param_after_call: execute: asm Ins=%r18 undefined (in cmpsel4_c)
func TestAsmOperandsDefinedAfterCoalesce(t *testing.T) {
	var bad []string
	for name, tc := range asmOperandSources {
		_, m := asmOperandModule(t, tc.src)
		if d := danglingAsmOperands(m); len(d) > 0 {
			bad = append(bad, name+": "+strings.Join(d, "; "))
		}
	}
	if len(bad) > 0 {
		t.Errorf("dangling asm operands after PreallocCoalesce:\n\t%s", strings.Join(bad, "\n\t"))
	}
}

// asmOperandVMResult runs fn on the MIR2 VM and returns the u16 global.
func asmOperandVMResult(t *testing.T, m *mir2.Module, fn, global string, globals map[string][]byte) uint16 {
	t.Helper()
	vm := mir2.NewVM(m)
	for k, v := range globals {
		vm.WriteHeapSlice(vm.GlobalAddr(k), v)
	}
	if _, err := vm.Call(fn, nil); err != nil {
		t.Fatalf("vm call %s: %v", fn, err)
	}
	return binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr(global), 2))
}

// asmOperandZ80Result assembles the final module and runs fn on the Z80
// emulator, returning the u16 global.
func asmOperandZ80Result(t *testing.T, src, fn, global string, globals map[string][]byte) uint16 {
	t.Helper()
	hm, err := nanz.Parse(src, "asm_operand.nanz")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	steps, err := CompileHIRSteps(hm, DefaultOptions())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	asm, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil || len(asm.Errors) != 0 {
		t.Fatalf("assemble: %v %v", err, asm.Errors)
	}
	z := emulator.NewRemogattoZ80()
	mem := z.Memory()
	copy(mem[0x8000:], asm.Binary)
	for k, v := range globals {
		copy(mem[asm.Symbols[k]:], v)
	}
	mem[0xeffe], mem[0xefff] = 0, 0xf1
	entry, ok := asm.Symbols[fn]
	if !ok {
		t.Fatalf("symbol %s not found; have %v", fn, asm.Symbols)
	}
	z.SetRegisters(emulator.Registers{PC: uint16(entry), SP: 0xeffe})
	for step := 0; step < 1000000 && z.GetPC() != 0xf100; step++ {
		z.Step()
	}
	r := z.GetRegisters()
	if r.PC != 0xf100 || r.SP != 0xf000 {
		t.Fatalf("%s did not return cleanly: PC/SP %04x/%04x\n%s", fn, r.PC, r.SP, steps.Assembly)
	}
	return binary.LittleEndian.Uint16(mem[asm.Symbols[global] : asm.Symbols[global]+2])
}

// TestAsmLoopCarriedOperandValue: the two shapes where the dangling operand
// produced a wrong value on the Z80.  Both templates write A while the
// loop-carried input was allocated to A precisely because the asm's use of it
// had been dropped from the register counts — so `ADD A, 1` landed on the
// carried value itself instead of on a value nobody else observes.
//
// On origin/main the Z80 returned 11 and 7 where both VMs and the source return
// 8 and 6.
func TestAsmLoopCarriedOperandValue(t *testing.T) {
	cases := map[string]asmOperandFixture{
		// acc = 5, three iterations of `acc = acc + 1`; the asm statement's
		// (ret A) result is discarded, so acc must end at 8.
		"loop_local_mut": {
			src: `global n: u16
global result: u16
fun execute() {
    let acc: u8 = 5
    let i: u16 = 0
    while i < n {
        asm z80 (in acc) (ret A) { ADD A, 1 }
        acc = acc + 1
        i = i + 1
    }
    result = u16(acc)
}
`,
			globals: map[string][]byte{"n": {3, 0}, "result": {0xef, 0xbe}},
		},
		// The asm runs once, when i == 2, after one `acc = acc + 1`; its result
		// is discarded, so result must observe acc == 6.
		"loop_local_last": {
			src: `global n: u16
global result: u16
fun execute() {
    let acc: u8 = 5
    let i: u16 = 0
    while i < n {
        i = i + 1
        if i == 2 {
            asm z80 (in acc) (ret A) { ADD A, 1 }
            result = u16(acc)
        }
        acc = acc + 1
    }
}
`,
			globals: map[string][]byte{"n": {3, 0}, "result": {0xef, 0xbe}},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			raw, final := asmOperandModule(t, tc.src)
			want := asmOperandVMResult(t, raw, "execute", "result", tc.globals)
			if got := asmOperandVMResult(t, final, "execute", "result", tc.globals); got != want {
				t.Fatalf("MIR2 VM disagrees with itself: raw=%d final=%d", want, got)
			}
			if got := asmOperandZ80Result(t, tc.src, "execute", "result", tc.globals); got != want {
				t.Fatalf("Z80=%d, MIR2 VM=%d — the asm template read the wrong register", got, want)
			}
		})
	}
}
