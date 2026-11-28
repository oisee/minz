package pipeline

import (
	"regexp"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// in_interval-shaped leaves: a byte bound computed from a parameter with a
// constant, two compares, a boolean join returned as 0/1. Each program is run
// on the Z80 for every (value, low) pair, in C and Nanz, default and --Osize;
// the low + 31 bound wraps for low >= 225 (low = 230 gives high = 5).
type inIntervalCase struct {
	name    string
	c, nanz string
	want    func(v, low byte) byte
}

var inIntervalCases = []inIntervalCase{
	{"in_interval",
		`unsigned char in_interval(unsigned char value, unsigned char low) {
    unsigned char high = low + 31;
    return value >= low && value <= high;
}`,
		`fun in_interval(value:u8, low:u8)->u8 {let high:u8 = low + 31
 if value >= low && value <= high {return 1}
 return 0}`,
		func(v, low byte) byte { return zeroOne(v >= low && v <= low+31) }},
	{"in_interval_sub",
		`unsigned char in_interval(unsigned char value, unsigned char low) {
    unsigned char high = low - 100;
    return value >= low && value <= high;
}`,
		`fun in_interval(value:u8, low:u8)->u8 {let high:u8 = low - 100
 if value >= low && value <= high {return 1}
 return 0}`,
		func(v, low byte) byte { return zeroOne(v >= low && v <= low-100) }},
	{"in_interval_ge",
		`unsigned char in_interval(unsigned char value, unsigned char low) {
    unsigned char high = low + 31;
    return value >= low && high >= value;
}`,
		`fun in_interval(value:u8, low:u8)->u8 {let high:u8 = low + 31
 if value >= low && high >= value {return 1}
 return 0}`,
		func(v, low byte) byte { return zeroOne(v >= low && low+31 >= v) }},
}

type inIntervalBuild struct {
	asm   string
	body  string // in_interval's instructions, comments and blank lines dropped
	bytes int
}

func buildInInterval(t *testing.T, tc inIntervalCase, language string, size bool) inIntervalBuild {
	t.Helper()
	var h *hir.Module
	var err error
	if language == "c" {
		h, err = c89.CompileWithOpts("unsigned char a, b; unsigned short result;\n"+tc.c+
			"\nvoid execute(void) { result = in_interval(a, b); }", "ii.c", c89.CompileOpts{NoLibc: true})
	} else {
		h, err = nanz.Parse("global a:u8=0\nglobal b:u8=0\nglobal result:u16=0\n"+tc.nanz+
			"\nfun execute(){result = in_interval(a, b) as u16}", "ii.nanz")
	}
	if err != nil {
		t.Fatal(err)
	}
	opts := DefaultOptions()
	opts.OptSize = size
	s, err := CompileHIRSteps(h, opts)
	if err != nil {
		t.Fatal(err)
	}
	var body []string
	in := false
	for _, line := range strings.Split(s.Assembly, "\n") {
		trimmed := strings.TrimSpace(line)
		if i := strings.Index(trimmed, ";"); i >= 0 {
			trimmed = strings.TrimSpace(trimmed[:i])
		}
		if strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, ".") && !strings.HasPrefix(line, " ") {
			in = trimmed == "in_interval:"
			continue
		}
		if in && trimmed != "" {
			body = append(body, trimmed)
		}
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Errors) > 0 {
		t.Fatal(a.Errors)
	}
	addr := func(name string) int {
		p := a.Symbols[name]
		if p == 0 {
			p = a.Symbols["v_"+name]
		}
		if p == 0 {
			t.Fatalf("missing symbol %s\n%s", name, s.Assembly)
		}
		return p
	}
	av, bv, rv, entry := addr("a"), addr("b"), addr("result"), addr("execute")
	z := emulator.NewRemogattoZ80()
	mem := z.Memory()
	copy(mem[0x8000:], a.Binary)
	for v := 0; v < 256; v++ {
		for low := 0; low < 256; low++ {
			want := tc.want(byte(v), byte(low))
			mem[av], mem[bv], mem[rv], mem[rv+1] = byte(v), byte(low), 0xa5, 0xa5
			mem[0xeffe], mem[0xefff] = 0, 0xf1
			z.SetRegisters(emulator.Registers{F: byte(v ^ low), PC: uint16(entry), SP: 0xeffe})
			for step := 0; step < 2000 && z.GetPC() != 0xf100; step++ {
				z.Step()
			}
			regs := z.GetRegisters()
			if regs.PC != 0xf100 || regs.SP != 0xf000 || mem[rv] != want || mem[rv+1] != 0 {
				t.Fatalf("value=%d low=%d: result=%02x%02x want %d PC/SP=%04x/%04x\n%s",
					v, low, mem[rv+1], mem[rv], want, regs.PC, regs.SP, s.Assembly)
			}
		}
	}
	return inIntervalBuild{asm: s.Assembly, body: strings.Join(body, "\n"), bytes: len(a.Binary)}
}

