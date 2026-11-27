package pipeline

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Native checks for the FAT12-style codegen improvements: a pointer indexed by
// `i + i/2`, a little-endian byte pair packed into a word, and a caller that
// passes a global address and a widened global byte and stores the word
// result.  Every program defines globals values/n/result and execute(); the
// Z80 run fills values from a fixed random stream, sets n, and compares
// result with want(values, n).

type globalsProgram struct {
	name    string
	c, nanz string
	maxN    int // largest n whose reads stay inside values
	want    func(values []byte, n int) uint16
}

const fat12GlobalsC = "unsigned char values[64], n;\nunsigned short result;\n"
const fat12GlobalsNanz = "global values: [u8; 64]\nglobal n: u8 = 0\nglobal result: u16 = 0\n"

func fat12Want(values []byte, n int) uint16 {
	off := n + n/2
	w := uint16(values[off]) | uint16(values[off+1])<<8
	if n&1 != 0 {
		return w >> 4
	}
	return w & 4095
}

var fat12Program = globalsProgram{
	name: "fat12",
	c: `unsigned short fat12_decode(unsigned char *fat, unsigned short cluster) {
    unsigned short offset = cluster + cluster / 2;
    unsigned short word = fat[offset] | ((unsigned short)fat[offset + 1] << 8);
    if (cluster & 1) return word >> 4;
    return word & 4095;
}
void execute(void) { result = fat12_decode(values, n); }`,
	nanz: `fun fat12_decode(fat: ^u8, cluster: u16) -> u16 {
    let offset: u16 = cluster + cluster / 2
    let word: u16 = u16(fat[offset]) | (u16(fat[offset + 1]) << 8)
    if cluster & 1 != 0 { return word >> 4 }
    return word & 4095
}
fun execute() { result = fat12_decode(&values, u16(n)) }`,
	maxN: 41,
	want: fat12Want,
}

// compileGlobalsProgram compiles p in language ("c"/"nanz") and mode
// ("default"/"grace"/"size") and runs execute() for every n in 0..maxN over
// three random fillings of values.  It returns the assembly.
func compileGlobalsProgram(t *testing.T, p globalsProgram, language, mode string) string {
	t.Helper()
	var h *hir.Module
	var err error
	if language == "c" {
		h, err = c89.CompileWithOpts(fat12GlobalsC+p.c, p.name+".c", c89.CompileOpts{NoLibc: true})
	} else {
		h, err = nanz.Parse(fat12GlobalsNanz+p.nanz, p.name+".nanz")
	}
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Errors) > 0 {
		t.Fatal(a.Errors)
	}
	addr := func(name string) int {
		v := a.Symbols[name]
		if v == 0 {
			v = a.Symbols["v_"+name]
		}
		if v == 0 {
			t.Fatal("missing symbol", name)
		}
		return v
	}
	values, n, result, entry := addr("values"), addr("n"), addr("result"), addr("execute")
	z := emulator.NewRemogattoZ80()
	mem := z.Memory()
	copy(mem[0x8000:], a.Binary)
	rng := rand.New(rand.NewSource(12))
	for fill := 0; fill < 3; fill++ {
		data := make([]byte, 64)
		for i := range data {
			data[i] = byte(rng.Intn(256))
		}
		for x := 0; x <= p.maxN; x++ {
			copy(mem[values:], data)
			mem[n], mem[result], mem[result+1] = byte(x), 0xa5, 0x5a
			mem[0xeffe], mem[0xefff] = 0, 0xf1
			z.SetRegisters(emulator.Registers{F: byte(x), PC: uint16(entry), SP: 0xeffe})
			for step := 0; step < 20000 && z.GetPC() != 0xf100; step++ {
				z.Step()
			}
			regs := z.GetRegisters()
			got := uint16(mem[result]) | uint16(mem[result+1])<<8
			if want := p.want(data, x); regs.PC != 0xf100 || regs.SP != 0xf000 || got != want {
				t.Fatalf("%s/%s/%s n=%d: got %04x want %04x PC/SP=%04x/%04x\n%s", p.name, language, mode, x, got, want, regs.PC, regs.SP, s.Assembly)
			}
		}
	}
	return s.Assembly
}

// asmFunction returns the instruction lines (comments and blank lines dropped)
// of the function labelled name.
func asmFunction(asm, name string) []string {
	var out []string
	in := false
	for _, ln := range strings.Split(asm, "\n") {
		trimmed := strings.TrimSpace(ln)
		if i := strings.Index(trimmed, ";"); i >= 0 {
			trimmed = strings.TrimSpace(trimmed[:i])
		}
		if trimmed == "" {
			continue
		}
		if strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, ".") {
			in = trimmed == name+":"
			continue
		}
		if in {
			out = append(out, strings.Join(strings.Fields(trimmed), " "))
		}
	}
	return out
}

