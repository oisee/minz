package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// runFlagReturnNative compiles src with the default options and runs each
// one-byte entry for every argument byte on the Z80, against the MIR2 VM.
// It returns the chosen bool return contracts.
func runFlagReturnNative(t *testing.T, src string, entries ...string) mir2.FlagReturnReport {
	t.Helper()
	h, e := nanz.Parse(src, "flagret.nanz")
	if e != nil {
		t.Fatal(e)
	}
	steps, e := CompileHIRSteps(h, DefaultOptions())
	if e != nil {
		t.Fatalf("%v\n%s", e, steps.Assembly)
	}
	var drivers strings.Builder
	for _, entry := range entries {
		fmt.Fprintf(&drivers, "\noracle_%s:\n CALL %s\n RET\n", entry, entry)
	}
	assembled, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly + drivers.String())
	if e != nil || len(assembled.Errors) > 0 {
		t.Fatal(e, assembled.Errors)
	}
	hirFuncs := map[string]*hir.Func{}
	for _, entry := range entries {
		hirFuncs[entry] = h.FuncByName(entry)
	}
	vm := mir2.NewVM(steps.MIR2Module)
	for _, entry := range entries {
		f := steps.MIR2Module.FuncByName(entry)
		if f == nil || len(f.Contract.Params) != 1 {
			t.Fatalf("entry %s missing or not unary", entry)
		}
		for x := int64(0); x < 256; x++ {
			v, e := vm.Call(entry, []mir2.Value{{I: x}})
			if e != nil || len(v) != 1 {
				t.Fatalf("VM %s(%d): %v %v", entry, x, v, e)
			}
			z := emulator.NewRemogattoZ80()
			if e = z.LoadMemory(0x8000, assembled.Binary); e != nil {
				t.Fatal(e)
			}
			mem := z.Memory()
			mem[0xeff0], mem[0xeff1] = 0, 0x70
			regs := emulator.Registers{PC: uint16(assembled.Symbols["oracle_"+entry]), SP: 0xeff0, BC: 0x5555, DE: 0x6666, HL: 0x7777, A: 0x42}
			switch loc := assertParamLocation(f.Contract.Params[0], steps.Allocation); loc.Name {
			case "A":
				regs.A = byte(x)
			case "B":
				regs.BC = regs.BC&255 | uint16(x)<<8
			case "C":
				regs.BC = regs.BC&0xff00 | uint16(x)
			case "D":
				regs.DE = regs.DE&255 | uint16(x)<<8
			case "E":
				regs.DE = regs.DE&0xff00 | uint16(x)
			case "H":
				regs.HL = regs.HL&255 | uint16(x)<<8
			case "L":
				regs.HL = regs.HL&0xff00 | uint16(x)
			default:
				if loc.Kind != mir2.LocMem {
					t.Fatalf("unhandled entry ABI %v", loc)
				}
				mem[assembled.Symbols[mir2.Z80SpillLabel(f.Name, f.Contract.Params[0].Reg)]] = byte(x)
			}
			z.SetRegisters(regs)
			done := false
			for j := 0; j < 1000000 && !done; j++ {
				done = z.GetPC() == 0x7000 && z.GetSP() == 0xeff2
				if !done {
					z.Step()
				}
			}
			if !done {
				t.Fatalf("%s(%d) did not return", entry, x)
			}
			if e = checkAssertZ80Result(z, hir.Assert{FuncName: entry, Args: []int64{x}, Expected: v[0].I}, f, hirFuncs); e != nil {
				t.Fatalf("%s(%d): %v\n%s", entry, x, e, steps.Assembly)
			}
		}
	}
	return steps.FlagReturns
}

func wantFlagReturns(t *testing.T, rep mir2.FlagReturnReport, chosen map[string]mir2.CmpCond, refused map[string]string) {
	t.Helper()
	for name, c := range chosen {
		if got, ok := rep.Chosen[name]; !ok || got != c {
			t.Errorf("%s: chosen %v (%v), want %v; refused: %q", name, got, ok, c, rep.Refused[name])
		}
	}
	for name, reason := range refused {
		if got := rep.Refused[name]; got != reason {
			t.Errorf("%s: refused %q, want %q (chosen: %v)", name, got, reason, rep.Chosen)
		}
	}
}

