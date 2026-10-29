package nanz_test

// E2E emulator tests for `global ... at(addr)` — an absolute-address global is
// a symbol for a fixed Z80 address, not data in the program image.  Reads and
// writes must reach that address in every addressing form the emitter uses,
// and no storage may be emitted for it in the image.

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

const atGlobalLoadAddr = 0x8000

// atGlobalSource covers byte, word and indexed-array accesses plus a pointer
// to the same global, so every emitter addressing form is exercised.
const atGlobalSource = `global port_shadow: u8 at(0x5000)
global word_shadow: u16 at(0x5002)
global buf: [u8; 4] at(0x5010)
global mirror: [u8; 4]

fun main() -> void {
    port_shadow = port_shadow + 1
    word_shadow = word_shadow + 1
    buf[2] = buf[2] + 1
    let p: ^u8 = &port_shadow
    p^ = p^ + 2
    mirror[0] = port_shadow
    mirror[1] = buf[2]
    mirror[3] = p^
}
`

// atGlobalCompile parses and compiles src, failing the test on any error.
func atGlobalCompile(t *testing.T, src, name string) pipeline.Steps {
	t.Helper()
	m, err := nanz.Parse(src, name)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return steps
}

// atGlobalSteps compiles atGlobalSource once; several tests inspect the same
// module and compilation is the expensive part of each.
var atGlobalSteps = sync.OnceValues(func() (pipeline.Steps, error) {
	m, err := nanz.Parse(atGlobalSource, "at_global_e2e")
	if err != nil {
		return pipeline.Steps{}, err
	}
	return pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
})

func atGlobalModule(t *testing.T) pipeline.Steps {
	t.Helper()
	steps, err := atGlobalSteps()
	if err != nil {
		t.Fatalf("compile atGlobalSource: %v", err)
	}
	return steps
}

// atGlobalRun assembles compiled assembly behind a boot stub that sets SP,
// runs seed (one instruction per line, used to preload the fixed addresses),
// calls main and halts; then it executes the result on the Z80 emulator.
func atGlobalRun(t *testing.T, asm, seed string) *emulator.RemogattoZ80 {
	t.Helper()
	boot := fmt.Sprintf("    ORG 0x%04X\n    LD SP, 0xFF00\n%s    CALL main\n    DI\n    HALT\n",
		atGlobalLoadAddr, seed)
	as := z80asm.NewAssembler()
	res, err := as.AssembleString(boot + "\n" + asm)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if len(res.Errors) > 0 {
		for _, e := range res.Errors {
			t.Errorf("assemble: %v", e)
		}
		t.FailNow()
	}
	z80 := emulator.NewRemogattoZ80()
	if lerr := z80.LoadMemory(atGlobalLoadAddr, res.Binary); lerr != nil {
		t.Fatalf("load: %v", lerr)
	}
	z80.SetPC(atGlobalLoadAddr)
	z80.SetSP(0xFF00)
	if rerr := z80.Run(); rerr != nil {
		t.Fatalf("run: %v", rerr)
	}
	return z80
}

// checkMemory compares emulator memory byte by byte.
func checkMemory(t *testing.T, z80 *emulator.RemogattoZ80, checks []struct {
	addr uint16
	want byte
	what string
}) {
	t.Helper()
	for _, c := range checks {
		if got := z80.GetMemory(c.addr); got != c.want {
			t.Errorf("%s = 0x%02X, want 0x%02X", c.what, got, c.want)
		} else {
			t.Logf("%s = 0x%02X ✓", c.what, got)
		}
	}
}

// TestAtGlobal_E2E_FixedAddress seeds the fixed addresses through the
// emulator, runs main and checks the memory at 0x5000.. directly.
func TestAtGlobal_E2E_FixedAddress(t *testing.T) {
	steps := atGlobalModule(t)
	t.Logf("generated Z80:\n%s", steps.Assembly)

	z80 := atGlobalRun(t, steps.Assembly, `    LD A, 0x0F
    LD (0x5000), A
    LD A, 0x34
    LD (0x5002), A
    LD A, 0x12
    LD (0x5003), A
    LD A, 0x21
    LD (0x5012), A
`)

	// port_shadow = 0x0F + 1, then p^ = that + 2.
	// word_shadow = 0x1234 + 1 (little-endian at 0x5002).
	// buf[2] = 0x21 + 1.
	checkMemory(t, z80, []struct {
		addr uint16
		want byte
		what string
	}{
		{0x5000, 0x12, "port_shadow byte at 0x5000 (fixed address not used)"},
		{0x5002, 0x35, "word_shadow low byte at 0x5002"},
		{0x5003, 0x12, "word_shadow high byte at 0x5003"},
		{0x5012, 0x22, "buf[2] element at 0x5012"},
	})
}

