package mir2_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	sim "github.com/cjbearman/sim6502/pkg"
	"github.com/minz/minzc/pkg/mir2"
)

// ── 6502 E2E test harness ───────────────────────────────────────────────────
//
// End-to-end test infrastructure for the MOS 6502 backend.  Mirrors the Z80
// harness in examples_test.go.
//
// Pipeline:
//   1. Build MIR2 IR with builder (or load from module)
//   2. Allocate registers with M6502CostTable (A, X, Y + 14 ZP locations)
//   3. Generate 6502 assembly with M6502Codegen
//   4. Prepend bootstrap (load args into A/X, JSR function, BRK)
//   5. Assemble with tinyAsm6502 (minimal 2-pass assembler, ~30 mnemonics)
//   6. Execute on sim6502 emulator (MIT, in-process, no GUI)
//   7. Assert A/X/Y register values + captured console output
//
// Console I/O:
//   MappableMemory traps writes to $F001 (bare-metal I/O port).
//   JSR stubs planted at $FDED (Apple II), $FFD2 (C64), $FFEE (BBC Micro)
//   route through the same port.  All four methods feed one output buffer.
//
// Dual-VM cross-check:
//   assertDual1/assertDual2 run the same IR through both the MIR2 VM
//   interpreter and the 6502 target pipeline, asserting results match.
//   The VM is the oracle — divergence means a codegen bug.

const m6502Origin = 0x0600 // common starting addr (avoids ZP and stack)

// ── Console I/O ─────────────────────────────────────────────────────────────
//
// Bare-metal console output using memory-mapped I/O port + JSR traps.
// All classic 6502 systems share the same convention: char in A, JSR to OS.
// We trap all four addresses simultaneously (they don't conflict):
//
//   $F001  — bare-metal I/O port (STA $F001 writes a byte)
//   $FDED  — Apple II COUT
//   $FFD2  — C64 KERNAL CHROUT
//   $FFEE  — BBC Micro OSWRCH
//
// The I/O port ($F001) is trapped via MappedMemoryWriteHandler — any STA
// to that address captures the byte directly.
//
// The JSR traps work differently: we plant an RTS (opcode $60) at each
// vector address.  When the program does JSR $FFEE, the CPU pushes the
// return address, jumps to $FFEE, finds RTS, and returns.  We intercept
// at the JSR *call site* by checking the PC on every step — but that's
// complex.  Instead, we use the simpler approach: plant a STA $F001; RTS
// stub at each vector.  The JSR lands there, writes A to the I/O port
// (which our handler captures), then returns.  3 bytes per stub.

// consoleDevice implements MappedMemoryWriteHandler for the I/O port.
type consoleDevice struct {
	buf bytes.Buffer
}

func (c *consoleDevice) AddressRange() []sim.MappedMemoryAddressRange {
	return []sim.MappedMemoryAddressRange{
		{Start: 0xF001, End: 0xF001}, // bare-metal I/O port
	}
}

func (c *consoleDevice) Write(addr uint16, val uint8) {
	c.buf.WriteByte(val)
}

// jsrTrapAddrs are the OS character-output entry points.
// We plant "STA $F001; RTS" stubs at each address.
var jsrTrapAddrs = []uint16{
	0xFDED, // Apple II COUT
	0xFFD2, // C64 KERNAL CHROUT
	0xFFEE, // BBC Micro OSWRCH
}

// stubSTA_F001_RTS is the 4-byte stub: STA $F001 (3 bytes) + RTS (1 byte).
// STA absolute = opcode $8D, addr lo, addr hi.
var stubSTA_F001_RTS = []byte{0x8D, 0x01, 0xF0, 0x60}

// run6502result holds everything from a 6502 execution.
type run6502result struct {
	A, X, Y uint8
	Output  string // captured console output
}

// run6502 assembles and runs 6502 code, returning registers after BRK.
// Uses Step() in a loop, stopping when we hit a BRK opcode (0x00).
func run6502(t *testing.T, src string) (a, x, y uint8, err error) {
	t.Helper()
	res, err := run6502WithIO(t, src)
	if err != nil {
		return 0, 0, 0, err
	}
	return res.A, res.X, res.Y, nil
}