func forEachInInterval(t *testing.T, check func(t *testing.T, tc inIntervalCase, size bool, b inIntervalBuild)) {
	for _, tc := range inIntervalCases {
		for _, language := range []string{"c", "nanz"} {
			for _, size := range []bool{false, true} {
				mode := "default"
				if size {
					mode = "size"
				}
				t.Run(tc.name+"/"+language+"/"+mode, func(t *testing.T) {
					check(t, tc, size, buildInInterval(t, tc, language, size))
				})
			}
		}
	}
}

// A byte add/sub of a constant is emitted as ADD A,n / SUB n, so the constant
// needs no register: no LD r,n of the bound's constant may remain.
func TestInIntervalByteConstantNotLoaded(t *testing.T) {
	load := regexp.MustCompile(`(?m)^LD (A|B|C|D|E|H|L|IXH|IXL|IYH|IYL), (31|100)$`)
	forEachInInterval(t, func(t *testing.T, tc inIntervalCase, size bool, b inIntervalBuild) {
		if load.MatchString(b.body) {
			t.Fatalf("constant loaded into a register:\n%s", b.body)
		}
	})
}

// The bound is used only after the first compare succeeds: it is computed in
// that successor alone (mir2.SinkIntoSingleUseSuccessor), and the compare
// reads it from A where the add left it, with value read from the register it
// was moved to (byteResultStaysInA, readsRelocatedLastUse) — no copy of the
// bound to its home and no reload of value into A.
func TestInIntervalBoundSunkAndComparedInA(t *testing.T) {
	alu := regexp.MustCompile(`^(ADD A, 31|SUB 100)$`)
	forEachInInterval(t, func(t *testing.T, tc inIntervalCase, size bool, b inIntervalBuild) {
		lines := strings.Split(b.body, "\n")
		branched := false
		for i, line := range lines {
			if strings.HasPrefix(line, "JR") || strings.HasPrefix(line, "JP") || strings.HasPrefix(line, "RET ") {
				branched = true
			}
			if !alu.MatchString(line) {
				continue
			}
			if !branched {
				t.Fatalf("bound computed before the first branch:\n%s", b.body)
			}
			if i+1 >= len(lines) || !strings.HasPrefix(lines[i+1], "CP ") {
				t.Fatalf("bound not compared in A right after it is computed:\n%s", b.body)
			}
			return
		}
		t.Fatalf("bound arithmetic not found:\n%s", b.body)
	})
}

// The 0/1 join is returned through a no-op bool->u8 ext. Every edge builds
// it in A (propagateClassHints pass 1b), so nothing copies it into A from
// another register before RET.
func TestInIntervalJoinBuiltInA(t *testing.T) {
	reload := regexp.MustCompile(`^LD A, (B|C|D|E|H|L|IXH|IXL|IYH|IYL)$`)
	forEachInInterval(t, func(t *testing.T, tc inIntervalCase, size bool, b inIntervalBuild) {
		lines := strings.Split(b.body, "\n")
		for i, line := range lines {
			if line != "RET" {
				continue
			}
			j := i - 1
			for j >= 0 && strings.HasSuffix(lines[j], ":") {
				j--
			}
			if j >= 0 && reload.MatchString(lines[j]) {
				t.Fatalf("result copied into A before RET:\n%s", b.body)
			}
		}
	})
}