// TestAtGlobal_NoImageStorage checks that at-globals resolve to their fixed
// addresses as EQU symbols and carry no data in the program image, while an
// ordinary global in the same module still gets image storage.
func TestAtGlobal_NoImageStorage(t *testing.T) {
	asm := atGlobalModule(t).Assembly
	for _, g := range []struct {
		name string
		addr int
	}{
		{"port_shadow", 0x5000},
		{"word_shadow", 0x5002},
		{"buf", 0x5010},
	} {
		if !strings.Contains(asm, fmt.Sprintf("%s    EQU  0x%04X", g.name, g.addr)) {
			t.Errorf("global %s: no EQU for fixed address 0x%04X in:\n%s", g.name, g.addr, asm)
		}
		if strings.Contains(asm, g.name+":\n") {
			t.Errorf("global %s: storage label emitted for an at-global in:\n%s", g.name, asm)
		}
	}
	// The ordinary global keeps its data-section storage.
	if !strings.Contains(asm, "mirror:\n") {
		t.Errorf("ordinary global mirror lost its storage label in:\n%s", asm)
	}

	// Assembling the module must place the at-globals at their fixed addresses
	// and the ordinary global inside the emitted image.
	res, err := z80asm.NewAssembler().AssembleString(
		fmt.Sprintf("    ORG 0x%04X\n    CALL main\n    DI\n    HALT\n", atGlobalLoadAddr) + "\n" + asm)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if len(res.Errors) > 0 {
		for _, e := range res.Errors {
			t.Errorf("assemble: %v", e)
		}
		t.FailNow()
	}
	for sym, want := range map[string]int{"port_shadow": 0x5000, "word_shadow": 0x5002, "buf": 0x5010} {
		if got, ok := res.Symbols[sym]; !ok {
			t.Errorf("symbol %s missing from assembled image", sym)
		} else if got != want {
			t.Errorf("symbol %s = 0x%04X, want fixed address 0x%04X", sym, got, want)
		}
	}
	if got, ok := res.Symbols["mirror"]; !ok {
		t.Errorf("symbol mirror missing from assembled image")
	} else if got < atGlobalLoadAddr {
		t.Errorf("ordinary global mirror at 0x%04X, want image storage at or above 0x%04X", got, atGlobalLoadAddr)
	}
}

// TestAtGlobal_ExcludedFromDataBudget pins the capacity accounting: an
// at-global places no bytes in the image, so it must not consume the 16-bit
// global-data budget. Before that rule this module was rejected with
// `Z80 global "pad" exceeds 16-bit address space (data before global: 60000
// bytes, global: 8000 bytes)`.
func TestAtGlobal_ExcludedFromDataBudget(t *testing.T) {
	steps := atGlobalCompile(t, `global big: [u8; 60000] at(0x4000)
global pad: [u8; 8000]

fun main() -> void {
    big[0] = 0x5A
    big[1] = big[0] + 1
    pad[0] = big[1]
    pad[7999] = 0x33
}
`, "at_global_budget")
	if !strings.Contains(steps.Assembly, "big    EQU  0x4000") {
		t.Errorf("missing EQU for the fixed address in:\n%s", steps.Assembly)
	}

	res, err := z80asm.NewAssembler().AssembleString(
		fmt.Sprintf("    ORG 0x%04X\n    LD SP, 0xFF00\n    CALL main\n    DI\n    HALT\n", atGlobalLoadAddr) + "\n" + steps.Assembly)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if len(res.Errors) > 0 {
		for _, e := range res.Errors {
			t.Errorf("assemble: %v", e)
		}
		t.FailNow()
	}
	pad, ok := res.Symbols["pad"]
	if !ok {
		t.Fatal("symbol pad missing from assembled image")
	}
	z80 := emulator.NewRemogattoZ80()
	if lerr := z80.LoadMemory(atGlobalLoadAddr, res.Binary); lerr != nil {
		t.Fatalf("load: %v", lerr)
	}
	z80.SetPC(atGlobalLoadAddr)
	z80.SetSP(0xFF00)
	if rerr := z80.Run(); rerr != nil {
		t.Fatalf("run: %v", rerr)
	}
	checkMemory(t, z80, []struct {
		addr uint16
		want byte
		what string
	}{
		{0x4000, 0x5A, "big[0] at 0x4000"},
		{0x4001, 0x5B, "big[1] at 0x4001 (read-modify-write)"},
		{uint16(pad), 0x5B, "pad[0] in image storage"},
		{uint16(pad + 7999), 0x33, "pad[7999] in image storage"},
	})
}