// run6502WithIO assembles and runs 6502 code with console I/O support.
// Returns registers + any captured console output.
func run6502WithIO(t *testing.T, src string) (run6502result, error) {
	t.Helper()

	bin, _, asmErr := asm6502(src, m6502Origin)
	if asmErr != nil {
		return run6502result{}, fmt.Errorf("assemble: %w", asmErr)
	}

	// Use MappableMemory so we can trap I/O writes.
	mem := &sim.MappableMemory{}
	console := &consoleDevice{}
	mem.Map(console)

	proc := sim.NewProcessor(mem)

	// Load program binary.
	for i, b := range bin {
		mem.Write(uint16(m6502Origin+i), b)
	}

	// Plant JSR trap stubs at OS vector addresses.
	// Each stub: STA $F001; RTS — writes A to our I/O port, then returns.
	for _, addr := range jsrTrapAddrs {
		for i, b := range stubSTA_F001_RTS {
			mem.Write(addr+uint16(i), b)
		}
	}

	proc.Registers().PC.Set(m6502Origin)

	const maxSteps = 100000
	for i := 0; i < maxSteps; i++ {
		pc := proc.Registers().PC.Current()
		if mem.Read(pc) == 0x00 { // BRK — stop BEFORE executing
			break
		}
		stepErr, stopped := proc.Step()
		if stopped || stepErr != nil {
			break
		}
	}

	regs := proc.Registers()
	return run6502result{
		A:      regs.A,
		X:      regs.X,
		Y:      regs.Y,
		Output: console.buf.String(),
	}, nil
}

func safeGet(b []byte, i int) byte {
	if i < len(b) {
		return b[i]
	}
	return 0
}

// compile6502Module compiles a MIR2 module to 6502 assembly text.
func compile6502Module(t *testing.T, m *mir2.Module) string {
	t.Helper()
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
	}
	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}
	combined := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		ar := mir2.Allocate(f, lr, mir2.M6502CostTable{})
		for r, loc := range ar.Locs {
			combined.Locs[r] = loc
		}
	}
	return mir2.M6502Codegen(m, combined)
}