// A predicate with several early returns: constants and comparisons mix,
// so true is NZ; both callers branch on it directly.
func TestFlagReturnNativeEarlyReturns(t *testing.T) {
	rep := runFlagReturnNative(t, `
fun classify(c: u8) -> bool {
    if c == 0 { return false }
    if c < 10 { return true }
    if c == 77 { return true }
    if c > 200 { return false }
    return c == 120
}
fun use_classify(c: u8) -> u8 {
    if classify(c) { return 1 }
    if !classify(c + 1) { return 2 }
    return 3
}
`, "classify", "use_classify")
	wantFlagReturns(t, rep, map[string]mir2.CmpCond{"classify": mir2.CmpNe}, nil)
}

// An is_digit-style range test and a single comparison with carry and Z
// conditions of their own.
func TestFlagReturnNativeRange(t *testing.T) {
	rep := runFlagReturnNative(t, `
fun is_digit(c: u8) -> bool {
    if c >= 48 {
        if c <= 57 { return true }
    }
    return false
}
fun below(c: u8) -> bool { return ((c + 3) & 127) + (c >> 2) < 100 }
fun is_seven(c: u8) -> bool { return ((c xor 85) & 63) + (c >> 3) == 7 }
fun kind(c: u8) -> u8 {
    if is_digit(c) { return 1 }
    if below(c) { return 2 }
    if is_seven(c - 3) { return 3 }
    if !below(c + 9) { return 5 }
    if is_seven(c + 1) { return 6 }
    return 4
}
`, "is_digit", "below", "is_seven", "kind")
	wantFlagReturns(t, rep, map[string]mir2.CmpCond{"is_digit": mir2.CmpNe, "below": mir2.CmpLt, "is_seven": mir2.CmpEq}, nil)
}

// Mutual recursion: each predicate returns the other's result, so both
// keep one condition and the returns become tail jumps.
func TestFlagReturnNativeMutualRecursion(t *testing.T) {
	rep := runFlagReturnNative(t, `
fun is_even(n: u8) -> bool {
    if n == 0 { return true }
    return is_odd(n - 1)
}
fun is_odd(n: u8) -> bool {
    if n == 0 { return false }
    return is_even(n - 1)
}
fun parity(n: u8) -> u8 {
    if is_even(n) { return 0 }
    return 1
}
`, "is_even", "is_odd", "parity")
	wantFlagReturns(t, rep, map[string]mir2.CmpCond{"is_even": mir2.CmpNe, "is_odd": mir2.CmpNe}, nil)
}

// A predicate whose result is also stored keeps the byte return; so does a
// predicate whose address is taken (the front end fixes its indirect ABI),
// and one named in inline asm.
func TestFlagReturnNativeRefusals(t *testing.T) {
	rep := runFlagReturnNative(t, `
global seen: bool = false
type Pred = fun(u8) -> bool
fun big(c: u8) -> bool {
    if c == 3 { return true }
    if c == 50 { return false }
    return c > 100
}
fun store_and_test(c: u8) -> u8 {
    seen = big(c)
    if big(c + 7) { return 1 }
    if seen { return 2 }
    if big(c - 9) { return 4 }
    return 3
}
fun odd(c: u8) -> bool { return (c & 1) == 1 }
fun apply(p: Pred, c: u8) -> bool { return p(c) }
fun via_pointer(c: u8) -> u8 {
    if apply(odd, c) { return 1 }
    if odd(c + 1) { return 2 }
    return 3
}
fun small(c: u8) -> bool { return c < 20 }
fun via_asm(c: u8) -> u8 {
    asm {
        CALL small
    }
    if small(c) { return 1 }
    return 2
}
`, "big", "store_and_test", "odd", "via_pointer", "small")
	wantFlagReturns(t, rep, nil, map[string]string{
		"big":   "result used as a value",
		"odd":   "indirect ABI",
		"small": "named in inline asm",
	})
}