// TestAtGlobal_AddressRejected checks the at() argument is validated where the
// source position is known: a non-literal or an address outside 0..0xFFFF is a
// diagnostic, not a truncated address (0x10000 → 0x0000) or a silently
// ordinary global.
func TestAtGlobal_AddressRejected(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{"non-literal", "global p: u8 at(0x5000 + 1)\n", "at() requires a literal address"},
		{"name", "global p: u8 at(PORT)\n", "unknown variable"},
		{"too-large", "global p: u8 at(0x10000)\n", "outside 0..0xFFFF"},
		{"decimal-too-large", "global p: u8 at(70000)\n", "outside 0..0xFFFF"},
		{"negative", "global p: u8 at(-1)\n", "outside 0..0xFFFF"},
		{"local-var", "fun main() -> void { var p: u8 at(0x10000) }\n", "outside 0..0xFFFF"},
	} {
		_, err := nanz.Parse(tc.src, "at_global_addr")
		if err == nil {
			t.Errorf("%s: accepted %q", tc.name, tc.src)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "line 1:") {
			t.Errorf("%s: error %q, want %q with a line number", tc.name, err, tc.want)
		} else {
			t.Logf("%s: rejected as expected: %v", tc.name, err)
		}
	}
}

// TestAtGlobal_InitializerRejected checks that an initialiser on an at-global
// is a compile error: the image carries no storage for it, so initial data
// would be silently dropped. The Nanz frontend reports it with the source line;
// MIR2 keeps the same rule for other producers.
func TestAtGlobal_InitializerRejected(t *testing.T) {
	_, err := nanz.Parse("global port_shadow: u8 at(0x5000) = 0xAB\nfun main() -> void { }\n", "at_global_init")
	if err == nil {
		t.Fatal("initialiser on an at-global must be rejected at parse time")
	}
	if !strings.Contains(err.Error(), "line 1:") || !strings.Contains(err.Error(), "cannot have an initialiser") {
		t.Fatalf("error %q, want a line-numbered initialiser diagnostic", err)
	}
	t.Logf("rejected as expected: %v", err)
}

// TestAtGlobal_E2E_StructFields covers a struct at a fixed address: direct
// field addressing (LD A,(sym__field)) must resolve through the EQU base too.
func TestAtGlobal_E2E_StructFields(t *testing.T) {
	steps := atGlobalCompile(t, `struct Regs { lo: u8, hi: u8, ctrl: u8 }
global regs: Regs at(0x5020)

fun main() -> void {
    regs.lo = 0x11
    regs.hi = regs.ctrl + 2
}
`, "at_global_struct_e2e")
	t.Logf("generated Z80:\n%s", steps.Assembly)

	z80 := atGlobalRun(t, steps.Assembly, "    LD A, 7\n    LD (0x5022), A\n")
	// regs.ctrl = 7 seeded at 0x5022, so regs.hi = 7 + 2 = 9.
	checkMemory(t, z80, []struct {
		addr uint16
		want byte
		what string
	}{
		{0x5020, 0x11, "regs.lo at 0x5020"},
		{0x5021, 9, "regs.hi at 0x5021 (read of regs.ctrl at 0x5022 + 2)"},
		{0x5022, 7, "regs.ctrl at 0x5022 unchanged"},
	})
}
