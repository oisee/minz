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
