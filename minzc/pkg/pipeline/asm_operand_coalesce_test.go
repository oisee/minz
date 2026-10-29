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

// asmOperandCase is one Nanz program and the globals it reads. Every asm block
// names its inputs with (in …), and every input reaches the block through a
// block parameter — the shape whose block-arg/param pairs PreallocCoalesce
// merges.
//
// value selects the cases the MIR2 VM can judge. The VM treats an asm block as
// a no-op, so it is an oracle exactly when the program's answer must not depend
// on what the template does: the block declares (ret A) and the result is
// discarded. A block with no result register is different — its inputs are
// in-out, and a template that writes the input's own register is how the value
// escapes, which is what TestOriginalRotateSledNative pins — so those cases are
// only checked for well-formedness.
type asmOperandCase struct {
	src     string
	globals map[string][]byte
	value   bool
}

var asmOperandCases = map[string]asmOperandCase{
	// A while loop carrying one and then two values across the back edge.
	"loop_carried": {
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
		value:   false,
	},
	// The asm is the last read of the carried value, inside an if.
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
	// An if/else diamond. Its join parameter interferes with both arms, so
	// nothing merges and the operand stays well formed.
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
    asm z80 (in v) (ret A) { ADD A, 1 }
    result = u16(v)
}
`,
		globals: map[string][]byte{"c": {10}, "result": {0xef, 0xbe}},
		value:   true,
	},
	// A value selected by an inlined compare-and-select inside a loop.
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
		value:   false,
	},
	// The compare-and-select shape with no loop: the one form where the
	// dangling operand used to make origin/main lucky, because the allocator
	// homed the selected value in B and the template's write to A missed it.
	"pick_then_asm": {
		src: `global ga: u8
global gb: u8
global result: u16
fun pick(a: u8, x: u8) -> u8 { if x > a { return x } return a }
fun execute() {
    let t: u8 = pick(ga, gb)
    asm z80 (in t) (ret A) { ADD A, 7 }
    result = u16(t)
}
`,
		globals: map[string][]byte{"ga": {199}, "gb": {100}, "result": {0xef, 0xbe}},
		value:   true,
	},
	// Two asm inputs and a template that writes the second one's register.
	// Plain while, no inlining and no compare-and-select: the shortest
	// demonstration that the clobber hole is not specific to either.
	"loop_two_inputs_ld_b_a": {
		src: `global n: u16
global result: u16
fun execute() {
    let v0: u8 = 5
    let v1: u8 = 9
    let i: u16 = 0
    while i < n {
        asm z80 (in v0, v1) (ret A) { LD B, A }
        v0 = v0 + 2
        v1 = v1 + 2
        i = i + 1
    }
    result = u16(v0) + u16(v1)
}
`,
		globals: map[string][]byte{"n": {3, 0}, "result": {0xef, 0xbe}},
		value:   true,
	},
}

// asmOperandBuild compiles one case once and returns everything the three
// assertions need: the unoptimised module for the VM oracle, the final module,
// and the assembled program for the Z80.
type asmOperandBuild struct {
	raw   *mir2.Module
	final *mir2.Module
	asm   *z80asm.Result
}

func asmOperandBuildCase(t *testing.T, c asmOperandCase) asmOperandBuild {
	t.Helper()
	hm, err := nanz.Parse(c.src, "asm_operand.nanz")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	raw := hir.LowerModule(hm)
	steps, err := CompileHIRSteps(hm, DefaultOptions())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	asm, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil || len(asm.Errors) != 0 {
		t.Fatalf("assemble: %v %v", err, asm.Errors)
	}
	return asmOperandBuild{raw: raw, final: steps.MIR2Module, asm: asm}
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
//	loop_carried:           execute: asm Ins=%r3 undefined (in loop_body2)
//	loop_carried_two:       execute: asm Ins=%r4 and %r6 undefined (in loop_body2)
//	block_param_after_call: execute: asm Ins=%r18 undefined (in cmpsel4_c)
func TestAsmOperandsDefinedAfterCoalesce(t *testing.T) {
	var bad []string
	for name, c := range asmOperandCases {
		if d := danglingAsmOperands(asmOperandBuildCase(t, c).final); len(d) > 0 {
			bad = append(bad, name+": "+strings.Join(d, "; "))
		}
	}
	if len(bad) > 0 {
		t.Errorf("dangling asm operands after PreallocCoalesce:\n\t%s", strings.Join(bad, "\n\t"))
	}
}

// TestTailRecursionAsmOperandFollowsLoopParam: elimTailRecursion moves the
// function body into a new loop-head block and rewrites its uses to the loop
// parameters. An inline-asm input is a use like any other; leaving it on the
// contract parameter makes the template read the first invocation's argument
// on every iteration.
func TestTailRecursionAsmOperandFollowsLoopParam(t *testing.T) {
	const src = `global result: u16