func eachGlobalsMode(t *testing.T, p globalsProgram, check func(t *testing.T, language, mode, asm string)) {
	for _, language := range []string{"c", "nanz"} {
		for _, mode := range []string{"default", "grace", "size"} {
			t.Run(p.name+"/"+language+"/"+mode, func(t *testing.T) {
				check(t, language, mode, compileGlobalsProgram(t, p, language, mode))
			})
		}
	}
}

var wordStoreFromHL = regexp.MustCompile(`^LD \(result\), HL$`)

// A word call result already in HL is stored with LD (result),HL: the
// addr_of @result feeding the store names its symbol and holds no home, so it
// no longer pushes the result out of HL into DE.
func TestDirectStoreAddressHoldsNoHomeNative(t *testing.T) {
	eachGlobalsMode(t, fat12Program, func(t *testing.T, language, mode, asm string) {
		lines := asmFunction(asm, "execute")
		call := -1
		for i, ln := range lines {
			if strings.HasPrefix(ln, "CALL fat12_decode") {
				call = i
			}
		}
		if call < 0 || call+1 >= len(lines) || !wordStoreFromHL.MatchString(lines[call+1]) {
			t.Fatalf("call result not stored straight from HL:\n%s", strings.Join(lines, "\n"))
		}
	})
}

