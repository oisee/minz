package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// forwardGlobalValueSrc references globals that are declared after every
// function that uses them. Each assert must hold on the raw MIR2 VM, the
// optimised MIR2 VM and the assembled Z80 program.
const forwardGlobalValueSrc = `global r: u16
fun fwd_let() -> u16 { let y = g + 1
 r = y
 return r }
fun fwd_direct() -> u16 { r = g + 1
 return r }
global g: u16 = 1000
fun fwd_signed() -> i16 { let y = sg - 1
 return y }
global sg: i16 = -1000
fun fwd_array() -> u16 { table[2] = 43981
 return table[2] }
global table: [u16; 3] = [10, 4660, 30]
struct Cell { v: u16, t: u8 }
fun fwd_struct() -> u16 { return cell.v }
global cell: Cell = Cell{v: 5000, t: 7}
assert fwd_let() == 1001
assert fwd_direct() == 1001
assert fwd_signed() == -1001
assert fwd_array() == 43981
assert fwd_struct() == 5000
`

// forwardGlobalValueSrcReordered is the same program with the functions moved
// below the globals; the emitted assembly must not change.
const forwardGlobalValueSrcReordered = `global r: u16
global g: u16 = 1000
global sg: i16 = -1000
global table: [u16; 3] = [10, 4660, 30]
struct Cell { v: u16, t: u8 }
global cell: Cell = Cell{v: 5000, t: 7}
fun fwd_let() -> u16 { let y = g + 1
 r = y
 return r }
fun fwd_direct() -> u16 { r = g + 1
 return r }
fun fwd_signed() -> i16 { let y = sg - 1
 return y }
fun fwd_array() -> u16 { table[2] = 43981
 return table[2] }
fun fwd_struct() -> u16 { return cell.v }
assert fwd_let() == 1001
assert fwd_direct() == 1001
assert fwd_signed() == -1001
assert fwd_array() == 43981
assert fwd_struct() == 5000
`

// lowerForwardModule lowers hm, turning a lowerer panic (a malformed module
// that reached pkg/hir) into a test failure instead of aborting the package.
func lowerForwardModule(t *testing.T, hm *hir.Module) *mir2.Module {
	t.Helper()
	var m *mir2.Module
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("LowerModule panicked: %v", r)
			m = nil
		}
	}()
	m = hir.LowerModule(hm)
	return m
}

// TestForwardGlobalReferencesVMZ80 checks the forward-referenced-global fix on
// both execution backends: the MIR2 VM (raw and optimised module) and the Z80
// emulator running the assembled program.
func TestForwardGlobalReferencesVMZ80(t *testing.T) {
	parse := func() *hir.Module {
		hm, err := nanz.Parse(forwardGlobalValueSrc, "forward_global_value.nanz")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		return hm
	}

	if raw := lowerForwardModule(t, parse()); raw != nil {
		if err := RunAssertsMIR2(parse(), raw); err != nil {
			t.Errorf("raw MIR2 VM: %v", err)
		}
	}

	hm := parse()
	steps, err := CompileHIRSteps(hm, Options{ContractOpt: true, UseGrace: true, AssertMode: "none"})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if err := RunAssertsMIR2(hm, steps.MIR2Module); err != nil {
		t.Errorf("optimised MIR2 VM: %v", err)
	}
	if err := RunAssertsZ80(hm, steps.MIR2Module, steps.Allocation, steps.Assembly); err != nil {
		t.Errorf("z80: %v", err)
	}
}

// TestForwardGlobalReferencesOrderIndependentAssembly checks the
// order-independence property end to end: moving the function declarations
// after the globals must leave the generated Z80 assembly identical.
func TestForwardGlobalReferencesOrderIndependentAssembly(t *testing.T) {
	compile := func(src, name string) string {
		hm, err := nanz.Parse(src, name)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		steps, err := CompileHIRSteps(hm, Options{ContractOpt: true, UseGrace: true, AssertMode: "none"})
		if err != nil {
			t.Fatalf("compile %s: %v", name, err)
		}
		return steps.Assembly
	}
	a := compile(forwardGlobalValueSrc, "forward_global_order_a.nanz")
	b := compile(forwardGlobalValueSrcReordered, "forward_global_order_b.nanz")
	if a != b {
		t.Fatalf("Z80 assembly differs between declaration orders:\n--- functions first ---\n%s\n--- globals first ---\n%s", a, b)
	}
}