// stripDirectives removes .cpu and * = directives that the assembler skips anyway.
func stripDirectives(asm string) string {
	var sb strings.Builder
	for _, line := range strings.Split(asm, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, ".cpu") || strings.HasPrefix(trimmed, "* =") {
			continue
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

// boot6502_1 generates bootstrap code: LDA #arg0; JSR funcName; BRK
func boot6502_1(funcName string, arg0 uint8) string {
	return fmt.Sprintf("    LDA #$%02X\n    JSR %s\n    BRK\n", arg0, funcName)
}

// boot6502_2 generates bootstrap: LDA #arg0; LDX #arg1; JSR funcName; BRK
func boot6502_2(funcName string, arg0, arg1 uint8) string {
	return fmt.Sprintf("    LDA #$%02X\n    LDX #$%02X\n    JSR %s\n    BRK\n",
		arg0, arg1, funcName)
}

// boot6502_3 generates bootstrap: LDA #arg0; LDX #arg1; LDY #arg2; JSR funcName; BRK
func boot6502_3(funcName string, arg0, arg1, arg2 uint8) string {
	return fmt.Sprintf("    LDA #$%02X\n    LDX #$%02X\n    LDY #$%02X\n    JSR %s\n    BRK\n",
		arg0, arg1, arg2, funcName)
}

// ── Dual-VM assertion ───────────────────────────────────────────────────────
//
// assertDual runs the same MIR2 function through both:
//   1. MIR2 VM (interpreter) — the "reference" result
//   2. 6502 target (codegen → asm → sim6502) — the "target" result
// and asserts they match.  This gives us confidence that the 6502 codegen
// produces semantically correct code without needing hand-written expected
// values — the VM is the oracle.
//
// funcName: the function to call in the module
// args:     VM-level arguments (as int64 values)
// arg0/arg1 are the 6502-level bootstrap arguments (loaded into A / A+X)

type dualResult struct {
	vmVal    int64  // from MIR2 VM
	hwA      uint8  // from sim6502 accumulator
	funcName string
}

// assertDual1 tests a 1-argument function through both VM and 6502.
func assertDual1(t *testing.T, m *mir2.Module, funcName string, arg uint8) dualResult {
	t.Helper()

	// 1. MIR2 VM
	vm := &mir2.VM{Module: m, MaxSteps: 100000}
	vmRes, vmErr := vm.Call(funcName, []mir2.Value{{I: int64(arg)}})
	if vmErr != nil {
		t.Fatalf("VM.Call(%s, %d): %v", funcName, arg, vmErr)
	}
	vmVal := vmRes[0].I & 0xFF

	// 2. 6502 target
	asmText := compile6502Module(t, m)
	src := boot6502_1(funcName, arg) + "\n" + stripDirectives(asmText)
	hwA, _, _, hwErr := run6502(t, src)
	if hwErr != nil {
		t.Fatalf("6502(%s, %d): %v", funcName, arg, hwErr)
	}

	// 3. Cross-check
	if uint8(vmVal) != hwA {
		t.Errorf("%s(%d): VM=%d, 6502=%d — MISMATCH", funcName, arg, vmVal, hwA)
	} else {
		t.Logf("%s(%d): VM=%d, 6502=%d ✓", funcName, arg, vmVal, hwA)
	}

	return dualResult{vmVal: vmVal, hwA: hwA, funcName: funcName}
}

// assertDual2 tests a 2-argument function through both VM and 6502.
func assertDual2(t *testing.T, m *mir2.Module, funcName string, arg0, arg1 uint8) dualResult {
	t.Helper()

	// 1. MIR2 VM
	vm := &mir2.VM{Module: m, MaxSteps: 100000}
	vmRes, vmErr := vm.Call(funcName, []mir2.Value{{I: int64(arg0)}, {I: int64(arg1)}})
	if vmErr != nil {
		t.Fatalf("VM.Call(%s, %d, %d): %v", funcName, arg0, arg1, vmErr)
	}
	vmVal := vmRes[0].I & 0xFF

	// 2. 6502 target
	asmText := compile6502Module(t, m)
	src := boot6502_2(funcName, arg0, arg1) + "\n" + stripDirectives(asmText)
	hwA, _, _, hwErr := run6502(t, src)
	if hwErr != nil {
		t.Fatalf("6502(%s, %d, %d): %v", funcName, arg0, arg1, hwErr)
	}

	// 3. Cross-check
	if uint8(vmVal) != hwA {
		t.Errorf("%s(%d, %d): VM=%d, 6502=%d — MISMATCH", funcName, arg0, arg1, vmVal, hwA)
	} else {
		t.Logf("%s(%d, %d): VM=%d, 6502=%d ✓", funcName, arg0, arg1, vmVal, hwA)
	}

	return dualResult{vmVal: vmVal, hwA: hwA, funcName: funcName}
}

// assertDual3 tests a 3-argument function through both VM and 6502.
func assertDual3(t *testing.T, m *mir2.Module, funcName string, arg0, arg1, arg2 uint8) dualResult {
	t.Helper()

	// 1. MIR2 VM
	vm := &mir2.VM{Module: m, MaxSteps: 100000}
	vmRes, vmErr := vm.Call(funcName, []mir2.Value{{I: int64(arg0)}, {I: int64(arg1)}, {I: int64(arg2)}})
	if vmErr != nil {
		t.Fatalf("VM.Call(%s, %d, %d, %d): %v", funcName, arg0, arg1, arg2, vmErr)
	}
	vmVal := vmRes[0].I & 0xFF

	// 2. 6502 target
	asmText := compile6502Module(t, m)
	src := boot6502_3(funcName, arg0, arg1, arg2) + "\n" + stripDirectives(asmText)
	hwA, _, _, hwErr := run6502(t, src)
	if hwErr != nil {
		t.Fatalf("6502(%s, %d, %d, %d): %v", funcName, arg0, arg1, arg2, hwErr)
	}

	// 3. Cross-check
	if uint8(vmVal) != hwA {
		t.Errorf("%s(%d, %d, %d): VM=%d, 6502=%d — MISMATCH", funcName, arg0, arg1, arg2, vmVal, hwA)
	} else {
		t.Logf("%s(%d, %d, %d): VM=%d, 6502=%d ✓", funcName, arg0, arg1, arg2, vmVal, hwA)
	}

	return dualResult{vmVal: vmVal, hwA: hwA, funcName: funcName}
}

// ── E2E Tests ───────────────────────────────────────────────────────────────

func TestE2E_6502_Const(t *testing.T) {
	m := &mir2.Module{Name: "const"}
	f := m.AddFunc("answer")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	c := bld.Const(42, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(c)

	asm := compile6502Module(t, m)
	t.Log("\n" + asm)

	src := boot6502_1("answer", 0) + "\n" + stripDirectives(asm)
	a, _, _, err := run6502(t, src)
	if err != nil {
		t.Fatal(err)
	}
	if a != 42 {
		t.Errorf("answer() = %d, want 42", a)
	} else {
		t.Logf("answer() = %d ✓", a)
	}
}

func TestE2E_6502_Add(t *testing.T) {
	m := &mir2.Module{Name: "add"}
	f := m.AddFunc("add")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	a := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	b := bld.Param("b", mir2.TyU8, mir2.ClassGeneral)
	sum := bld.Add(a, b, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(sum)

	asmText := compile6502Module(t, m)
	t.Log("\n" + asmText)

	cases := []struct{ a, b, want uint8 }{
		{3, 5, 8},
		{0, 0, 0},
		{100, 55, 155},
		{200, 55, 255},
		{1, 254, 255},
	}
	for _, tc := range cases {
		src := boot6502_2("add", tc.a, tc.b) + "\n" + stripDirectives(asmText)
		got, _, _, err := run6502(t, src)
		if err != nil {
			t.Errorf("add(%d, %d): %v", tc.a, tc.b, err)
			continue
		}
		if got != tc.want {
			t.Errorf("add(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
		} else {
			t.Logf("add(%d, %d) = %d ✓", tc.a, tc.b, got)
		}
	}
}

func TestE2E_6502_Sub(t *testing.T) {
	m := &mir2.Module{Name: "sub"}
	f := m.AddFunc("sub")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	a := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	b := bld.Param("b", mir2.TyU8, mir2.ClassGeneral)
	diff := bld.Sub(a, b, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(diff)

	asmText := compile6502Module(t, m)
	t.Log("\n" + asmText)

	cases := []struct{ a, b, want uint8 }{
		{10, 3, 7},
		{255, 1, 254},
		{100, 100, 0},
		{0, 0, 0},
	}
	for _, tc := range cases {
		src := boot6502_2("sub", tc.a, tc.b) + "\n" + stripDirectives(asmText)
		got, _, _, err := run6502(t, src)
		if err != nil {
			t.Errorf("sub(%d, %d): %v", tc.a, tc.b, err)
			continue
		}
		if got != tc.want {
			t.Errorf("sub(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
		} else {
			t.Logf("sub(%d, %d) = %d ✓", tc.a, tc.b, got)
		}
	}
}

// TestE2E_6502_AsmRoundtrip tests the assembler+emulator independently.
func TestE2E_6502_AsmRoundtrip(t *testing.T) {
	// Hand-written: load 42, return
	src := `
    LDA #$2A
    BRK
`
	a, _, _, err := run6502(t, src)
	if err != nil {
		t.Fatal(err)
	}
	if a != 0x2A {
		t.Errorf("got A=%02X, want $2A", a)
	} else {
		t.Logf("roundtrip: A=$%02X ✓", a)
	}
}

// TestE2E_6502_AsmAddSub tests hand-written add and subtract.
func TestE2E_6502_AsmAddSub(t *testing.T) {
	src := `
    LDA #$10
    CLC
    ADC #$05
    BRK
`
	a, _, _, err := run6502(t, src)
	if err != nil {
		t.Fatal(err)
	}
	if a != 0x15 {
		t.Errorf("$10+$05: got A=$%02X, want $15", a)
	} else {
		t.Logf("$10+$05 = $%02X ✓", a)
	}

	src2 := `
    LDA #$20
    SEC
    SBC #$08
    BRK
`
	a2, _, _, err := run6502(t, src2)
	if err != nil {
		t.Fatal(err)
	}
	if a2 != 0x18 {
		t.Errorf("$20-$08: got A=$%02X, want $18", a2)
	} else {
		t.Logf("$20-$08 = $%02X ✓", a2)
	}
}

func TestE2E_6502_Double(t *testing.T) {
	m := &mir2.Module{Name: "double"}
	f := m.AddFunc("double")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	a := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	sum := bld.Add(a, a, mir2.TyU8, mir2.ClassAcc) // double = a + a
	bld.Ret(sum)

	asmText := compile6502Module(t, m)
	t.Log("\n" + asmText)

	cases := []struct{ in, want uint8 }{
		{0, 0}, {1, 2}, {5, 10}, {64, 128}, {127, 254},
	}
	for _, tc := range cases {
		src := boot6502_1("double", tc.in) + "\n" + stripDirectives(asmText)
		got, _, _, err := run6502(t, src)
		if err != nil {
			t.Errorf("double(%d): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("double(%d) = %d, want %d", tc.in, got, tc.want)
		} else {
			t.Logf("double(%d) = %d ✓", tc.in, got)
		}
	}
}

func TestE2E_6502_Neg(t *testing.T) {
	m := &mir2.Module{Name: "neg"}
	f := m.AddFunc("neg")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	a := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	n := bld.Neg(a, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(n)

	asmText := compile6502Module(t, m)
	t.Log("\n" + asmText)

	cases := []struct{ in, want uint8 }{
		{0, 0}, {1, 255}, {42, 214}, {128, 128}, {255, 1},
	}
	for _, tc := range cases {
		src := boot6502_1("neg", tc.in) + "\n" + stripDirectives(asmText)
		got, _, _, err := run6502(t, src)
		if err != nil {
			t.Errorf("neg(%d): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("neg(%d) = %d, want %d", tc.in, got, tc.want)
		} else {
			t.Logf("neg(%d) = %d ✓", tc.in, got)
		}
	}
}

func TestE2E_6502_BitwiseAnd(t *testing.T) {
	m := &mir2.Module{Name: "bitand"}
	f := m.AddFunc("bitand")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	a := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	b := bld.Param("b", mir2.TyU8, mir2.ClassGeneral)
	r := bld.And(a, b, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(r)

	asmText := compile6502Module(t, m)
	t.Log("\n" + asmText)

	cases := []struct{ a, b, want uint8 }{
		{0xFF, 0x0F, 0x0F},
		{0xAA, 0x55, 0x00},
		{0xFF, 0xFF, 0xFF},
		{0x00, 0xFF, 0x00},
	}
	for _, tc := range cases {
		src := boot6502_2("bitand", tc.a, tc.b) + "\n" + stripDirectives(asmText)
		got, _, _, err := run6502(t, src)
		if err != nil {
			t.Errorf("bitand($%02X, $%02X): %v", tc.a, tc.b, err)
			continue
		}
		if got != tc.want {
			t.Errorf("bitand($%02X, $%02X) = $%02X, want $%02X", tc.a, tc.b, got, tc.want)
		} else {
			t.Logf("bitand($%02X, $%02X) = $%02X ✓", tc.a, tc.b, got)
		}
	}
}

func TestE2E_6502_BitwiseOr(t *testing.T) {
	m := &mir2.Module{Name: "bitor"}
	f := m.AddFunc("bitor")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	a := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	b := bld.Param("b", mir2.TyU8, mir2.ClassGeneral)
	r := bld.Or(a, b, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(r)

	asmText := compile6502Module(t, m)
	t.Log("\n" + asmText)

	cases := []struct{ a, b, want uint8 }{
		{0xF0, 0x0F, 0xFF},
		{0xAA, 0x55, 0xFF},
		{0x00, 0x00, 0x00},
		{0x80, 0x01, 0x81},
	}
	for _, tc := range cases {
		src := boot6502_2("bitor", tc.a, tc.b) + "\n" + stripDirectives(asmText)
		got, _, _, err := run6502(t, src)
		if err != nil {
			t.Errorf("bitor($%02X, $%02X): %v", tc.a, tc.b, err)
			continue
		}
		if got != tc.want {
			t.Errorf("bitor($%02X, $%02X) = $%02X, want $%02X", tc.a, tc.b, got, tc.want)
		} else {
			t.Logf("bitor($%02X, $%02X) = $%02X ✓", tc.a, tc.b, got)
		}
	}
}

func TestE2E_6502_BitwiseXor(t *testing.T) {
	m := &mir2.Module{Name: "bitxor"}
	f := m.AddFunc("bitxor")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	a := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	b := bld.Param("b", mir2.TyU8, mir2.ClassGeneral)
	r := bld.Xor(a, b, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(r)

	asmText := compile6502Module(t, m)
	t.Log("\n" + asmText)

	cases := []struct{ a, b, want uint8 }{
		{0xFF, 0xFF, 0x00},
		{0xAA, 0x55, 0xFF},
		{0x00, 0x00, 0x00},
		{0x12, 0x34, 0x26},
	}
	for _, tc := range cases {
		src := boot6502_2("bitxor", tc.a, tc.b) + "\n" + stripDirectives(asmText)
		got, _, _, err := run6502(t, src)
		if err != nil {
			t.Errorf("bitxor($%02X, $%02X): %v", tc.a, tc.b, err)
			continue
		}
		if got != tc.want {
			t.Errorf("bitxor($%02X, $%02X) = $%02X, want $%02X", tc.a, tc.b, got, tc.want)
		} else {
			t.Logf("bitxor($%02X, $%02X) = $%02X ✓", tc.a, tc.b, got)
		}
	}
}

// TestE2E_6502_CallChain tests JSR to a helper function.
func TestE2E_6502_CallChain(t *testing.T) {
	m := &mir2.Module{Name: "chain"}

	// double(a) = a + a
	fDouble := m.AddFunc("double")
	fDouble.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(fDouble)
	bld.SwitchToNewBlock("entry")
	a := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	sum := bld.Add(a, a, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(sum)

	// quad(a) = double(double(a))
	fQuad := m.AddFunc("quad")
	fQuad.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld2 := mir2.NewBuilder(fQuad)
	bld2.SwitchToNewBlock("entry")
	a2 := bld2.Param("a", mir2.TyU8, mir2.ClassAcc)
	call1 := bld2.Call("double", []mir2.Reg{a2}, mir2.TyU8, mir2.ClassAcc, mir2.CallAttrs{})
	call2 := bld2.Call("double", []mir2.Reg{call1}, mir2.TyU8, mir2.ClassAcc, mir2.CallAttrs{})
	bld2.Ret(call2)

	asmText := compile6502Module(t, m)
	t.Log("\n" + asmText)

	cases := []struct{ in, want uint8 }{
		{0, 0}, {1, 4}, {5, 20}, {10, 40}, {63, 252},
	}
	for _, tc := range cases {
		src := boot6502_1("quad", tc.in) + "\n" + stripDirectives(asmText)
		got, _, _, err := run6502(t, src)
		if err != nil {
			t.Errorf("quad(%d): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("quad(%d) = %d, want %d", tc.in, got, tc.want)
		} else {
			t.Logf("quad(%d) = %d ✓", tc.in, got)
		}
	}
}

// ── Dual-VM E2E Tests (MIR2 VM ↔ 6502 cross-check) ─────────────────────────

func TestDual_6502_Add(t *testing.T) {
	m := &mir2.Module{Name: "add"}
	f := m.AddFunc("add")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	a := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	b := bld.Param("b", mir2.TyU8, mir2.ClassGeneral)
	sum := bld.Add(a, b, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(sum)

	for _, tc := range [][2]uint8{{3, 5}, {0, 0}, {100, 55}, {200, 55}, {1, 254}} {
		assertDual2(t, m, "add", tc[0], tc[1])
	}
}

func TestDual_6502_Sub(t *testing.T) {
	m := &mir2.Module{Name: "sub"}
	f := m.AddFunc("sub")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	a := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	b := bld.Param("b", mir2.TyU8, mir2.ClassGeneral)
	diff := bld.Sub(a, b, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(diff)

	for _, tc := range [][2]uint8{{10, 3}, {255, 1}, {100, 100}, {0, 0}} {
		assertDual2(t, m, "sub", tc[0], tc[1])
	}
}

func TestDual_6502_Double(t *testing.T) {
	m := &mir2.Module{Name: "double"}
	f := m.AddFunc("double")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	a := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	sum := bld.Add(a, a, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(sum)

	for _, v := range []uint8{0, 1, 5, 64, 127} {
		assertDual1(t, m, "double", v)
	}
}

func TestDual_6502_Neg(t *testing.T) {
	m := &mir2.Module{Name: "neg"}
	f := m.AddFunc("neg")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	a := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	n := bld.Neg(a, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(n)

	for _, v := range []uint8{0, 1, 42, 128, 255} {
		assertDual1(t, m, "neg", v)
	}
}

func TestDual_6502_BitwiseOps(t *testing.T) {
	for _, op := range []struct {
		name string
		build func(*mir2.Builder, mir2.Reg, mir2.Reg) mir2.Reg
	}{
		{"and", func(b *mir2.Builder, a, x mir2.Reg) mir2.Reg { return b.And(a, x, mir2.TyU8, mir2.ClassAcc) }},
		{"or", func(b *mir2.Builder, a, x mir2.Reg) mir2.Reg { return b.Or(a, x, mir2.TyU8, mir2.ClassAcc) }},
		{"xor", func(b *mir2.Builder, a, x mir2.Reg) mir2.Reg { return b.Xor(a, x, mir2.TyU8, mir2.ClassAcc) }},
	} {
		t.Run(op.name, func(t *testing.T) {
			m := &mir2.Module{Name: op.name}
			f := m.AddFunc(op.name)
			f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
			bld := mir2.NewBuilder(f)
			bld.SwitchToNewBlock("entry")
			a := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
			b := bld.Param("b", mir2.TyU8, mir2.ClassGeneral)
			r := op.build(bld, a, b)
			bld.Ret(r)

			for _, tc := range [][2]uint8{{0xFF, 0x0F}, {0xAA, 0x55}, {0x00, 0x00}, {0x12, 0x34}} {
				assertDual2(t, m, op.name, tc[0], tc[1])
			}
		})
	}
}

// ── Console I/O Tests ───────────────────────────────────────────────────────

// TestE2E_6502_ConsolePort tests bare-metal I/O port output (STA $F001).
func TestE2E_6502_ConsolePort(t *testing.T) {
	// Hand-written: write "Hi" to I/O port, then BRK
	src := `
    LDA #$48
    STA $F001
    LDA #$69
    STA $F001
    BRK
`
	res, err := run6502WithIO(t, src)
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "Hi" {
		t.Errorf("output = %q, want %q", res.Output, "Hi")
	} else {
		t.Logf("I/O port output: %q ✓", res.Output)
	}
}

// TestE2E_6502_ConsoleAppleII tests Apple II COUT trap (JSR $FDED).
func TestE2E_6502_ConsoleAppleII(t *testing.T) {
	src := `
    LDA #$41
    JSR $FDED
    LDA #$32
    JSR $FDED
    BRK
`
	res, err := run6502WithIO(t, src)
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "A2" {
		t.Errorf("output = %q, want %q", res.Output, "A2")
	} else {
		t.Logf("Apple II COUT: %q ✓", res.Output)
	}
}

// TestE2E_6502_ConsoleBBCMicro tests BBC Micro OSWRCH trap (JSR $FFEE).
func TestE2E_6502_ConsoleBBCMicro(t *testing.T) {
	src := `
    LDA #$42
    JSR $FFEE
    LDA #$42
    JSR $FFEE
    LDA #$43
    JSR $FFEE
    BRK
`
	res, err := run6502WithIO(t, src)
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "BBC" {
		t.Errorf("output = %q, want %q", res.Output, "BBC")
	} else {
		t.Logf("BBC Micro OSWRCH: %q ✓", res.Output)
	}
}

// TestE2E_6502_ConsoleC64 tests C64 CHROUT trap (JSR $FFD2).
func TestE2E_6502_ConsoleC64(t *testing.T) {
	src := `
    LDA #$43
    JSR $FFD2
    LDA #$36
    JSR $FFD2
    LDA #$34
    JSR $FFD2
    BRK
`
	res, err := run6502WithIO(t, src)
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "C64" {
		t.Errorf("output = %q, want %q", res.Output, "C64")
	} else {
		t.Logf("C64 CHROUT: %q ✓", res.Output)
	}
}

// TestE2E_6502_ConsoleLoop tests printing a string via a loop.
func TestE2E_6502_ConsoleLoop(t *testing.T) {
	// Print "HELLO" by loading each char and calling bare-metal putchar.
	// Tests that JSR→STA $F001→RTS round-trip preserves state.
	src := `
    LDA #$48
    STA $F001
    LDA #$45
    STA $F001
    LDA #$4C
    STA $F001
    LDA #$4C
    STA $F001
    LDA #$4F
    STA $F001
    BRK
`
	res, err := run6502WithIO(t, src)
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "HELLO" {
		t.Errorf("output = %q, want %q", res.Output, "HELLO")
	} else {
		t.Logf("console output: %q ✓", res.Output)
	}
}

// TestE2E_6502_ConsoleMixed tests mixing different OS vectors in one program.
// All four methods should produce output in the same buffer.
func TestE2E_6502_ConsoleMixed(t *testing.T) {
	src := `
    LDA #$31
    STA $F001
    LDA #$32
    JSR $FDED
    LDA #$33
    JSR $FFD2
    LDA #$34
    JSR $FFEE
    BRK
`
	res, err := run6502WithIO(t, src)
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "1234" {
		t.Errorf("output = %q, want %q", res.Output, "1234")
	} else {
		t.Logf("mixed console: %q (port/A2/C64/BBC) ✓", res.Output)
	}
}

// ── Branching E2E Tests (BrIf2 three-way branch) ────────────────────────────

// buildAbsDiff6502 builds: abs_diff(a, b) = if a>b then a-b elif a<b then b-a else 0
func buildAbsDiff6502() *mir2.Module {
	m := &mir2.Module{Name: "abs_diff"}
	f := m.AddFunc("abs_diff")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bv := b.Param("b", mir2.TyU8, mir2.ClassGeneral)
	b.BrIf2(a, bv, "eq", nil, "a_lt_b", nil, "a_gt_b", nil)

	b.SwitchToNewBlock("eq")
	zero := b.Const(0, mir2.TyU8, mir2.ClassAcc)
	b.Ret(zero)

	b.SwitchToNewBlock("a_lt_b")
	diff1 := b.Sub(bv, a, mir2.TyU8, mir2.ClassAcc)
	b.Ret(diff1)

	b.SwitchToNewBlock("a_gt_b")
	diff2 := b.Sub(a, bv, mir2.TyU8, mir2.ClassAcc)
	b.Ret(diff2)

	return m
}

func TestE2E_6502_AbsDiff(t *testing.T) {
	m := buildAbsDiff6502()
	asmText := compile6502Module(t, m)
	t.Log("\n" + asmText)

	cases := []struct{ a, b, want uint8 }{
		{10, 3, 7}, {3, 10, 7}, {5, 5, 0}, {200, 100, 100}, {0, 255, 255},
	}
	for _, tc := range cases {
		src := boot6502_2("abs_diff", tc.a, tc.b) + "\n" + stripDirectives(asmText)
		got, _, _, err := run6502(t, src)
		if err != nil {
			t.Errorf("abs_diff(%d, %d): %v", tc.a, tc.b, err)
			continue
		}
		if got != tc.want {
			t.Errorf("abs_diff(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
		} else {
			t.Logf("abs_diff(%d, %d) = %d ✓", tc.a, tc.b, got)
		}
	}
}

func TestDual_6502_AbsDiff(t *testing.T) {
	m := buildAbsDiff6502()
	for _, tc := range [][2]uint8{{10, 3}, {3, 10}, {5, 5}, {200, 100}, {0, 255}, {1, 0}, {0, 0}} {
		assertDual2(t, m, "abs_diff", tc[0], tc[1])
	}
}

// buildClamp6502 builds: clamp(x, lo, hi) — cascaded BrIf2, no block params
func buildClamp6502() *mir2.Module {
	m := &mir2.Module{Name: "clamp"}
	f := m.AddFunc("clamp")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.TyU8, mir2.ClassAcc)
	lo := b.Param("lo", mir2.TyU8, mir2.ClassGeneral)
	hi := b.Param("hi", mir2.TyU8, mir2.ClassGeneral)
	// Compare x vs lo: x==lo → check_hi, x<lo → ret_lo, x>lo → check_hi
	b.BrIf2(x, lo, "check_hi", nil, "ret_lo", nil, "check_hi", nil)

	b.SwitchToNewBlock("ret_lo")
	loA := b.Move(lo, mir2.TyU8, mir2.ClassAcc)
	b.Ret(loA)

	b.SwitchToNewBlock("check_hi")
	// Compare x vs hi: x==hi → ret_x, x<hi → ret_x, x>hi → ret_hi
	b.BrIf2(x, hi, "ret_x", nil, "ret_x", nil, "ret_hi", nil)

	b.SwitchToNewBlock("ret_hi")
	hiA := b.Move(hi, mir2.TyU8, mir2.ClassAcc)
	b.Ret(hiA)

	b.SwitchToNewBlock("ret_x")
	b.Ret(x)

	return m
}

func TestDual_6502_Clamp(t *testing.T) {
	m := buildClamp6502()
	for _, tc := range [][3]uint8{
		{5, 0, 10},     // in range
		{0, 5, 10},     // below lo → 5
		{15, 5, 10},    // above hi → 10
		{5, 5, 5},      // all equal
		{128, 10, 200}, // in range
		{0, 0, 255},    // lo edge
		{255, 0, 255},  // hi edge
		{0, 100, 200},  // below
		{250, 100, 200}, // above
	} {
		assertDual3(t, m, "clamp", tc[0], tc[1], tc[2])
	}
}

// buildMax36502 builds: max3(a, b, c) — cascaded BrIf2, no block params
func buildMax36502() *mir2.Module {
	m := &mir2.Module{Name: "max3"}
	f := m.AddFunc("max3")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bv := b.Param("b", mir2.TyU8, mir2.ClassGeneral)
	c := b.Param("c", mir2.TyU8, mir2.ClassGeneral)
	// Compare a vs b: a==b → check_ac, a<b → check_bc, a>b → check_ac
	b.BrIf2(a, bv, "check_ac", nil, "check_bc", nil, "check_ac", nil)

	b.SwitchToNewBlock("check_ac")
	// max(a,b) = a. Compare a vs c.
	b.BrIf2(a, c, "ret_a", nil, "ret_c", nil, "ret_a", nil)

	b.SwitchToNewBlock("check_bc")
	// max(a,b) = b. Compare b vs c.
	b.BrIf2(bv, c, "ret_b", nil, "ret_c2", nil, "ret_b", nil)

	b.SwitchToNewBlock("ret_a")
	b.Ret(a)

	b.SwitchToNewBlock("ret_b")
	bvA := b.Move(bv, mir2.TyU8, mir2.ClassAcc)
	b.Ret(bvA)

	b.SwitchToNewBlock("ret_c")
	cA := b.Move(c, mir2.TyU8, mir2.ClassAcc)
	b.Ret(cA)

	b.SwitchToNewBlock("ret_c2")
	cA2 := b.Move(c, mir2.TyU8, mir2.ClassAcc)
	b.Ret(cA2)

	return m
}

func TestDual_6502_Max3(t *testing.T) {
	m := buildMax36502()
	for _, tc := range [][3]uint8{
		{1, 2, 3}, {3, 2, 1}, {2, 3, 1}, {7, 7, 7}, {100, 200, 150},
		{0, 0, 0}, {255, 0, 0}, {0, 255, 0}, {0, 0, 255},
	} {
		assertDual3(t, m, "max3", tc[0], tc[1], tc[2])
	}
}

// TestDual_6502_CmpBrIf tests OpCmp (boolean materialization) + TermBrIf path.
// This is the alternative to BrIf2 — compare produces a boolean, BrIf tests it.
func TestDual_6502_CmpBrIf(t *testing.T) {
	// max2(a, b) = if a > b then a else b  (using OpCmp + BrIf)
	m := &mir2.Module{Name: "max2"}
	f := m.AddFunc("max2")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bv := b.Param("b", mir2.TyU8, mir2.ClassGeneral)
	cond := b.Cmp(mir2.CmpUgt, a, bv, mir2.ClassAcc, false)
	b.BrIf(cond, "ret_a", nil, "ret_b", nil)

	b.SwitchToNewBlock("ret_a")
	b.Ret(a)

	b.SwitchToNewBlock("ret_b")
	b.Ret(bv)

	for _, tc := range [][2]uint8{
		{10, 3}, {3, 10}, {5, 5}, {255, 0}, {0, 255}, {100, 100},
	} {
		assertDual2(t, m, "max2", tc[0], tc[1])
	}
}

// Ensure we don't use the bytes import unnecessarily — suppress unused import
var _ = bytes.Compare