// rawFunction returns the trimmed lines of the function labelled name,
// comments included.
func rawFunction(asm, name string) []string {
	var out []string
	in := false
	for _, ln := range strings.Split(asm, "\n") {
		trimmed := strings.TrimSpace(ln)
		if strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, ".") && !strings.HasPrefix(trimmed, ";") {
			in = trimmed == name+":"
			continue
		}
		if in && trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// pack_bytes lo, hi into a pair: the allocator gives lo the pair's low byte
// and hi its high byte, so the two loads land in place and the pack emits at
// most one byte copy (previously LD D,(HL) / INC HL / LD E,(HL) and a
// PUSH AF byte swap).  The one copy is the HL form that appears once the
// pointer is reassociated into HL: LD C,(HL) / INC HL / LD H,(HL) / LD L,C,
// where the high byte overwrites the dying pointer in place.
func TestPackBytesLaneHomesNative(t *testing.T) {
	eachGlobalsMode(t, fat12Program, func(t *testing.T, language, mode, asm string) {
		lines := rawFunction(asm, "fat12_decode")
		for i, ln := range lines {
			if strings.HasPrefix(ln, "; pack disjoint byte lanes") {
				// A crossed pack is a swap inside one pair (LD D,E / LD E,A …).
				for _, next := range lines[i+1 : min(i+5, len(lines))] {
					for _, swap := range []string{"LD D, E", "LD E, D", "LD H, L", "LD L, H", "LD B, C", "LD C, B"} {
						if next == swap {
							t.Fatalf("pack still swaps bytes:\n%s", strings.Join(lines, "\n"))
						}
					}
				}
			}
			if strings.HasPrefix(ln, "PUSH AF") {
				t.Fatalf("pack saves AF:\n%s", strings.Join(lines, "\n"))
			}
		}
	})
}

// p[a + b] where both p + a and a + b wrap past 0xFFFF: reassociating the
// address to (p + a) + b must give the same byte.
var wrapIndexProgram = globalsProgram{
	name: "wrapindex",
	c: `unsigned char get(unsigned char *p, unsigned short a, unsigned short b) { return p[a + b]; }
void execute(void) { result = get(values, (unsigned short)(0xFFF0u + n), (unsigned short)(0x20u + n)); }`,
	nanz: `fun get(p: ^u8, a: u16, b: u16) -> u8 { return p[a + b] }
fun execute() { result = u16(get(&values, 0xFFF0 + u16(n), 0x20 + u16(n))) }`,
	maxN: 23,
	want: func(values []byte, n int) uint16 { return uint16(values[0x10+2*n]) },
}

// A pointer indexed by a single-use sum is advanced one term at a time, so
// the pointer stays in HL for two ADD HL,rr steps (fat12_decode previously
// built cluster + cluster/2 in HL with the pointer pushed aside).
func TestReassociatedPointerOffsetNative(t *testing.T) {
	eachGlobalsMode(t, fat12Program, func(t *testing.T, language, mode, asm string) {
		lines := asmFunction(asm, "fat12_decode")
		for i := 0; i+1 < len(lines); i++ {
			if strings.HasPrefix(lines[i], "ADD HL, ") && strings.HasPrefix(lines[i+1], "ADD HL, ") {
				return
			}
		}
		t.Fatalf("pointer not advanced by two consecutive ADD HL steps:\n%s", strings.Join(lines, "\n"))
	})
	eachGlobalsMode(t, wrapIndexProgram, func(t *testing.T, language, mode, asm string) {})
}

var addressArgument = regexp.MustCompile(`^LD (HL|DE|BC|IX|IY), values$`)

// A global address used only as a call argument is loaded straight into the
// parameter register after the other argument copies; it no longer holds a
// pair from its definition and gets shuffled into place.
func TestCallArgumentAddressLoadedInPlaceNative(t *testing.T) {
	for _, p := range []globalsProgram{fat12Program, wrapIndexProgram} {
		callee := "fat12_decode"
		if p.name == "wrapindex" {
			callee = "get"
		}
		eachGlobalsMode(t, p, func(t *testing.T, language, mode, asm string) {
			lines := asmFunction(asm, "execute")
			for i, ln := range lines {
				if strings.HasPrefix(ln, "CALL "+callee) {
					if i == 0 || !addressArgument.MatchString(lines[i-1]) {
						t.Fatalf("address argument not loaded at the call:\n%s", strings.Join(lines, "\n"))
					}
					for _, prev := range lines[:i-1] {
						if strings.Contains(prev, "values") {
							t.Fatalf("address materialised early:\n%s", strings.Join(lines, "\n"))
						}
					}
					return
				}
			}
			t.Fatalf("no call to %s:\n%s", callee, strings.Join(lines, "\n"))
		})
	}
}

var registerCopy = regexp.MustCompile(`^LD (A|B|C|D|E|H|L|BC|DE|HL), (A|B|C|D|E|H|L|BC|DE|HL)$`)

// The widened global byte argument is loaded into the low byte of its
// parameter pair and zero-extended there (LD E,(HL) / LD D,0): the copy
// lowering put between the extension and the call is gone, and the byte
// takes the low lane of the extension's pair.
func TestWidenedByteArgumentBuiltInPlaceNative(t *testing.T) {
	eachGlobalsMode(t, fat12Program, func(t *testing.T, language, mode, asm string) {
		lines := asmFunction(asm, "execute")
		for _, ln := range lines {
			if registerCopy.MatchString(ln) {
				t.Fatalf("argument still copied between registers (%s):\n%s", ln, strings.Join(lines, "\n"))
			}
		}
	})
}

// Byte pairs packed from loads that do not come from the pointer the result
// overwrites: their lanes are fixed by the allocator (recolorPackLanes), so
// get2 and get4 need at most one byte copy.  Without the lane preference get2
// loads its high byte into C and pays LD L,D / LD H,C.  get3 is only checked
// for correctness: under Nanz --Osize its high byte is loaded into A, which
// the lane preference does not take a value out of (A is cheaper by the
// model), so it keeps two copies there.
var packLanesProgram = globalsProgram{
	name: "packlanes",
	c: `unsigned short get2(unsigned char *p, unsigned char *q) { return *p | ((unsigned short)*q << 8); }
unsigned short get3(unsigned char *p, unsigned char i) { return p[i] | ((unsigned short)p[i + 3] << 8); }
unsigned short get4(unsigned char *p) { unsigned char a = p[5]; unsigned char b = p[9]; return (a | ((unsigned short)b << 8)) + 1; }
void execute(void) { result = get2(values + n, values + 1) + get3(values, n) + get4(values + n); }`,
	nanz: `fun get2(p: ^u8, q: ^u8) -> u16 { return u16(p[0]) | (u16(q[0]) << 8) }
fun get3(p: ^u8, i: u8) -> u16 { return u16(p[i]) | (u16(p[i + 3]) << 8) }
fun get4(p: ^u8) -> u16 {
    let a: u8 = p[5]
    let b: u8 = p[9]
    return (u16(a) | (u16(b) << 8)) + 1
}
fun execute() { result = get2(&values + u16(n), &values + 1) + get3(&values, n) + get4(&values + u16(n)) }`,
	maxN: 54,
	want: func(v []byte, n int) uint16 {
		w := func(lo, hi byte) uint16 { return uint16(lo) | uint16(hi)<<8 }
		return w(v[n], v[1]) + w(v[n], v[n+3]) + w(v[n+5], v[n+9]) + 1
	},
}

func TestPackBytesLaneHomesAllocatedNative(t *testing.T) {
	eachGlobalsMode(t, packLanesProgram, func(t *testing.T, language, mode, asm string) {
		packs := 0
		for _, fn := range []string{"get2", "get4"} {
			lines := rawFunction(asm, fn)
			for i, ln := range lines {
				if !strings.HasPrefix(ln, "; pack disjoint byte lanes") {
					continue
				}
				packs++
				copies := 0
				for _, next := range lines[i+1:] {
					if !registerCopy.MatchString(next) {
						break
					}
					copies++
				}
				if copies > 1 || i+1 < len(lines) && strings.HasPrefix(lines[i+1], "PUSH AF") {
					t.Fatalf("%s: pack copies %d bytes:\n%s", fn, copies, strings.Join(lines, "\n"))
				}
			}
		}
		if packs == 0 {
			t.Fatalf("no byte pack emitted:\n%s", asm)
		}
	})
}