fun tail(n: u8, acc: u8) -> u8 {
    asm z80 (in n) (ret A) { ADD A, 1 }
    if n == 0 { return acc }
    return tail(n - 1, acc + n + 1)
}
fun execute() {
    result = u16(tail(3, 0))
}
`
	hm, err := nanz.Parse(src, "asm_operand.nanz")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	steps, err := CompileHIRSteps(hm, DefaultOptions())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	tail := steps.MIR2Module.FuncByName("tail")
	if tail == nil {
		t.Fatal("tail not found")
	}
	head := tail.BlockByLabel("tail_tail_loop")
	if head == nil {
		t.Skip("elimTailRecursion did not fire for this shape")
	}
	for _, b := range tail.Blocks {
		for _, in := range b.Insts {
			if in.Asm == nil {
				continue
			}
			for _, r := range in.Asm.Ins {
				for _, cp := range tail.Contract.Params {
					if r == cp.Reg {
						t.Fatalf("asm input names the contract param %%r%d; the loop param is %%r%d", r, head.Params[0].Dst)
					}
				}
			}
		}
	}
}

// asmOperandVMResult runs fn on the MIR2 VM and returns the u16 global.
func asmOperandVMResult(t *testing.T, m *mir2.Module, globals map[string][]byte) uint16 {
	t.Helper()
	vm := mir2.NewVM(m)
	for k, v := range globals {
		vm.WriteHeapSlice(vm.GlobalAddr(k), v)
	}
	if _, err := vm.Call("execute", nil); err != nil {
		t.Fatalf("vm call execute: %v", err)
	}
	return binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("result"), 2))
}

// asmOperandZ80Result runs the assembled program and returns the u16 global.
func asmOperandZ80Result(t *testing.T, b asmOperandBuild, globals map[string][]byte) uint16 {
	t.Helper()
	sym := func(k string) int {
		if a, ok := b.asm.Symbols[k]; ok {
			return a
		}
		return b.asm.Symbols["v_"+k]
	}
	z := emulator.NewRemogattoZ80()
	mem := z.Memory()
	copy(mem[0x8000:], b.asm.Binary)
	for k, v := range globals {
		copy(mem[sym(k):], v)
	}
	mem[0xeffe], mem[0xefff] = 0, 0xf1
	z.SetRegisters(emulator.Registers{PC: uint16(sym("execute")), SP: 0xeffe})
	for step := 0; step < 1000000 && z.GetPC() != 0xf100; step++ {
		z.Step()
	}
	r := z.GetRegisters()
	if r.PC != 0xf100 || r.SP != 0xf000 {
		t.Fatalf("execute did not return cleanly: PC/SP %04x/%04x", r.PC, r.SP)
	}
	return binary.LittleEndian.Uint16(mem[sym("result") : sym("result")+2])
}

// TestAsmLoopCarriedOperandValue asserts the Z80 agrees with the MIR2 VM for
// every case in the table. Two of them were wrong on origin/main for the
// renaming bug (the loop-carried input was homed in A, where the template's own
// ADD landed on it: 11 and 7 where the source says 8 and 6); two more were
// wrong for the clobber bug (pick_then_asm 206 vs 199, loop_two_inputs_ld_b_a
// 17 vs 26 — the second input was homed in B, which `LD B, A` overwrites).
func TestAsmLoopCarriedOperandValue(t *testing.T) {
	for name, c := range asmOperandCases {
		if !c.value {
			continue
		}
		t.Run(name, func(t *testing.T) {
			b := asmOperandBuildCase(t, c)
			want := asmOperandVMResult(t, b.raw, c.globals)
			if got := asmOperandVMResult(t, b.final, c.globals); got != want {
				t.Fatalf("MIR2 VM disagrees with itself: raw=%d final=%d", want, got)
			}
			if got := asmOperandZ80Result(t, b, c.globals); got != want {
				t.Fatalf("Z80=%d, MIR2 VM=%d — the asm template read the wrong register", got, want)
			}
		})
	}
}
